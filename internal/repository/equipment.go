package repository

import (
	"context"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// EquipmentRepository — контракт хранилища оборудования. Реализация живёт в
// инфраструктурном слое (pgxpool), домен зависит только от этого интерфейса.
type EquipmentRepository interface {
	// Create вставляет новое оборудование и заполняет id/метки времени
	// возвращёнными значениями БД.
	Create(ctx context.Context, eq *domain.Equipment) error
	// GetByID возвращает оборудование по id в любом статусе (активное и
	// удалённое) либо domain.ErrNotFound. При id <= 0 — domain.ErrInvalidID.
	GetByID(ctx context.Context, id int) (*domain.Equipment, error)
	// GetByInventoryNumber возвращает оборудование по инвентарному номеру
	// в любом статусе либо domain.ErrNotFound. Некорректный формат —
	// domain.ErrInvalidInventoryNumber (проверяется до запроса к БД).
	GetByInventoryNumber(ctx context.Context, number string) (*domain.Equipment, error)
	// ListActive возвращает только активные (deleted_at IS NULL) записи
	// с учётом фильтра, отсортированные по id.
	ListActive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error)
	// ListInactive возвращает только удалённые (deleted_at IS NOT NULL)
	// записи с учётом фильтра, отсортированные по id.
	ListInactive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error)
	// ExistsInventoryNumber сообщает, есть ли оборудование с таким номером
	// (в любом статусе — UNIQUE действует и для удалённых записей).
	ExistsInventoryNumber(ctx context.Context, number string) (bool, error)
	// Update обновляет атрибуты оборудования по id (inventory_number,
	// created_at и deleted_at не меняются). Отсутствующая запись —
	// domain.ErrNotFound; несуществующая номенклатура —
	// domain.ErrInvalidNomenclature.
	Update(ctx context.Context, id int, eq *domain.Equipment) error
	// SoftDelete заполняет deleted_at (повторно не перезаписывает —
	// COALESCE сохраняет исходную метку). Отсутствующая запись —
	// domain.ErrNotFound.
	SoftDelete(ctx context.Context, id int) error
}
