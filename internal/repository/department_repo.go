package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DepartmentRepo — реализация DepartmentRepository поверх pgxpool.
type DepartmentRepo struct {
	pool *pgxpool.Pool
}

// NewDepartmentRepo создаёт репозиторий подразделений на переданном пуле.
func NewDepartmentRepo(pool *pgxpool.Pool) *DepartmentRepo {
	return &DepartmentRepo{pool: pool}
}

// Compile-time проверка, что DepartmentRepo реализует DepartmentRepository.
var _ DepartmentRepository = (*DepartmentRepo)(nil)

// departmentColumns — список колонок для чтения подразделения.
const departmentColumns = `code, name, is_active, created_at`

func (r *DepartmentRepo) Create(ctx context.Context, dept *domain.Department) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO departments (code, name, is_active, created_at)
        VALUES ($1, $2, $3, $4)`,
		dept.Code, dept.Name, dept.IsActive, dept.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("%w: код %d или наименование %q", domain.ErrAlreadyExists, dept.Code, dept.Name)
		}
		return fmt.Errorf("insert department %d: %w", dept.Code, err)
	}
	return nil
}

func (r *DepartmentRepo) GetByCode(ctx context.Context, code int16) (*domain.Department, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT `+departmentColumns+` FROM departments WHERE code = $1`, code)

	dept, err := scanDepartment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: код %d", domain.ErrNotFound, code)
		}
		return nil, fmt.Errorf("select department %d: %w", code, err)
	}
	return dept, nil
}

func (r *DepartmentRepo) ListActive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	return r.list(ctx, true, filter)
}

func (r *DepartmentRepo) ListInactive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	return r.list(ctx, false, filter)
}

// list загружает подразделения с фильтром по активности и пользовательским
// фильтром, отсортированные по коду.
func (r *DepartmentRepo) list(ctx context.Context, active bool, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	query := `SELECT ` + departmentColumns + ` FROM departments WHERE is_active = $1`
	args := []any{active}
	if filter.Name != nil {
		args = append(args, *filter.Name)
		query += fmt.Sprintf(" AND name ILIKE '%%' || $%d || '%%'", len(args))
	}
	if filter.CreatedAtFrom != nil {
		args = append(args, *filter.CreatedAtFrom)
		query += fmt.Sprintf(" AND created_at >= $%d", len(args))
	}
	if filter.CreatedAtTo != nil {
		args = append(args, *filter.CreatedAtTo)
		query += fmt.Sprintf(" AND created_at <= $%d", len(args))
	}
	query += " ORDER BY code"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query departments (active=%v): %w", active, err)
	}
	defer rows.Close()

	items := make([]*domain.Department, 0)
	for rows.Next() {
		dept, err := scanDepartment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan department: %w", err)
		}
		items = append(items, dept)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate departments: %w", err)
	}
	return items, nil
}

func (r *DepartmentRepo) Exists(ctx context.Context, code int16) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM departments WHERE code = $1)`, code,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check department %d exists: %w", code, err)
	}
	return exists, nil
}

func (r *DepartmentRepo) UpdateName(ctx context.Context, code int16, name string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE departments SET name = $1 WHERE code = $2`,
		name, code,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("%w: наименование %q", domain.ErrAlreadyExists, name)
		}
		return fmt.Errorf("update department %d: %w", code, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: код %d", domain.ErrNotFound, code)
	}
	return nil
}

func (r *DepartmentRepo) Deactivate(ctx context.Context, code int16) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE departments SET is_active = false WHERE code = $1 AND is_active = true`,
		code,
	)
	if err != nil {
		return fmt.Errorf("deactivate department %d: %w", code, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: код %d", domain.ErrNotFound, code)
	}
	return nil
}

// scanDepartment читает одну строку departments в доменную модель.
func scanDepartment(row rowScanner) (*domain.Department, error) {
	var d domain.Department
	if err := row.Scan(&d.Code, &d.Name, &d.IsActive, &d.CreatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}
