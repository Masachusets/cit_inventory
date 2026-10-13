package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// fakeNomenclatureRepo — in-memory реализация repository.NomenclatureRepository
// для тестов сервиса без БД. Фильтрация имитирует ILIKE '%...%'
// (частичное совпадение без учёта регистра).
type fakeNomenclatureRepo struct {
	items map[int]*domain.Nomenclature
}

func newFakeNomenclatureRepo() *fakeNomenclatureRepo {
	return &fakeNomenclatureRepo{items: map[int]*domain.Nomenclature{}}
}

func (f *fakeNomenclatureRepo) GetByID(_ context.Context, id int) (*domain.Nomenclature, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *item
	return &cp, nil
}

func (f *fakeNomenclatureRepo) List(_ context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error) {
	var out []*domain.Nomenclature
	for _, n := range f.items {
		if filter.Code != nil && !strings.Contains(strings.ToLower(n.Code), strings.ToLower(*filter.Code)) {
			continue
		}
		if filter.Name != nil && !strings.Contains(strings.ToLower(n.Name), strings.ToLower(*filter.Name)) {
			continue
		}
		cp := *n
		out = append(out, &cp)
	}
	return out, nil
}

func seedNomenclatures(f *fakeNomenclatureRepo) {
	f.items[1] = &domain.Nomenclature{ID: 1, Code: "08.01.02.00.00/0041", Name: "Ноутбук Clevo 15"}
	f.items[2] = &domain.Nomenclature{ID: 2, Code: "08.01.02.00.00/0042", Name: "Монитор LG 24\""}
	f.items[3] = &domain.Nomenclature{ID: 3, Code: "04.02.01/0010", Name: "Стол офисный"}
}

func TestNomenclatureService_Get(t *testing.T) {
	repo := newFakeNomenclatureRepo()
	seedNomenclatures(repo)
	svc := NewNomenclatureService(repo)

	n, err := svc.Get(context.Background(), 1)
	if err != nil {
		t.Fatalf("Get() ошибка: %v", err)
	}
	if n.ID != 1 || n.Code != "08.01.02.00.00/0041" {
		t.Errorf("nomenclature = %+v", n)
	}
}

func TestNomenclatureService_Get_NotFound(t *testing.T) {
	svc := NewNomenclatureService(newFakeNomenclatureRepo())

	_, err := svc.Get(context.Background(), 999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNomenclatureService_Get_InvalidID(t *testing.T) {
	svc := NewNomenclatureService(newFakeNomenclatureRepo())

	if _, err := svc.Get(context.Background(), 0); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("id=0: err = %v, want ErrInvalidID", err)
	}
	if _, err := svc.Get(context.Background(), -5); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("id=-5: err = %v, want ErrInvalidID", err)
	}
}

func TestNomenclatureService_List_NoFilter(t *testing.T) {
	repo := newFakeNomenclatureRepo()
	seedNomenclatures(repo)
	svc := NewNomenclatureService(repo)

	items, err := svc.List(context.Background(), domain.NomenclatureFilter{})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("len = %d, want 3", len(items))
	}
}

func TestNomenclatureService_List_FilterByCodeAndName(t *testing.T) {
	repo := newFakeNomenclatureRepo()
	seedNomenclatures(repo)
	svc := NewNomenclatureService(repo)

	code := "0041"
	items, err := svc.List(context.Background(), domain.NomenclatureFilter{Code: &code})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 {
		t.Errorf("фильтр по code: %+v", items)
	}

	// Частичное совпадение без учёта регистра: «ноутбук» найдёт «Ноутбук...».
	name := "ноутбук"
	items, err = svc.List(context.Background(), domain.NomenclatureFilter{Name: &name})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 {
		t.Errorf("фильтр по name: %+v", items)
	}

	// Комбинация: код+имя, условие AND.
	code = "0042"
	name = "ноутбук"
	items, err = svc.List(context.Background(), domain.NomenclatureFilter{Code: &code, Name: &name})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("AND-фильтр без совпадений: %+v", items)
	}
}
