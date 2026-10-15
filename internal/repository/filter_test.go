package repository

import (
	"reflect"
	"testing"
	"time"
)

// Тесты общего сборщика WHERE-условий: нумерация плейсхолдеров, ILIKE
// по подстроке и диапазоны дат по дню (включительно).

func TestFilterBuilder(t *testing.T) {
	ivan := "Иван"
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		build    func(f *filterBuilder)
		wantSQL  string
		wantArgs []any
	}{
		{
			name:    "без условий запрос не меняется",
			build:   func(f *filterBuilder) {},
			wantSQL: "SELECT 1",
		},
		{
			name: "add нумерует плейсхолдеры по порядку",
			build: func(f *filterBuilder) {
				f.add("a = $%d", 10)
				f.add("b = $%d", 20)
			},
			wantSQL:  "SELECT 1 WHERE a = $1 AND b = $2",
			wantArgs: []any{10, 20},
		},
		{
			name: "cond добавляет условие без аргумента",
			build: func(f *filterBuilder) {
				f.add("a = $%d", 1)
				f.cond("x IS NULL")
			},
			wantSQL:  "SELECT 1 WHERE a = $1 AND x IS NULL",
			wantArgs: []any{1},
		},
		{
			name: "ilike: nil-значение фильтра пропускается",
			build: func(f *filterBuilder) {
				f.ilike("name", nil)
			},
			wantSQL: "SELECT 1",
		},
		{
			name: "ilike: подстрока оборачивается в % как литерал",
			build: func(f *filterBuilder) {
				f.ilike("name", &ivan)
			},
			wantSQL:  "SELECT 1 WHERE name ILIKE '%' || $1 || '%'",
			wantArgs: []any{ivan},
		},
		{
			name: "dateRange: обе границы включительно, по дню",
			build: func(f *filterBuilder) {
				f.dateRange("created_at", &from, &to)
			},
			wantSQL: "SELECT 1 WHERE created_at::date >= $1::date" +
				" AND created_at::date <= $2::date",
			wantArgs: []any{from, to},
		},
		{
			name: "dateRange: только нижняя граница",
			build: func(f *filterBuilder) {
				f.dateRange("created_at", &from, nil)
			},
			wantSQL:  "SELECT 1 WHERE created_at::date >= $1::date",
			wantArgs: []any{from},
		},
		{
			name: "dateRange: только верхняя граница — оператор <=",
			build: func(f *filterBuilder) {
				f.dateRange("created_at", nil, &to)
			},
			wantSQL:  "SELECT 1 WHERE created_at::date <= $1::date",
			wantArgs: []any{to},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f filterBuilder
			tt.build(&f)

			if got := f.where("SELECT 1"); got != tt.wantSQL {
				t.Errorf("where() = %q, хотели %q", got, tt.wantSQL)
			}
			if !reflect.DeepEqual(f.args, tt.wantArgs) {
				t.Errorf("args = %v, хотели %v", f.args, tt.wantArgs)
			}
		})
	}
}
