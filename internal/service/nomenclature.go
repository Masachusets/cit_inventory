package service

import (
	"context"
	"fmt"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
)

// NomenclatureService — бизнес-логика чтения справочника номенклатуры.
// Изменение справочника выполняется импортом из Excel, не через API.
type NomenclatureService interface {
	// Get возвращает номенклатуру по id либо domain.ErrNotFound.
	// При id <= 0 — domain.ErrInvalidID.
	Get(ctx context.Context, id int) (*domain.Nomenclature, error)
	// List возвращает номенклатуры, отфильтрованные по заданным полям
	// (частичное совпадение без учёта регистра), отсортированные по id.
	List(ctx context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error)
}

// NomenclatureServiceImpl — реализация NomenclatureService поверх NomenclatureRepository.
type NomenclatureServiceImpl struct {
	repo repository.NomenclatureRepository
}

// NewNomenclatureService создаёт сервис номенклатуры с переданным репозиторием.
func NewNomenclatureService(repo repository.NomenclatureRepository) *NomenclatureServiceImpl {
	return &NomenclatureServiceImpl{repo: repo}
}

// Compile-time проверка реализации интерфейса.
var _ NomenclatureService = (*NomenclatureServiceImpl)(nil)

func (s *NomenclatureServiceImpl) Get(ctx context.Context, id int) (*domain.Nomenclature, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	return s.repo.GetByID(ctx, id)
}

func (s *NomenclatureServiceImpl) List(ctx context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error) {
	return s.repo.List(ctx, filter)
}
