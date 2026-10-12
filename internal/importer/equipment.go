package importer

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// equipmentSheet — имя листа с учётом оборудования в UMS.xlsx.
const equipmentSheet = "Учет"

// inventoryPattern — инвентарный номер вида ИТ00205 (^ИТ\d{5}$).
var inventoryPattern = regexp.MustCompile(`^ИТ\d{5}$`)

// Equipment — одна строка листа «Учет» для таблицы equipments.
// Поле NomenclatureCode и сырые данные закрепления (Assign*) резолвятся
// в store при наличии доступа к справочникам (nomenclatures/departments/cards).
type Equipment struct {
	InventoryNumber  string
	SerialNumber     string
	NomenclatureCode string     // код из колонки F (для поиска nomenclature_id)
	ModelName        string     // колонка P (Модель)
	ManufactureDate  *time.Time // колонка G (Год выпуска)
	ArrivalDate      *time.Time // колонка H (Дата прибытия)
	Status           equipmentStatus
	FormNumber       string // колонка L (№ формуляра)
	Location         string // колонка O (№ кабинета/место установки)
	Notes            string // сборное примечание (E, C, I, J, Q, R, S, T)

	// Сырые данные закрепления из колонок M/N — резолвятся в store.
	AssignTargetM  string // «Закреплены» (ФИО / подразделение / СКЛАД)
	AssignCardRefN string // «Номер карточки/накладной»
}

// column indexes (0-based) in sheet «Учет».
const (
	colNo        = 0  // A: № п/п
	colInv       = 1  // B: Инвентарный номер
	colMAC       = 2  // C: MAC
	colSerial    = 3  // D: Заводской номер
	colName      = 4  // E: Наименование изделия
	colNomenCode = 5  // F: Код номенклатуры
	colManufDate = 6  // G: Год выпуска
	colArrival   = 7  // H: Дата прибытия
	colSVTNo     = 8  // I: № объекта СВТ
	colSVTClose  = 9  // J: Дата закрытия объекта СВТ
	colStatus    = 10 // K: Состояние
	colForm      = 11 // L: № формуляра
	colAssign    = 12 // M: Закреплены
	colCardRef   = 13 // N: Номер карточки/накладной
	colLocation  = 14 // O: № кабинета/место установки
	colModel     = 15 // P: Модель
	colRepairDt  = 16 // Q: Дата ремонта
	colRepair    = 17 // R: Ремонт
	colDefect    = 18 // S: Характер неисправности
	colNote      = 19 // T: Примечание
)

// cell безопасно возвращает строку ячейки по индексу (или "" при выходе за границы).
func cell(row []string, i int) string {
	if i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

// EquipmentParseResult — итог разбора листа «Учет».
type EquipmentParseResult struct {
	Equipments []Equipment
	Warnings   []ParseWarning
}

// ParseEquipment читает лист «Учет» UMS.xlsx и извлекает оборудование.
//
// Строка распознаётся как оборудование, если инвентарный номер (колонка B)
// соответствует формату ИТ\d{5}. Trailing-пустые строки (в файле сотни в
// хвосте) отбрасываются. Пустые/служебные колонки (C, I, J, Q, R, S),
// не имеющие полей в схеме equipments, консервируются в notes с префиксом.
func ParseEquipment(f *excelize.File) (EquipmentParseResult, error) {
	var res EquipmentParseResult

	// RawCellValue: серийные даты читаются как числа (для колонок с датами).
	rows, err := f.GetRows(equipmentSheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return res, fmt.Errorf("read sheet %q: %w", equipmentSheet, err)
	}

	idxByInv := map[string]int{} // инвентарный номер → индекс в res.Equipments
	for i, row := range rows {
		line := i + 1

		inv := cell(row, colInv)
		if inv == "" {
			continue // пустая строка или строка без оборудования
		}
		if !inventoryPattern.MatchString(inv) {
			res.Warnings = append(res.Warnings, ParseWarning{
				Sheet:   equipmentSheet,
				Row:     line,
				Message: fmt.Sprintf("инвентарный номер %q не формата ИТ#####, строка пропущена", inv),
			})
			continue
		}

		eq := Equipment{
			InventoryNumber:  inv,
			SerialNumber:     cell(row, colSerial),
			NomenclatureCode: cell(row, colNomenCode),
			ModelName:        cell(row, colModel),
			Status:           mapStatus(cell(row, colStatus)),
			FormNumber:       parseFormNumber(cell(row, colForm)),
			Location:         cell(row, colLocation),
			Notes:            buildNotes(row),
			AssignTargetM:    cell(row, colAssign),
			AssignCardRefN:   cell(row, colCardRef),
		}
		if y, m, ok := parseYearMonth(cell(row, colManufDate)); ok {
			eq.ManufactureDate = timePtr(time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC))
		}
		if y, m, ok := parseYearMonth(cell(row, colArrival)); ok {
			eq.ArrivalDate = timePtr(time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC))
		}

		if j, exists := idxByInv[inv]; exists {
			res.Warnings = append(res.Warnings, ParseWarning{
				Sheet:   equipmentSheet,
				Row:     line,
				Message: fmt.Sprintf("инвентарный номер %s повторяется (строка %d), оставляем последнее", inv, j+1),
			})
			res.Equipments[j] = eq
			continue
		}
		idxByInv[inv] = len(res.Equipments)
		res.Equipments = append(res.Equipments, eq)
	}

	return res, nil
}

// parseFormNumber приводит «№ формуляра» к строке: «1391.0» → «1391».
// Нечисловые значения возвращаются как есть (обрезанные пробелы).
func parseFormNumber(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if n, ok := parseCardRef(s); ok {
		return fmt.Sprintf("%d", *n)
	}
	return s
}

// buildNotes собирает примечание из колонок без выделенных полей в схеме
// (E «Наименование изделия», C MAC, I/J объект СВТ, Q/R/S ремонт и
// неисправность) плюс собственно T «Примечание». Пустые пропускаются.
func buildNotes(row []string) string {
	var parts []string
	add := func(prefix, val string) {
		if v := strings.TrimSpace(val); v != "" {
			parts = append(parts, prefix+v)
		}
	}
	add("Наименование: ", cell(row, colName))
	add("MAC: ", cell(row, colMAC))
	add("№ объекта СВТ: ", cell(row, colSVTNo))
	add("Закрытие объекта СВТ: ", cell(row, colSVTClose))
	add("Дата ремонта: ", cell(row, colRepairDt))
	add("Ремонт: ", cell(row, colRepair))
	add("Характер неисправности: ", cell(row, colDefect))
	add("Примечание: ", cell(row, colNote))
	return strings.Join(parts, "; ")
}

// timePtr возвращает указатель на время (nil-безопасный хелпер).
func timePtr(t time.Time) *time.Time { return &t }
