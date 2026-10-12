package importer

import (
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// buildCardsXlsx создаёт в памяти xlsx с единственным листом «КУ Ф-111».
func buildCardsXlsx(t *testing.T, rows [][]string) *excelize.File {
	t.Helper()
	return buildXlsx(t, map[string][][]string{cardsSheet: rows}, []string{cardsSheet})
}

func TestStripRanks(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Кузьменко А.В.", "Кузьменко А.В."},
		{"м-р Сидорович С.А.", "Сидорович С.А."},
		{"М-р Сидорович С.А.", "Сидорович С.А."},
		{"п/п-к Бырков С.В.", "Бырков С.В."},
		{"ст. л-т Дехтяр В.А.", "Дехтяр В.А."},
		{"ст.л-т Дехтяр В.А.", "Дехтяр В.А."},
		{"мл. л-т Кирченко С.В.", "Кирченко С.В."},
		{"ст. пр-к Медин С.П.", "Медин С.П."},
		{"ст.с-т Шульга Р.М.", "Шульга Р.М."},
		{"с-на Рыбакова Е.С.", "Рыбакова Е.С."},
		{"ряд. Семученко Д.В.", "Семученко Д.В."},
		{"раб. Пугач Е.В.", "Пугач Е.В."},
		{"сл. Буракова Е.С.", "Буракова Е.С."},
		// фамилия через дефис — не звание, не трогаем
		{"Трухнова-Дроздова И.С.", "Трухнова-Дроздова И.С."},
	}
	for _, tt := range tests {
		if got := stripRanks(tt.in); got != tt.want {
			t.Errorf("stripRanks(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseExcelDate(t *testing.T) {
	tests := []struct {
		in     string
		want   time.Time
		wantOK bool
	}{
		{in: "42080", want: time.Date(2015, 3, 17, 0, 0, 0, 0, time.UTC), wantOK: true},
		{in: "42080.0", want: time.Date(2015, 3, 17, 0, 0, 0, 0, time.UTC), wantOK: true},
		{in: "46087", want: time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC), wantOK: true},
		{in: "", wantOK: false},
		{in: "abc", wantOK: false},
		{in: "-5", wantOK: false},
		{in: "999999999", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := parseExcelDate(tt.in)
		if ok != tt.wantOK {
			t.Errorf("parseExcelDate(%q) ok = %v, want %v", tt.in, ok, tt.wantOK)
			continue
		}
		if ok && !got.Equal(tt.want) {
			t.Errorf("parseExcelDate(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseCards(t *testing.T) {
	header := []string{"№ п/п", "Регистрационный №", "Дата записи", "ФИО", "Отдел", "Примечание"}

	tests := []struct {
		name            string
		rows            [][]string
		want            []Card
		wantWrittenOff  int
		wantWarnContain string // подстрока одного из предупреждений ("" — не проверяем)
	}{
		{
			name: "обычные строки: звания сняты, даты распознаны",
			rows: [][]string{
				header,
				{"1", "1", "42080", "Кузьменко А.В.", "", ""},
				{"2", "2", "42080", "м-р Сидорович С.А.", "", ""},
				{"3", "3", "42089", "ст. л-т Дехтяр В.А.", "", ""},
				{"4", "4", "42089", "Трухнова-Дроздова И.С.", "", ""},
			},
			want: []Card{
				{Number: 1, FullName: "Кузьменко А.В.", Date: time.Date(2015, 3, 17, 0, 0, 0, 0, time.UTC)},
				{Number: 2, FullName: "Сидорович С.А.", Date: time.Date(2015, 3, 17, 0, 0, 0, 0, time.UTC)},
				{Number: 3, FullName: "Дехтяр В.А.", Date: time.Date(2015, 3, 26, 0, 0, 0, 0, time.UTC)},
				{Number: 4, FullName: "Трухнова-Дроздова И.С.", Date: time.Date(2015, 3, 26, 0, 0, 0, 0, time.UTC)},
			},
		},
		{
			name: "СПИСАНА пропускается без предупреждений (разный регистр)",
			rows: [][]string{
				header,
				{"1", "1", "42080", "Кузьменко А.В.", "", "СПИСАНА"},
				{"2", "2", "42080", "Траньков А.В.", "", "списана"},
				{"3", "3", "42080", "Винников Д.А.", "", ""},
			},
			want: []Card{
				{Number: 3, FullName: "Винников Д.А.", Date: time.Date(2015, 3, 17, 0, 0, 0, 0, time.UTC)},
			},
			wantWrittenOff: 2,
		},
		{
			name:            "пустое ФИО — пропуск с предупреждением",
			rows:            [][]string{header, {"1", "1", "42080", "", "", ""}},
			wantWarnContain: "пустое ФИО",
		},
		{
			name:            "пустая дата — пропуск с предупреждением",
			rows:            [][]string{header, {"1", "1", "", "Кузьменко А.В.", "", ""}},
			wantWarnContain: "дата",
		},
		{
			name: "повтор номера: last-wins + предупреждение",
			rows: [][]string{
				header,
				{"1", "5", "42080", "Кузьменко А.В.", "", ""},
				{"2", "5", "42089", "Траньков А.В.", "", ""},
			},
			want: []Card{
				{Number: 5, FullName: "Траньков А.В.", Date: time.Date(2015, 3, 26, 0, 0, 0, 0, time.UTC)},
			},
			wantWarnContain: "повторяется",
		},
		{
			name: "шапка и пустые строки в хвосте игнорируются",
			rows: [][]string{
				header,
				{"1", "1", "42080", "Кузьменко А.В.", "", ""},
				{"", "", "", "", "", ""},
				{"", "", "", "", "", ""},
			},
			want: []Card{
				{Number: 1, FullName: "Кузьменко А.В.", Date: time.Date(2015, 3, 17, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := buildCardsXlsx(t, tt.rows)
			defer func() { _ = f.Close() }()

			res, err := ParseCards(f)
			if err != nil {
				t.Fatalf("ParseCards(): %v", err)
			}

			if len(res.Cards) != len(tt.want) {
				t.Fatalf("cards = %d, want %d; got=%+v", len(res.Cards), len(tt.want), res.Cards)
			}
			for i, c := range res.Cards {
				if c != tt.want[i] {
					t.Errorf("card[%d] = %+v, want %+v", i, c, tt.want[i])
				}
			}

			if res.WrittenOff != tt.wantWrittenOff {
				t.Errorf("writtenOff = %d, want %d", res.WrittenOff, tt.wantWrittenOff)
			}

			if tt.wantWarnContain == "" {
				if len(res.Warnings) != 0 {
					t.Errorf("warnings = %v, want none", res.Warnings)
				}
				return
			}
			found := false
			for _, w := range res.Warnings {
				if strings.Contains(w.Message, tt.wantWarnContain) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("warnings = %v, want any containing %q", res.Warnings, tt.wantWarnContain)
			}
		})
	}
}
