// Package domain описывает доменную модель оборудования (таблица equipments)
// и правила работы с ней. Удаление — soft delete через deleted_at; выборка
// возможна как по id, так и по инвентарному номеру. Пакет не зависит от БД,
// HTTP или любых внешних инфраструктурных слоёв.
package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Лимиты колонок equipments (VARCHAR-ограничения схемы).
const (
	tMaxSerialNumberLen = 100 // serial_number VARCHAR(100)
	tMaxModelNameLen    = 255 // model_name VARCHAR(255)
	tMaxFormNumberLen   = 10  // form_number VARCHAR(10)
	tMaxLocationLen     = 255 // location VARCHAR(255)
)

// inventoryNumberPattern — формат инвентарного номера из CHECK-ограничения
// схемы: «ИТ» и ровно пять цифр (VARCHAR(7)).
var inventoryNumberPattern = regexp.MustCompile(`^ИТ\d{5}$`)

// Сентинельные ошибки домена оборудования. ErrNotFound, ErrAlreadyExists,
// ErrInvalidID и ErrInvalidName общие с остальными сущностями (пакет domain
// единый) — здесь объявлены только специфичные для equipments.
var (
	// ErrInvalidInventoryNumber — инвентарный номер не соответствует формату.
	ErrInvalidInventoryNumber = errors.New("invalid inventory number")
	// ErrInvalidStatus — статус не входит в enum equipment_status.
	ErrInvalidStatus = errors.New("invalid equipment status")
	// ErrInvalidField — некорректное значение атрибута (пустое/слишком
	// длинное). В сообщении указывается имя поля.
	ErrInvalidField = errors.New("invalid equipment field")
	// ErrInvalidNomenclature — ссылка на несуществующую номенклатуру
	// (нарушение FK nomenclature_id → nomenclatures.id).
	ErrInvalidNomenclature = errors.New("invalid nomenclature reference")
)

// EquipmentStatus — значение enum equipment_status.
type EquipmentStatus string

// Значения enum equipment_status (по порядку схемы).
const (
	StatusExp        EquipmentStatus = "exp"         // эксплуатируемый
	StatusExpInt     EquipmentStatus = "exp_int"     // эксплуатируемый (интернет)
	StatusExpSp      EquipmentStatus = "exp_sp"      // эксплуатируемый (категорированный)
	StatusBroken     EquipmentStatus = "broken"      // неисправно
	StatusWrittenOff EquipmentStatus = "written_off" // списано
)

// ParseEquipmentStatus проверяет строку на принадлежность enum equipment_status.
// Сравнение точное (после обрезки краевых пробелов); при неудаче —
// ErrInvalidStatus с полученным значением.
func ParseEquipmentStatus(s string) (EquipmentStatus, error) {
	st := EquipmentStatus(strings.TrimSpace(s))
	switch st {
	case StatusExp, StatusExpInt, StatusExpSp, StatusBroken, StatusWrittenOff:
		return st, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidStatus, s)
}

// ValidateInventoryNumber проверяет инвентарный номер по формату схемы.
// Экспортирована, чтобы сервис валидировал номер до обращения к репозиторию.
func ValidateInventoryNumber(number string) error {
	if !inventoryNumberPattern.MatchString(number) {
		return fmt.Errorf("%w: %q (ожидается ИТ и пять цифр)", ErrInvalidInventoryNumber, number)
	}
	return nil
}

// Equipment — запись оборудования (таблица equipments, aggregate root).
// Nullable-колонки хранятся указателями: nil — значение не задано (NULL).
type Equipment struct {
	ID              int             // идентификатор (PK, identity)
	InventoryNumber string          // инвентарный номер (UNIQUE, «ИТxxxxx»)
	SerialNumber    *string         // серийный номер
	NomenclatureID  *int            // ссылка на справочник номенклатуры
	ModelName       *string         // модель
	ManufactureDate *time.Time      // год и месяц изготовления (DATE)
	ArrivalDate     *time.Time      // дата прибытия (DATE)
	Status          EquipmentStatus // состояние (enum, NOT NULL DEFAULT exp)
	FormNumber      *string         // № формуляра
	Location        *string         // № кабинета / место установки
	Notes           *string         // примечания (TEXT)
	CreatedAt       time.Time       // техническая метка создания
	UpdatedAt       time.Time       // техническая метка изменения
	DeletedAt       *time.Time      // метка удаления (soft delete), nil — активна
	// Сводные поля, заполняемые репозиторием при чтении списка:
	// — nomenclatures.code / nomenclatures.name (LEFT JOIN по nomenclature_id);
	// — текущее закрепление: cards.number/full_name (employee) или
	//   departments.code/name (department). nil до заполнения.
	NomenclatureCode *string
	NomenclatureName *string
	CardNumber       *int
	CardFullName     *string
	DepartmentCode   *int16
	DepartmentName   *string
}

// EquipmentInput — входные данные для создания оборудования. Пустые строки
// приводятся к nil (NULL в БД); пустой Status — значение по умолчанию
// StatusExp (DEFAULT exp в схеме).
type EquipmentInput struct {
	InventoryNumber string
	SerialNumber    string
	NomenclatureID  *int
	ModelName       string
	ManufactureDate *time.Time
	ArrivalDate     *time.Time
	Status          string
	FormNumber      string
	Location        string
	Notes           string
}

// NewEquipment создаёт оборудование с валидацией входных данных.
// created_at/updated_at выставляются в now, deleted_at — nil.
func NewEquipment(in EquipmentInput) (*Equipment, error) {
	inventoryNumber := strings.TrimSpace(in.InventoryNumber)
	if err := ValidateInventoryNumber(inventoryNumber); err != nil {
		return nil, err
	}

	status := StatusExp
	if strings.TrimSpace(in.Status) != "" {
		parsed, err := ParseEquipmentStatus(in.Status)
		if err != nil {
			return nil, err
		}
		status = parsed
	}

	if in.NomenclatureID != nil && *in.NomenclatureID <= 0 {
		return nil, fmt.Errorf("%w: nomenclature_id должен быть положительным, получено %d",
			ErrInvalidField, *in.NomenclatureID)
	}

	serialNumber, err := optString(in.SerialNumber, "serial_number", tMaxSerialNumberLen)
	if err != nil {
		return nil, err
	}
	modelName, err := optString(in.ModelName, "model_name", tMaxModelNameLen)
	if err != nil {
		return nil, err
	}
	formNumber, err := optString(in.FormNumber, "form_number", tMaxFormNumberLen)
	if err != nil {
		return nil, err
	}
	location, err := optString(in.Location, "location", tMaxLocationLen)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &Equipment{
		InventoryNumber: inventoryNumber,
		SerialNumber:    serialNumber,
		NomenclatureID:  in.NomenclatureID,
		ModelName:       modelName,
		ManufactureDate: trimDate(in.ManufactureDate),
		ArrivalDate:     trimDate(in.ArrivalDate),
		Status:          status,
		FormNumber:      formNumber,
		Location:        location,
		Notes:           optText(in.Notes),
		CreatedAt:       now,
		UpdatedAt:       now,
		DeletedAt:       nil,
	}, nil
}

// EquipmentUpdate — частичное обновление оборудования. Семантика полей:
//   - nil — не изменять значение;
//   - строки: пустая строка очищает колонку (NULL), иначе — нормализация
//     и проверка длины;
//   - NomenclatureID: значение <= 0 очищает ссылку (NULL), > 0 устанавливает;
//   - даты: нулевое время (IsZero) очищает колонку (NULL), иначе — обрезка
//     по дню.
type EquipmentUpdate struct {
	SerialNumber    *string
	NomenclatureID  *int
	ModelName       *string
	ManufactureDate *time.Time
	ArrivalDate     *time.Time
	Status          *string
	FormNumber      *string
	Location        *string
	Notes           *string
}

// ApplyUpdate применяет частичное обновление к оборудованию, валидирует
// переданные значения и обновляет метку updated_at. Непереданные (nil) поля
// не изменяются. При некорректном значении возвращает ошибку, не меняя модель.
func (e *Equipment) ApplyUpdate(up EquipmentUpdate) error {
	if up.SerialNumber != nil {
		v, err := optString(*up.SerialNumber, "serial_number", tMaxSerialNumberLen)
		if err != nil {
			return err
		}
		e.SerialNumber = v
	}
	if up.ModelName != nil {
		v, err := optString(*up.ModelName, "model_name", tMaxModelNameLen)
		if err != nil {
			return err
		}
		e.ModelName = v
	}
	if up.FormNumber != nil {
		v, err := optString(*up.FormNumber, "form_number", tMaxFormNumberLen)
		if err != nil {
			return err
		}
		e.FormNumber = v
	}
	if up.Location != nil {
		v, err := optString(*up.Location, "location", tMaxLocationLen)
		if err != nil {
			return err
		}
		e.Location = v
	}
	if up.Notes != nil {
		e.Notes = optText(*up.Notes)
	}
	if up.NomenclatureID != nil {
		if *up.NomenclatureID <= 0 {
			e.NomenclatureID = nil
		} else {
			id := *up.NomenclatureID
			e.NomenclatureID = &id
		}
	}
	if up.ManufactureDate != nil {
		e.ManufactureDate = trimDate(up.ManufactureDate)
	}
	if up.ArrivalDate != nil {
		e.ArrivalDate = trimDate(up.ArrivalDate)
	}
	if up.Status != nil {
		st, err := ParseEquipmentStatus(*up.Status)
		if err != nil {
			return err
		}
		e.Status = st
	}
	e.UpdatedAt = time.Now()
	return nil
}

// SoftDelete помечает оборудование удалённым: заполняет deleted_at.
// Идемпотентно: повторный вызов для уже удалённого не изменяет метки
// и не является ошибкой (физического удаления в API нет).
func (e *Equipment) SoftDelete() {
	if e.DeletedAt != nil {
		return
	}
	now := time.Now()
	e.DeletedAt = &now
	e.UpdatedAt = now
}

// EquipmentFilter — параметры фильтрации списка оборудования. nil-поле
// означает «не фильтровать по полю». Строковые поля матчатся частично без
// учёта регистра (ILIKE %...%), коды/номера/статус — точное совпадение.
// Активность разведена по маршрутам /equipments и /equipments/inactive.
type EquipmentFilter struct {
	InventoryNumber  *string          // подстрока инвентарного номера
	SerialNumber     *string          // подстрока серийного номера
	ModelName        *string          // подстрока модели
	Status           *EquipmentStatus // точное значение статуса
	Location         *string          // подстрока локации
	NomenclatureCode *string          // точный код номенклатуры
	NomenclatureName *string          // подстрока наименования номенклатуры
	CardNumber       *int             // точный номер карточки
	CardFullName     *string          // подстрока ФИО ответственного
	DepartmentCode   *int16           // точный код подразделения
	DepartmentName   *string          // подстрока наименования подразделения
}

// optString нормализует опциональную строку: пустая (после обрезки краёв) →
// nil (NULL), иначе — значение с проверкой лимита.
func optString(field, name string, maxLen int) (*string, error) {
	v := normalizeName(field)
	if v == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(v) > maxLen {
		return nil, fmt.Errorf("%w: %s длиннее %d символов", ErrInvalidField, name, maxLen)
	}
	return &v, nil
}

// optText нормализует многострочный текст (notes): пустая (после обрезки
// краёв) строка → nil, иначе — обрезанное значение. Переносы строк внутри
// сохраняются (колонка TEXT без лимита).
func optText(field string) *string {
	v := strings.TrimSpace(field)
	if v == "" {
		return nil
	}
	return &v
}

// trimDate приводит дату к полуночи (колонка DATE) либо возвращает nil
// для nil/нулевого значения (сигнал «очистить колонку»).
func trimDate(t *time.Time) *time.Time {
	if t == nil || t.IsZero() {
		return nil
	}
	d := t.Truncate(24 * time.Hour)
	return &d
}
