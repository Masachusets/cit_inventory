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

// AssignmentRepo — реализация AssignmentRepository поверх pgxpool.
type AssignmentRepo struct {
	pool *pgxpool.Pool
}

// NewAssignmentRepo создаёт репозиторий закреплений на переданном пуле.
func NewAssignmentRepo(pool *pgxpool.Pool) *AssignmentRepo {
	return &AssignmentRepo{pool: pool}
}

// Compile-time проверка, что AssignmentRepo реализует AssignmentRepository.
var _ AssignmentRepository = (*AssignmentRepo)(nil)

// pgCheckViolation — код ошибки PostgreSQL при нарушении CHECK-ограничения
// (chk_target): несогласованные target_type и целевые ссылки.
const pgCheckViolation = "23514"

// assignmentSelect — базовый SELECT закрепления со связанными сущностями.
// equipment — INNER JOIN (equipment_id NOT NULL), остальные — LEFT JOIN
// (NULL-ссылки и несовместимые типы субъекта дают NULL-колонки).
const assignmentSelect = `
    SELECT
        a.id, a.equipment_id, a.target_type, a.card_number, a.department_code,
        a.waybill_id, a.assigned_at, a.unassigned_at, a.operator_comment,
        e.id, e.inventory_number,
        w.id, w.name, w.number, w.issue_date,
        c.number, c.full_name,
        d.code, d.name
    FROM equipments_assignments a
    JOIN equipments e ON e.id = a.equipment_id
    LEFT JOIN waybills w ON w.id = a.waybill_id
    LEFT JOIN cards c ON c.number = a.card_number
    LEFT JOIN departments d ON d.code = a.department_code`

// assignmentColumns — список собственных колонок закрепления (для INSERT/UPDATE).
const assignmentColumns = `equipment_id, target_type, card_number, department_code,
    waybill_id, assigned_at, unassigned_at, operator_comment`

func (r *AssignmentRepo) Create(ctx context.Context, a *domain.Assignment) error {
	row := r.pool.QueryRow(ctx, `
        INSERT INTO equipments_assignments (`+assignmentColumns+`)
        VALUES ($1, $2::assignment_target, $3, $4, $5, $6, $7, $8)
        RETURNING id`,
		a.EquipmentID, string(a.TargetType), a.CardNumber, a.DepartmentCode,
		a.WaybillID, a.AssignedAt, a.UnassignedAt, a.OperatorComment,
	)
	if err := row.Scan(&a.ID); err != nil {
		return mapAssignmentPgError(err, fmt.Sprintf("insert assignment equipment_id %d", a.EquipmentID))
	}
	return nil
}

func (r *AssignmentRepo) GetByID(ctx context.Context, id int) (*domain.Assignment, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	row := r.pool.QueryRow(ctx, assignmentSelect+` WHERE a.id = $1`, id)

	a, err := scanAssignment(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: assignment id %d", domain.ErrNotFound, id)
		}
		return nil, fmt.Errorf("select assignment %d: %w", id, err)
	}
	return a, nil
}

func (r *AssignmentRepo) List(ctx context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error) {
	query := assignmentSelect
	var (
		conds []string
		args  []any
	)
	// Точные совпадения по идентификаторам и enum.
	if filter.EquipmentID != nil {
		args = append(args, *filter.EquipmentID)
		conds = append(conds, fmt.Sprintf("a.equipment_id = $%d", len(args)))
	}
	if filter.TargetType != nil {
		args = append(args, string(*filter.TargetType))
		conds = append(conds, fmt.Sprintf("a.target_type = $%d::assignment_target", len(args)))
	}
	if filter.CardNumber != nil {
		args = append(args, *filter.CardNumber)
		conds = append(conds, fmt.Sprintf("a.card_number = $%d", len(args)))
	}
	if filter.DepartmentCode != nil {
		args = append(args, *filter.DepartmentCode)
		conds = append(conds, fmt.Sprintf("a.department_code = $%d", len(args)))
	}
	if filter.WaybillID != nil {
		args = append(args, *filter.WaybillID)
		conds = append(conds, fmt.Sprintf("a.waybill_id = $%d", len(args)))
	}
	// Частичное совпадение строк (ILIKE) по закреплению и связанным сущностям.
	for _, cond := range []struct {
		value *string
		expr  string
	}{
		{filter.InventoryNumber, "e.inventory_number"},
		{filter.CardFullName, "c.full_name"},
		{filter.DepartmentName, "d.name"},
		{filter.WaybillName, "w.name"},
		{filter.WaybillNumber, "w.number"},
		{filter.OperatorComment, "a.operator_comment"},
	} {
		if cond.value != nil {
			args = append(args, *cond.value)
			conds = append(conds, fmt.Sprintf("%s ILIKE '%%' || $%d || '%%'", cond.expr, len(args)))
		}
	}
	// Диапазоны дат: по дню (TIMESTAMPTZ → date), включительно.
	for _, cond := range []struct {
		value *time.Time
		expr  string
		op    string
	}{
		{filter.WaybillIssueDateFrom, "w.issue_date", ">="},
		{filter.WaybillIssueDateTo, "w.issue_date", "<="},
		{filter.AssignedAtFrom, "a.assigned_at::date", ">="},
		{filter.AssignedAtTo, "a.assigned_at::date", "<="},
		{filter.UnassignedAtFrom, "a.unassigned_at::date", ">="},
		{filter.UnassignedAtTo, "a.unassigned_at::date", "<="},
	} {
		if cond.value != nil {
			args = append(args, *cond.value)
			conds = append(conds, fmt.Sprintf("%s %s $%d::date", cond.expr, cond.op, len(args)))
		}
	}
	// Активность: true — ещё закреплено, false — снято с закрепления.
	if filter.Active != nil {
		if *filter.Active {
			conds = append(conds, "a.unassigned_at IS NULL")
		} else {
			conds = append(conds, "a.unassigned_at IS NOT NULL")
		}
	}
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY a.id"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query assignments: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.Assignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate assignments: %w", err)
	}
	return items, nil
}

func (r *AssignmentRepo) Update(ctx context.Context, id int, a *domain.Assignment) error {
	// equipment_id намеренно не обновляется — запись истории привязана
	// к оборудованию навсегда (как inventory_number у equipments).
	tag, err := r.pool.Exec(ctx, `
        UPDATE equipments_assignments SET
            target_type      = $1::assignment_target,
            card_number      = $2,
            department_code  = $3,
            waybill_id       = $4,
            assigned_at      = $5,
            unassigned_at    = $6,
            operator_comment = $7
        WHERE id = $8`,
		string(a.TargetType), a.CardNumber, a.DepartmentCode,
		a.WaybillID, a.AssignedAt, a.UnassignedAt, a.OperatorComment, id,
	)
	if err != nil {
		return mapAssignmentPgError(err, fmt.Sprintf("update assignment id %d", id))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: assignment id %d", domain.ErrNotFound, id)
	}
	return nil
}

func (r *AssignmentRepo) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `
        DELETE FROM equipments_assignments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete assignment id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: assignment id %d", domain.ErrNotFound, id)
	}
	return nil
}

// mapAssignmentPgError маппит коды ошибок PostgreSQL на доменные ошибки:
// нарушение FK (23503) → ErrInvalidReference, нарушение chk_target (23514) →
// ErrInvalidTarget, остальное — оборачивается с контекстом вызывающего.
func mapAssignmentPgError(err error, context string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgForeignKeyViolation:
			return fmt.Errorf("%w: %s (%s)", domain.ErrInvalidReference, context, pgErr.Detail)
		case pgCheckViolation:
			return fmt.Errorf("%w: %s", domain.ErrInvalidTarget, context)
		}
	}
	return fmt.Errorf("%s: %w", context, err)
}

// scanAssignment читает одну строку выборки со JOIN-ами в доменную модель.
// NULL-колонки сканируются в указатели; вложенные сущности собираются из
// JOIN-колонок (все поля NULL → сущность не заполнена, nil).
func scanAssignment(row rowScanner) (*domain.Assignment, error) {
	var a domain.Assignment
	var targetType string
	var (
		cardNumber      *int
		departmentCode  *int
		waybillID       *int
		unassignedAt    *time.Time
		operatorComment *string

		eqID   int
		eqNum  string
		wbID   *int
		wbName *string
		wbNum  *string
		wbDate *time.Time
		cNum   *int
		cName  *string
		dCode  *int16
		dName  *string
	)
	if err := row.Scan(
		&a.ID, &a.EquipmentID, &targetType, &cardNumber, &departmentCode,
		&waybillID, &a.AssignedAt, &unassignedAt, &operatorComment,
		&eqID, &eqNum,
		&wbID, &wbName, &wbNum, &wbDate,
		&cNum, &cName,
		&dCode, &dName,
	); err != nil {
		return nil, err
	}

	a.TargetType = domain.AssignmentTarget(targetType)
	a.CardNumber = cardNumber
	a.DepartmentCode = departmentCode
	a.WaybillID = waybillID
	a.UnassignedAt = unassignedAt
	a.OperatorComment = operatorComment

	a.Equipment = &domain.Equipment{ID: eqID, InventoryNumber: eqNum}
	if wbID != nil {
		a.Waybill = &domain.Waybill{ID: *wbID}
		if wbName != nil {
			a.Waybill.Name = *wbName
		}
		a.Waybill.Number = wbNum
		a.Waybill.IssueDate = wbDate
	}
	if cNum != nil {
		a.Card = &domain.Card{Number: *cNum}
		if cName != nil {
			a.Card.FullName = *cName
		}
	}
	if dCode != nil {
		a.Department = &domain.Department{Code: *dCode}
		if dName != nil {
			a.Department.Name = *dName
		}
	}
	return &a, nil
}
