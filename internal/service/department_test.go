package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// fakeDepartmentRepo — in-memory реализация repository.DepartmentRepository
// для тестов сервиса без БД.
type fakeDepartmentRepo struct {
	depts     map[int16]*domain.Department
	createErr error

	// deactivateCalls считает обращения к Deactivate для проверки идемпотентности.
	deactivateCalls int
}

func newFakeDepartmentRepo() *fakeDepartmentRepo {
	return &fakeDepartmentRepo{depts: map[int16]*domain.Department{}}
}

func (f *fakeDepartmentRepo) Create(_ context.Context, dept *domain.Department) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.depts[dept.Code] = dept
	return nil
}

func (f *fakeDepartmentRepo) GetByCode(_ context.Context, code int16) (*domain.Department, error) {
	dept, ok := f.depts[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *dept
	return &cp, nil
}

func (f *fakeDepartmentRepo) ListActive(_ context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	return f.list(true, filter), nil
}

func (f *fakeDepartmentRepo) ListInactive(_ context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	return f.list(false, filter), nil
}

// list имитирует выборку с фильтром: ILIKE '%...%' по name и диапазон
// created_at (>= от, <= до).
func (f *fakeDepartmentRepo) list(active bool, filter domain.DepartmentFilter) []*domain.Department {
	var out []*domain.Department
	for _, d := range f.depts {
		if d.IsActive != active {
			continue
		}
		if filter.Name != nil && !strings.Contains(strings.ToLower(d.Name), strings.ToLower(*filter.Name)) {
			continue
		}
		day := d.CreatedAt.Truncate(24 * time.Hour)
		if filter.CreatedAtFrom != nil && day.Before(filter.CreatedAtFrom.Truncate(24*time.Hour)) {
			continue
		}
		if filter.CreatedAtTo != nil && day.After(filter.CreatedAtTo.Truncate(24*time.Hour)) {
			continue
		}
		cp := *d
		out = append(out, &cp)
	}
	return out
}

func (f *fakeDepartmentRepo) Exists(_ context.Context, code int16) (bool, error) {
	_, ok := f.depts[code]
	return ok, nil
}

func (f *fakeDepartmentRepo) UpdateName(_ context.Context, code int16, name string) error {
	dept, ok := f.depts[code]
	if !ok {
		return domain.ErrNotFound
	}
	dept.Name = name
	return nil
}

func (f *fakeDepartmentRepo) Deactivate(_ context.Context, code int16) error {
	f.deactivateCalls++
	dept, ok := f.depts[code]
	if !ok {
		return domain.ErrNotFound
	}
	dept.IsActive = false
	return nil
}

func TestDepartmentService_Create(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	dept, err := svc.Create(context.Background(), 301, "опк Гомель")
	if err != nil {
		t.Fatalf("Create() ошибка: %v", err)
	}
	if dept.Code != 301 || dept.Name != "опк Гомель" || !dept.IsActive {
		t.Errorf("department = %+v", dept)
	}
}

func TestDepartmentService_Create_Duplicate(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	if _, err := svc.Create(context.Background(), 100, "Первый"); err != nil {
		t.Fatalf("первый Create() ошибка: %v", err)
	}
	_, err := svc.Create(context.Background(), 100, "Второй")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("дубль кода: err = %v, want ErrAlreadyExists", err)
	}
}

func TestDepartmentService_Create_InvalidInput(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	if _, err := svc.Create(context.Background(), 0, "Имя"); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("code=0: err = %v, want ErrInvalidCode", err)
	}
	if _, err := svc.Create(context.Background(), 100, ""); !errors.Is(err, domain.ErrInvalidName) {
		t.Errorf("пустое имя: err = %v, want ErrInvalidName", err)
	}
}

func TestDepartmentService_Get(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())
	_, _ = svc.Create(context.Background(), 100, "Имя")

	dept, err := svc.Get(context.Background(), 100)
	if err != nil {
		t.Fatalf("Get() ошибка: %v", err)
	}
	if dept.Code != 100 {
		t.Errorf("code = %d", dept.Code)
	}
}

func TestDepartmentService_Get_NotFound(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	if _, err := svc.Get(context.Background(), 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDepartmentService_Get_InvalidCode(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	if _, err := svc.Get(context.Background(), -1); !errors.Is(err, domain.ErrInvalidCode) {
		t.Errorf("err = %v, want ErrInvalidCode", err)
	}
}

func TestDepartmentService_List_SplitByStatus(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())
	_, _ = svc.Create(context.Background(), 1, "Активный")
	_, _ = svc.Create(context.Background(), 2, "Неактивный")
	_, _ = svc.Deactivate(context.Background(), 2)

	actives, _ := svc.ListActive(context.Background(), domain.DepartmentFilter{})
	if len(actives) != 1 || actives[0].Code != 1 {
		t.Errorf("ListActive = %+v, want только код 1", actives)
	}

	inactives, _ := svc.ListInactive(context.Background(), domain.DepartmentFilter{})
	if len(inactives) != 1 || inactives[0].Code != 2 {
		t.Errorf("ListInactive = %+v, want только код 2", inactives)
	}
}

func TestDepartmentService_List_Filter(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())
	_, _ = svc.Create(context.Background(), 100, "опк Гомель")
	_, _ = svc.Create(context.Background(), 200, "погз Лоев")

	name := "гомель"
	items, err := svc.ListActive(context.Background(), domain.DepartmentFilter{Name: &name})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 1 || items[0].Code != 100 {
		t.Errorf("фильтр по name: %+v", items)
	}
}

func TestDepartmentService_UpdateName(t *testing.T) {
	repo := newFakeDepartmentRepo()
	svc := NewDepartmentService(repo)
	_, _ = svc.Create(context.Background(), 100, "Старое")

	dept, err := svc.UpdateName(context.Background(), 100, "Новое Имя")
	if err != nil {
		t.Fatalf("UpdateName() ошибка: %v", err)
	}
	if dept.Name != "Новое Имя" {
		t.Errorf("Name = %q", dept.Name)
	}
	if got, _ := repo.GetByCode(context.Background(), 100); got.Name != "Новое Имя" {
		t.Errorf("репозиторий не обновился: %q", got.Name)
	}
}

func TestDepartmentService_UpdateName_NotFound(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	if _, err := svc.UpdateName(context.Background(), 5, "Имя"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDepartmentService_Deactivate_Idempotent(t *testing.T) {
	repo := newFakeDepartmentRepo()
	svc := NewDepartmentService(repo)
	_, _ = svc.Create(context.Background(), 100, "Имя")

	if _, err := svc.Deactivate(context.Background(), 100); err != nil {
		t.Fatalf("первый Deactivate() ошибка: %v", err)
	}
	if _, err := svc.Deactivate(context.Background(), 100); err != nil {
		t.Fatalf("повторный Deactivate() ошибка: %v", err)
	}
	if repo.deactivateCalls != 1 {
		t.Errorf("repo.Deactivate вызван %d раз, want 1 (идемпотентность)", repo.deactivateCalls)
	}
}

func TestDepartmentService_Deactivate_NotFound(t *testing.T) {
	svc := NewDepartmentService(newFakeDepartmentRepo())

	if _, err := svc.Deactivate(context.Background(), 5); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
