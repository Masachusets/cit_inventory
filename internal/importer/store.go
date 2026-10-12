package importer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Stats — итог синхронизации таблицы nomenclatures с Excel-файлом.
type Stats struct {
	Added      int      // добавлено новых кодов
	Updated    int      // обновлено наименований
	Deleted    int      // удалено кодов, исчезнувших из файла
	Unresolved []string // не удалены: на код ссылаются записи equipments
}

// existingRow — текущая строка таблицы nomenclatures.
type existingRow struct {
	ID   int
	Name string
}

// diff сравнивает состояние БД с записями из файла и вычисляет операции
// синхронизации: что вставить, что обновить, что удалить.
// Функция чистая — вся логика покрывается тестами без БД.
func diff(existing map[string]existingRow, records []Nomenclature) (inserts, updates, deletes []string) {
	inFile := make(map[string]struct{}, len(records))
	for _, rec := range records {
		inFile[rec.Code] = struct{}{}
		if cur, ok := existing[rec.Code]; ok {
			if cur.Name != rec.Name {
				updates = append(updates, rec.Code)
			}
			continue
		}
		inserts = append(inserts, rec.Code)
	}
	for code := range existing {
		if _, ok := inFile[code]; !ok {
			deletes = append(deletes, code)
		}
	}
	return inserts, updates, deletes
}

// Sync синхронизирует таблицу nomenclatures с записями из Excel-файла
// в рамках одной транзакции:
//   - новые коды вставляются (CopyFrom);
//   - изменённые наименования обновляются;
//   - коды, исчезнувшие из файла, удаляются, если на них не ссылается
//     оборудование (FK RESTRICT), иначе остаются с предупреждением.
func Sync(ctx context.Context, pool *pgxpool.Pool, records []Nomenclature) (Stats, error) {
	var st Stats

	tx, err := pool.Begin(ctx)
	if err != nil {
		return st, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // откат после Commit безвреден

	existing := map[string]existingRow{}
	rows, err := tx.Query(ctx, `SELECT id, code, name FROM nomenclatures`)
	if err != nil {
		return st, fmt.Errorf("select nomenclatures: %w", err)
	}
	for rows.Next() {
		var (
			id   int
			code string
			name string
		)
		if err := rows.Scan(&id, &code, &name); err != nil {
			rows.Close()
			return st, fmt.Errorf("scan nomenclature: %w", err)
		}
		existing[code] = existingRow{ID: id, Name: name}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, fmt.Errorf("iterate nomenclatures: %w", err)
	}

	byCode := make(map[string]Nomenclature, len(records))
	for _, rec := range records {
		byCode[rec.Code] = rec
	}

	insertCodes, updateCodes, deleteCodes := diff(existing, records)

	if len(insertCodes) > 0 {
		rowsInsert := make([][]any, 0, len(insertCodes))
		for _, code := range insertCodes {
			rowsInsert = append(rowsInsert, []any{code, byCode[code].Name})
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"nomenclatures"}, []string{"code", "name"}, pgx.CopyFromRows(rowsInsert)); err != nil {
			return st, fmt.Errorf("copy nomenclatures: %w", err)
		}
		st.Added = len(insertCodes)
	}

	for _, code := range updateCodes {
		if _, err := tx.Exec(ctx,
			`UPDATE nomenclatures SET name = $1 WHERE code = $2`,
			byCode[code].Name, code); err != nil {
			return st, fmt.Errorf("update nomenclature %s: %w", code, err)
		}
		st.Updated = len(updateCodes)
	}

	if len(deleteCodes) > 0 {
		// Удаляем только те коды, на которые не ссылается оборудование.
		// Остальные (FK ON DELETE RESTRICT) оставляем и сообщаем об этом.
		deleted := map[string]struct{}{}
		drows, err := tx.Query(ctx, `
			DELETE FROM nomenclatures n
			WHERE n.code = ANY($1)
			  AND NOT EXISTS (
			      SELECT 1 FROM equipments e WHERE e.nomenclature_id = n.id
			  )
			RETURNING n.code`, deleteCodes)
		if err != nil {
			return st, fmt.Errorf("delete nomenclatures: %w", err)
		}
		for drows.Next() {
			var code string
			if err := drows.Scan(&code); err != nil {
				drows.Close()
				return st, fmt.Errorf("scan deleted code: %w", err)
			}
			deleted[code] = struct{}{}
		}
		drows.Close()
		if err := drows.Err(); err != nil {
			return st, fmt.Errorf("iterate deleted codes: %w", err)
		}
		st.Deleted = len(deleted)
		for _, code := range deleteCodes {
			if _, ok := deleted[code]; !ok {
				st.Unresolved = append(st.Unresolved, code)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return st, fmt.Errorf("commit tx: %w", err)
	}

	return st, nil
}
