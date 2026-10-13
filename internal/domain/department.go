// Department — запись справочника подразделений. Деактивация выполняется
// только через is_active (без deleted_at/updated_at — схема departments их
// не содержит). Валидация name реюзит normalizeName из card.go (пакет общий).
package domain

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// maxDepartmentNameLen — лимит колонки departments.name (VARCHAR(150)).
const maxDepartmentNameLen = 150

// ErrInvalidCode — некорректный код подразделения (не положительный
// или вне диапазона SMALLINT). ErrNotFound, ErrAlreadyExists и
// ErrInvalidName общие с cards (пакет domain единый).
var ErrInvalidCode = errors.New("invalid department code")

// Department — запись справочника подразделений (таблица departments).
// code задаётся вручную (100, 200, 301...) и является первичным ключом.
type Department struct {
	Code      int16     // код подразделения (PK)
	Name      string    // наименование (UNIQUE), например «опк Гомель»
	IsActive  bool      // признак активности
	CreatedAt time.Time // дата создания (DATE, DEFAULT CURRENT_DATE)
}

// NewDepartment создаёт новое активное подразделение с валидацией.
func NewDepartment(code int, name string) (*Department, error) {
	if err := ValidateDepartmentCode(code); err != nil {
		return nil, err
	}
	name = normalizeName(name)
	if err := validateDepartmentName(name); err != nil {
		return nil, err
	}
	return &Department{
		Code:      int16(code),
		Name:      name,
		IsActive:  true,
		CreatedAt: time.Now(),
	}, nil
}

// ValidateDepartmentCode проверяет код подразделения: положительное целое
// в диапазоне SMALLINT (тип PK-колонки departments.code). Экспортирована,
// чтобы сервис валидировал код до обращения к репозиторию.
func ValidateDepartmentCode(code int) error {
	if code <= 0 || code > math.MaxInt16 {
		return fmt.Errorf("%w: код должен быть в диапазоне 1..%d, получено %d",
			ErrInvalidCode, math.MaxInt16, code)
	}
	return nil
}

// UpdateName изменяет наименование подразделения. Единственное редактируемое
// поле (по требованию задачи); updated_at в схеме нет, метка не обновляется.
func (d *Department) UpdateName(name string) error {
	name = normalizeName(name)
	if err := validateDepartmentName(name); err != nil {
		return err
	}
	d.Name = name
	return nil
}

// Deactivate помечает подразделение неактивным (is_active=false).
// Идемпотентно: повторный вызов для уже неактивного не является ошибкой.
func (d *Department) Deactivate() {
	if !d.IsActive {
		return
	}
	d.IsActive = false
}

// validateDepartmentName проверяет наименование: непустое (после обрезки
// краёв) и не длиннее лимита колонки. Проверяет уже нормализованное значение.
func validateDepartmentName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: наименование не может быть пустым", ErrInvalidName)
	}
	if len([]rune(name)) > maxDepartmentNameLen {
		return fmt.Errorf("%w: наименование длиннее %d символов", ErrInvalidName, maxDepartmentNameLen)
	}
	return nil
}

// DepartmentFilter — параметры фильтрации списка подразделений (все поля,
// кроме PK). nil-поле означает «не фильтровать». Фильтра по is_active нет:
// активность разведена по маршрутам /departments и /departments/inactive.
// Границы created_at — включительные (>= / <=).
type DepartmentFilter struct {
	Name          *string    // подстрока наименования (ILIKE '%...%')
	CreatedAtFrom *time.Time // created_at >= от (включительно)
	CreatedAtTo   *time.Time // created_at <= до (включительно)
}
