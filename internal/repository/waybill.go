package repository

import (
	"context"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// WaybillRepository — контракт хранилища путевых листов. Реализация живёт
// в инфраструктурном слое (pgxpool), домен зависит только от этого интерфейса.
type WaybillRepository interface {
	// Create вставляет новый путевой лист. При существующем номере
	// (UNIQUE) — domain.ErrAlreadyExists.
	Create(ctx context.Context, wb *domain.Waybill) error
	// GetByID возвращает путевой лист по идентификатору либо domain.ErrNotFound.
	GetByID(ctx context.Context, id int) (*domain.Waybill, error)
	// List возвращает путевые листы, отфильтрованные по заданным полям
	// (частичное совпадение строк, диапазон дат), отсортированные по id.
	List(ctx context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error)
	// ExistsByNumber сообщает, есть ли путевой лист с таким номером.
	ExistsByNumber(ctx context.Context, number string) (bool, error)
	// Update обновляет путевой лист по идентификатору.
	// При существующем номере (UNIQUE) — domain.ErrAlreadyExists.
	Update(ctx context.Context, id int, wb *domain.Waybill) error
}
