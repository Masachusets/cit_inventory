package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// ptrInt/ptrTime — краткая запись указателей в тестах пакета service.
func ptrInt(v int) *int              { return &v }
func ptrTime(v time.Time) *time.Time { return &v }

// fakeAssignmentRepo — in-memory реализация repository.AssignmentRepository
// для тестов сервиса без БД.
type fakeAssignmentRepo struct {
	items map[int]*domain.Assignment
	seq   int
}

func newFakeAssignmentRepo() *fakeAssignmentRepo {
	return &fakeAssignmentRepo{items: map[int]*domain.Assignment{}}
}

func (f *fakeAssignmentRepo) Create(_ context.Context, a *domain.Assignment) error {
	f.seq++
	a.ID = f.seq
	cp := *a
	f.items[cp.ID] = &cp
	return nil
}

// hydrate заполняет вложенные сущности, как это делает реальный JOIN
// в pgx-репозитории (в fake-репо они иначе остаются nil).
func hydrate(a *domain.Assignment) *domain.Assignment {
	if a.Equipment == nil {
		a.Equipment = &domain.Equipment{ID: a.EquipmentID, InventoryNumber: "ИТ00001"}
	}
	if a.Card == nil && a.CardNumber != nil {
		a.Card = &domain.Card{Number: *a.CardNumber, FullName: "Иванов Иван Иванович"}
	}
	if a.Department == nil && a.DepartmentCode != nil {
		a.Department = &domain.Department{Code: int16(*a.DepartmentCode), Name: "опк Тестовск"}
	}
	return a
}

func (f *fakeAssignmentRepo) GetByID(_ context.Context, id int) (*domain.Assignment, error) {
	a, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *a
	return hydrate(&cp), nil
}

func (f *fakeAssignmentRepo) List(_ context.Context, filter domain.AssignmentFilter) ([]*domain.Assignment, error) {
	var out []*domain.Assignment
	for _, a := range f.items {
		if filter.EquipmentID != nil && a.EquipmentID != *filter.EquipmentID {
			continue
		}
		if filter.TargetType != nil && a.TargetType != *filter.TargetType {
			continue
		}
		if filter.CardNumber != nil {
			if a.CardNumber == nil || *a.CardNumber != *filter.CardNumber {
				continue
			}
		}
		if filter.Active != nil {
			if *filter.Active && a.UnassignedAt != nil {
				continue
			}
			if !*filter.Active && a.UnassignedAt == nil {
				continue
			}
		}
		cp := *a
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeAssignmentRepo) Update(_ context.Context, id int, a *domain.Assignment) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrNotFound
	}
	cp := *a
	f.items[id] = &cp
	return nil
}

func (f *fakeAssignmentRepo) Delete(_ context.Context, id int) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func TestAssignmentService_Create(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())

	a, err := svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 1, TargetType: "employee", CardNumber: ptrInt(101),
	})
	if err != nil {
		t.Fatalf("Create() ошибка: %v", err)
	}
	if a.ID == 0 || a.TargetType != domain.TargetEmployee {
		t.Errorf("assignment = %+v", a)
	}
	if a.Equipment == nil || a.Equipment.ID != 1 {
		t.Errorf("equipment = %+v, want id=1 (вложенная сущность)", a.Equipment)
	}
}

func TestAssignmentService_Create_InvalidTarget(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())

	_, err := svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 1, TargetType: "employee",
	})
	if !errors.Is(err, domain.ErrInvalidTarget) {
		t.Errorf("err = %v, want ErrInvalidTarget", err)
	}
}

func TestAssignmentService_Get(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())
	created, _ := svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 5, TargetType: "warehouse",
	})

	a, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() ошибка: %v", err)
	}
	if a.ID != created.ID || a.EquipmentID != 5 {
		t.Errorf("assignment = %+v", a)
	}
}

func TestAssignmentService_Get_NotFound(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())

	if _, err := svc.Get(context.Background(), 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestAssignmentService_Get_InvalidID(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())

	if _, err := svc.Get(context.Background(), 0); !errors.Is(err, domain.ErrInvalidID) {
		t.Errorf("err = %v, want ErrInvalidID", err)
	}
}

func TestAssignmentService_List_Filter(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())
	_, _ = svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 1, TargetType: "employee", CardNumber: ptrInt(101),
	})
	// Второе закрепление сразу снимается (unassigned_at задан при создании).
	_, _ = svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 2, TargetType: "warehouse",
		AssignedAt:   ptrTime(time.Now().Add(-2 * time.Hour)),
		UnassignedAt: ptrTime(time.Now()),
	})

	active := true
	items, err := svc.List(context.Background(), domain.AssignmentFilter{Active: &active})
	if err != nil {
		t.Fatalf("List(active) ошибка: %v", err)
	}
	if len(items) != 1 || items[0].EquipmentID != 1 {
		t.Errorf("active: %+v", items)
	}

	target := domain.TargetWarehouse
	items, err = svc.List(context.Background(), domain.AssignmentFilter{TargetType: &target})
	if err != nil {
		t.Fatalf("List(target) ошибка: %v", err)
	}
	if len(items) != 1 || items[0].EquipmentID != 2 {
		t.Errorf("warehouse: %+v", items)
	}
}

func TestAssignmentService_Update(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())
	created, _ := svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 1, TargetType: "employee", CardNumber: ptrInt(101),
	})

	comment := "передано в ПОГЗ"
	a, err := svc.Update(context.Background(), created.ID, domain.AssignmentUpdate{
		OperatorComment: &comment,
	})
	if err != nil {
		t.Fatalf("Update() ошибка: %v", err)
	}
	if a.OperatorComment == nil || *a.OperatorComment != comment {
		t.Errorf("operator_comment = %v", a.OperatorComment)
	}
	if a.CardNumber == nil || *a.CardNumber != 101 {
		t.Errorf("card_number = %v, want 101 (не изменился)", a.CardNumber)
	}
}

func TestAssignmentService_Update_InvalidTargetKeepsData(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())
	created, _ := svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 1, TargetType: "employee", CardNumber: ptrInt(101),
	})

	// Смена на warehouse без очистки card_number нарушает chk_target.
	target := "warehouse"
	if _, err := svc.Update(context.Background(), created.ID, domain.AssignmentUpdate{
		TargetType: &target,
	}); !errors.Is(err, domain.ErrInvalidTarget) {
		t.Fatalf("err = %v, want ErrInvalidTarget", err)
	}
	// Запись в репозитории не изменилась.
	a, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() ошибка: %v", err)
	}
	if a.TargetType != domain.TargetEmployee {
		t.Errorf("target = %q, want employee (без изменений)", a.TargetType)
	}
}

func TestAssignmentService_Update_NotFound(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())

	if _, err := svc.Update(context.Background(), 999, domain.AssignmentUpdate{}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestAssignmentService_Delete(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())
	created, _ := svc.Create(context.Background(), domain.AssignmentInput{
		EquipmentID: 1, TargetType: "warehouse",
	})

	a, err := svc.Delete(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Delete() ошибка: %v", err)
	}
	if a.ID != created.ID {
		t.Errorf("удалённая запись = %+v", a)
	}
	if _, err := svc.Get(context.Background(), created.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("после удаления: err = %v, want ErrNotFound", err)
	}
}

func TestAssignmentService_Delete_NotFound(t *testing.T) {
	svc := NewAssignmentService(newFakeAssignmentRepo())

	if _, err := svc.Delete(context.Background(), 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
