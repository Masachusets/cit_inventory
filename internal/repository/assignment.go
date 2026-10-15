package repository

import (
	"context"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// AssignmentRepository — контракт хранилища закреплений оборудования
// (таблица equipments_assignments). Реализация живёт в инфраструктурном слое
// (pgxpool), домен зависит только от этого интерфейса.
type AssignmentRepository interface {
	// Create вставляет новое закрепление и заполняет id возвращённым
	// значением БД. Несуществующие связанные сущности —
	// domain.ErrInvalidReference; нарушение chk_target — domain.ErrInvalidTarget.
	Create(ctx context.Context, a *domain.Assignment) error
	// GetByID возвращает закрепление вместе со связанными сущностями
	// (equipment/waybill/card/department) либо domain.ErrNotFound.
	// При id <= 0 — domain.ErrInvalidID.
	GetByID(ctx context.Context, id int) (*domain.Assignment, error)
	// List возвращает закрепления с фильтрацией по всем поддерживаемым полям,
	// отсортированные по id.
	List(ctx context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error)
	// Update обновляет поля закрепления по id (equipment_id не меняется).
	// Отсутствующая запись — domain.ErrNotFound; битая ссылка —
	// domain.ErrInvalidReference; нарушение chk_target — domain.ErrInvalidTarget.
	Update(ctx context.Context, id int, a *domain.Assignment) error
	// Delete физически удаляет запись истории (таблица без deleted_at).
	// Отсутствующая запись — domain.ErrNotFound.
	Delete(ctx context.Context, id int) error
}
