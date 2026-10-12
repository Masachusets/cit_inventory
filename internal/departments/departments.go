package departments

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// maxNameLen — лимит колонки departments.name (VARCHAR(150)).
	maxNameLen = 150
	// maxCode — верхняя граница SMALLINT для departments.code.
	maxCode = 32767
)

// Department — строка справочника подразделений.
type Department struct {
	Code int16  // код подразделения (100, 200, 500…)
	Name string // наименование (опк Гомель, склад/ЗПУ)
}

// Parse разбирает строки текстового файла формата «<код> <наименование>»:
// первое слово — код, остаток строки — наименование.
// Всё после «#» — комментарий; пустые строки игнорируются.
// Возвращает записи, отсортированные по коду.
func Parse(lines []string) ([]Department, error) {
	var (
		depts    []Department
		seenCode = map[int16]int{}    // код → номер строки первого вхождения
		seenName = map[string]int16{} // наименование → код
	)

	for i, line := range lines {
		lineNo := i + 1

		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		codeToken := line
		rest := ""
		if idx := strings.IndexFunc(line, unicode.IsSpace); idx >= 0 {
			codeToken = line[:idx]
			rest = strings.TrimSpace(line[idx:])
		}
		code, err := strconv.Atoi(codeToken)
		if err != nil || code < 1 || code > maxCode {
			return nil, fmt.Errorf(
				"строка %d: код %q не является числом 1..%d", lineNo, codeToken, maxCode)
		}

		name := rest
		if name == "" {
			return nil, fmt.Errorf("строка %d: пустое наименование (код %d)", lineNo, code)
		}
		if utf8.RuneCountInString(name) > maxNameLen {
			return nil, fmt.Errorf(
				"строка %d: наименование «%s» длиннее %d символов", lineNo, name, maxNameLen)
		}
		if firstLine, dup := seenCode[int16(code)]; dup {
			return nil, fmt.Errorf(
				"строка %d: код %d уже задан в строке %d", lineNo, code, firstLine)
		}
		if firstCode, dup := seenName[name]; dup {
			return nil, fmt.Errorf(
				"строка %d: наименование «%s» уже используется с кодом %d (departments.name должен быть уникален)",
				lineNo, name, firstCode)
		}
		seenCode[int16(code)] = lineNo
		seenName[name] = int16(code)

		depts = append(depts, Department{Code: int16(code), Name: name})
	}

	if len(depts) == 0 {
		return nil, fmt.Errorf("не найдено ни одной записи (формат строки: «<код> <наименование>»)")
	}

	sort.Slice(depts, func(i, j int) bool { return depts[i].Code < depts[j].Code })
	return depts, nil
}
