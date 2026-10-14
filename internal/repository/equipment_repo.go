package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// list загружает оборудование с фильтром по активности (deleted_at) и
// пользовательским фильтром (ILIKE по строкам, точное совпадение по статусу),
// отсортированное по id.
func (r *EquipmentRepo) list(ctx context.Context, active bool, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	query := `SELECT ` + equipmentColumns + ` FROM equipments WHERE deleted_at IS NULL`
	if !active {
		query = `SELECT ` + equipmentColumns + ` FROM equipments WHERE deleted_at IS NOT NULL`
	}
	var (
		conds []string
		args  []any
	)
	if filter.InventoryNumber != nil {
		args = append(args, *filter.InventoryNumber)
		conds = append(conds, fmt.Sprintf("inventory_number ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.SerialNumber != nil {
		args = append(args, *filter.SerialNumber)
		conds = append(conds, fmt.Sprintf("serial_number ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.ModelName != nil {
		args = append(args, *filter.ModelName)
		conds = append(conds, fmt.Sprintf("model_name ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.Status != nil {
		args = append(args, string(*filter.Status))
		conds = append(conds, fmt.Sprintf("status = $%d::equipment_status", len(args)))
	}
	if filter.Location != nil {
		args = append(args, *filter.Location)
		conds = append(conds, fmt.Sprintf("location ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if len(conds) > 0 {
		query += " AND " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query equipments (active=%v): %w", active, err)
	}
	defer rows.Close()

	items := make([]*domain.Equipment, 0)
	for rows.Next() {
		eq, err := scanEquipment(rows)
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
