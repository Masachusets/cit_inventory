package importer

import (
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// buildXlsx создаёт xlsx-файл в памяти из заданных листов:
// имя листа → двумерный массив значений ячеек (колонки A, B, C...).
func buildXlsx(t *testing.T, sheets map[string][][]string, order []string) *excelize.File {
	t.Helper()

	f := excelize.NewFile()
	first := order[0]
	if first != "Sheet1" {
		if err := f.SetSheetName("Sheet1", first); err != nil {
			t.Fatalf("SetSheetName: %v", err)
		}
	}
	for _, sheet := range order {
		if sheet != first {
			if _, err := f.NewSheet(sheet); err != nil {
				t.Fatalf("NewSheet(%q): %v", sheet, err)
			}
		}
		for r, row := range sheets[sheet] {
			for c, val := range row {
				if val == "" {
					continue
				}
				cell, err := excelize.CoordinatesToCellName(c+1, r+1)
				if err != nil {
					t.Fatalf("CoordinatesToCellName: %v", err)
				}
				if err := f.SetCellValue(sheet, cell, val); err != nil {
					t.Fatalf("SetCellValue(%q, %s): %v", sheet, cell, err)
				}
			}
		}
	}
	return f
}

func TestParseNomenclature(t *testing.T) {
	tests := []struct {
		name        string
		sheets      map[string][][]string
		order       []string
		want        []Nomenclature
		wantWarnSub string // подстрока в последнем предупреждении ("" — предупреждений нет)
	}{
		{
			name: "обычный лист: заголовки и группы отбрасываются",
			sheets: map[string][][]string{
				"СВТ": {
					{"Номенклатурный номер", "Наименование материальных средств", "735-ж/3"},
					{"08.01. Электронно-вычислительные машины", ""},
					{"08.01.01 ПЭВМ", ""},
					{"08.01.01.00.00/0001", "ПЭВМ sX"},
					{"08.01.01.00.00/0002", "ПЭВМ Slot A"},
				},
			},
			order: []string{"СВТ"},
			want: []Nomenclature{
				{Code: "08.01.01.00.00/0001", Name: "ПЭВМ sX"},
				{Code: "08.01.01.00.00/0002", Name: "ПЭВМ Slot A"},
			},
		},
		{
			name: "код с префиксом (лист ОООГГ)",
			sheets: map[string][][]string{
				"ОООГГ": {
					{"2/1 - Отдел", ""},
					{"02/1.01.03.00.00/0001", "GPS-навигатор Ritmix RGP 485", "135-ж/66"},
				},
			},
			order: []string{"ОООГГ"},
			want: []Nomenclature{
				{Code: "02/1.01.03.00.00/0001", Name: "GPS-навигатор Ritmix RGP 485"},
			},
		},
		{
			name: "несколько листов склеиваются",
			sheets: map[string][][]string{
				"УП":    {{"41.19.03.00.00/0001", "чернила черные"}},
				"УРиДО": {{"08.01.01.00.00/0001", "ПЭВМ sX"}},
			},
			order: []string{"УП", "УРиДО"},
			want: []Nomenclature{
				{Code: "41.19.03.00.00/0001", Name: "чернила черные"},
				{Code: "08.01.01.00.00/0001", Name: "ПЭВМ sX"},
			},
		},
		{
			name: "дубликат кода: побеждает последнее вхождение",
			sheets: map[string][][]string{
				"СВТ": {
					{"08.10.01.00.00/0120", "Лицензия Microsoft Office 2021"},
					{"08.10.01.00.00/0120", "Лицензия ZyXEL"},
				},
			},
			order: []string{"СВТ"},
			want: []Nomenclature{
				{Code: "08.10.01.00.00/0120", Name: "Лицензия ZyXEL"},
			},
			wantWarnSub: "оставляем последнее",
		},
		{
			name: "пустое наименование пропускается с предупреждением",
			sheets: map[string][][]string{
				"СВТ": {
					{"08.10.03.00.00/0001", ""},
					{"08.10.03.00.00/0002", "Антивирус"},
				},
			},
			order: []string{"СВТ"},
			want: []Nomenclature{
				{Code: "08.10.03.00.00/0002", Name: "Антивирус"},
			},
			wantWarnSub: "пустое наименование",
		},
		{
			name: "NBSP и переносы строк нормализуются",
			sheets: map[string][][]string{
				"СВТ": {
					{"08.10.01.00.00/0118", "Лицензия на ПО «InfoWatch Traffic Monitor Education Lab \nStandart»"},
					{"08.10.01.00.00/0119", "ПАК «Шлюз безопасности»"},
				},
			},
			order: []string{"СВТ"},
			want: []Nomenclature{
				{Code: "08.10.01.00.00/0118", Name: "Лицензия на ПО «InfoWatch Traffic Monitor Education Lab Standart»"},
				{Code: "08.10.01.00.00/0119", Name: "ПАК «Шлюз безопасности»"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := buildXlsx(t, tt.sheets, tt.order)
			defer func() { _ = f.Close() }()

			got, warns, err := ParseNomenclature(f)
			if err != nil {
				t.Fatalf("ParseNomenclature(): %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("records = %d, want %d; got=%v", len(got), len(tt.want), got)
			}
			for i, rec := range got {
				if rec != tt.want[i] {
					t.Errorf("record[%d] = %+v, want %+v", i, rec, tt.want[i])
				}
			}

			if tt.wantWarnSub == "" {
				if len(warns) != 0 {
					t.Errorf("warnings = %v, want none", warns)
				}
				return
			}
			for _, w := range warns {
				if strings.Contains(w.Message, tt.wantWarnSub) {
					return
				}
			}
			t.Errorf("warnings = %v, want any containing %q", warns, tt.wantWarnSub)
		})
	}
}
