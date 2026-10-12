package importer

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EquipmentImportStats — итог импорта оборудования.
type EquipmentImportStats struct {
	Equipments  int // вставлено строк equipments
	Assignments int // вставлено строк equipments_assignments
	// Разбивка закреплений по типу (для лога).
	AssignEmployee   int
	AssignDepartment int
	AssignWarehouse  int
	Warnings         int // предупреждений резолва (нет номенклатуры/карточки/подразделения)
}

// refTable — загруженные справочники для резолва ссылок.
type refTable struct {
	// код номенклатуры → id.
	nomenByCode map[string]int32
	// нормализованное наименование подразделения → code.
	deptByName map[string]int32
	// number → есть ли такая карточка.
	cardExists map[int32]struct{}
	// ФИО (нормализованное, без званий) → number (первое вхождение).
	cardByName map[string]int32
}

// loadRefTable читает справочники nomenclatures/departments/cards в памяти.
func loadRefTable(ctx context.Context, tx pgx.Tx) (*refTable, error) {
	rt := &refTable{
		nomenByCode: map[string]int32{},
		deptByName:  map[string]int32{},
		cardExists:  map[int32]struct{}{},
		cardByName:  map[string]int32{},
	}

	// nomenclatures: code → id.
	rows, err := tx.Query(ctx, `SELECT code, id FROM nomenclatures`)
	if err != nil {
		return nil, fmt.Errorf("select nomenclatures: %w", err)
	}
	for rows.Next() {
		var (
			code string
			id   int32
		)
		if err := rows.Scan(&code, &id); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan nomenclature: %w", err)
		}
		rt.nomenByCode[code] = id
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nomenclatures: %w", err)
	}

	// departments: name (нормализованное) → code.
	drows, err := tx.Query(ctx, `SELECT code, name FROM departments`)
	if err != nil {
		return nil, fmt.Errorf("select departments: %w", err)
	}
	for drows.Next() {
		var (
			code int32
			name string
		)
		if err := drows.Scan(&code, &name); err != nil {
			drows.Close()
			return nil, fmt.Errorf("scan department: %w", err)
		}
		key := strings.ToLower(strings.TrimSpace(name))
		rt.deptByName[key] = code
		// Индексируем и нормализованную форму «тип название» (склад/ЗПУ и т.п.
		// совпадают как есть, опк/погз — приведение не требуется, т.к. в
		// справочнике уже «тип название»).
	}
	drows.Close()
	if err := drows.Err(); err != nil {
		return nil, fmt.Errorf("iterate departments: %w", err)
	}

	// cards: number + full_name → для FK и матчинга по ФИО.
	crows, err := tx.Query(ctx, `SELECT number, full_name FROM cards`)
	if err != nil {
		return nil, fmt.Errorf("select cards: %w", err)
	}
	for crows.Next() {
		var (
			number int32
			name   string
		)
		if err := crows.Scan(&number, &name); err != nil {
			crows.Close()
			return nil, fmt.Errorf("scan card: %w", err)
		}
		rt.cardExists[number] = struct{}{}
		key := stripRanks(normalizeName(name))
		if _, ok := rt.cardByName[key]; !ok {
			rt.cardByName[key] = number
		}
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards: %w", err)
	}

	return rt, nil
}

// resolveAssignment вычисляет поля equipments_assignments для одной строки.
// Возвращает target, card_number, department_code (указатели nil допустимы),
// и признак, удалось ли резолвить (false → закрепление не создаём + warning).
func (rt *refTable) resolveAssignment(eq Equipment) (targetType, *int32, *int32, bool) {
	ttype, deptKey := classifyTarget(eq.AssignTargetM)

	switch ttype {
	case targetWarehouse:
		return targetWarehouse, nil, nil, true

	case targetDepartment:
		if code, ok := rt.deptByName[deptKey]; ok {
			return targetDepartment, nil, &code, true
		}
		return targetDepartment, nil, nil, false

	case targetEmployee:
		// 1) номер карточки из колонки N — приоритетнее, если есть в cards.
		if n, ok := parseCardRef(eq.AssignCardRefN); ok {
			nn := int32(*n)
			if _, exists := rt.cardExists[nn]; exists {
				return targetEmployee, &nn, nil, true
			}
		}
		// 2) матчинг по ФИО (без званий).
		name := stripRanks(normalizeName(eq.AssignTargetM))
		if number, ok := rt.cardByName[name]; ok {
			return targetEmployee, &number, nil, true
		}
		return targetEmployee, nil, nil, false
	}
	return targetWarehouse, nil, nil, false
}

// ImportEquipment загружает оборудование и его закрепления в БД одной
// транзакцией. Импорт одноразовый: повторный запуск с теми же инвентарными
// номерами упадёт на UNIQUE (inventory_number) — это ожидаемо.
//
// Резолв ссылок:
//   - nomenclature_id — по коду F (нет в справочнике → NULL + warning);
//   - закрепление — по колонкам M/N (см. resolveAssignment);
//     нерезолвящиеся строки не получают запись в equipments_assignments.
func ImportEquipment(ctx context.Context, pool *pgxpool.Pool, eqs []Equipment) (EquipmentImportStats, []ParseWarning, error) {
	var st EquipmentImportStats
	var warnings []ParseWarning

	if len(eqs) == 0 {
		return st, warnings, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return st, warnings, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // откат после Commit безвреден

	rt, err := loadRefTable(ctx, tx)
	if err != nil {
		return st, warnings, err
	}

	// Готовим строки equipments и запоминаем, какая ссылка/закрепление у каждой.
	type eqRow struct {
		eq       Equipment
		nomenID  *int32
		target   targetType
		cardNum  *int32
		deptCode *int32
		hasAsg   bool
	}
	prepared := make([]eqRow, 0, len(eqs))

	for _, eq := range eqs {
		er := eqRow{eq: eq}
		if eq.NomenclatureCode != "" {
			if id, ok := rt.nomenByCode[eq.NomenclatureCode]; ok {
				er.nomenID = &id
			} else {
				warnings = append(warnings, ParseWarning{
					Sheet: equipmentSheet,
					Row:   0,
					Message: fmt.Sprintf("%s: код номенклатуры %q не найден в справочнике, nomenclature_id=NULL",
						eq.InventoryNumber, eq.NomenclatureCode),
				})
			}
		}
		er.target, er.cardNum, er.deptCode, er.hasAsg = rt.resolveAssignment(eq)
		if !er.hasAsg {
			warnings = append(warnings, ParseWarning{
				Sheet: equipmentSheet,
				Row:   0,
				Message: fmt.Sprintf("%s: не удалось резолвить закрепление %q, assignments не создан",
					eq.InventoryNumber, eq.AssignTargetM),
			})
		}
		prepared = append(prepared, er)
	}

	// Вставка equipments с получением id по инвентарному номеру.
	// CopyFrom не возвращает id, поэтому вставляем и тут же читаем id
	// через RETURNING в одном запросе (по одному — 700 строк, приемлемо).
	idByInv := map[string]int32{}
	for _, er := range prepared {
		eq := er.eq
		var id int32
		err := tx.QueryRow(ctx, `
			INSERT INTO equipments
				(inventory_number, serial_number, nomenclature_id, model_name,
				 manufacture_date, arrival_date, status, form_number, location, notes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id`,
			eq.InventoryNumber, nullIfEmpty(eq.SerialNumber), er.nomenID, nullIfEmpty(eq.ModelName),
			eq.ManufactureDate, eq.ArrivalDate, string(eq.Status), nullIfEmpty(eq.FormNumber),
			nullIfEmpty(eq.Location), nullIfEmpty(eq.Notes),
		).Scan(&id)
		if err != nil {
			return st, warnings, fmt.Errorf("insert equipment %s: %w", eq.InventoryNumber, err)
		}
		idByInv[eq.InventoryNumber] = id
		st.Equipments++
	}

	// Вставка equipments_assignments пачкой (CopyFrom).
	asgRows := make([][]any, 0, len(prepared))
	for _, er := range prepared {
		if !er.hasAsg {
			continue
		}
		eqID := idByInv[er.eq.InventoryNumber]
		asgRows = append(asgRows, []any{eqID, string(er.target), er.cardNum, er.deptCode})
		switch er.target {
		case targetEmployee:
			st.AssignEmployee++
		case targetDepartment:
			st.AssignDepartment++
		case targetWarehouse:
			st.AssignWarehouse++
		}
	}

	if len(asgRows) > 0 {
		if _, err := tx.CopyFrom(ctx,
			pgx.Identifier{"equipments_assignments"},
			[]string{"equipment_id", "target_type", "card_number", "department_code"},
			pgx.CopyFromRows(asgRows)); err != nil {
			return st, warnings, fmt.Errorf("copy equipments_assignments: %w", err)
		}
		st.Assignments = len(asgRows)
	}

	if err := tx.Commit(ctx); err != nil {
		return st, warnings, fmt.Errorf("commit tx: %w", err)
	}

	st.Warnings = len(warnings)
	return st, warnings, nil
}

// nullIfEmpty приводит пустую строку к nil (для NOT NULL-полей без значения).
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
