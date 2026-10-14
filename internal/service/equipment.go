package service

import (
	"context"
	"fmt"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
)

// EquipmentService — бизнес-логика работы с оборудованием. Инкапсулирует
// валидацию (инвентарный номер, статус, атрибуты), проверку уникальности
// номера и правила soft delete, не зная деталей HTTP и SQL.
type EquipmentService interface {
	// Create создаёт оборудование. При существующем инвентарном номере —
	// domain.ErrAlreadyExists; при несуществующей номенклатуре —
	// domain.ErrInvalidNomenclature.
	Create(ctx context.Context, in domain.EquipmentInput) (*domain.Equipment, error)
	// GetByID возвращает оборудование по id в любом статусе (активное и
	// удалённое) либо domain.ErrNotFound. При id <= 0 — domain.ErrInvalidID.
	GetByID(ctx context.Context, id int) (*domain.Equipment, error)
	// GetByInventoryNumber возвращает оборудование по инвентарному номеру
	// в любом статусе либо domain.ErrNotFound.
	GetByInventoryNumber(ctx context.Context, number string) (*domain.Equipment, error)
	// ListActive возвращает список активных (deleted_at IS NULL) записей.
	ListActive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error)
	// ListInactive возвращает список удалённых (deleted_at IS NOT NULL) записей.
	ListInactive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error)
	// UpdateByID применяет частичное обновление к оборудованию по id.
	UpdateByID(ctx context.Context, id int, up domain.EquipmentUpdate) (*domain.Equipment, error)
	// UpdateByInventoryNumber применяет частичное обновление к оборудованию
	// по инвентарному номеру.
	UpdateByInventoryNumber(ctx context.Context, number string, up domain.EquipmentUpdate) (*domain.Equipment, error)
	// DeleteByID помечает оборудование удалённым (заполняет deleted_at).
	// Идемпотентно: повторный вызов не изменяет метку удаления.
	DeleteByID(ctx context.Context, id int) (*domain.Equipment, error)
	// DeleteByInventoryNumber помечает оборудование удалённым по инвентарному
	// номеру. Идемпотентно.
	DeleteByInventoryNumber(ctx context.Context, number string) (*domain.Equipment, error)
}

// EquipmentServiceImpl — реализация EquipmentService поверх EquipmentRepository.
type EquipmentServiceImpl struct {
	repo repository.EquipmentRepository
}

// NewEquipmentService создаёт сервис оборудования с переданным репозиторием.
func NewEquipmentService(repo repository.EquipmentRepository) *EquipmentServiceImpl {
	return &EquipmentServiceImpl{repo: repo}
}

// Compile-time проверка реализации интерфейса.
var _ EquipmentService = (*EquipmentServiceImpl)(nil)

func (s *EquipmentServiceImpl) Create(ctx context.Context, in domain.EquipmentInput) (*domain.Equipment, error) {
	eq, err := domain.NewEquipment(in)
	if err != nil {
		return nil, err
	}

	// Проверка дубля инвентарного номера — по образцу cards: UNIQUE действует
	// и для soft-deleted записей, поэтому ищем в любом статусе.
	exists, err := s.repo.ExistsInventoryNumber(ctx, eq.InventoryNumber)
	if err != nil {
		return nil, fmt.Errorf("check equipment %s exists: %w", eq.InventoryNumber, err)
	}
	if exists {
		return nil, fmt.Errorf("%w: инвентарный номер %s", domain.ErrAlreadyExists, eq.InventoryNumber)
	}

	if err := s.repo.Create(ctx, eq); err != nil {
		return nil, err
	}
	return eq, nil
}

func (s *EquipmentServiceImpl) GetByID(ctx context.Context, id int) (*domain.Equipment, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	return s.repo.GetByID(ctx, id)
}

func (s *EquipmentServiceImpl) GetByInventoryNumber(ctx context.Context, number string) (*domain.Equipment, error) {
	if err := domain.ValidateInventoryNumber(number); err != nil {
		return nil, err
	}
	return s.repo.GetByInventoryNumber(ctx, number)
}

func (s *EquipmentServiceImpl) ListActive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	return s.repo.ListActive(ctx, filter)
}

func (s *EquipmentServiceImpl) ListInactive(ctx context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	return s.repo.ListInactive(ctx, filter)
}

func (s *EquipmentServiceImpl) UpdateByID(ctx context.Context, id int, up domain.EquipmentUpdate) (*domain.Equipment, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	eq, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.applyUpdate(ctx, id, eq, up)
}

func (s *EquipmentServiceImpl) UpdateByInventoryNumber(ctx context.Context, number string, up domain.EquipmentUpdate) (*domain.Equipment, error) {
	eq, err := s.GetByInventoryNumber(ctx, number)
	if err != nil {
		return nil, err
	}
	return s.applyUpdate(ctx, eq.ID, eq, up)
}

// applyUpdate применяет обновление к загруженной модели и сохраняет его.
func (s *EquipmentServiceImpl) applyUpdate(ctx context.Context, id int, eq *domain.Equipment, up domain.EquipmentUpdate) (*domain.Equipment, error) {
	if err := eq.ApplyUpdate(up); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, eq); err != nil {
		return nil, err
	}
	return eq, nil
}

func (s *EquipmentServiceImpl) DeleteByID(ctx context.Context, id int) (*domain.Equipment, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	eq, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Идемпотентность: уже удалённое оборудование не трогаем (метка
	// deleted_at сохраняется исходной).
	if eq.DeletedAt != nil {
		return eq, nil
	}
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return nil, err
	}
	eq.SoftDelete()
	return eq, nil
}

func (s *EquipmentServiceImpl) DeleteByInventoryNumber(ctx context.Context, number string) (*domain.Equipment, error) {
	eq, err := s.GetByInventoryNumber(ctx, number)
	if err != nil {
		return nil, err
	}
	return s.DeleteByID(ctx, eq.ID)
}
