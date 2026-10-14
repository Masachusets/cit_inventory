package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// validInput — корректный минимальный набор входных данных для NewEquipment.
func validInput() EquipmentInput {
	return EquipmentInput{InventoryNumber: "ИТ00123"}
}

func TestNewEquipment_Valid(t *testing.T) {
	nomenID := 7
	manuf := time.Date(2015, 3, 10, 0, 0, 0, 0, time.UTC)
	arrival := time.Date(2012, 8, 1, 0, 0, 0, 0, time.UTC)

	eq, err := NewEquipment(EquipmentInput{
		InventoryNumber: "  ИТ00123  ", // краевые пробелы обрезаются
		SerialNumber:    "SN-001",
		NomenclatureID:  &nomenID,
		ModelName:       "Ноутбук Clevo 15\"",
		ManufactureDate: &manuf,
		ArrivalDate:     &arrival,
		Status:          "broken",
		FormNumber:      "Ф-1",
		Location:        "каб. 101",
		Notes:           "примечание",
	})
	if err != nil {
		t.Fatalf("NewEquipment() ошибка: %v", err)
	}
	if eq.InventoryNumber != "ИТ00123" {
		t.Errorf("InventoryNumber = %q", eq.InventoryNumber)
	}
	if eq.Status != StatusBroken {
		t.Errorf("Status = %q, want broken", eq.Status)
	}
	if eq.SerialNumber == nil || *eq.SerialNumber != "SN-001" {
		t.Errorf("SerialNumber = %v", eq.SerialNumber)
	}
	if eq.NomenclatureID == nil || *eq.NomenclatureID != nomenID {
		t.Errorf("NomenclatureID = %v", eq.NomenclatureID)
	}
	if eq.DeletedAt != nil {
		t.Errorf("DeletedAt должен быть nil у нового оборудования")
	}
	if eq.CreatedAt.IsZero() || eq.UpdatedAt.IsZero() {
		t.Errorf("метки времени не заполнены: %v / %v", eq.CreatedAt, eq.UpdatedAt)
	}
}

func TestNewEquipment_EmptyOptionalFields(t *testing.T) {
	eq, err := NewEquipment(validInput())
	if err != nil {
		t.Fatalf("NewEquipment() ошибка: %v", err)
	}
	if eq.SerialNumber != nil || eq.NomenclatureID != nil || eq.ModelName != nil ||
		eq.ManufactureDate != nil || eq.ArrivalDate != nil ||
		eq.FormNumber != nil || eq.Location != nil || eq.Notes != nil {
		t.Errorf("пустые опции должны быть nil: %+v", eq)
	}
	if eq.Status != StatusExp {
		t.Errorf("Status = %q, want exp (DEFAULT 'exp')", eq.Status)
	}
}

func TestNewEquipment_InvalidInventoryNumber(t *testing.T) {
	invalid := []string{"", "ИТ0001", "ИТ000001", "IT00001", "ХТ00001", "ИТ0000a", "12345"}
	for _, number := range invalid {
		_, err := NewEquipment(EquipmentInput{InventoryNumber: number})
		if !errors.Is(err, ErrInvalidInventoryNumber) {
			t.Errorf("номер %q: err = %v, want ErrInvalidInventoryNumber", number, err)
		}
	}
}

func TestNewEquipment_Status(t *testing.T) {
	cases := []struct {
		status  string
		want    EquipmentStatus
		wantErr bool
	}{
		{"", StatusExp, false},
		{"exp_int", StatusExpInt, false},
		{" exp_sp ", StatusExpSp, false},
		{"written_off", StatusWrittenOff, false},
		{"списано", "", true},
		{"broke", "", true},
	}
	for _, c := range cases {
		in := validInput()
		in.Status = c.status
		eq, err := NewEquipment(in)
		if c.wantErr {
			if !errors.Is(err, ErrInvalidStatus) {
				t.Errorf("статус %q: err = %v, want ErrInvalidStatus", c.status, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("статус %q: неожиданная ошибка: %v", c.status, err)
			continue
		}
		if eq.Status != c.want {
			t.Errorf("статус %q: got %q, want %q", c.status, eq.Status, c.want)
		}
	}
}

func TestNewEquipment_FieldLimits(t *testing.T) {
	in := validInput()
	in.SerialNumber = strings.Repeat("s", maxSerialNumberLen+1)
	if _, err := NewEquipment(in); !errors.Is(err, ErrInvalidField) {
		t.Errorf("длинный serial_number: err = %v, want ErrInvalidField", err)
	}

	in = validInput()
	in.NomenclatureID = ptrInt(0)
	if _, err := NewEquipment(in); !errors.Is(err, ErrInvalidField) {
		t.Errorf("nomenclature_id = 0: err = %v, want ErrInvalidField", err)
	}
}

func TestEquipment_ApplyUpdate_Partial(t *testing.T) {
	eq, _ := NewEquipment(validInput())
	oldUpdatedAt := eq.UpdatedAt

	model := "Новая модель"
	if err := eq.ApplyUpdate(EquipmentUpdate{ModelName: &model}); err != nil {
		t.Fatalf("ApplyUpdate() ошибка: %v", err)
	}
	if eq.ModelName == nil || *eq.ModelName != "Новая модель" {
		t.Errorf("ModelName = %v", eq.ModelName)
	}
	if eq.SerialNumber != nil {
		t.Errorf("непереданное поле SerialNumber изменилось: %v", eq.SerialNumber)
	}
	if !eq.UpdatedAt.After(oldUpdatedAt) {
		t.Errorf("UpdatedAt не обновился")
	}
}

func TestEquipment_ApplyUpdate_Clear(t *testing.T) {
	nomenID := 5
	empty := ""
	zero := time.Time{}
	eq, _ := NewEquipment(EquipmentInput{InventoryNumber: "ИТ00123", SerialNumber: "SN-1", NomenclatureID: &nomenID})

	if err := eq.ApplyUpdate(EquipmentUpdate{
		SerialNumber:    &empty,
		NomenclatureID:  ptrInt(0),
		ManufactureDate: &zero,
	}); err != nil {
		t.Fatalf("ApplyUpdate() ошибка: %v", err)
	}
	if eq.SerialNumber != nil {
		t.Errorf("SerialNumber должен очиститься (NULL): %v", eq.SerialNumber)
	}
	if eq.NomenclatureID != nil {
		t.Errorf("NomenclatureID должен очиститься (NULL): %v", eq.NomenclatureID)
	}
	if eq.ManufactureDate != nil {
		t.Errorf("ManufactureDate должна очиститься (NULL): %v", eq.ManufactureDate)
	}
}

func TestEquipment_ApplyUpdate_InvalidStatus(t *testing.T) {
	eq, _ := NewEquipment(validInput())
	bad := "нет"
	if err := eq.ApplyUpdate(EquipmentUpdate{Status: &bad}); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("err = %v, want ErrInvalidStatus", err)
	}
	if eq.Status != StatusExp {
		t.Errorf("модель не должна меняться при ошибке: Status = %q", eq.Status)
	}
}

func TestEquipment_SoftDelete_Idempotent(t *testing.T) {
	eq, _ := NewEquipment(validInput())

	eq.SoftDelete()
	if eq.DeletedAt == nil {
		t.Fatalf("после SoftDelete() DeletedAt = nil")
	}
	first := *eq.DeletedAt
	eq.SoftDelete()
	if !eq.DeletedAt.Equal(first) {
		t.Errorf("повторный SoftDelete перезаписал метку: %v → %v", first, *eq.DeletedAt)
	}
}

func TestParseEquipmentStatus(t *testing.T) {
	for _, s := range []string{"exp", "exp_int", "exp_sp", "broken", "written_off"} {
		if _, err := ParseEquipmentStatus(s); err != nil {
			t.Errorf("ParseEquipmentStatus(%q) ошибка: %v", s, err)
		}
	}
	if _, err := ParseEquipmentStatus("EXP"); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("регистрозависимое сравнение: err = %v, want ErrInvalidStatus", err)
	}
}

func TestValidateInventoryNumber(t *testing.T) {
	if err := ValidateInventoryNumber("ИТ00001"); err != nil {
		t.Errorf("корректный номер: %v", err)
	}
	if err := ValidateInventoryNumber("ИТ000001"); !errors.Is(err, ErrInvalidInventoryNumber) {
		t.Errorf("err = %v, want ErrInvalidInventoryNumber", err)
	}
}

// ptrInt — вспомогательный указатель на int для тестов.
func ptrInt(v int) *int { return &v }
