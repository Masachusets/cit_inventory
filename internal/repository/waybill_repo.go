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

// WaybillRepo — реализация WaybillRepository поверх pgxpool.
type WaybillRepo struct {
	pool *pgxpool.Pool
}

// NewWaybillRepo создаёт репозиторий путевых листов на переданном пуле.
func NewWaybillRepo(pool *pgxpool.Pool) *WaybillRepo {
	return &WaybillRepo{pool: pool}
}

// Compile-time проверка, что WaybillRepo реализует WaybillRepository.
var _ WaybillRepository = (*WaybillRepo)(nil)

// waybillColumns — список колонок для чтения путевого листа.
const waybillColumns = `id, name, number, issue_date`

func (r *WaybillRepo) Create(ctx context.Context, wb *domain.Waybill) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO waybills (name, number, issue_date)
        VALUES ($1, $2, $3)`,
		wb.Name, wb.Number, wb.IssueDate,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("%w: номер %v", domain.ErrAlreadyExists, derefString(wb.Number))
		}
		return fmt.Errorf("insert waybill: %w", err)
	}
	return nil
}

func (r *WaybillRepo) GetByID(ctx context.Context, id int) (*domain.Waybill, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT `+waybillColumns+` FROM waybills WHERE id = $1`, id)

	wb, err := scanWaybill(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("select waybill %d: %w", id, err)
	}
	return wb, nil
}

// waybillsListQuery собирает SELECT списка путевых листов (чистая функция —
// тестируется без БД). Диапазон issue_date — по дню, включительно.
func waybillsListQuery(filter domain.WaybillFilter) (string, []any) {
	var f filterBuilder
	f.ilike("name", filter.Name)
	f.ilike("number", filter.Number)
	f.dateRange("issue_date", filter.IssueDateFrom, filter.IssueDateTo)
	return f.where("SELECT "+waybillColumns+" FROM waybills") + " ORDER BY id", f.args
}

func (r *WaybillRepo) List(ctx context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error) {
	query, args := waybillsListQuery(filter)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query waybills: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Waybill, 0)
	for rows.Next() {
		wb, err := scanWaybill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan waybill: %w", err)
		}
		items = append(items, wb)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate waybills: %w", err)
	}
	return items, nil
}

// ExistsByNumber сообщает, есть ли путевой лист с таким номером.
func (r *WaybillRepo) ExistsByNumber(ctx context.Context, number string) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM waybills WHERE number = $1)`, number,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check waybill %q exists: %w", number, err)
	}
	return exists, nil
}

// Update обновляет путевой лист по идентификатору.
// При нарушении UNIQUE на номере — domain.ErrAlreadyExists.
func (r *WaybillRepo) Update(ctx context.Context, id int, wb *domain.Waybill) error {
	_, err := r.pool.Exec(ctx, `
        UPDATE waybills
        SET name = $1, number = $2, issue_date = $3
        WHERE id = $4`,
		wb.Name, wb.Number, wb.IssueDate, id,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("%w: номер %v", domain.ErrAlreadyExists, derefString(wb.Number))
		}
		return fmt.Errorf("update waybill %d: %w", id, err)
	}
	return nil
}

// scanWaybill читает одну строку waybills в доменную модель.
func scanWaybill(row rowScanner) (*domain.Waybill, error) {
	var wb domain.Waybill
	var number *string
	var issueDate *time.Time
	if err := row.Scan(&wb.ID, &wb.Name, &number, &issueDate); err != nil {
		return nil, err
	}
	wb.Number = number
	wb.IssueDate = issueDate
	return &wb, nil
}

// derefString возвращает значение указателя либо "NULL" для логов.
func derefString(s *string) string {
	if s == nil {
		return "NULL"
	}
	return *s
}
