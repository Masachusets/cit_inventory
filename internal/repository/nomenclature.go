package repository

import (
	"context"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// NomenclatureRepository — контракт хранилища справочника номенклатуры.
// Реализация живёт в инфраструктурном слое (pgxpool), домен зависит только
// от этого интерфейса.
type NomenclatureRepository interface {
	// GetByID возвращает номенклатуру по идентификатору либо domain.ErrNotFound.
	GetByID(ctx context.Context, id int) (*domain.Nomenclature, error)
	// List возвращает номенклатуры, отфильтрованные по заданным полям
	// (частичное совпадение без учёта регистра), отсортированные по id.
	List(ctx context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error)
}
