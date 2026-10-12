package importer

import "testing"

func TestDiff(t *testing.T) {
	tests := []struct {
		name        string
		existing    map[string]existingRow
		records     []Nomenclature
		wantInserts []string
		wantUpdates []string
		wantDeletes []string
	}{
		{
			name:        "пустая БД: всё вставляется",
			existing:    map[string]existingRow{},
			records:     []Nomenclature{{Code: "08.01.01.00.00/0001", Name: "ПЭВМ"}},
			wantInserts: []string{"08.01.01.00.00/0001"},
		},
		{
			name: "нет изменений: ничего не делаем",
			existing: map[string]existingRow{
				"08.01.01.00.00/0001": {ID: 1, Name: "ПЭВМ"},
			},
			records: []Nomenclature{{Code: "08.01.01.00.00/0001", Name: "ПЭВМ"}},
		},
		{
			name: "изменилось наименование: update",
			existing: map[string]existingRow{
				"08.01.01.00.00/0001": {ID: 1, Name: "ПЭВМ sX"},
			},
			records:     []Nomenclature{{Code: "08.01.01.00.00/0001", Name: "ПЭВМ Slot A"}},
			wantUpdates: []string{"08.01.01.00.00/0001"},
		},
		{
			name: "код исчез из файла: delete",
			existing: map[string]existingRow{
				"08.01.01.00.00/0001": {ID: 1, Name: "ПЭВМ"},
				"08.01.02.00.00/0001": {ID: 2, Name: "Ноутбук"},
			},
			records:     []Nomenclature{{Code: "08.01.01.00.00/0001", Name: "ПЭВМ"}},
			wantDeletes: []string{"08.01.02.00.00/0001"},
		},
		{
			name: "смешанный сценарий",
			existing: map[string]existingRow{
				"08.01.01.00.00/0001": {ID: 1, Name: "ПЭВМ sX"}, // переименование
				"08.01.02.00.00/0001": {ID: 2, Name: "Ноутбук"}, // без изменений
				"08.01.03.00.00/0001": {ID: 3, Name: "Планшет"}, // исчез
			},
			records: []Nomenclature{
				{Code: "08.01.01.00.00/0001", Name: "ПЭВМ Slot A"}, // обновится
				{Code: "08.01.02.00.00/0001", Name: "Ноутбук"},     // ок
				{Code: "08.10.01.00.00/0120", Name: "Лицензия"},    // новая
			},
			wantInserts: []string{"08.10.01.00.00/0120"},
			wantUpdates: []string{"08.01.01.00.00/0001"},
			wantDeletes: []string{"08.01.03.00.00/0001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inserts, updates, deletes := diff(tt.existing, tt.records)

			assertCodes(t, "inserts", inserts, tt.wantInserts)
			assertCodes(t, "updates", updates, tt.wantUpdates)
			assertCodes(t, "deletes", deletes, tt.wantDeletes)
		})
	}
}

// assertCodes сравнивает два множества кодов (порядок не важен).
func assertCodes(t *testing.T, label string, got, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", label, got, want)
		return
	}
	set := make(map[string]struct{}, len(got))
	for _, c := range got {
		set[c] = struct{}{}
	}
	for _, c := range want {
		if _, ok := set[c]; !ok {
			t.Errorf("%s = %v, want %v (нет %q)", label, got, want, c)
			return
		}
	}
}
