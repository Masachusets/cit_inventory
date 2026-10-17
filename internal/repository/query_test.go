package repository

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// Тесты чистых функций сборки SELECT-запросов list/List-методов:
// проверяется точный SQL и набор аргументов без обращения к БД.

func TestCardsListQuery(t *testing.T) {
	ivan := "Иван"
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		active   bool
		filter   domain.CardFilter
		wantSQL  string
		wantArgs []any
	}{
		{
			name:   "без фильтров — только активность",
			active: true,
			wantSQL: "SELECT " + cardColumns + " FROM cards" +
				" WHERE is_active = $1" +
				" ORDER BY number",
			wantArgs: []any{true},
		},
		{
			name:   "ФИО и диапазон дат created_at",
			active: false,
			filter: domain.CardFilter{
				FullName:      &ivan,
				CreatedAtFrom: &from,
				CreatedAtTo:   &to,
			},
			wantSQL: "SELECT " + cardColumns + " FROM cards" +
				" WHERE is_active = $1" +
				" AND full_name ILIKE '%' || $2 || '%'" +
				" AND created_at::date >= $3::date" +
				" AND created_at::date <= $4::date" +
				" ORDER BY number",
			wantArgs: []any{false, ivan, from, to},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := cardsListQuery(tt.active, tt.filter)
			if query != tt.wantSQL {
				t.Errorf("query = %q, хотели %q", query, tt.wantSQL)
			}
			if !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("args = %v, хотели %v", args, tt.wantArgs)
			}
		})
	}
}

func TestDepartmentsListQuery(t *testing.T) {
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	// Регрессия: верхняя граница должна фильтровать как «не позже» (<=),
	// а не как «не раньше» — такая опечатка однажды была в card_repo.
	query, args := departmentsListQuery(true, domain.DepartmentFilter{CreatedAtTo: &to})

	want := "SELECT " + departmentColumns + " FROM departments" +
		" WHERE is_active = $1" +
		" AND created_at::date <= $2::date" +
		" ORDER BY code"
	if query != want {
		t.Errorf("query = %q, хотели %q", query, want)
	}
	if !reflect.DeepEqual(args, []any{true, to}) {
		t.Errorf("args = %v, хотели %v", args, []any{true, to})
	}
}

func TestNomenclaturesListQuery(t *testing.T) {
	code := "МЕБ/0001"

	t.Run("без фильтров нет WHERE", func(t *testing.T) {
		query, args := nomenclaturesListQuery(domain.NomenclatureFilter{})

		want := "SELECT " + nomenclatureColumns + " FROM nomenclatures ORDER BY id"
		if query != want {
			t.Errorf("query = %q, хотели %q", query, want)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, хотели пустой набор", args)
		}
	})

	t.Run("подстрока кода", func(t *testing.T) {
		query, args := nomenclaturesListQuery(domain.NomenclatureFilter{Code: &code})

		want := "SELECT " + nomenclatureColumns + " FROM nomenclatures" +
			" WHERE code ILIKE '%' || $1 || '%'" +
			" ORDER BY id"
		if query != want {
			t.Errorf("query = %q, хотели %q", query, want)
		}
		if !reflect.DeepEqual(args, []any{code}) {
			t.Errorf("args = %v, хотели %v", args, []any{code})
		}
	})
}

func TestWaybillsListQuery(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	query, args := waybillsListQuery(domain.WaybillFilter{
		IssueDateFrom: &from,
		IssueDateTo:   &to,
	})

	want := "SELECT " + waybillColumns + " FROM waybills" +
		" WHERE issue_date::date >= $1::date" +
		" AND issue_date::date <= $2::date" +
		" ORDER BY id"
	if query != want {
		t.Errorf("query = %q, хотели %q", query, want)
	}
	if !reflect.DeepEqual(args, []any{from, to}) {
		t.Errorf("args = %v, хотели %v", args, []any{from, to})
	}
}

func TestEquipmentsListQuery(t *testing.T) {
	num := "ИТ00001"
	status := domain.EquipmentStatus("exp")

	query, args := equipmentsListQuery(true, domain.EquipmentFilter{
		InventoryNumber: &num,
		Status:          &status,
	})

	// Регрессия: ILIKE-шаблон должен быть строковым литералом
	// '%' || $n || '%' — иначе PostgreSQL падает с syntax error.
	// cond без аргумента не сдвигает нумерацию: $1 — inventory_number.
	// Колонки префиксируются e.: после JOIN-ов непрефиксированный
	// deleted_at неоднозначен (есть у cards и departments).
	wantConds := "e.deleted_at IS NULL" +
		" AND e.inventory_number ILIKE '%' || $1 || '%'" +
		" AND e.status = $2::equipment_status"
	if !strings.Contains(query, " WHERE "+wantConds) {
		t.Errorf("query не содержит ожидаемых условий:\n%s\nхотели: %s", query, wantConds)
	}
	if !strings.HasSuffix(query, " ORDER BY e.id") {
		t.Errorf("query должен сортироваться по e.id:\n%s", query)
	}
	if !reflect.DeepEqual(args, []any{num, string(status)}) {
		t.Errorf("args = %v, хотели %v", args, []any{num, string(status)})
	}
}

func TestAssignmentsListQuery(t *testing.T) {
	ivan := "Иван"
	target := domain.AssignmentTarget("employee")
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	active := true

	t.Run("без фильтров нет WHERE", func(t *testing.T) {
		query, args := assignmentsListQuery(domain.AssignmentFilter{})

		want := assignmentSelect + " ORDER BY a.id"
		if query != want {
			t.Errorf("query = %q, хотели %q", query, want)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, хотели пустой набор", args)
		}
	})

	t.Run("enum, ILIKE, диапазон дат и активность", func(t *testing.T) {
		query, args := assignmentsListQuery(domain.AssignmentFilter{
			TargetType:     &target,
			CardFullName:   &ivan,
			AssignedAtFrom: &from,
			Active:         &active,
		})

		want := assignmentSelect +
			" WHERE a.target_type = $1::assignment_target" +
			" AND c.full_name ILIKE '%' || $2 || '%'" +
			" AND a.assigned_at::date >= $3::date" +
			" AND a.unassigned_at IS NULL" +
			" ORDER BY a.id"
		if query != want {
			t.Errorf("query = %q, хотели %q", query, want)
		}
		if !reflect.DeepEqual(args, []any{string(target), ivan, from}) {
			t.Errorf("args = %v, хотели %v", args, []any{string(target), ivan, from})
		}
	})
}
