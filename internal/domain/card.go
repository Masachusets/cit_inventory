// Package card описывает доменную модель карточки учёта (ответственного лица)
// и правила работы с ней. Пакет не зависит от БД, HTTP или любых внешних
// инфраструктурных слоёв — только от стандартной библиотеки.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// maxFullNameLen — лимит колонки cards.full_name (VARCHAR(150)).
const maxFullNameLen = 150

// Сентинельные ошибки домена. Сервис и хендлеры маппят их на HTTP-коды,
// поэтому набор значений фиксирован и не должен пересекаться по смыслу.
var (
	// ErrNotFound — карточка с таким номером отсутствует.
	ErrNotFound = errors.New("card not found")
	// ErrAlreadyExists — карточка с таким номером уже есть (нарушение PK).
	ErrAlreadyExists = errors.New("card already exists")
	// ErrInvalidNumber — некорректный номер карточки.
	ErrInvalidNumber = errors.New("invalid card number")
	// ErrInvalidName — некорректное ФИО (пустое или длиннее лимита).
	ErrInvalidName = errors.New("invalid card full name")
	// ErrStillActive — попытка физически удалить активную карточку.
	ErrStillActive = errors.New("card is still active")
	// ErrInUse — на карточку ссылается оборудование (FK RESTRICT), удалить нельзя.
	ErrInUse = errors.New("card is referenced by equipment")
)

// Card — карточка учёта (ответственное лицо) из таблицы cards.
// number задаётся вручную (из КУ Ф-111) и является первичным ключом.
type Card struct {
	Number    int        // регистрационный номер (PK)
	FullName  string     // ФИО ответственного
	IsActive  bool       // признак активности
	CreatedAt time.Time  // техническая метка создания
	UpdatedAt time.Time  // техническая метка изменения
	DeletedAt *time.Time // техническая метка удаления (soft delete), nil — активна
}

// NewCard создаёт новую активную карточку с валидацией входных данных.
// created_at/updated_at выставляются в now, deleted_at — nil.
func NewCard(number int, fullName string) (*Card, error) {
	if err := validateNumber(number); err != nil {
		return nil, err
	}
	fullName = normalizeName(fullName)
	if err := validateName(fullName); err != nil {
		return nil, err
	}

	now := time.Now()
	return &Card{
		Number:    number,
		FullName:  fullName,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
		DeletedAt: nil,
	}, nil
}

// UpdateFullName изменяет ФИО карточки и обновляет метку updated_at.
// Единственное редактируемое поле карточки (по требованию задачи).
func (c *Card) UpdateFullName(fullName string) error {
	fullName = normalizeName(fullName)
	if err := validateName(fullName); err != nil {
		return err
	}
	c.FullName = fullName
	c.UpdatedAt = time.Now()
	return nil
}

// Deactivate помечает карточку неактивной: is_active=false и заполняет
// deleted_at. Идемпотентно: повторный вызов для уже неактивной карточки
// не изменяет deleted_at и не является ошибкой.
func (c *Card) Deactivate() {
	if !c.IsActive {
		return
	}
	now := time.Now()
	c.IsActive = false
	c.DeletedAt = &now
	c.UpdatedAt = now
}

// validateNumber проверяет номер карточки: должен быть положительным целым.
func validateNumber(number int) error {
	if number <= 0 {
		return fmt.Errorf("%w: номер должен быть положительным, получено %d", ErrInvalidNumber, number)
	}
	return nil
}

// validateName проверяет ФИО: непустое (после обрезки краёв) и не длиннее
// лимита колонки. Проверяет уже нормализованное значение.
func validateName(fullName string) error {
	if fullName == "" {
		return fmt.Errorf("%w: ФИО не может быть пустым", ErrInvalidName)
	}
	if utf8.RuneCountInString(fullName) > maxFullNameLen {
		return fmt.Errorf("%w: ФИО длиннее %d символов", ErrInvalidName, maxFullNameLen)
	}
	return nil
}

// normalizeName обрезает краевые пробелы и приводит внутренние пробельные
// последовательности к одинарному пробелу. Так ФИО из одних пробелов
// превращается в пустую строку и отсеивается валидацией.
func normalizeName(fullName string) string {
	return strings.Join(strings.Fields(fullName), " ")
}
