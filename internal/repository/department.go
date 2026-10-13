package repository

import (
	"context"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// DepartmentRepository — контракт хранилища справочника подразделений.
// Реализация живёт в инфраструктурном слое (pgxpool), домен зависит только
// от этого интерфейса.
type DepartmentRepository interface {
	// Create вставляет новое подразделение. При существующем коде или
	// наименовании (UNIQUE) — domain.ErrAlreadyExists.
	Create(ctx context.Context, dept *domain.Department) error
	// GetByCode возвращает подразделение по коду в любом статусе
	// либо domain.ErrNotFound.
	GetByCode(ctx context.Context, code int16) (*domain.Department, error)
	// ListActive возвращает только активные подразделения с учётом фильтра.
	ListActive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error)
	// ListInactive возвращает только неактивные подразделения с учётом фильтра.
	ListInactive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error)
	// Exists сообщает, есть ли подразделение с таким кодом.
	Exists(ctx context.Context, code int16) (bool, error)
	// UpdateName обновляет наименование подразделения.
	UpdateName(ctx context.Context, code int16, name string) error
	// Deactivate помечает подразделение неактивным (is_active=false).
	Deactivate(ctx context.Context, code int16) error
}
