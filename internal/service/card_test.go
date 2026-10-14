package service

import (
	"context"
	"errors"
	"strings"
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

func (f *fakeCardRepo) ListActive(_ context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	return f.list(true, filter), nil
}

func (f *fakeCardRepo) ListInactive(_ context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	return f.list(false, filter), nil
}

// list имитирует выборку с фильтром: ILIKE '%...%' по full_name и диапазоны
// дат created_at/updated_at/deleted_at (>= от, <= до) по дню.
func (f *fakeCardRepo) list(active bool, filter domain.CardFilter) []*domain.Card {
	var out []*domain.Card
	for _, c := range f.cards {
		if c.IsActive != active {
			continue
		}
		if filter.FullName != nil && !strings.Contains(strings.ToLower(c.FullName), strings.ToLower(*filter.FullName)) {
			continue
		}
		if !inRange(c.CreatedAt, filter.CreatedAtFrom, filter.CreatedAtTo) {
			continue
		}
		if !inRange(c.UpdatedAt, filter.UpdatedAtFrom, filter.UpdatedAtTo) {
			continue
		}
		if c.DeletedAt != nil && !inRange(*c.DeletedAt, filter.DeletedAtFrom, filter.DeletedAtTo) {
			continue
		}
		cp := *c
		out = append(out, &cp)
	}
	return out
}

// inRange сообщает, попадает ли момент времени в дневной диапазон [from; to].
// nil-границы означают отсутствие ограничения с соответствующей стороны.
func inRange(t time.Time, from, to *time.Time) bool {
	day := t.Truncate(24 * time.Hour)
	if from != nil && day.Before(from.Truncate(24*time.Hour)) {
		return false
	}
	if to != nil && day.After(to.Truncate(24*time.Hour)) {
		return false
	}
	return true
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

	actives, _ := svc.ListActive(context.Background(), domain.CardFilter{})
	if len(actives) != 1 || actives[0].Number != active.Number {
		t.Errorf("ListActive = %+v, want только номер 1", actives)
	}

	inactives, _ := svc.ListInactive(context.Background(), domain.CardFilter{})
	if len(inactives) != 1 || inactives[0].Number != inactive.Number {
		t.Errorf("ListInactive = %+v, want только номер 2", inactives)
	}
}

func TestService_List_Filter(t *testing.T) {
	svc := NewCardService(newFakeCardRepo())
	_, _ = svc.Create(context.Background(), 1, "Иванов Иван")
	_, _ = svc.Create(context.Background(), 2, "Петров Пётр")
	_, _ = svc.Create(context.Background(), 3, "Иванова Мария")

	name := "иванов"
	items, err := svc.ListActive(context.Background(), domain.CardFilter{FullName: &name})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	// Подстрока «иванов» без учёта регистра матчит «Иванов Иван» и «Иванова Мария».
	if len(items) != 2 {
		t.Errorf("фильтр по full_name: получено %d, want 2 (%+v)", len(items), items)
	}
}

func TestService_List_FilterCreatedAtRange(t *testing.T) {
	repo := newFakeCardRepo()
	// Карточки с управляемыми метками created_at.
	mk := func(number int, name string, created time.Time) {
		repo.cards[number] = &domain.Card{Number: number, FullName: name, IsActive: true, CreatedAt: created}
	}
	early := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	mid := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)
	late := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)
	mk(1, "Ранняя", early)
	mk(2, "Средняя", mid)
	mk(3, "Поздняя", late)

	svc := NewCardService(repo)
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	items, err := svc.ListActive(context.Background(), domain.CardFilter{CreatedAtFrom: &from, CreatedAtTo: &to})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("диапазон created_at: получено %d, want 2 (%+v)", len(items), items)
	}
	got := map[int]bool{items[0].Number: true, items[1].Number: true}
	if !got[2] || !got[3] {
		t.Errorf("диапазон created_at: номера %v, want {2, 3}", got)
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
