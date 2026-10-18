package domain

import (
	"fmt"
	"time"
	"unicode/utf8"
)

// Лимиты колонок waybills.
const (
	maxWaybillNameLen   = 50 // name VARCHAR(50) NOT NULL
	maxWaybillNumberLen = 50 // number VARCHAR(50) UNIQUE
)

// Waybill — запись справочника путевых листов (таблица waybills).
// Справочник пополняется через API; деактивация не применяется
// (в схеме нет is_active).
type Waybill struct {
	ID        int        // идентификатор (PK, identity)
	Name      string     // наименование документа (NOT NULL)
	Number    *string    // номер документа (UNIQUE, может быть NULL)
	IssueDate *time.Time // дата документа (DATE, может быть NULL)
}

// NewWaybill создаёт новый путевой лист с валидацией. Пустые number/issueDate
// приводятся к nil — так хранятся NULL-значения колонок.
func NewWaybill(name, number string, issueDate *time.Time) (*Waybill, error) {
	name = normalizeName(name)
	if name == "" {
		return nil, fmt.Errorf("%w: наименование не может быть пустым", ErrInvalidName)
	}
	if utf8.RuneCountInString(name) > maxWaybillNameLen {
		return nil, fmt.Errorf("%w: наименование длиннее %d символов", ErrInvalidName, maxWaybillNameLen)
	}

	w := &Waybill{Name: name}

	number = normalizeName(number)
	if number != "" {
		if utf8.RuneCountInString(number) > maxWaybillNumberLen {
			return nil, fmt.Errorf("%w: номер документа длиннее %d символов", ErrInvalidName, maxWaybillNumberLen)
		}
		w.Number = &number
	}

	if issueDate != nil {
		d := issueDate.Truncate(24 * time.Hour)
		w.IssueDate = &d
	}
	return w, nil
}

// Update изменяет поля путевого листа. Валидирует наименование и номер,
// пустые значения number/issueDate приводятся к nil (NULL в БД).
func (w *Waybill) Update(name, number string, issueDate *time.Time) error {
	name = normalizeName(name)
	if name == "" {
		return fmt.Errorf("%w: наименование не может быть пустым", ErrInvalidName)
	}
	if utf8.RuneCountInString(name) > maxWaybillNameLen {
		return fmt.Errorf("%w: наименование длиннее %d символов", ErrInvalidName, maxWaybillNameLen)
	}

	w.Name = name

	number = normalizeName(number)
	if number != "" {
		if utf8.RuneCountInString(number) > maxWaybillNumberLen {
			return fmt.Errorf("%w: номер документа длиннее %d символов", ErrInvalidName, maxWaybillNumberLen)
		}
		w.Number = &number
	} else {
		w.Number = nil
	}

	if issueDate != nil {
		d := issueDate.Truncate(24 * time.Hour)
		w.IssueDate = &d
	} else {
		w.IssueDate = nil
	}
	return nil
}

// WaybillFilter — параметры фильтрации списка путевых листов (все поля,
// кроме PK). nil-поле означает «не фильтровать». Границы issue_date —
// включительные (>= / <=); записи с NULL-датой при заданной границе
// не попадают в выборку (SQL-семантика сравнений с NULL).
type WaybillFilter struct {
	Name          *string    // подстрока наименования (ILIKE '%...%')
	Number        *string    // подстрока номера (ILIKE '%...%')
	IssueDateFrom *time.Time // issue_date >= от (включительно)
	IssueDateTo   *time.Time // issue_date <= до (включительно)
}
