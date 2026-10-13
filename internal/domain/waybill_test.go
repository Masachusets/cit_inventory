package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewWaybill_OK(t *testing.T) {
	num := "123/2026"
	date := time.Date(2026, 10, 13, 15, 30, 0, 0, time.UTC)

	wb, err := NewWaybill("  Путевой   лист ", num, &date)
	if err != nil {
		t.Fatalf("NewWaybill() ошибка: %v", err)
	}
	if wb.Name != "Путевой лист" {
		t.Errorf("Name = %q", wb.Name)
	}
	if wb.Number == nil || *wb.Number != num {
		t.Errorf("Number = %v, want %q", wb.Number, num)
	}
	// Дата обрезается до суток (DATE-колонка).
	if wb.IssueDate == nil || wb.IssueDate.Hour() != 0 || !wb.IssueDate.Equal(date.Truncate(24*time.Hour)) {
		t.Errorf("IssueDate = %v", wb.IssueDate)
	}
}

func TestNewWaybill_OptionalFieldsNil(t *testing.T) {
	wb, err := NewWaybill("Без номера", "  ", nil)
	if err != nil {
		t.Fatalf("NewWaybill() ошибка: %v", err)
	}
	if wb.Number != nil {
		t.Errorf("пустой номер должен быть nil, got %v", wb.Number)
	}
	if wb.IssueDate != nil {
		t.Errorf("nil-дата должна остаться nil, got %v", wb.IssueDate)
	}
}

func TestNewWaybill_InvalidName(t *testing.T) {
	if _, err := NewWaybill("", "", nil); !errors.Is(err, ErrInvalidName) {
		t.Errorf("пустое имя: err = %v, want ErrInvalidName", err)
	}
	long := strings.Repeat("а", maxWaybillNameLen+1)
	if _, err := NewWaybill(long, "", nil); !errors.Is(err, ErrInvalidName) {
		t.Errorf("длинное имя: err = %v, want ErrInvalidName", err)
	}
}

func TestNewWaybill_TooLongNumber(t *testing.T) {
	long := strings.Repeat("9", maxWaybillNumberLen+1)
	if _, err := NewWaybill("Имя", long, nil); !errors.Is(err, ErrInvalidName) {
		t.Errorf("длинный номер: err = %v, want ErrInvalidName", err)
	}
}

func TestWaybillFilter_ZeroValue(t *testing.T) {
	var f WaybillFilter
	if f.Name != nil || f.Number != nil || f.IssueDateFrom != nil || f.IssueDateTo != nil {
		t.Errorf("нулевой фильтр должен иметь nil-поля: %+v", f)
	}
}
