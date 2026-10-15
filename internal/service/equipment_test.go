package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// fakeEquipmentRepo — in-memory реализация repository.EquipmentRepository
// для тестов сервиса без БД.
type fakeEquipmentRepo struct {
	items     map[int]*domain.Equipment
	nextID    int
	byKey     map[string]int // инвентарный номер → id
	createErr error

	// softDeleteCalls считает обращения к SoftDelete (проверка идемпотентности).
	softDeleteCalls int
	// existsCalls считает обращения к ExistsInventoryNumber.
	existsCalls int
}

func newFakeEquipmentRepo() *fakeEquipmentRepo {
	return &fakeEquipmentRepo{
		items:  map[int]*domain.Equipment{},
		byKey:  map[string]int{},
		nextID: 1,
	}
}

// seed добавляет готовую запись в хранилище, возвращая её id.
func (f *fakeEquipmentRepo) seed(eq *domain.Equipment) int {
	eq.ID = f.nextID
	f.nextID++
	f.items[eq.ID] = eq
	f.byKey[eq.InventoryNumber] = eq.ID
	return eq.ID
}

func (f *fakeEquipmentRepo) Create(_ context.Context, eq *domain.Equipment) error {
	if f.createErr != nil {
		return f.createErr
	}
	eq.ID = f.nextID
	f.nextID++
	f.items[eq.ID] = eq
	f.byKey[eq.InventoryNumber] = eq.ID
	return nil
}

func (f *fakeEquipmentRepo) GetByID(_ context.Context, id int) (*domain.Equipment, error) {
	eq, ok := f.items[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *eq
	return &cp, nil
}

func (f *fakeEquipmentRepo) GetByInventoryNumber(_ context.Context, number string) (*domain.Equipment, error) {
	id, ok := f.byKey[number]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *f.items[id]
	return &cp, nil
}

func (f *fakeEquipmentRepo) ListActive(_ context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	var out []*domain.Equipment
	for _, eq := range f.items {
		if eq.DeletedAt != nil || !matchFilter(eq, filter) {
			continue
		}
		cp := *eq
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeEquipmentRepo) ListInactive(_ context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	var out []*domain.Equipment
	for _, eq := range f.items {
		if eq.DeletedAt == nil || !matchFilter(eq, filter) {
			continue
		}
		cp := *eq
		out = append(out, &cp)
	}
	return out, nil
}

// ExistsInventoryNumber считает обращения и сообщает о наличии номера.
func (f *fakeEquipmentRepo) ExistsInventoryNumber(_ context.Context, number string) (bool, error) {
	f.existsCalls++
	_, ok := f.byKey[number]
	return ok, nil
}

// Update сохраняет обновлённую модель по id (копию не создаёт — сервис
// и так работает с загруженной копией).
func (f *fakeEquipmentRepo) Update(_ context.Context, id int, eq *domain.Equipment) error {
	if _, ok := f.items[id]; !ok {
		return domain.ErrNotFound
	}
	f.items[id] = eq
	f.byKey[eq.InventoryNumber] = id
	return nil
}

// SoftDelete заполняет deleted_at (идемпотентно, как COALESCE в SQL).
func (f *fakeEquipmentRepo) SoftDelete(_ context.Context, id int) error {
	f.softDeleteCalls++
	eq, ok := f.items[id]
	if !ok {
		return domain.ErrNotFound
	}
	if eq.DeletedAt == nil {
		now := time.Now()
		eq.DeletedAt = &now
	}
	return nil
}

// matchFilter имитирует фильтрацию: ILIKE %...% по строкам и точное
// совпадение по статусу, кодам и номерам.
func matchFilter(eq *domain.Equipment, f domain.EquipmentFilter) bool {
	if f.InventoryNumber != nil && !strings.Contains(strings.ToLower(eq.InventoryNumber), strings.ToLower(*f.InventoryNumber)) {
		return false
	}
	if f.SerialNumber != nil && (eq.SerialNumber == nil || !strings.Contains(strings.ToLower(*eq.SerialNumber), strings.ToLower(*f.SerialNumber))) {
		return false
	}
	if f.ModelName != nil && (eq.ModelName == nil || !strings.Contains(strings.ToLower(*eq.ModelName), strings.ToLower(*f.ModelName))) {
		return false
	}
	if f.Status != nil && eq.Status != *f.Status {
		return false
	}
	if f.Location != nil && (eq.Location == nil || !strings.Contains(strings.ToLower(*eq.Location), strings.ToLower(*f.Location))) {
		return false
	}
	if f.NomenclatureCode != nil && (eq.NomenclatureCode == nil || *f.NomenclatureCode != *eq.NomenclatureCode) {
		return false
	}
	if f.NomenclatureName != nil && (eq.NomenclatureName == nil || !strings.Contains(strings.ToLower(*eq.NomenclatureName), strings.ToLower(*f.NomenclatureName))) {
		return false
	}
	if f.CardNumber != nil && (eq.CardNumber == nil || *f.CardNumber != *eq.CardNumber) {
		return false
	}
	if f.CardFullName != nil && (eq.CardFullName == nil || !strings.Contains(strings.ToLower(*eq.CardFullName), strings.ToLower(*f.CardFullName))) {
		return false
	}
	if f.DepartmentCode != nil && (eq.DepartmentCode == nil || *f.DepartmentCode != *eq.DepartmentCode) {
		return false
	}
	if f.DepartmentName != nil && (eq.DepartmentName == nil || !strings.Contains(strings.ToLower(*eq.DepartmentName), strings.ToLower(*f.DepartmentName))) {
		return false
	}
	return true
}

// equipmentWithRelations — тестовая запись оборудования со сводными полями
// (номенклатура, карточка, подразделение) для проверки фильтров списков.
func equipmentWithRelations() *domain.Equipment {
	eq, _ := domain.NewEquipment(domain.EquipmentInput{InventoryNumber: "ИТ00042"})
	eq.NomenclatureCode = strPtrTest("08.01.02.00.00/0041")
	eq.NomenclatureName = strPtrTest("Ноутбук")
	eq.CardNumber = intPtrTest(101)
	eq.CardFullName = strPtrTest("Иванов")
	eq.DepartmentCode = int16PtrTest(100)
	eq.DepartmentName = strPtrTest("опк Гомель")
	return eq
}

// strPtrTest — локальный хелпер указателя на строку.
func strPtrTest(v string) *string { return &v }

// intPtrTest — локальный хелпер указателя на int.
func intPtrTest(v int) *int { return &v }

// int16PtrTest — локальный хелпер указателя на int16.
func int16PtrTest(v int16) *int16 { return &v }

func TestEquipmentService_Create(t *testing.T) {
	repo := newFakeEquipmentRepo()
	svc := NewEquipmentService(repo)

	eq, err := svc.Create(context.Background(), domain.EquipmentInput{InventoryNumber: "ИТ00001"})
	if err != nil {
		t.Fatalf("Create() ошибка: %v", err)
	}
	if eq.ID == 0 {
		t.Errorf("ID не присвоен")
	}
	if eq.Status != domain.StatusExp {
		t.Errorf("Status = %q, want exp", eq.Status)
	}
	if repo.existsCalls != 1 {
		t.Errorf("ExistsInventoryNumber вызван %d раз, want 1", repo.existsCalls)
	}
}

func TestEquipmentService_Create_Duplicate(t *testing.T) {
	repo := newFakeEquipmentRepo()
	repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	_, err := svc.Create(context.Background(), domain.EquipmentInput{InventoryNumber: "ИТ00001"})
	if errors.Is(err, domain.ErrAlreadyExists) == false {
		t.Errorf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestEquipmentService_Create_InvalidNumber(t *testing.T) {
	repo := newFakeEquipmentRepo()
	svc := NewEquipmentService(repo)

	_, err := svc.Create(context.Background(), domain.EquipmentInput{InventoryNumber: "123"})
	if errors.Is(err, domain.ErrInvalidInventoryNumber) == false {
		t.Errorf("err = %v, want ErrInvalidInventoryNumber", err)
	}
	if repo.existsCalls != 0 {
		t.Errorf("валидация должна сработать до обращения к репозиторию")
	}
}

func TestEquipmentService_GetByID(t *testing.T) {
	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	eq, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID() ошибка: %v", err)
	}
	if eq.ID != id {
		t.Errorf("ID = %d, want %d", eq.ID, id)
	}
}

func TestEquipmentService_GetByID_InvalidID(t *testing.T) {
	svc := NewEquipmentService(newFakeEquipmentRepo())

	if _, err := svc.GetByID(context.Background(), 0); errors.Is(err, domain.ErrInvalidID) == false {
		t.Errorf("id=0: err = %v, want ErrInvalidID", err)
	}
	if _, err := svc.GetByID(context.Background(), 999); errors.Is(err, domain.ErrNotFound) == false {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestEquipmentService_GetByInventoryNumber(t *testing.T) {
	repo := newFakeEquipmentRepo()
	repo.seed(&domain.Equipment{InventoryNumber: "ИТ00042", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	eq, err := svc.GetByInventoryNumber(context.Background(), "ИТ00042")
	if err != nil {
		t.Fatalf("GetByInventoryNumber() ошибка: %v", err)
	}
	if eq.InventoryNumber != "ИТ00042" {
		t.Errorf("InventoryNumber = %q", eq.InventoryNumber)
	}

	if _, err := svc.GetByInventoryNumber(context.Background(), "12345"); errors.Is(err, domain.ErrInvalidInventoryNumber) == false {
		t.Errorf("невалидный номер: err = %v, want ErrInvalidInventoryNumber", err)
	}
	if _, err := svc.GetByInventoryNumber(context.Background(), "ИТ99999"); errors.Is(err, domain.ErrNotFound) == false {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestEquipmentService_ListsSplitByDeletedAt(t *testing.T) {
	repo := newFakeEquipmentRepo()
	repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	deleted := &domain.Equipment{InventoryNumber: "ИТ00002", Status: domain.StatusExp}
	repo.seed(deleted)
	if err := repo.SoftDelete(context.Background(), deleted.ID); err != nil {
		t.Fatalf("seed SoftDelete: %v", err)
	}
	svc := NewEquipmentService(repo)

	active, err := svc.ListActive(context.Background(), domain.EquipmentFilter{})
	if err != nil {
		t.Fatalf("ListActive() ошибка: %v", err)
	}
	if len(active) != 1 || active[0].InventoryNumber != "ИТ00001" {
		t.Errorf("ListActive: %+v, want только ИТ00001", active)
	}

	inactive, err := svc.ListInactive(context.Background(), domain.EquipmentFilter{})
	if err != nil {
		t.Fatalf("ListInactive() ошибка: %v", err)
	}
	if len(inactive) != 1 || inactive[0].InventoryNumber != "ИТ00002" {
		t.Errorf("ListInactive: %+v, want только ИТ00002", inactive)
	}
}

func TestEquipmentService_UpdateByID(t *testing.T) {
	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	model := "Новая модель"
	eq, err := svc.UpdateByID(context.Background(), id, domain.EquipmentUpdate{ModelName: &model})
	if err != nil {
		t.Fatalf("UpdateByID() ошибка: %v", err)
	}
	if eq.ModelName == nil || *eq.ModelName != "Новая модель" {
		t.Errorf("ModelName = %v", eq.ModelName)
	}
	stored, _ := repo.GetByID(context.Background(), id)
	if stored.ModelName == nil || *stored.ModelName != "Новая модель" {
		t.Errorf("репозиторий не обновился: %v", stored.ModelName)
	}
}

func TestEquipmentService_UpdateByID_Errors(t *testing.T) {
	svc := NewEquipmentService(newFakeEquipmentRepo())
	bad := "нет"

	if _, err := svc.UpdateByID(context.Background(), 0, domain.EquipmentUpdate{Status: &bad}); errors.Is(err, domain.ErrInvalidID) == false {
		t.Errorf("id=0: err = %v, want ErrInvalidID", err)
	}
	if _, err := svc.UpdateByID(context.Background(), 999, domain.EquipmentUpdate{Status: &bad}); errors.Is(err, domain.ErrNotFound) == false {
		t.Errorf("err = %v, want ErrNotFound", err)
	}

	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	svc = NewEquipmentService(repo)
	if _, err := svc.UpdateByID(context.Background(), id, domain.EquipmentUpdate{Status: &bad}); errors.Is(err, domain.ErrInvalidStatus) == false {
		t.Errorf("err = %v, want ErrInvalidStatus", err)
	}
}

func TestEquipmentService_UpdateByInventoryNumber(t *testing.T) {
	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00042", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	serial := "SN-7"
	eq, err := svc.UpdateByInventoryNumber(context.Background(), "ИТ00042", domain.EquipmentUpdate{SerialNumber: &serial})
	if err != nil {
		t.Fatalf("UpdateByInventoryNumber() ошибка: %v", err)
	}
	if eq.ID != id || eq.SerialNumber == nil || *eq.SerialNumber != "SN-7" {
		t.Errorf("eq = %+v", eq)
	}
}

func TestEquipmentService_DeleteByID_SetsDeletedAt(t *testing.T) {
	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	eq, err := svc.DeleteByID(context.Background(), id)
	if err != nil {
		t.Fatalf("DeleteByID() ошибка: %v", err)
	}
	if eq.DeletedAt == nil {
		t.Fatalf("DeletedAt не заполнен")
	}
	if repo.softDeleteCalls != 1 {
		t.Errorf("SoftDelete вызван %d раз, want 1", repo.softDeleteCalls)
	}
}

func TestEquipmentService_DeleteByID_Idempotent(t *testing.T) {
	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00001", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)
	ctx := context.Background()

	if _, err := svc.DeleteByID(ctx, id); err != nil {
		t.Fatalf("первый DeleteByID() ошибка: %v", err)
	}
	// Метка удаления живёт в репозитории (в БД — NOW()); сервис на копии
	// выставляет собственное время, поэтому идемпотентность проверяем
	// по состоянию хранилища.
	stored, err := repo.GetByID(ctx, id)
	if err != nil || stored.DeletedAt == nil {
		t.Fatalf("хранилище: %v / DeletedAt=%v", err, stored.DeletedAt)
	}
	first := *stored.DeletedAt

	if _, err := svc.DeleteByID(ctx, id); err != nil {
		t.Fatalf("повторный DeleteByID() ошибка: %v", err)
	}
	if repo.softDeleteCalls != 1 {
		t.Errorf("повторный вызов не должен обращаться к SoftDelete: %d", repo.softDeleteCalls)
	}
	stored, err = repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("повторное чтение: %v", err)
	}
	if !stored.DeletedAt.Equal(first) {
		t.Errorf("метка удаления перезаписана: %v → %v", first, *stored.DeletedAt)
	}
}

func TestEquipmentService_DeleteByID_Errors(t *testing.T) {
	svc := NewEquipmentService(newFakeEquipmentRepo())
	if _, err := svc.DeleteByID(context.Background(), 0); errors.Is(err, domain.ErrInvalidID) == false {
		t.Errorf("id=0: err = %v, want ErrInvalidID", err)
	}
	if _, err := svc.DeleteByID(context.Background(), 999); errors.Is(err, domain.ErrNotFound) == false {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestEquipmentService_DeleteByInventoryNumber(t *testing.T) {
	repo := newFakeEquipmentRepo()
	id := repo.seed(&domain.Equipment{InventoryNumber: "ИТ00042", Status: domain.StatusExp})
	svc := NewEquipmentService(repo)

	eq, err := svc.DeleteByInventoryNumber(context.Background(), "ИТ00042")
	if err != nil {
		t.Fatalf("DeleteByInventoryNumber() ошибка: %v", err)
	}
	if eq.ID != id || eq.DeletedAt == nil {
		t.Errorf("eq = %+v", eq)
	}
	if _, err := svc.DeleteByInventoryNumber(context.Background(), "ИТ99999"); errors.Is(err, domain.ErrNotFound) == false {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestEquipmentService_ListActive_Filters(t *testing.T) {
	repo := newFakeEquipmentRepo()
	eq := equipmentWithRelations()
	repo.seed(eq)
	svc := NewEquipmentService(repo)
	ctx := context.Background()

	check := func(name string, f domain.EquipmentFilter, wantLen int) {
		got, err := svc.ListActive(ctx, f)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if len(got) != wantLen {
			t.Errorf("%s: len(got) = %d, want %d", name, len(got), wantLen)
		}
	}

	check("NomenclatureCode", domain.EquipmentFilter{NomenclatureCode: strPtrTest("08.01.02.00.00/0041")}, 1)
	check("NomenclatureName", domain.EquipmentFilter{NomenclatureName: strPtrTest("Ноутбук")}, 1)
	check("CardNumber", domain.EquipmentFilter{CardNumber: intPtrTest(101)}, 1)
	check("CardFullName", domain.EquipmentFilter{CardFullName: strPtrTest("Иван")}, 1)
	check("DepartmentCode", domain.EquipmentFilter{DepartmentCode: int16PtrTest(100)}, 1)
	check("DepartmentName", domain.EquipmentFilter{DepartmentName: strPtrTest("Гомель")}, 1)
	status := domain.StatusExp
	check("Status", domain.EquipmentFilter{Status: &status}, 1)
}

func TestEquipmentService_ListActive_Filters_Empty(t *testing.T) {
	repo := newFakeEquipmentRepo()
	eq := equipmentWithRelations()
	repo.seed(eq)
	svc := NewEquipmentService(repo)
	ctx := context.Background()

	got, err := svc.ListActive(ctx, domain.EquipmentFilter{NomenclatureCode: strPtrTest("000000")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}
