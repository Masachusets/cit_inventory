package domain

import (
	"errors"
	"testing"
	"time"
)

// aref возвращает указатель на значение — краткая запись в тестах.
func aref(v int) *int { return &v }

func TestParseAssignmentTarget(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    AssignmentTarget
		wantErr error
	}{
		{"employee", "employee", TargetEmployee, nil},
		{"department", "department", TargetDepartment, nil},
		{"warehouse", "warehouse", TargetWarehouse, nil},
		{"с пробелами", "  employee  ", TargetEmployee, nil},
		{"пустая строка", "", "", ErrInvalidTarget},
		{"неизвестный тип", "shelf", "", ErrInvalidTarget},
		{"регистр", "Employee", "", ErrInvalidTarget},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAssignmentTarget(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("target = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewAssignment(t *testing.T) {
	assigned := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	tooEarly := time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		in      AssignmentInput
		wantErr error
		check   func(t *testing.T, a *Assignment)
	}{
		{
			name: "employee с карточкой",
			in: AssignmentInput{
				EquipmentID: 1, TargetType: "employee",
				CardNumber: aref(101), AssignedAt: &assigned,
			},
			check: func(t *testing.T, a *Assignment) {
				if a.CardNumber == nil || *a.CardNumber != 101 {
					t.Errorf("card_number = %v, want 101", a.CardNumber)
				}
				if a.DepartmentCode != nil {
					t.Errorf("department_code = %v, want nil", a.DepartmentCode)
				}
			},
		},
		{
			name: "department с кодом и путевым листом",
			in: AssignmentInput{
				EquipmentID: 2, TargetType: "department",
				DepartmentCode: aref(100), WaybillID: aref(7), AssignedAt: &assigned,
			},
			check: func(t *testing.T, a *Assignment) {
				if a.DepartmentCode == nil || *a.DepartmentCode != 100 {
					t.Errorf("department_code = %v, want 100", a.DepartmentCode)
				}
				if a.WaybillID == nil || *a.WaybillID != 7 {
					t.Errorf("waybill_id = %v, want 7", a.WaybillID)
				}
			},
		},
		{
			name: "warehouse без ссылок",
			in:   AssignmentInput{EquipmentID: 3, TargetType: "warehouse", AssignedAt: &assigned},
			check: func(t *testing.T, a *Assignment) {
				if a.CardNumber != nil || a.DepartmentCode != nil {
					t.Errorf("ссылки должны быть nil: %+v", a)
				}
			},
		},
		{
			name:    "equipment_id = 0",
			in:      AssignmentInput{EquipmentID: 0, TargetType: "warehouse"},
			wantErr: ErrInvalidID,
		},
		{
			name:    "неизвестный target_type",
			in:      AssignmentInput{EquipmentID: 1, TargetType: "shelf"},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "employee без card_number",
			in:      AssignmentInput{EquipmentID: 1, TargetType: "employee"},
			wantErr: ErrInvalidTarget,
		},
		{
			name: "employee с department_code",
			in: AssignmentInput{
				EquipmentID: 1, TargetType: "employee",
				CardNumber: aref(1), DepartmentCode: aref(100),
			},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "department без department_code",
			in:      AssignmentInput{EquipmentID: 1, TargetType: "department"},
			wantErr: ErrInvalidTarget,
		},
		{
			name:    "warehouse с card_number",
			in:      AssignmentInput{EquipmentID: 1, TargetType: "warehouse", CardNumber: aref(1)},
			wantErr: ErrInvalidTarget,
		},
		{
			name: "card_number = 0",
			in: AssignmentInput{
				EquipmentID: 1, TargetType: "employee", CardNumber: aref(0),
			},
			wantErr: ErrInvalidField,
		},
		{
			name: "unassigned раньше assigned",
			in: AssignmentInput{
				EquipmentID: 1, TargetType: "warehouse",
				AssignedAt: &assigned, UnassignedAt: &tooEarly,
			},
			wantErr: ErrInvalidField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := NewAssignment(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if a != nil {
					t.Errorf("модель должна быть nil при ошибке")
				}
				return
			}
			if a.AssignedAt.IsZero() {
				t.Error("assigned_at должен быть заполнен")
			}
			if tt.check != nil {
				tt.check(t, a)
			}
		})
	}
}

func TestNewAssignment_DefaultAssignedAt(t *testing.T) {
	a, err := NewAssignment(AssignmentInput{EquipmentID: 1, TargetType: "warehouse"})
	if err != nil {
		t.Fatalf("NewAssignment() ошибка: %v", err)
	}
	if a.AssignedAt.IsZero() {
		t.Error("assigned_at по умолчанию должен быть текущим временем")
	}
	if a.UnassignedAt != nil {
		t.Errorf("unassigned_at = %v, want nil", a.UnassignedAt)
	}
}

func TestNewAssignment_OperatorCommentEmpty(t *testing.T) {
	a, err := NewAssignment(AssignmentInput{
		EquipmentID: 1, TargetType: "warehouse", OperatorComment: "   ",
	})
	if err != nil {
		t.Fatalf("NewAssignment() ошибка: %v", err)
	}
	if a.OperatorComment != nil {
		t.Errorf("operator_comment = %v, want nil", a.OperatorComment)
	}
}

func TestAssignment_ApplyUpdate(t *testing.T) {
	assigned := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	later := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

	base := func(t *testing.T) *Assignment {
		t.Helper()
		a, err := NewAssignment(AssignmentInput{
			EquipmentID: 1, TargetType: "employee",
			CardNumber: aref(101), AssignedAt: &assigned,
		})
		if err != nil {
			t.Fatalf("NewAssignment() ошибка: %v", err)
		}
		return a
	}

	t.Run("частичное обновление не трогает остальные поля", func(t *testing.T) {
		a := base(t)
		comment := "передано в ОПК"
		if err := a.ApplyUpdate(AssignmentUpdate{OperatorComment: &comment}); err != nil {
			t.Fatalf("ApplyUpdate() ошибка: %v", err)
		}
		if a.OperatorComment == nil || *a.OperatorComment != comment {
			t.Errorf("operator_comment = %v", a.OperatorComment)
		}
		if a.CardNumber == nil || *a.CardNumber != 101 {
			t.Errorf("card_number = %v, want 101", a.CardNumber)
		}
	})

	t.Run("смена employee на department", func(t *testing.T) {
		a := base(t)
		target := "department"
		if err := a.ApplyUpdate(AssignmentUpdate{
			TargetType: &target, CardNumber: aref(0), DepartmentCode: aref(200),
		}); err != nil {
			t.Fatalf("ApplyUpdate() ошибка: %v", err)
		}
		if a.TargetType != TargetDepartment {
			t.Errorf("target = %q, want department", a.TargetType)
		}
		if a.CardNumber != nil {
			t.Errorf("card_number = %v, want nil (очищен)", a.CardNumber)
		}
		if a.DepartmentCode == nil || *a.DepartmentCode != 200 {
			t.Errorf("department_code = %v, want 200", a.DepartmentCode)
		}
	})

	t.Run("непоследовательная смена — ошибка chk_target", func(t *testing.T) {
		a := base(t)
		target := "warehouse"
		err := a.ApplyUpdate(AssignmentUpdate{TargetType: &target})
		if !errors.Is(err, ErrInvalidTarget) {
			t.Fatalf("err = %v, want ErrInvalidTarget", err)
		}
		// Модель не изменена: target остался прежним.
		if a.TargetType != TargetEmployee {
			t.Errorf("target = %q, want employee (без изменений)", a.TargetType)
		}
	})

	t.Run("некорректный target_type", func(t *testing.T) {
		a := base(t)
		target := "shelf"
		if err := a.ApplyUpdate(AssignmentUpdate{TargetType: &target}); !errors.Is(err, ErrInvalidTarget) {
			t.Errorf("err = %v, want ErrInvalidTarget", err)
		}
	})

	t.Run("пустой assigned_at — ошибка", func(t *testing.T) {
		a := base(t)
		zero := time.Time{}
		if err := a.ApplyUpdate(AssignmentUpdate{AssignedAt: &zero}); !errors.Is(err, ErrInvalidField) {
			t.Errorf("err = %v, want ErrInvalidField", err)
		}
	})

	t.Run("снятие с закрепления и возврат", func(t *testing.T) {
		a := base(t)
		if err := a.ApplyUpdate(AssignmentUpdate{UnassignedAt: &later}); err != nil {
			t.Fatalf("снятие: %v", err)
		}
		if a.UnassignedAt == nil {
			t.Fatal("unassigned_at = nil, want время снятия")
		}
		zero := time.Time{}
		if err := a.ApplyUpdate(AssignmentUpdate{UnassignedAt: &zero}); err != nil {
			t.Fatalf("возврат: %v", err)
		}
		if a.UnassignedAt != nil {
			t.Errorf("unassigned_at = %v, want nil (очищен)", a.UnassignedAt)
		}
	})

	t.Run("unassigned раньше assigned — ошибка", func(t *testing.T) {
		a := base(t)
		if err := a.ApplyUpdate(AssignmentUpdate{UnassignedAt: &earlier}); !errors.Is(err, ErrInvalidField) {
			t.Errorf("err = %v, want ErrInvalidField", err)
		}
	})
}
