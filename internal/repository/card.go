package repository

import (
	"context"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// CardRepository — контракт хранилища карточек учёта. Реализация живёт в
// инфраструктурном слое (pgxpool), домен зависит только от этого интерфейса.
type CardRepository interface {
	// Create вставляет новую карточку.
	Create(ctx context.Context, card *domain.Card) error
	// GetByNumber возвращает карточку по номеру в любом статусе
	// (активную и неактивную) либо domain.ErrNotFound.
	GetByNumber(ctx context.Context, number int) (*domain.Card, error)
	// ListActive возвращает только активные карточки.
	ListActive(ctx context.Context) ([]*domain.Card, error)
	// ListInactive возвращает только неактивные карточки.
	ListInactive(ctx context.Context) ([]*domain.Card, error)
	// Exists сообщает, есть ли карточка с таким номером (в любом статусе).
	Exists(ctx context.Context, number int) (bool, error)
	// UpdateFullName обновляет ФИО и метку updated_at.
	UpdateFullName(ctx context.Context, number int, fullName string) error
	// Deactivate помечает карточку неактивной (is_active=false, deleted_at=now).
	Deactivate(ctx context.Context, number int) error
	// Delete физически удаляет карточку. При ссылке из equipments_assignments
	// (FK RESTRICT) возвращает domain.ErrInUse.
	Delete(ctx context.Context, number int) error
}
