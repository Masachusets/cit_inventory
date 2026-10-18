package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
)

// WaybillService — бизнес-логика работы с путевыми листами. Инкапсулирует
// валидацию и проверку уникальности номера, не зная деталей HTTP и SQL.
type WaybillService interface {
	// Create создаёт путевой лист. При существующем номере — domain.ErrAlreadyExists.
	Create(ctx context.Context, name, number string, issueDate *time.Time) (*domain.Waybill, error)
	// Get возвращает путевой лист по id либо domain.ErrNotFound.
	// При id <= 0 — domain.ErrInvalidID.
	Get(ctx context.Context, id int) (*domain.Waybill, error)
	// List возвращает путевые листы, отфильтрованные по заданным полям.
	List(ctx context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error)
	// Update обновляет путевой лист по id. При существующем номере — domain.ErrAlreadyExists.
	Update(ctx context.Context, id int, name, number string, issueDate *time.Time) (*domain.Waybill, error)
}

// WaybillServiceImpl — реализация WaybillService поверх WaybillRepository.
type WaybillServiceImpl struct {
	repo repository.WaybillRepository
}

// NewWaybillService создаёт сервис путевых листов с переданным репозиторием.
func NewWaybillService(repo repository.WaybillRepository) *WaybillServiceImpl {
	return &WaybillServiceImpl{repo: repo}
}

// Compile-time проверка реализации интерфейса.
var _ WaybillService = (*WaybillServiceImpl)(nil)

func (s *WaybillServiceImpl) Create(ctx context.Context, name, number string, issueDate *time.Time) (*domain.Waybill, error) {
	wb, err := domain.NewWaybill(name, number, issueDate)
	if err != nil {
		return nil, err
	}

	// Проверка дубля номера — по образцу cards (номер опционален).
	if wb.Number != nil {
		exists, err := s.repo.ExistsByNumber(ctx, *wb.Number)
		if err != nil {
			return nil, fmt.Errorf("check waybill %q exists: %w", *wb.Number, err)
		}
		if exists {
			return nil, fmt.Errorf("%w: номер %q", domain.ErrAlreadyExists, *wb.Number)
		}
	}

	if err := s.repo.Create(ctx, wb); err != nil {
		return nil, err
	}
	return wb, nil
}

func (s *WaybillServiceImpl) Get(ctx context.Context, id int) (*domain.Waybill, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}
	return s.repo.GetByID(ctx, id)
}

func (s *WaybillServiceImpl) List(ctx context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error) {
	return s.repo.List(ctx, filter)
}

// Update обновляет путевой лист по id.
func (s *WaybillServiceImpl) Update(ctx context.Context, id int, name, number string, issueDate *time.Time) (*domain.Waybill, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id %d должен быть положительным", domain.ErrInvalidID, id)
	}

	wb, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := wb.Update(name, number, issueDate); err != nil {
		return nil, err
	}

	// Проверка дубля номера (если номер задан).
	if wb.Number != nil {
		exists, err := s.repo.ExistsByNumber(ctx, *wb.Number)
		if err != nil {
			return nil, fmt.Errorf("check waybill %q exists: %w", *wb.Number, err)
		}
		if exists {
			return nil, fmt.Errorf("%w: номер %q", domain.ErrAlreadyExists, *wb.Number)
		}
	}

	if err := s.repo.Update(ctx, id, wb); err != nil {
		return nil, err
	}
	return wb, nil
}
