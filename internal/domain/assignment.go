// Assignment — запись закрепления оборудования (таблица equipments_assignments).
// Хранит, где и с какого момента находится оборудование: сотрудник (card_number),
// подразделение (department_code) или склад (оба поля NULL). Nullable-колонки
// хранятся указателями: nil — значение не задано (NULL). Связанные сущности
// (Equipment/Waybill/Card/Department) заполняются репозиторием при чтении из БД
// и участвуют в JSON-ответе API.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Сентинельные ошибки домена закреплений. ErrNotFound, ErrInvalidID и
// ErrInvalidField общие с остальными сущностями (пакет domain единый) —
// здесь объявлены только специфичные для equipments_assignments.
var (
	// ErrInvalidTarget — target_type вне enum assignment_target либо нарушение
	// CHECK chk_target (employee требует card_number, department — department_code,
	// warehouse — оба поля NULL).
	ErrInvalidTarget = errors.New("invalid assignment target")
	// ErrInvalidReference — некорректная ссылка на связанную сущность
	// (нарушение FK equipment_id/waybill_id/card_number/department_code).
	ErrInvalidReference = errors.New("invalid assignment reference")
)

// AssignmentTarget — значение enum assignment_target.
type AssignmentTarget string

// Значения enum assignment_target (по порядку схемы).
const (
	TargetEmployee   AssignmentTarget = "employee"   // закреплён за сотрудником
	TargetDepartment AssignmentTarget = "department" // закреплён за подразделением
	TargetWarehouse  AssignmentTarget = "warehouse"  // на складе
)

// ParseAssignmentTarget проверяет строку на принадлежность enum assignment_target.
// Сравнение точное (после обрезки краевых пробелов); при неудаче —
// ErrInvalidTarget с полученным значением.
func ParseAssignmentTarget(s string) (AssignmentTarget, error) {
	t := AssignmentTarget(strings.TrimSpace(s))
	switch t {
	case TargetEmployee, TargetDepartment, TargetWarehouse:
		return t, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidTarget, s)
}

// Assignment — запись закрепления оборудования (история перемещений).
// Поля ID и AssignedAt задаются системой (identity / DEFAULT NOW()), остальные — API.
type Assignment struct {
	ID              int              // идентификатор (PK, identity ALWAYS)
	EquipmentID     int              // закреплённое оборудование (FK, NOT NULL)
	TargetType      AssignmentTarget // тип субъекта закрепления (enum, NOT NULL)
	CardNumber      *int             // карточка ответственного (FK, только employee)
	DepartmentCode  *int             // подразделение (FK, только department)
	WaybillID       *int             // путевой лист (FK, опционально для любого типа)
	AssignedAt      time.Time        // момент закрепления (TIMESTAMPTZ, NOT NULL)
	UnassignedAt    *time.Time       // момент снятия, nil — ещё закреплено
	OperatorComment *string          // комментарий оператора (TEXT, может быть NULL)

	// Связанные сущности — заполняются репозиторием при чтении из БД
	// (nil, если ссылка не задана); в INSERT/UPDATE не участвуют.
	Equipment  *Equipment  // equipments (INNER JOIN — всегда заполнено при чтении)
	Waybill    *Waybill    // waybills (LEFT JOIN, NULL при waybill_id IS NULL)
	Card       *Card       // cards (LEFT JOIN, только target_type=employee)
	Department *Department // departments (LEFT JOIN, только target_type=department)
}

// AssignmentInput — входные данные для создания закрепления. operator_comment
// пустой → nil (NULL); assigned_at nil → текущее время (DEFAULT NOW() в схеме).
type AssignmentInput struct {
	EquipmentID     int
	TargetType      string
	CardNumber      *int
	DepartmentCode  *int
	WaybillID       *int
	AssignedAt      *time.Time
	UnassignedAt    *time.Time
	OperatorComment string
}

// NewAssignment создаёт закрепление с валидацией входных данных по правилам
// схемы: CHECK chk_target, NOT NULL assigned_at. Ссылки <= 0 — ошибка
// (в отличие от PATCH, где <= 0 очищает колонку).
func NewAssignment(in AssignmentInput) (*Assignment, error) {
	if in.EquipmentID <= 0 {
		return nil, fmt.Errorf("%w: equipment_id должен быть положительным, получено %d",
			ErrInvalidID, in.EquipmentID)
	}
	targetType, err := ParseAssignmentTarget(in.TargetType)
	if err != nil {
		return nil, err
	}

	cardNumber, err := inputRef(in.CardNumber, "card_number")
	if err != nil {
		return nil, err
	}
	departmentCode, err := inputRef(in.DepartmentCode, "department_code")
	if err != nil {
		return nil, err
	}
	waybillID, err := inputRef(in.WaybillID, "waybill_id")
	if err != nil {
		return nil, err
	}

	assignedAt := time.Now()
	if in.AssignedAt != nil {
		if in.AssignedAt.IsZero() {
			return nil, fmt.Errorf("%w: assigned_at не может быть нулевым", ErrInvalidField)
		}
		assignedAt = *in.AssignedAt
	}

	var unassignedAt *time.Time
	if in.UnassignedAt != nil && !in.UnassignedAt.IsZero() {
		t := *in.UnassignedAt
		unassignedAt = &t
	}
	if unassignedAt != nil && unassignedAt.Before(assignedAt) {
		return nil, fmt.Errorf("%w: unassigned_at раньше assigned_at", ErrInvalidField)
	}

	a := &Assignment{
		EquipmentID:     in.EquipmentID,
		TargetType:      targetType,
		CardNumber:      cardNumber,
		DepartmentCode:  departmentCode,
		WaybillID:       waybillID,
		AssignedAt:      assignedAt,
		UnassignedAt:    unassignedAt,
		OperatorComment: optText(in.OperatorComment),
	}
	if err := a.validateTarget(); err != nil {
		return nil, err
	}
	return a, nil
}

// AssignmentUpdate — частичное обновление закрепления. Семантика полей:
//   - nil — не изменять значение;
//   - TargetType: строка парсится в enum;
//   - CardNumber/DepartmentCode/WaybillID: значение <= 0 очищает ссылку (NULL),
//     > 0 устанавливает новую;
//   - AssignedAt: нулевое время — ошибка (колонка NOT NULL), иначе устанавливает;
//   - UnassignedAt: нулевое время очищает колонку (NULL — «снова закреплено»);
//   - OperatorComment: пустая строка очищает колонку (NULL).
type AssignmentUpdate struct {
	TargetType      *string
	CardNumber      *int
	DepartmentCode  *int
	WaybillID       *int
	AssignedAt      *time.Time
	UnassignedAt    *time.Time
	OperatorComment *string
}

// ApplyUpdate применяет частичное обновление к закреплению: парсит и
// валидирует переданные значения, после чего повторно проверяет CHECK chk_target
// и хронологию assigned_at/unassigned_at. Непереданные (nil) поля не изменяются.
// Обновление атомарно: при некорректном значении возвращается ошибка,
// а модель остаётся неизменной (изменения применяются к копии).
func (a *Assignment) ApplyUpdate(up AssignmentUpdate) error {
	cp := *a
	if up.TargetType != nil {
		t, err := ParseAssignmentTarget(*up.TargetType)
		if err != nil {
			return err
		}
		cp.TargetType = t
	}
	if up.CardNumber != nil {
		cp.CardNumber = clearableRef(*up.CardNumber)
	}
	if up.DepartmentCode != nil {
		cp.DepartmentCode = clearableRef(*up.DepartmentCode)
	}
	if up.WaybillID != nil {
		cp.WaybillID = clearableRef(*up.WaybillID)
	}
	if up.AssignedAt != nil {
		if up.AssignedAt.IsZero() {
			return fmt.Errorf("%w: assigned_at не может быть пустым", ErrInvalidField)
		}
		cp.AssignedAt = *up.AssignedAt
	}
	if up.UnassignedAt != nil {
		if up.UnassignedAt.IsZero() {
			cp.UnassignedAt = nil
		} else {
			t := *up.UnassignedAt
			cp.UnassignedAt = &t
		}
	}
	if up.OperatorComment != nil {
		cp.OperatorComment = optText(*up.OperatorComment)
	}

	if cp.UnassignedAt != nil && cp.UnassignedAt.Before(cp.AssignedAt) {
		return fmt.Errorf("%w: unassigned_at раньше assigned_at", ErrInvalidField)
	}
	if err := cp.validateTarget(); err != nil {
		return err
	}
	*a = cp
	return nil
}

// validateTarget проверяет соответствие CHECK chk_target: связанные поля
// обязаны согласовываться с типом субъекта закрепления.
func (a *Assignment) validateTarget() error {
	switch a.TargetType {
	case TargetEmployee:
		if a.CardNumber == nil || a.DepartmentCode != nil {
			return fmt.Errorf("%w: employee требует card_number и запрещает department_code", ErrInvalidTarget)
		}
	case TargetDepartment:
		if a.DepartmentCode == nil || a.CardNumber != nil {
			return fmt.Errorf("%w: department требует department_code и запрещает card_number", ErrInvalidTarget)
		}
	case TargetWarehouse:
		if a.CardNumber != nil || a.DepartmentCode != nil {
			return fmt.Errorf("%w: warehouse не допускает card_number и department_code", ErrInvalidTarget)
		}
	default:
		return fmt.Errorf("%w: %q", ErrInvalidTarget, a.TargetType)
	}
	return nil
}

// inputRef нормализует ссылку при создании: nil → nil (NULL), <= 0 → ошибка
// (в отличие от PATCH, где неположительное значение очищает колонку).
func inputRef(v *int, name string) (*int, error) {
	if v == nil {
		return nil, nil
	}
	if *v <= 0 {
		return nil, fmt.Errorf("%w: %s должен быть положительным, получено %d",
			ErrInvalidField, name, *v)
	}
	cp := *v
	return &cp, nil
}

// clearableRef нормализует ссылку при PATCH: значение <= 0 очищает колонку
// (nil → NULL), иначе устанавливает положительный идентификатор.
func clearableRef(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// AssignmentFilter — параметры фильтрации списка закреплений. nil-поле означает
// «не фильтровать по полю». Строковые поля матчатся частично без учёта регистра
// (ILIKE '%...%'), идентификаторы и target_type — точным совпадением.
// Границы дат включительные и сравниваются по дню (TIMESTAMPTZ → date):
// записи с NULL-датой при заданной границе не попадают в выборку.
// Active: true — ещё закреплены (unassigned_at IS NULL), false — сняты.
type AssignmentFilter struct {
	EquipmentID          *int              // точный id оборудования
	InventoryNumber      *string           // подстрока инвентарного номера (ILIKE)
	TargetType           *AssignmentTarget // точное значение enum
	CardNumber           *int              // точный номер карточки
	CardFullName         *string           // подстрока ФИО (ILIKE)
	DepartmentCode       *int              // точный код подразделения
	DepartmentName       *string           // подстрока наименования подразделения (ILIKE)
	WaybillID            *int              // точный id путевого листа
	WaybillName          *string           // подстрока наименования путевого листа (ILIKE)
	WaybillNumber        *string           // подстрока номера путевого листа (ILIKE)
	WaybillIssueDateFrom *time.Time        // waybills.issue_date >= от (включительно)
	WaybillIssueDateTo   *time.Time        // waybills.issue_date <= до (включительно)
	Active               *bool             // true — unassigned_at IS NULL, false — NOT NULL
	AssignedAtFrom       *time.Time        // assigned_at >= от (включительно, начало дня)
	AssignedAtTo         *time.Time        // assigned_at <= до (включительно, весь день)
	UnassignedAtFrom     *time.Time        // unassigned_at >= от (включительно)
	UnassignedAtTo       *time.Time        // unassigned_at <= до (включительно)
	OperatorComment      *string           // подстрока комментария (ILIKE)
}
