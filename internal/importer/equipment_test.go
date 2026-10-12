package importer

import (
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// buildEquipmentXlsx создаёт xlsx с листом «Учет» из заданных строк.
func buildEquipmentXlsx(t *testing.T, rows [][]string) *excelize.File {
	t.Helper()
	return buildXlsx(t, map[string][][]string{equipmentSheet: rows}, []string{equipmentSheet})
}

// equipmentHeader — шапка листа «Учет» (20 колонок A..T).
func equipmentHeader() []string {
	return []string{
		"№ п/п", "Инвентарный номер", "MAC", "Заводской номер", "Наименование изделия",
		"Код номенклатуры", "Год выпуска", "Дата прибытия", "№ объекта СВТ",
		"Дата закрытия объекта СВТ", "Состояние", "№ формуляра", "Закреплены",
		"Номер карточки/ накладной", "№ кабинета/место установки", "Модель",
		"Дата ремонта", "Ремонт", "Характер неисправности", "Примечание",
	}
}

// eqRow формирует строку данных листа «Учет» по заданным полям (остальное — пусто).
func eqRow(inv, serial, name, code, manuf, arrival, status, form, assign, cardRef, loc, model, note string) []string {
	r := make([]string, 20)
	set := func(i int, v string) {
		if i < len(r) {
			r[i] = v
		}
	}
	set(colInv, inv)
	set(colSerial, serial)
	set(colName, name)
	set(colNomenCode, code)
	set(colManufDate, manuf)
	set(colArrival, arrival)
	set(colStatus, status)
	set(colForm, form)
	set(colAssign, assign)
	set(colCardRef, cardRef)
	set(colLocation, loc)
	set(colModel, model)
	set(colNote, note)
	return r
}

func TestParseEquipment(t *testing.T) {
	rows := [][]string{
		equipmentHeader(),
		eqRow("ИТ00205", "35602", "Ноутбук Clevo 15\"", "08.01.02.00.00/0041", "", "03.2015", "эксп", "1391", "СКЛАД", "", "СБД", "", "от Свентецкий"),
		eqRow("ИТ00207", "77807", "ПЭВМ s1155", "08.01.01.00.00/0015", "2010", "03.2015", "эксп", "1475", "Веселовка ОПК", "", "КП Ленино", "", ""),
		eqRow("ИТ00211", "ИК123", "ПЭВМ s1155", "08.01.01.00.00/0015", "", "03.2015", "эксп", "1237", "Шандрак А.С.", "51", "203", "Доп рабочее место", ""),
		eqRow("ИТ00217", "31469", "ПЭВМ s1155", "08.01.01.00.00/0015", "", "03.2015", "эксп(инт)-", "1065", "Жоров В.В.", "69", "125", "", ""),
		// строка без инвентарного номера — пропускается
		{"", "", "", "", "Призрак", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""},
		// некорректный инвентарный номер — warning + пропуск
		eqRow("XYZ123", "1", "E", "c", "", "", "эксп", "", "СКЛАД", "", "", "", ""),
	}

	res, err := ParseEquipment(buildEquipmentXlsx(t, rows))
	if err != nil {
		t.Fatalf("ParseEquipment: %v", err)
	}

	if len(res.Equipments) != 4 {
		t.Fatalf("получено %d записей, want 4", len(res.Equipments))
	}
	// Должно быть минимум 1 предупреждение (некорректный инв. номер).
	if len(res.Warnings) == 0 {
		t.Errorf("ожидались предупреждения о некорректном инвентарном номере")
	}

	eq := res.Equipments[0]
	if eq.InventoryNumber != "ИТ00205" {
		t.Errorf("InventoryNumber = %q", eq.InventoryNumber)
	}
	if eq.NomenclatureCode != "08.01.02.00.00/0041" {
		t.Errorf("NomenclatureCode = %q", eq.NomenclatureCode)
	}
	if eq.Status != statusExp {
		t.Errorf("Status = %q, want exp", eq.Status)
	}
	if eq.FormNumber != "1391" {
		t.Errorf("FormNumber = %q, want 1391", eq.FormNumber)
	}
	if eq.AssignTargetM != "СКЛАД" {
		t.Errorf("AssignTargetM = %q", eq.AssignTargetM)
	}
	if eq.ArrivalDate == nil || !eq.ArrivalDate.Equal(time.Date(2015, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ArrivalDate = %v, want 2015-03-01", eq.ArrivalDate)
	}
	// notes должен содержать наименование и примечание.
	if eq.Notes == "" {
		t.Errorf("Notes пустой, ожидалось содержимое")
	}

	// manufacture_date из года «2010».
	eq2 := res.Equipments[1]
	if eq2.ManufactureDate == nil || !eq2.ManufactureDate.Equal(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ManufactureDate = %v, want 2010-01-01", eq2.ManufactureDate)
	}

	// status эксп(инт)- → exp_int.
	eq4 := res.Equipments[3]
	if eq4.Status != statusExpInt {
		t.Errorf("Status = %q, want exp_int", eq4.Status)
	}
	if eq4.AssignCardRefN != "69" {
		t.Errorf("AssignCardRefN = %q, want 69", eq4.AssignCardRefN)
	}
}

func TestParseEquipmentEmpty(t *testing.T) {
	rows := [][]string{equipmentHeader()}
	res, err := ParseEquipment(buildEquipmentXlsx(t, rows))
	if err != nil {
		t.Fatalf("ParseEquipment: %v", err)
	}
	if len(res.Equipments) != 0 {
		t.Errorf("получено %d записей, want 0", len(res.Equipments))
	}
}
