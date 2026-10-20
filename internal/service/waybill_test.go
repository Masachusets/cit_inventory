package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/jackc/pgx/v5"
)

// fakeWaybillRepo — in-memory реализация repository.WaybillRepository
// для тестов сервиса без БД.
type fakeWaybillRepo struct {
	items map[int]*domain.Waybill
	seq   int
}

func newFakeWaybillRepo() *fakeWaybillRepo {
	return &fakeWaybillRepo{items: map[int]*domain.Waybill{}}
}

func (f *fakeWaybillRepo) Create(_ context.Context, wb *domain.Waybill) error {
	f.seq++
	// Присваиваем ID исходному объекту (сервис возвращает его же).
	wb.ID = f.seq
	tcp := *wb
	f.items[tcp.ID] = &tcp
	return nil
}

func (f *fakeWaybillRepo) CreateWithID(_ context.Context, wb *domain.Waybill) (int, error) {
	if err := f.Create(context.Background(), wb); err != nil {
		return 0, err
	}
	return wb.ID, nil
}

func (f *fakeWaybillRepo) CreateWithIDWithTx(_ context.Context, _ pgx.Tx, wb *domain.Waybill) (int, error) {
	if err := f.Create(context.Background(), wb); err != nil {
		return 0, err
	}
	return wb.ID, nil
}

func (f *fakeWaybillRepo) GetByID(_ context.Context, id int) (*domain.Waybill, error) {
	wb, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	tcp := *wb
	return &tcp, nil
}

func (f *fakeWaybillRepo) List(_ context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error) {
	var out []*domain.Waybill
	for _, wb := range f.items {
		if filter.Name != nil && !strings.Contains(strings.ToLower(wb.Name), strings.ToLower(*filter.Name)) {
			continue
		}
		if filter.Number != nil {
			if wb.Number == nil || !strings.Contains(strings.ToLower(*wb.Number), strings.ToLower(*filter.Number)) {
				continue
			}
		}
		if filter.IssueDateFrom != nil {
			if wb.IssueDate == nil || wb.IssueDate.Before(*filter.IssueDateFrom) {
				continue
			}
		}
		if filter.IssueDateTo != nil {
			if wb.IssueDate == nil || wb.IssueDate.After(*filter.IssueDateTo) {
				continue
			}
		}
		cp := *wb
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeWaybillRepo) ExistsByNumber(_ context.Context, number string) (bool, error) {
	for _, wb := range f.items {
		if wb.Number != nil && *wb.Number == number {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeWaybillRepo) Update(_ context.Context, id int, wb *domain.Waybill) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrNotFound
	}
	f.items[id] = wb
	return nil
}

func TestWaybillService_Create(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())
	num := "123/2026"
	date := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)

	wb, err := svc.Create(context.Background(), "Путевой лист", num, &date)
	if err != nil {
		t.Fatalf("Create() ошибка: %v", err)
	}
	if wb.ID == 0 || wb.Name != "Путевой лист" || wb.Number == nil || *wb.Number != num {
		t.Errorf("waybill = %+v", wb)
	}
}

func TestWaybillService_Create_DuplicateNumber(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())
	num := "123/2026"

	if _, err := svc.Create(context.Background(), "Первый", num, nil); err != nil {
		t.Fatalf("первый Create() ошибка: %v", err)
	}
	_, err := svc.Create(context.Background(), "Второй", num, nil)
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("дубль номера: err = %v, want ErrAlreadyExists", err)
	}
}

func TestWaybillService_Create_InvalidName(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())

	if _, err := svc.Create(context.Background(), "", "", nil); !errors.Is(err, domain.ErrInvalidName) {
		t.Errorf("err = %v, want ErrInvalidName", err)
	}
}

func TestWaybillService_Get(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())
	created, _ := svc.Create(context.Background(), "Имя", "1/2026", nil)

	wb, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() ошибка: %v", err)
	}
	if wb.ID != created.ID {
		t.Errorf("id = %d, want %d", wb.ID, created.ID)
	}
}

func TestWaybillService_Get_NotFound(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())

	if _, err := svc.Get(context.Background(), 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestWaybillService_Get_InvalidID(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())

	if _, err := svc.Get(context.Background(), 0); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("err = %v, want ErrInvalidID", err)
	}
}

func TestWaybillService_List_DateRange(t *testing.T) {
	svc := NewWaybillService(newFakeWaybillRepo())
	d1 := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	_, _ = svc.Create(context.Background(), "Ранний", "1/2026", &d1)
	_, _ = svc.Create(context.Background(), "Поздний", "2/2026", &d2)
	_, _ = svc.Create(context.Background(), "Без даты", "", nil) // NULL не матчится

	from := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	items, err := svc.List(context.Background(), domain.WaybillFilter{IssueDateFrom: &from})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Поздний" {
		t.Errorf("IssueDateFrom: %+v", items)
	}

	to := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	items, err = svc.List(context.Background(), domain.WaybillFilter{IssueDateTo: &to})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Ранний" {
		t.Errorf("IssueDateTo: %+v", items)
	}

	items, err = svc.List(context.Background(), domain.WaybillFilter{IssueDateFrom: &from, IssueDateTo: &d2})
	if err != nil {
		t.Fatalf("List() ошибка: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Поздний" {
		t.Errorf("диапазон: %+v", items)
	}
}
