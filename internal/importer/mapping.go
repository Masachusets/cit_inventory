package importer

import (
	"regexp"
	"strconv"
	"strings"
)

// equipmentStatus — значение enum equipment_status для записи equipments.
type equipmentStatus string

const (
	statusExp    equipmentStatus = "exp"     // эксплуатируемый
	statusExpInt equipmentStatus = "exp_int" // эксплуатируемый (интернет)
	statusExpSP  equipmentStatus = "exp_sp"  // эксплуатируемый (категорированный)
	statusBroken equipmentStatus = "broken"  // неисправно
)

// targetType — значение enum assignment_target для записи equipments_assignments.
type targetType string

const (
	targetEmployee   targetType = "employee"   // закреплено за сотрудником
	targetDepartment targetType = "department" // закреплено за подразделением
	targetWarehouse  targetType = "warehouse"  // на обобщённом складе (без кода)
)

// statusPatterns сопоставляет текст колонки K («Состояние») со значением enum.
// Порядок важен: специфичные префиксы («эксп(инт)», «эксп(сп)») — раньше
// общего «эксп». Сравнение по нижнему регистру, хвостовые «-»/«*» игнорируются.
var statusPatterns = []struct {
	prefix string
	status equipmentStatus
}{
	{"эксп(инт)", statusExpInt},
	{"эксп(сп)", statusExpSP},
	{"эксп", statusExp},
	{"неиспр", statusBroken},
}

// mapStatus переводит текст колонки K в значение enum equipment_status.
// Пустая или нераспознанная строка → статус по умолчанию «эксп» (exp),
// что соответствует DEFAULT 'exp' в схеме БД.
func mapStatus(raw string) equipmentStatus {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return statusExp
	}
	for _, p := range statusPatterns {
		if strings.HasPrefix(s, p.prefix) {
			return p.status
		}
	}
	return statusExp
}

// warehousePattern — обобщённый склад без уточнения: ровно «СКЛАД» (без «/»).
var warehousePattern = regexp.MustCompile(`(?i)^\s*склад\s*$`)

// skladPrefix — склады с уточнением («склад/ЗПУ» и т.п.) маппятся как department.
var skladPrefix = "склад/"

// deptSuffixes — типы подразделений, встречающиеся в колонке M
// (используются для отличения наименования подразделения от ФИО).
var deptSuffixes = map[string]struct{}{
	"опк": {}, "погз": {}, "погк": {}, "мпогз": {},
}

// classifyTarget определяет тип закрепления по колонке M («Закреплены»):
//   - ровно «СКЛАД» (без уточнения) → warehouse;
//   - «склад/…» или строка с типом подразделения (опк/погз/погк/мпогз) → department;
//   - иначе (ФИО, «!», устройство) → employee.
//
// Для department дополнительно возвращается нормализованное наименование
// для поиска кода в справочнике departments.
func classifyTarget(m string) (targetType, string) {
	s := strings.TrimSpace(m)
	if s == "" {
		// Пустая колонка M не встречается в реальных данных (0 строк),
		// но на всякий случай трактуем как «на складе» без привязки.
		return targetWarehouse, ""
	}
	if warehousePattern.MatchString(s) {
		return targetWarehouse, ""
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, skladPrefix) {
		return targetDepartment, lower
	}
	for _, f := range strings.Fields(lower) {
		if _, ok := deptSuffixes[strings.Trim(f, ".,")]; ok {
			return targetDepartment, normalizeDeptName(s)
		}
	}
	return targetEmployee, ""
}

// normalizeDeptName приводит наименование подразделения к единому виду для
// сопоставления со справочником departments. Формат в файле «Название ТИП»
// (напр. «Гомель ОПК»), в справочнике — «тип Название» (напр. «опк Гомель»).
// Приводим к «тип название» в нижнем регистре. Опечатка источника
// («Лоев ПОГК») нейтрализуется тем, что в справочник добавлен код 200
// «погк Лоев», — нормализованные ключи совпадают.
func normalizeDeptName(s string) string {
	fields := strings.Fields(strings.ToLower(s))
	if len(fields) == 0 {
		return ""
	}
	suffix := ""
	for i := len(fields) - 1; i >= 0; i-- {
		tok := strings.Trim(fields[i], ".,")
		if _, ok := deptSuffixes[tok]; ok {
			suffix = tok
			fields = append(fields[:i:i], fields[i+1:]...)
			break
		}
	}
	title := strings.Join(fields, " ")
	if suffix != "" {
		return suffix + " " + title
	}
	return title
}

// cardNumberPattern — номер карточки/накладной в колонке N («51», «51.0»).
var cardNumberPattern = regexp.MustCompile(`^\d+(\.\d+)?$`)

// parseCardRef разбирает номер карточки/накладной из колонки N.
// Возвращает (*n, true) при успехе, (nil, false) — если значение пустое
// или не является целым числом.
func parseCardRef(raw string) (*int, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || !cardNumberPattern.MatchString(s) {
		return nil, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f != float64(int(f)) {
		return nil, false
	}
	v := int(f)
	return &v, true
}

// mmYYYYPattern — месяц.год (разделитель «.» или «/»).
var mmYYYYPattern = regexp.MustCompile(`^(\d{1,2})[./](\d{4})$`)

// parseYearMonth парсит год и месяц из колонок G («Год выпуска»)/H («Дата
// прибытия»), которые могут содержать «2015», «03.2015», «07.2007»,
// «08.2012(03.2015)». При наличии скобок берётся их содержимое (свежее).
// Месяц по умолчанию = 1, если не указан. ok=false для пустых/непарсируемых.
func parseYearMonth(raw string) (year, month int, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, 0, false
	}
	if l := strings.LastIndexByte(s, '('); l >= 0 {
		if r := strings.LastIndexByte(s, ')'); r > l {
			if y, m, ok2 := parseYearMonth(s[l+1 : r]); ok2 {
				return y, m, true
			}
		}
	}
	if m := mmYYYYPattern.FindStringSubmatch(s); m != nil {
		mm, _ := strconv.Atoi(m[1])
		yy, _ := strconv.Atoi(m[2])
		if mm < 1 || mm > 12 {
			mm = 1
		}
		return yy, mm, true
	}
	if y, err := strconv.Atoi(s); err == nil && y >= 1900 && y <= 2200 {
		return y, 1, true
	}
	return 0, 0, false
}
