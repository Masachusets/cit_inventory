package importer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Nomenclature — одна строка номенклатурной таблицы: код и наименование.
type Nomenclature struct {
	Code string // номенклатурный номер, напр. 08.01.01.00.00/0001
	Name string // наименование материальных средств
}

// ParseWarning — предупреждение при разборе Excel-файла
// (пропущенные строки, дубликаты и т.п.).
type ParseWarning struct {
	Sheet   string // имя листа
	Row     int    // номер строки (1-based)
	Message string
}

// codePattern описывает номенклатурный номер: непробельные символы,
// завершающиеся «/NNNN». Строки-заголовки и группы («08.01.01 ПЭВМ»,
// «Номенклатурный номер») содержат пробелы и под шаблон не попадают.
var codePattern = regexp.MustCompile(`^\S+/\d{4}$`)

// ParseNomenclature читает все листы Excel-файла и извлекает строки
// номенклатурной таблицы (колонка A — код, колонка B — наименование).
//
// Особенности разбора:
//   - строки-заголовки и группы отбрасываются (код должен быть без пробелов);
//   - строки с пустым наименованием пропускаются с предупреждением;
//   - при повторе кода побеждает последнее вхождение (более поздняя правка).
func ParseNomenclature(f *excelize.File) ([]Nomenclature, []ParseWarning, error) {
	type entry struct {
		rec   Nomenclature
		sheet string
		row   int
		dupN  int // сколько раз встречался код
	}

	var (
		warnings []ParseWarning
		order    []string // порядок первых вхождений кодов
		byCode   = map[string]*entry{}
	)

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, nil, fmt.Errorf("read sheet %q: %w", sheet, err)
		}

		for i, row := range rows {
			if len(row) == 0 {
				continue
			}
			code := strings.TrimSpace(row[0])
			if !codePattern.MatchString(code) {
				continue
			}

			name := ""
			if len(row) > 1 {
				name = normalizeName(row[1])
			}
			if name == "" {
				warnings = append(warnings, ParseWarning{
					Sheet:   sheet,
					Row:     i + 1,
					Message: fmt.Sprintf("код %s: пустое наименование, строка пропущена", code),
				})
				continue
			}

			rec := Nomenclature{Code: code, Name: name}
			if e, ok := byCode[code]; ok {
				e.dupN++
				if e.rec.Name != name {
					warnings = append(warnings, ParseWarning{
						Sheet: sheet,
						Row:   i + 1,
						Message: fmt.Sprintf(
							"код %s: наименование отличается от листа %q (строка %d), оставляем последнее",
							code, e.sheet, e.row),
					})
				}
				e.rec = rec
				e.sheet = sheet
				e.row = i + 1
				continue
			}

			byCode[code] = &entry{rec: rec, sheet: sheet, row: i + 1}
			order = append(order, code)
		}
	}

	records := make([]Nomenclature, 0, len(order))
	for _, code := range order {
		records = append(records, byCode[code].rec)
	}

	totalDups := 0
	for _, e := range byCode {
		totalDups += e.dupN
	}
	if totalDups > 0 {
		warnings = append(warnings, ParseWarning{
			Message: fmt.Sprintf("всего повторов кода в файле: %d (оставлено последнее вхождение)", totalDups),
		})
	}

	return records, warnings, nil
}

// normalizeName приводит наименование к «плоской» форме:
// NBSP и прочие пробельные символы (включая переносы строк) → одиночный пробел.
func normalizeName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
