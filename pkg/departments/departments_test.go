package departments

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		lines   []string
		want    []Department
		wantErr string // подстрока ошибки ("" — ошибки нет)
	}{
		{
			name:  "базовый формат: код первым словом, наименование с пробелами",
			lines: []string{"500 склад", "510 склад/ЗПУ"},
			want: []Department{
				{Code: 500, Name: "склад"},
				{Code: 510, Name: "склад/ЗПУ"},
			},
		},
		{
			name:  "комментарии и пустые строки игнорируются, сортировка по коду",
			lines: []string{"# шапка", "", "540 склад/Селище", "  ", "100 опк Гомель # inline-комментарий", "\t"},
			want: []Department{
				{Code: 100, Name: "опк Гомель"},
				{Code: 540, Name: "склад/Селище"},
			},
		},
		{
			name:  "табуляция как разделитель",
			lines: []string{"260\tпогз Лоев"},
			want:  []Department{{Code: 260, Name: "погз Лоев"}},
		},
		{
			name:    "нечисловой код",
			lines:   []string{"склад"},
			wantErr: "не является числом",
		},
		{
			name:    "код вне диапазона SMALLINT",
			lines:   []string{"40000 склад"},
			wantErr: "не является числом",
		},
		{
			name:    "ноль — невалидный код",
			lines:   []string{"0 склад"},
			wantErr: "не является числом",
		},
		{
			name:    "пустое наименование",
			lines:   []string{"500"},
			wantErr: "пустое наименование",
		},
		{
			name:    "слишком длинное наименование",
			lines:   []string{"500 " + strings.Repeat("а", 151)},
			wantErr: "длиннее",
		},
		{
			name:    "дубликат кода",
			lines:   []string{"500 склад", "500 склад/ЗПУ"},
			wantErr: "уже задан в строке",
		},
		{
			name:    "дубликат наименования (UNIQUE)",
			lines:   []string{"500 склад", "501 склад"},
			wantErr: "уже используется с кодом",
		},
		{
			name:    "нет записей",
			lines:   []string{"# только комментарии", ""},
			wantErr: "не найдено ни одной записи",
		},
		{
			name:  "граница: наименование ровно 150 символов",
			lines: []string{"500 " + strings.Repeat("а", 150)},
			want:  []Department{{Code: 500, Name: strings.Repeat("а", 150)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.lines)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Parse() error = nil, want containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Parse(): %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("depts = %d, want %d; got=%+v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("dept[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParse_RealFileShape(t *testing.T) {
	// Воспроизводит структуру data/departments.txt: группы с комментариями,
	// составные наименования, дефисы, сортировка по коду.
	data := `# Справочник подразделений
# Формат: <код> <наименование>

# --- Склады ---
500 склад
510 склад/ЗПУ

# --- ОПК ---
100 опк Гомель
130 опк Новая Гута

# --- Погз ---
310 погз Круговец-Калинино
400 мпогз Лоев
`

	got, err := Parse(strings.Split(data, "\n"))
	if err != nil {
		t.Fatalf("Parse(): %v", err)
	}

	want := []Department{
		{Code: 100, Name: "опк Гомель"},
		{Code: 130, Name: "опк Новая Гута"},
		{Code: 310, Name: "погз Круговец-Калинино"},
		{Code: 400, Name: "мпогз Лоев"},
		{Code: 500, Name: "склад"},
		{Code: 510, Name: "склад/ЗПУ"},
	}
	if len(got) != len(want) {
		t.Fatalf("depts = %d, want %d; got=%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dept[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
