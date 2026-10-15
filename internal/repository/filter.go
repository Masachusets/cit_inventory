package repository

import (
	"fmt"
	"strings"
	"time"
)

// Файл содержит общий сборщик WHERE-условий для list/List-методов
// pgx-репозиториев пакета: единые правила подстановки плейсхолдеров $n,
// ILIKE-поиска по подстроке и диапазонов дат по дню (включительно).

// filterBuilder накапливает условия WHERE и аргументы плейсхолдеров.
// Условия и аргументы добавляются парно, поэтому номер плейсхолдера
// всегда соответствует позиции значения в args.
type filterBuilder struct {
	conds []string
	args  []any
}

// add добавляет условие с плейсхолдером: в format %d подставляется номер
// будущего аргумента, само значение добавляется в args.
func (f *filterBuilder) add(format string, val any) {
	f.args = append(f.args, val)
	f.conds = append(f.conds, fmt.Sprintf(format, len(f.args)))
}

// cond добавляет готовое условие без аргументов (например, "x IS NULL").
func (f *filterBuilder) cond(cond string) {
	f.conds = append(f.conds, cond)
}

// ilike добавляет условие частичного совпадения по подстроке без учёта
// регистра: col ILIKE '%' || $n || '%'; nil-значение фильтра пропускается.
func (f *filterBuilder) ilike(col string, val *string) {
	if val != nil {
		f.add(col+" ILIKE '%%' || $%d || '%%'", *val)
	}
}

// dateRange добавляет диапазон дат по дню: колонка и граница приводятся к
// date, поэтому значения «от» и «до» включаются в выборку целиком.
func (f *filterBuilder) dateRange(col string, from, to *time.Time) {
	if from != nil {
		f.add(col+"::date >= $%d::date", *from)
	}
	if to != nil {
		f.add(col+"::date <= $%d::date", *to)
	}
}

// where дописывает к запросу накопленную WHERE-клаузу; при отсутствии
// условий запрос возвращается без изменений.
func (f *filterBuilder) where(query string) string {
	if len(f.conds) == 0 {
		return query
	}
	return query + " WHERE " + strings.Join(f.conds, " AND ")
}
