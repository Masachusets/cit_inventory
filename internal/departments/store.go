package departments

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Insert загружает подразделения в таблицу departments одной транзакцией.
// Импорт одноразовый (без синхронизации): повторный запуск с теми же
// кодами завершится ошибкой UNIQUE — это ожидаемо.
// Возвращает количество вставленных строк.
func Insert(ctx context.Context, pool *pgxpool.Pool, depts []Department) (int, error) {
	if len(depts) == 0 {
		return 0, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // откат после Commit безвреден

	rows := make([][]any, 0, len(depts))
	for _, d := range depts {
		rows = append(rows, []any{d.Code, d.Name})
	}

	tag, err := tx.CopyFrom(ctx,
		pgx.Identifier{"departments"},
		[]string{"code", "name"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return 0, fmt.Errorf("copy departments: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}

	return int(tag), nil
}
