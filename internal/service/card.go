package service

import (
	"context"
	"fmt"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
)

// CardService — бизнес-логика работы с карточками учёта. Инкапсулирует
// валидацию, проверку уникальности номера и правила деактивации/удаления,
// не зная деталей HTTP и SQL.
type CardService interface {
	// Create создаёт карточку. При существующем номере — domain.ErrAlreadyExists.
	Create(ctx context.Context, number int, fullName string) (*domain.Card, error)
	// Get возвращает карточку по номеру в любом статусе либо domain.ErrNotFound.
	Get(ctx context.Context, number int) (*domain.Card, error)
	// ListActive возвращает список активных карточек с учётом фильтра.
	ListActive(ctx context.Context, filter domain.CardFilter) ([]*domain.Card, error)
	// ListInactive возвращает список неактивных карточек с учётом фильтра.
	ListInactive(ctx context.Context, filter domain.CardFilter) ([]*domain.Card, error)
	// UpdateName изменяет только ФИО карточки (в любом статусе).
	UpdateName(ctx context.Context, number int, fullName string) (*domain.Card, error)
	// Deactivate помечает карточку неактивной (идемпотентно).
	Deactivate(ctx context.Context, number int) (*domain.Card, error)
	// Delete физически удаляет неактивную карточку. Для активной —
	// domain.ErrStillActive; при ссылке из equipments — domain.ErrInUse.
	Delete(ctx context.Context, number int) error
}

// CardServiceImpl — реализация CardService поверх CardRepository.
type CardServiceImpl struct {
	repo repository.CardRepository
}

// NewCardService создаёт сервис карточек с переданным репозиторием.
func NewCardService(repo repository.CardRepository) *CardServiceImpl {
	return &CardServiceImpl{repo: repo}
}

// Compile-time проверка реализации интерфейса.
var _ CardService = (*CardServiceImpl)(nil)

func (s *CardServiceImpl) Create(ctx context.Context, number int, fullName string) (*domain.Card, error) {
	card, err := domain.NewCard(number, fullName)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.Exists(ctx, card.Number)
	if err != nil {
		return nil, fmt.Errorf("check card %d exists: %w", card.Number, err)
	}
	if exists {
		return nil, fmt.Errorf("%w: номер %d", domain.ErrAlreadyExists, card.Number)
	}

	if err := s.repo.Create(ctx, card); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *CardServiceImpl) Get(ctx context.Context, number int) (*domain.Card, error) {
	return s.repo.GetByNumber(ctx, number)
}

func (s *CardServiceImpl) ListActive(ctx context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	return s.repo.ListActive(ctx, filter)
}

func (s *CardServiceImpl) ListInactive(ctx context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	return s.repo.ListInactive(ctx, filter)
}

func (s *CardServiceImpl) UpdateName(ctx context.Context, number int, fullName string) (*domain.Card, error) {
	card, err := s.repo.GetByNumber(ctx, number)
	if err != nil {
		return nil, err
	}
	if err := card.UpdateFullName(fullName); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateFullName(ctx, number, card.FullName); err != nil {
		return nil, err
	}
	return card, nil
}

func (s *CardServiceImpl) Deactivate(ctx context.Context, number int) (*domain.Card, error) {
	card, err := s.repo.GetByNumber(ctx, number)
	if err != nil {
		return nil, err
	}

	// Идемпотентность: если уже неактивна — возвращаем как есть без записи в БД.
	if !card.IsActive {
		return card, nil
	}

	if err := s.repo.Deactivate(ctx, number); err != nil {
		return nil, err
	}
	card.Deactivate()
	return card, nil
}

func (s *CardServiceImpl) Delete(ctx context.Context, number int) error {
	card, err := s.repo.GetByNumber(ctx, number)
	if err != nil {
		return err
	}
	if card.IsActive {
		return fmt.Errorf("%w: номер %d", domain.ErrStillActive, number)
	}
	return s.repo.Delete(ctx, number)
}
