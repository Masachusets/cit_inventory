package service

import (
	"context"
	"fmt"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
)

// AssignmentService — бизнес-логика работы с закреплениями оборудования
// (equipments_assignments). Инкапсулирует валидацию (CHECK chk_target, даты,
// ссылки), не зная деталей HTTP и SQL. Уникальности активных закреплений нет:
// каждый вызов Create добавляет новую запись истории.
type AssignmentService interface {
	// Create создаёт закрепление. При некорректной комбинации тип/ссылки —
	// domain.ErrInvalidTarget; при битой FK — domain.ErrInvalidReference.
	Create(ctx context.Context, in domain.AssignmentInput) (*domain.Assignment, error)
	// Get возвращает закрепление со связанными сущностями по id
	// либо domain.ErrNotFound. При id <= 0 — domain.ErrInvalidID.
	Get(ctx context.Context, id int) (*domain.Assignment, error)
	// List возвращает закрепления, отфильтрованные по заданным полям.
	List(ctx context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error)
	// Update применяет частичное обновление к закреплению по id и возвращает
	// актуальную запись со связанными сущностями.
	Update(ctx context.Context, id int, up domain.AssignmentUpdate) (*domain.Assignment, error)
	// Delete физически удаляет запись истории и возвращает удалённую запись.
	// Отсутствующая запись — domain.ErrNotFound.
	Delete(ctx context.Context, id int) (*domain.Assignment, error)
	// CloseActiveForEquipment закрывает все активные закрепления для указанного
	// оборудования (unassigned_at = NOW()).
	CloseActiveForEquipment(ctx context.Context, equipmentID int) error
}

// AssignmentServiceImpl — реализация AssignmentService поверх AssignmentRepository.
type AssignmentServiceImpl struct {
	repo repository.AssignmentRepository
}

// NewAssignmentService создаёт сервис закреплений с переданным репозиторием.
func NewAssignmentService(repo repository.AssignmentRepository) *AssignmentServiceImpl {
	return &AssignmentServiceImpl{repo: repo}
}

// Compile-time проверка реализации интерфейса.
var _ AssignmentService = (*AssignmentServiceImpl)(nil)

func (s *AssignmentServiceImpl) Create(ctx context.Context, in domain.AssignmentInput) (*domain.Assignment, error) {
	a, err := domain.NewAssignment(in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, a); err != nil {
		return nil, err
	}
	// Повторное чтение — чтобы вернуть связанные сущности (equipment, waybill,
	// card/department), как в остальных GET-ответах.
	return s.repo.GetByID(ctx, a.ID)
}

func (s *AssignmentServiceImpl) Get(ctx context.Context, id int) (*domain.Assignment, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	return s.repo.GetByID(ctx, id)
}

func (s *AssignmentServiceImpl) List(ctx context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error) {
	return s.repo.List(ctx, filter)
}

func (s *AssignmentServiceImpl) Update(ctx context.Context, id int, up domain.AssignmentUpdate) (*domain.Assignment, error) {
	a, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := a.ApplyUpdate(up); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, a); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *AssignmentServiceImpl) CloseActiveForEquipment(ctx context.Context, equipmentID int) error {
	return s.repo.CloseActiveForEquipment(ctx, equipmentID)
}

func (s *AssignmentServiceImpl) Delete(ctx context.Context, id int) (*domain.Assignment, error) {
	a, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	return a, nil
}
