package importer

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// cardsSheet — имя листа с карточками учёта в UMS.xlsx.
const cardsSheet = "КУ Ф-111"

// excelEpoch — эпоха датирования Excel (серийная дата 0 = 1899-12-30).
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// Card — карточка учёта (строка листа «КУ Ф-111») для таблицы cards.
type Card struct {
	Number   int       // Регистрационный № (колонка B, равен № п/п)
	FullName string    // ФИО без званий/чинов (колонка D)
	Date     time.Time // Дата записи (колонка C) → created_at
}

// CardsParseResult — итог разбора листа «КУ Ф-111».
type CardsParseResult struct {
	Cards      []Card
	Warnings   []ParseWarning
	WrittenOff int // пропущено строк с пометкой «СПИСАНА»
}

// rankTokens — звания/чины и служебные сокращения, стоящие перед ФИО.
// Токены сравниваются в нижнем регистре; составные звания («ст. л-т»,
// «мл. л-т») снимаются послементно, поэтому здесь и «ст.», и «мл.».
var rankTokens = map[string]struct{}{
	"пр-к": {}, "ст.пр-к": {}, "л-т": {}, "ст.л-т": {}, "мл.л-т": {},
	"к-н": {}, "м-р": {}, "п/п-к": {}, "п-к": {},
	"с-на": {}, "с-т": {}, "ст.с-т": {}, "сл.": {}, "ряд.": {}, "раб.": {},
	"ст.": {}, "мл.": {},
}

// stripRanks убирает из начала строки известные звания/чины,
// оставляя только ФИО. Фамилии с дефисами не затрагиваются.
func stripRanks(s string) string {
	fields := strings.Fields(s)
	i := 0
	for i < len(fields) {
		if _, ok := rankTokens[strings.ToLower(fields[i])]; !ok {
			break
		}
		i++
	}
	return strings.Join(fields[i:], " ")
}

// parseExcelDate преобразует серийную дату Excel в time.Time (полночь UTC).
func parseExcelDate(raw string) (time.Time, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return time.Time{}, false
	}
	serial := int(v)
	if serial <= 0 || serial > 200000 {
		return time.Time{}, false
	}
	return excelEpoch.AddDate(0, 0, serial), true
}

// ParseCards читает лист «КУ Ф-111» и извлекает карточки учёта.
//
// Строки пропускаются (с предупреждением), если:
//   - примечание содержит «СПИСАНА» (считается отдельно, без предупреждений);
//   - ФИО пустое даже после снятия званий;
//   - дата записи пустая или не парсится.
//
// Повтор регистрационного номера — last-wins + предупреждение.
func ParseCards(f *excelize.File) (CardsParseResult, error) {
	var res CardsParseResult

	// RawCellValue: даты читаются как серийные числа, а не как
	// отформатированные строки («3-9-15»), иначе парсинг дат ненадёжен.
	rows, err := f.GetRows(cardsSheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return res, fmt.Errorf("read sheet %q: %w", cardsSheet, err)
	}

	idxByNumber := map[int]int{} // number → индекс в res.Cards
	for i, row := range rows {
		line := i + 1
		cells := make([]string, 6)
		copy(cells, row)

		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}
		if cells[0] == "№ п/п" {
			continue // шапка таблицы
		}
		if cells[0] == "" && cells[1] == "" && cells[2] == "" &&
			cells[3] == "" && cells[4] == "" && cells[5] == "" {
			continue // пустая строка (в файле их много в хвосте)
		}

		number := parseCardNumber(cells[1])
		if number == nil {
			res.Warnings = append(res.Warnings, ParseWarning{
				Sheet:   cardsSheet,
				Row:     line,
				Message: fmt.Sprintf("не удалось прочитать регистрационный № %q", cells[1]),
			})
			continue
		}

		if strings.Contains(strings.ToUpper(cells[5]), "СПИСАНА") {
			res.WrittenOff++
			continue
		}

		fullName := stripRanks(normalizeName(cells[3]))
		if fullName == "" {
			res.Warnings = append(res.Warnings, ParseWarning{
				Sheet:   cardsSheet,
				Row:     line,
				Message: fmt.Sprintf("карточка №%d: пустое ФИО, строка пропущена", *number),
			})
			continue
		}

		date, ok := parseExcelDate(cells[2])
		if !ok {
			res.Warnings = append(res.Warnings, ParseWarning{
				Sheet:   cardsSheet,
				Row:     line,
				Message: fmt.Sprintf("карточка №%d: пустая или непарсируемая дата %q, строка пропущена", *number, cells[2]),
			})
			continue
		}

		card := Card{Number: *number, FullName: fullName, Date: date}
		if j, exists := idxByNumber[card.Number]; exists {
			res.Warnings = append(res.Warnings, ParseWarning{
				Sheet: cardsSheet,
				Row:   line,
				Message: fmt.Sprintf(
					"регистрационный №%d повторяется (строка %d), оставляем последнее",
					card.Number, j+1),
			})
			res.Cards[j] = card
			continue
		}
		idxByNumber[card.Number] = len(res.Cards)
		res.Cards = append(res.Cards, card)
	}

	return res, nil
}

// parseCardNumber разбирает регистрационный номер («42», «42.0» → 42).
func parseCardNumber(raw string) *int {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || v != float64(int(v)) {
		return nil
	}
	n := int(v)
	return &n
}
