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

// pgUniqueViolation — код ошибки PostgreSQL при нарушении уникальности (PK).
const pgUniqueViolation = "23505"

// pgForeignKeyViolation — код ошибки PostgreSQL при нарушении внешнего ключа
// (ON DELETE RESTRICT): на карточку ссылается оборудование.
const pgForeignKeyViolation = "23503"

// CardRepo — реализация CardRepository поверх pgxpool.
type CardRepo struct {
	pool *pgxpool.Pool
}

// NewCardRepo создаёт репозиторий карточек на переданном пуле.
func NewCardRepo(pool *pgxpool.Pool) *CardRepo {
	return &CardRepo{pool: pool}
}

// Compile-time проверка, что CardRepo реализует CardRepository.
var _ CardRepository = (*CardRepo)(nil)

// cardColumns — список колонок для чтения карточки (единый для всех SELECT).
const cardColumns = `number, full_name, is_active, created_at, updated_at, deleted_at`

func (r *CardRepo) Create(ctx context.Context, card *domain.Card) error {
	_, err := r.pool.Exec(ctx, `
        INSERT INTO cards (number, full_name, is_active, created_at, updated_at, deleted_at)
        VALUES ($1, $2, $3, $4, $5, $6)`,
		card.Number, card.FullName, card.IsActive, card.CreatedAt, card.UpdatedAt, card.DeletedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return fmt.Errorf("%w: номер %d", domain.ErrAlreadyExists, card.Number)
		}
		return fmt.Errorf("insert card %d: %w", card.Number, err)
	}
	return nil
}

func (r *CardRepo) GetByNumber(ctx context.Context, number int) (*domain.Card, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT `+cardColumns+` FROM cards WHERE number = $1`, number)

	card, err := scanCard(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
		}
		return nil, fmt.Errorf("select card %d: %w", number, err)
	}
	return card, nil
}

func (r *CardRepo) ListActive(ctx context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	return r.list(ctx, true, filter)
}

func (r *CardRepo) ListInactive(ctx context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	return r.list(ctx, false, filter)
}

// list загружает карточки с фильтром по активности и пользовательским
// фильтром (подстрока ФИО, диапазоны дат), отсортированные по номеру.
// Границы дат сравниваются по дню: колонка TIMESTAMPTZ приводится к date,
// поэтому весь день «до» включён в выборку.
func (r *CardRepo) list(ctx context.Context, active bool, filter domain.CardFilter) ([]*domain.Card, error) {
	query := `SELECT ` + cardColumns + ` FROM cards WHERE is_active = $1`
	args := []any{active}
	if filter.FullName != nil {
		args = append(args, *filter.FullName)
		query += fmt.Sprintf(` AND full_name ILIKE '%%' || $%d || '%%'`, len(args))
	}
	if filter.CreatedAtFrom != nil {
		args = append(args, *filter.CreatedAtFrom)
		query += fmt.Sprintf(" AND created_at::date >= $%d::date", len(args))
	}
	if filter.CreatedAtTo != nil {
		args = append(args, *filter.CreatedAtTo)
		query += fmt.Sprintf(" AND created_at::date <= $%d::date", len(args))
	}
	if filter.UpdatedAtFrom != nil {
		args = append(args, *filter.UpdatedAtFrom)
		query += fmt.Sprintf(" AND updated_at::date >= $%d::date", len(args))
	}
	if filter.UpdatedAtTo != nil {
		args = append(args, *filter.UpdatedAtTo)
		query += fmt.Sprintf(" AND updated_at::date <= $%d::date", len(args))
	}
	if filter.DeletedAtFrom != nil {
		args = append(args, *filter.DeletedAtFrom)
		query += fmt.Sprintf(" AND deleted_at::date >= $%d::date", len(args))
	}
	if filter.DeletedAtTo != nil {
		args = append(args, *filter.DeletedAtTo)
		query += fmt.Sprintf(" AND deleted_at::date <= $%d::date", len(args))
	}
	query += " ORDER BY number"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query cards (active=%v): %w", active, err)
	}
	defer rows.Close()

	cards := make([]*domain.Card, 0)
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards: %w", err)
	}
	return cards, nil
}

func (r *CardRepo) Exists(ctx context.Context, number int) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
        SELECT EXISTS(SELECT 1 FROM cards WHERE number = $1)`, number,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check card %d exists: %w", number, err)
	}
	return exists, nil
}

func (r *CardRepo) UpdateFullName(ctx context.Context, number int, fullName string) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE cards SET full_name = $1 WHERE number = $2`,
		fullName, number,
	)
	if err != nil {
		return fmt.Errorf("update card %d: %w", number, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
	}
	return nil
}

func (r *CardRepo) Deactivate(ctx context.Context, number int) error {
	tag, err := r.pool.Exec(ctx, `
        UPDATE cards
        SET is_active = false,
            deleted_at = NOW(),
            updated_at = NOW()
        WHERE number = $1 AND is_active = true`,
		number,
	)
	if err != nil {
		return fmt.Errorf("deactivate card %d: %w", number, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
	}
	return nil
}

func (r *CardRepo) Delete(ctx context.Context, number int) error {
	tag, err := r.pool.Exec(ctx, `
        DELETE FROM cards WHERE number = $1`, number)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
			return fmt.Errorf("%w: номер %d", domain.ErrInUse, number)
		}
		return fmt.Errorf("delete card %d: %w", number, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: номер %d", domain.ErrNotFound, number)
	}
	return nil
}

// cardScanner — то, что умеют и *pgx.Row, и pgx.Rows.
type cardScanner interface {
	Scan(dest ...any) error
}

// scanCard читает одну строку cards в доменную модель.
func scanCard(row cardScanner) (*domain.Card, error) {
	var c domain.Card
	var deletedAt *time.Time
	if err := row.Scan(
		&c.Number, &c.FullName, &c.IsActive,
		&c.CreatedAt, &c.UpdatedAt, &deletedAt,
	); err != nil {
		return nil, err
	}
	c.DeletedAt = deletedAt
	return &c, nil
}

// NomenclatureRepo — реализация NomenclatureRepository поверх pgxpool.
type NomenclatureRepo struct {
	pool *pgxpool.Pool
}

// NewNomenclatureRepo создаёт репозиторий номенклатуры на переданном пуле.
func NewNomenclatureRepo(pool *pgxpool.Pool) *NomenclatureRepo {
	return &NomenclatureRepo{pool: pool}
}

// Compile-time проверка, что NomenclatureRepo реализует NomenclatureRepository.
var _ NomenclatureRepository = (*NomenclatureRepo)(nil)

// nomenclatureColumns — список колонок для чтения номенклатуры.
const nomenclatureColumns = `id, code, name`

func (r *NomenclatureRepo) GetByID(ctx context.Context, id int) (*domain.Nomenclature, error) {
	row := r.pool.QueryRow(ctx, `
        SELECT `+nomenclatureColumns+` FROM nomenclatures WHERE id = $1`, id)

	var n domain.Nomenclature
	if err := row.Scan(&n.ID, &n.Code, &n.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("select nomenclature %d: %w", id, err)
	}
	return &n, nil
}

func (r *NomenclatureRepo) List(ctx context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error) {
	query := `SELECT ` + nomenclatureColumns + ` FROM nomenclatures`
	var (
		conds []string
		args  []any
	)
	if filter.Code != nil {
		args = append(args, *filter.Code)
		conds = append(conds, fmt.Sprintf("code ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.Name != nil {
		args = append(args, *filter.Name)
		conds = append(conds, fmt.Sprintf("name ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query nomenclatures: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Nomenclature, 0)
	for rows.Next() {
		var n domain.Nomenclature
		if err := rows.Scan(&n.ID, &n.Code, &n.Name); err != nil {
			return nil, fmt.Errorf("scan nomenclature: %w", err)
		}
		items = append(items, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nomenclatures: %w", err)
	}
	return items, nil
}

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
func scanDepartment(row cardScanner) (*domain.Department, error) {
	var d domain.Department
	if err := row.Scan(&d.Code, &d.Name, &d.IsActive, &d.CreatedAt); err != nil {
		return nil, err
	}
	return &d, nil
}

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

func (r *WaybillRepo) List(ctx context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error) {
	query := `SELECT ` + waybillColumns + ` FROM waybills`
	var (
		conds []string
		args  []any
	)
	if filter.Name != nil {
		args = append(args, *filter.Name)
		conds = append(conds, fmt.Sprintf("name ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.Number != nil {
		args = append(args, *filter.Number)
		conds = append(conds, fmt.Sprintf("number ILIKE '%%' || $%d || '%%'", len(args)))
	}
	if filter.IssueDateFrom != nil {
		args = append(args, *filter.IssueDateFrom)
		conds = append(conds, fmt.Sprintf("issue_date >= $%d", len(args)))
	}
	if filter.IssueDateTo != nil {
		args = append(args, *filter.IssueDateTo)
		conds = append(conds, fmt.Sprintf("issue_date <= $%d", len(args)))
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY id"

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

// scanWaybill читает одну строку waybills в доменную модель.
func scanWaybill(row cardScanner) (*domain.Waybill, error) {
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
