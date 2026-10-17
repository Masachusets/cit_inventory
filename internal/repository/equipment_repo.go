package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EquipmentRepo — реализация EquipmentRepository поверх pgxpool.
type EquipmentRepo struct {
	pool *pgxpool.Pool
}

// NewEquipmentRepo создаёт репозиторий оборудования на переданном пуле.
func NewEquipmentRepo(pool *pgxpool.Pool) *EquipmentRepo {
	return &EquipmentRepo{pool: pool}
}

// Compile-time проверка, что EquipmentRepo реализует EquipmentRepository.
var _ EquipmentRepository = (*EquipmentRepo)(nil)

// equipmentColumns — список колонок для чтения оборудования (единый для всех SELECT).
const equipmentColumns = `id, inventory_number, serial_number, nomenclature_id, model_name,
    manufacture_date, arrival_date, status, form_number, location, notes,
    created_at, updated_at, deleted_at`

// listEquipmentColumns — SELECT для list: оборудование + справочные данные
// (номенклатура, текущее закрепление: карточка/подразделение).
// Каждой записи соответствует ровно одна строка благодаря LATERAL-подзапросу
// с LIMIT 1 (выбирается последнее закрепление по assigned_at, с учётом
// связанных rows с тем же временем, идентификатором).
const listEquipmentColumns = `
    e.id, e.inventory_number, e.serial_number, e.nomenclature_id, e.model_name,
    e.manufacture_date, e.arrival_date, e.status, e.form_number, e.location, e.notes,
    e.created_at, e.updated_at, e.deleted_at,
    n.code AS nomenclature_code, n.name AS nomenclature_name,
    a.card_number AS assignment_card_number, c.full_name AS assignment_card_full_name,
    d.code AS assignment_department_code, d.name AS assignment_department_name`

func (r *EquipmentRepo) Create(ctx context.Context, eq *domain.Equipment) error {
	row := r.pool.QueryRow(ctx, `
        INSERT INTO equipments (
            inventory_number, serial_number, nomenclature_id, model_name,
            manufacture_date, arrival_date, status, form_number, location, notes,
            created_at, updated_at, deleted_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7::equipment_status, $8, $9, $10, $11, $11, NULL)
        RETURNING id, created_at, updated_at`,
		eq.InventoryNumber, eq.SerialNumber, eq.NomenclatureID, eq.ModelName,
		eq.ManufactureDate, eq.ArrivalDate, string(eq.Status),
		eq.FormNumber, eq.Location, eq.Notes, eq.CreatedAt,
	)

	if err := row.Scan(&eq.ID, &eq.CreatedAt, &eq.UpdatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case pgUniqueViolation:
				return fmt.Errorf("%w: инвентарный номер %s", domain.ErrAlreadyExists, eq.InventoryNumber)
			case pgForeignKeyViolation:
				return fmt.Errorf("%w: nomenclature_id %v", domain.ErrInvalidNomenclature, eq.NomenclatureID)
			}
		}
		return fmt.Errorf("insert equipment %s: %w", eq.InventoryNumber, err)
	}
	return nil
}

func (r *EquipmentRepo) GetByID(ctx context.Context, id int) (*domain.Equipment, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	row := r.pool.QueryRow(ctx, `
        SELECT `+equipmentColumns+` FROM equipments WHERE id = $1`, id)

	eq, err := scanEquipment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("select equipment id %d: %w", id, err)
	}
	return eq, nil
}

func (r *EquipmentRepo) GetByInventoryNumber(ctx context.Context, number string) (*domain.Equipment, error) {
	if err := domain.ValidateInventoryNumber(number); err != nil {
		return nil, err
	}
	row := r.pool.QueryRow(ctx, `
        SELECT `+equipmentColumns+` FROM equipments WHERE inventory_number = $1`, number)

	eq, err := scanEquipment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: инвентарный номер %s", domain.ErrNotFound, number)
		}
		return nil, fmt.Errorf("select equipment %s: %w", number, err)
	}
	return eq, nil
}

func (r *EquipmentRepo) ListActive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	return r.list(ctx, true, filter)
}

func (r *EquipmentRepo) ListInactive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	return r.list(ctx, false, filter)
}

// scanEquipmentList читает одну строку списка оборудования (с JOIN-ав) в
// доменную модель. NULL-колонки сканируются в указатели; сводные поля
// (номенклатура, текущее закрепление) заполняются из LEFT JOIN-ов.
func scanEquipmentList(row rowScanner) (*domain.Equipment, error) {
	var e domain.Equipment
	var (
		serialNumber             *string
		nomenclatureID           *int
		modelName                *string
		manufactureDate          *time.Time
		arrivalDate              *time.Time
		formNumber               *string
		location                 *string
		notes                    *string
		deletedAt                *time.Time
		status                   string
		nomenclatureCode         *string
		nomenclatureName         *string
		assignmentCardNumber     *int
		assignmentCardFullName   *string
		assignmentDepartmentCode *int16
		assignmentDepartmentName *string
	)
	if err := row.Scan(
		&e.ID, &e.InventoryNumber, &serialNumber, &nomenclatureID, &modelName,
		&manufactureDate, &arrivalDate, &status, &formNumber, &location, &notes,
		&e.CreatedAt, &e.UpdatedAt, &deletedAt,
		&nomenclatureCode, &nomenclatureName,
		&assignmentCardNumber, &assignmentCardFullName,
		&assignmentDepartmentCode, &assignmentDepartmentName,
	); err != nil {
		return nil, err
	}
	e.SerialNumber = serialNumber
	e.NomenclatureID = nomenclatureID
	e.ModelName = modelName
	e.ManufactureDate = manufactureDate
	e.ArrivalDate = arrivalDate
	e.Status = domain.EquipmentStatus(status)
	e.FormNumber = formNumber
	e.Location = location
	e.Notes = notes
	e.DeletedAt = deletedAt

	e.NomenclatureCode = nomenclatureCode
	e.NomenclatureName = nomenclatureName
	e.CardNumber = assignmentCardNumber
	e.CardFullName = assignmentCardFullName
	e.DepartmentCode = assignmentDepartmentCode
	e.DepartmentName = assignmentDepartmentName
	return &e, nil
}

// equipmentsListQuery собирает SELECT списка оборудования (чистая функция —
// тестируется без БД). Все колонки в WHERE-условиях префиксируются таблицами
// (e./n./a./c./d.): после LEFT JOIN-ов непрефиксированные имена неоднозначны
// (например, deleted_at есть и у cards, и у departments), а алиасы SELECT
// (nomenclature_code, assignment_*) в WHERE недоступны в PostgreSQL.
func equipmentsListQuery(active bool, filter domain.EquipmentFilter) (string, []any) {
	var f filterBuilder
	if active {
		f.cond("e.deleted_at IS NULL")
	} else {
		f.cond("e.deleted_at IS NOT NULL")
	}
	f.ilike("e.inventory_number", filter.InventoryNumber)
	f.ilike("e.serial_number", filter.SerialNumber)
	f.ilike("e.model_name", filter.ModelName)
	if filter.Status != nil {
		f.add("e.status = $%d::equipment_status", string(*filter.Status))
	}
	f.ilike("e.location", filter.Location)
	if filter.NomenclatureCode != nil {
		f.add("n.code = $%d", *filter.NomenclatureCode)
	}
	f.ilike("n.name", filter.NomenclatureName)
	if filter.CardNumber != nil {
		f.add("a.card_number = $%d", *filter.CardNumber)
	}
	f.ilike("c.full_name", filter.CardFullName)
	if filter.DepartmentCode != nil {
		f.add("d.code = $%d", *filter.DepartmentCode)
	}
	f.ilike("d.name", filter.DepartmentName)

	query := `
        SELECT ` + listEquipmentColumns + `
        FROM equipments e
        LEFT JOIN nomenclatures n ON n.id = e.nomenclature_id
        LEFT JOIN LATERAL (
            SELECT a.card_number, a.department_code
            FROM equipments_assignments a
            WHERE a.equipment_id = e.id
            ORDER BY a.assigned_at DESC, a.id DESC
            LIMIT 1
        ) a ON TRUE
        LEFT JOIN cards c ON c.number = a.card_number
        LEFT JOIN departments d ON d.code = a.department_code`
	return f.where(query) + " ORDER BY e.id", f.args
}

// list загружает оборудование с фильтром по активности и пользовательским
// фильтром (ILIKE по строкам, точное совпадение по кодам/номерам/статусу),
// отсортированное по id. Для каждой записи выбирается текущее (самое свежее)
// закрепление: номенклатура, карточка или подразделение.
func (r *EquipmentRepo) list(ctx context.Context, active bool, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	query, args := equipmentsListQuery(active, filter)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query equipments (active=%v): %w", active, err)
	}
	defer rows.Close()

	items := make([]*domain.Equipment, 0)
	for rows.Next() {
		eq, err := scanEquipmentList(rows)
		if err != nil {
			return nil, fmt.Errorf("scan equipment: %w", err)
		}
		items = append(items, eq)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate equipments: %w", err)
	}
	return items, nil
}

// ExistsInventoryNumber сообщает, есть ли оборудование с таким номером
// (в любом статусе — UNIQUE действует и для удалённых записей).
func (r *EquipmentRepo) ExistsInventoryNumber(ctx context.Context, number string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM equipments WHERE inventory_number = $1)`, number,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check equipment %s exists: %w", number, err)
	}
	return exists, nil
}

func (r *EquipmentRepo) Update(ctx context.Context, id int, eq *domain.Equipment) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE equipments SET
            serial_number    = $1,
            nomenclature_id  = $2,
            model_name       = $3,
            manufacture_date = $4,
            arrival_date     = $5,
            status           = $6::equipment_status,
            form_number      = $7,
            location         = $8,
            notes            = $9
        WHERE id = $10`,
		eq.SerialNumber, eq.NomenclatureID, eq.ModelName,
		eq.ManufactureDate, eq.ArrivalDate, string(eq.Status),
		eq.FormNumber, eq.Location, eq.Notes, id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: nomenclature_id %v", domain.ErrInvalidNomenclature, eq.NomenclatureID)
		}
		return fmt.Errorf("update equipment id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
	}
	return nil
}

func (r *EquipmentRepo) SoftDelete(ctx context.Context, id int) error {
	// COALESCE сохраняет исходную метку удаления при повторном вызове,
	// updated_at обновляется триггером trg_eq_updated_at.
	tag, err := r.pool.Exec(ctx, `
        UPDATE equipments SET deleted_at = COALESCE(deleted_at, NOW()) WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("soft delete equipment id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
	}
	return nil
}

// scanEquipment читает одну строку equipments в доменную модель.
// NULL-колонки сканируются в указатели (nil — значения нет).
func scanEquipment(row rowScanner) (*domain.Equipment, error) {
	var e domain.Equipment
	var (
		serialNumber    *string
		nomenclatureID  *int
		modelName       *string
		manufactureDate *time.Time
		arrivalDate     *time.Time
		formNumber      *string
		location        *string
		notes           *string
		deletedAt       *time.Time
		status          string
	)
	if err := row.Scan(
		&e.ID, &e.InventoryNumber, &serialNumber, &nomenclatureID, &modelName,
		&manufactureDate, &arrivalDate, &status, &formNumber, &location, &notes,
		&e.CreatedAt, &e.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	e.SerialNumber = serialNumber
	e.NomenclatureID = nomenclatureID
	e.ModelName = modelName
	e.ManufactureDate = manufactureDate
	e.ArrivalDate = arrivalDate
	e.Status = domain.EquipmentStatus(status)
	e.FormNumber = formNumber
	e.Location = location
	e.Notes = notes
	e.DeletedAt = deletedAt
	return &e, nil
}
