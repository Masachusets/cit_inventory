package service

import (
	"context"
	"fmt"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
)

// DepartmentService — бизнес-логика работы с справочником подразделений.
// Инкапсулирует валидацию, проверку уникальности кода и правила
// деактивации, не зная деталей HTTP и SQL.
type DepartmentService interface {
	// Create создаёт подразделение. При существующем коде или наименовании —
	// domain.ErrAlreadyExists.
	Create(ctx context.Context, code int, name string) (*domain.Department, error)
	// Get возвращает подразделение по коду в любом статусе либо domain.ErrNotFound.
	Get(ctx context.Context, code int) (*domain.Department, error)
	// ListActive возвращает список активных подразделений с учётом фильтра.
	ListActive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error)
	// ListInactive возвращает список неактивных подразделений с учётом фильтра.
	ListInactive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error)
	// UpdateName изменяет только наименование подразделения.
	UpdateName(ctx context.Context, code int, name string) (*domain.Department, error)
	// Deactivate помечает подразделение неактивным (идемпотентно).
	Deactivate(ctx context.Context, code int) (*domain.Department, error)
}

// DepartmentServiceImpl — реализация DepartmentService поверх DepartmentRepository.
type DepartmentServiceImpl struct {
	repo repository.DepartmentRepository
}

// NewDepartmentService создаёт сервис подразделений с переданным репозиторием.
func NewDepartmentService(repo repository.DepartmentRepository) *DepartmentServiceImpl {
	return &DepartmentServiceImpl{repo: repo}
}

// Compile-time проверка реализации интерфейса.
var _ DepartmentService = (*DepartmentServiceImpl)(nil)

func (s *DepartmentServiceImpl) Create(ctx context.Context, code int, name string) (*domain.Department, error) {
	dept, err := domain.NewDepartment(code, name)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.Exists(ctx, dept.Code)
	if err != nil {
		return nil, fmt.Errorf("check department %d exists: %w", dept.Code, err)
	}
	if exists {
		return nil, fmt.Errorf("%w: код %d", domain.ErrAlreadyExists, dept.Code)
	}

	if err := s.repo.Create(ctx, dept); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *DepartmentServiceImpl) Get(ctx context.Context, code int) (*domain.Department, error) {
	if err := checkCode(code); err != nil {
		return nil, err
	}
	return s.repo.GetByCode(ctx, int16(code))
}

func (s *DepartmentServiceImpl) ListActive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	return s.repo.ListActive(ctx, filter)
}

func (s *DepartmentServiceImpl) ListInactive(ctx context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	return s.repo.ListInactive(ctx, filter)
}

func (s *DepartmentServiceImpl) UpdateName(ctx context.Context, code int, name string) (*domain.Department, error) {
	if err := checkCode(code); err != nil {
		return nil, err
	}
	dept, err := s.repo.GetByCode(ctx, int16(code))
	if err != nil {
		return nil, err
	}
	if err := dept.UpdateName(name); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateName(ctx, int16(code), dept.Name); err != nil {
		return nil, err
	}
	return dept, nil
}

func (s *DepartmentServiceImpl) Deactivate(ctx context.Context, code int) (*domain.Department, error) {
	if err := checkCode(code); err != nil {
		return nil, err
	}
	dept, err := s.repo.GetByCode(ctx, int16(code))
	if err != nil {
		return nil, err
	}

	// Идемпотентность: если уже неактивно — возвращаем как есть без записи в БД.
	if !dept.IsActive {
		return dept, nil
	}

	if err := s.repo.Deactivate(ctx, int16(code)); err != nil {
		return nil, err
	}
	dept.Deactivate()
	return dept, nil
}

// checkCode валидирует код до обращения к репозиторию (ручная проверка,
// т.к. int шире диапазона SMALLINT).
func checkCode(code int) error {
	return domain.ValidateDepartmentCode(code)
}
