package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewDepartment_OK(t *testing.T) {
	d, err := NewDepartment(301, "  опк   Гомель  ")
	if err != nil {
		t.Fatalf("NewDepartment() ошибка: %v", err)
	}
	if d.Code != 301 || d.Name != "опк Гомель" {
		t.Errorf("department = %+v", d)
	}
	if !d.IsActive {
		t.Error("новое подразделение должно быть активным")
	}
	if d.CreatedAt.IsZero() {
		t.Error("CreatedAt не заполнен")
	}
}

func TestNewDepartment_InvalidCode(t *testing.T) {
	for _, code := range []int{0, -1, 1 << 20} {
		if _, err := NewDepartment(code, "Имя"); !errors.Is(err, ErrInvalidCode) {
			t.Errorf("code=%d: err = %v, want ErrInvalidCode", code, err)
		}
	}
}

func TestNewDepartment_InvalidName(t *testing.T) {
	if _, err := NewDepartment(100, "   "); !errors.Is(err, ErrInvalidName) {
		t.Errorf("пустое имя: err = %v, want ErrInvalidName", err)
	}
	long := strings.Repeat("а", maxDepartmentNameLen+1)
	if _, err := NewDepartment(100, long); !errors.Is(err, ErrInvalidName) {
		t.Errorf("длинное имя: err = %v, want ErrInvalidName", err)
	}
}

func TestDepartment_UpdateName(t *testing.T) {
	d, _ := NewDepartment(100, "Старое")

	if err := d.UpdateName("  Новое   имя "); err != nil {
		t.Fatalf("UpdateName() ошибка: %v", err)
	}
	if d.Name != "Новое имя" {
		t.Errorf("Name = %q", d.Name)
	}

	if err := d.UpdateName(""); !errors.Is(err, ErrInvalidName) {
		t.Errorf("пустое имя: err = %v, want ErrInvalidName", err)
	}
}

func TestDepartment_Deactivate_Idempotent(t *testing.T) {
	d, _ := NewDepartment(100, "Имя")

	d.Deactivate()
	if d.IsActive {
		t.Error("после Deactivate IsActive должно быть false")
	}
	d.Deactivate() // повторный вызов не паникует и не меняет ничего
	if d.IsActive {
		t.Error("повторная деактивация не должна возвращать активность")
	}
}

func TestValidateDepartmentCode(t *testing.T) {
	if err := ValidateDepartmentCode(1); err != nil {
		t.Errorf("code=1: %v", err)
	}
	if err := ValidateDepartmentCode(32767); err != nil {
		t.Errorf("code=32767: %v", err)
	}
	if err := ValidateDepartmentCode(32768); !errors.Is(err, ErrInvalidCode) {
		t.Errorf("code=32768: err = %v, want ErrInvalidCode", err)
	}
}

func TestDepartmentFilter_ZeroValue(t *testing.T) {
	var f DepartmentFilter
	if f.Name != nil || f.CreatedAtFrom != nil || f.CreatedAtTo != nil {
		t.Errorf("нулевой фильтр должен иметь nil-поля: %+v", f)
	}
}
