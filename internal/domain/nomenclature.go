// Package domain описывает доменные модели. Nomenclature — запись справочника
// номенклатуры. Справочник пополняется только импортом из Excel, поэтому
// конструктор, валидаторы и методы изменения не нужны — только модель и фильтр.
package domain

import "errors"

// ErrInvalidID — некорректный идентификатор (не положительное целое).
var ErrInvalidID = errors.New("invalid id")

// Nomenclature — запись справочника номенклатуры (таблица nomenclatures).
type Nomenclature struct {
	ID   int    // идентификатор (PK, identity)
	Code string // код номенклатуры (UNIQUE, например 08.01.02.00.00/0041)
	Name string // наименование (например «Ноутбук Clevo 15»)
}

// NomenclatureFilter — параметры фильтрации списка номенклатур.
// nil-поле означает «не фильтровать по полю»; непустые строки матчингуются
// частично без учёта регистра (ILIKE '%...%').
type NomenclatureFilter struct {
	Code *string // подстрока кода
	Name *string // подстрока наименования
}
