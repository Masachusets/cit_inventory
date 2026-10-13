package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// fakeCardRepo — in-memory реализация repository.CardRepository для тестов
// сервиса без БД.
type fakeCardRepo struct {
	cards     map[int]*domain.Card
	createErr error
	deleteErr error

	// deactivateCalls считает обращения к Deactivate для проверки идемпотентности.
	deactivateCalls int
}

func newFakeCardRepo() *fakeCardRepo {
	return &fakeCardRepo{cards: map[int]*domain.Card{}}
}

func (f *fakeCardRepo) Create(_ context.Context, card *domain.Card) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.cards[card.Number] = card
	return nil
}

func (f *fakeCardRepo) GetByNumber(_ context.Context, number int) (*domain.Card, error) {
	card, ok := f.cards[number]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *card
	return &cp, nil
}

func (f *fakeCardRepo) ListActive(_ context.Context) ([]*domain.Card, error) {
	var out []*domain.Card
	for _, c := range f.cards {
		if c.IsActive {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeCardRepo) ListInactive(_ context.Context) ([]*domain.Card, error) {
	var out []*domain.Card
	for _, c := range f.cards {
		if !c.IsActive {
			cp := *c
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeCardRepo) Exists(_ context.Context, number int) (bool, error) {
	_, ok := f.cards[number]
	return ok, nil
}

func (f *fakeCardRepo) UpdateFullName(_ context.Context, number int, fullName string) error {
	card, ok := f.cards[number]
	if !ok {
		return domain.ErrNotFound
	}
	card.FullName = fullName
	return nil
}

func (f *fakeCardRepo) Deactivate(_ context.Context, number int) error {
	f.deactivateCalls++
	card, ok := f.cards[number]
	if !ok {
		return domain.ErrNotFound
	}
	now := time.Now()
	card.IsActive = false
	card.DeletedAt = &now
	return nil
}

func (f *fakeCardRepo) Delete(_ context.Context, number int) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.cards[number]; !ok {
		return domain.ErrNotFound
	}
	delete(f.cards, number)
	return nil
}

func TestService_Create(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())

	card, err := svc.Create(context.Background(), 10, "Сидоров С.С.")
	if err != nil {
		t.Fatalf("Create() ошибка: %v", err)
	}
	if card.Number != 10 || card.FullName != "Сидоров С.С." {
		t.Errorf("card = %+v", card)
	}
}

func TestService_Create_Duplicate(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())

	if _, err := svc.Create(context.Background(), 1, "Первый"); err != nil {
		t.Fatalf("первый Create() ошибка: %v", err)
	}
	_, err := svc.Create(context.Background(), 1, "Второй")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("дубль: err = %v, want ErrAlreadyExists", err)
	}
}

func TestService_Create_InvalidInput(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())

	if _, err := svc.Create(context.Background(), 0, "Имя"); !errors.Is(err, domain.ErrInvalidNumber) {
		t.Errorf("number=0: err = %v, want ErrInvalidNumber", err)
	}
	if _, err := svc.Create(context.Background(), 1, ""); !errors.Is(err, domain.ErrInvalidName) {
		t.Errorf("пустое имя: err = %v, want ErrInvalidName", err)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())

	_, err := svc.Get(context.Background(), 999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get(): err = %v, want ErrNotFound", err)
	}
}

func TestService_List_SplitByStatus(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())

	active, _ := svc.Create(context.Background(), 1, "Активный")
	inactive, _ := svc.Create(context.Background(), 2, "Неактивный")
	_, _ = svc.Deactivate(context.Background(), 2)

	actives, _ := svc.ListActive(context.Background())
	if len(actives) != 1 || actives[0].Number != active.Number {
		t.Errorf("ListActive = %+v, want только номер 1", actives)
	}

	inactives, _ := svc.ListInactive(context.Background())
	if len(inactives) != 1 || inactives[0].Number != inactive.Number {
		t.Errorf("ListInactive = %+v, want только номер 2", inactives)
	}
}

func TestService_UpdateName(t *testing.T) {
	repo := newFakeCardRepo()
	svc := NewCardService(repo)
	_, _ = svc.Create(context.Background(), 1, "Старое")

	card, err := svc.UpdateName(context.Background(), 1, "Новое Имя")
	if err != nil {
		t.Fatalf("UpdateName() ошибка: %v", err)
	}
	if card.FullName != "Новое Имя" {
		t.Errorf("FullName = %q", card.FullName)
	}
	if got, _ := repo.GetByNumber(context.Background(), 1); got.FullName != "Новое Имя" {
		t.Errorf("репозиторий не обновился: %q", got.FullName)
	}
}

func TestService_UpdateName_NotFound(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())
	_, err := svc.UpdateName(context.Background(), 5, "Имя")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestService_Deactivate_Idempotent(t *testing.T) {
	repo := newFakeCardRepo()
	svc := NewCardService(repo)
	_, _ = svc.Create(context.Background(), 1, "Имя")

	if _, err := svc.Deactivate(context.Background(), 1); err != nil {
		t.Fatalf("первый Deactivate() ошибка: %v", err)
	}
	if _, err := svc.Deactivate(context.Background(), 1); err != nil {
		t.Fatalf("повторный Deactivate() ошибка: %v", err)
	}
	if repo.deactivateCalls != 1 {
		t.Errorf("repo.Deactivate вызван %d раз, want 1 (идемпотентность)", repo.deactivateCalls)
	}
}

func TestService_Delete_Active(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())
	_, _ = svc.Create(context.Background(), 1, "Имя")

	err := svc.Delete(context.Background(), 1)
	if !errors.Is(err, domain.ErrStillActive) {
		t.Errorf("удаление активной: err = %v, want ErrStillActive", err)
	}
}

func TestService_Delete_Inactive(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())
	_, _ = svc.Create(context.Background(), 1, "Имя")
	_, _ = svc.Deactivate(context.Background(), 1)

	if err := svc.Delete(context.Background(), 1); err != nil {
		t.Fatalf("Delete() ошибка: %v", err)
	}
	if _, err := svc.Get(context.Background(), 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("карточка не удалена: %v", err)
	}
}

func TestService_Delete_InUse(t *testing.T) {
	repo := newFakeCardRepo()
	svc := NewCardService(repo)
	_, _ = svc.Create(context.Background(), 1, "Имя")
	_, _ = svc.Deactivate(context.Background(), 1)
	repo.deleteErr = domain.ErrInUse

	err := svc.Delete(context.Background(), 1)
	if !errors.Is(err, domain.ErrInUse) {
		t.Errorf("err = %v, want ErrInUse", err)
	}
}
