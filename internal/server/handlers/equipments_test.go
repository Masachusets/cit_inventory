package handlers

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/go-chi/chi/v5"
)

// stubEquipmentService — заглушка EquipmentService для тестов хендлеров без БД.
type stubEquipmentService struct {
	createResult *domain.Equipment
	createErr    error
	getResult    *domain.Equipment
	getErr       error
	getByInv     *domain.Equipment
	getByInvErr  error
	listActive   []*domain.Equipment
	listInactive []*domain.Equipment
	listErr      error
	updateResult *domain.Equipment
	updateErr    error
	updateByInv  *domain.Equipment
	updateInvErr error
	deleteErr    error
	deleteInvErr error

	// последние переданные аргументы для проверки корректности трансляции
	lastInput  domain.EquipmentInput
	lastUpdate domain.EquipmentUpdate
	lastFilter domain.EquipmentFilter
}

func (s *stubEquipmentService) Create(_ context.Context, in domain.EquipmentInput) (*domain.Equipment, error) {
	s.lastInput = in
	return s.createResult, s.createErr
}

func (s *stubEquipmentService) GetByID(_ context.Context, _ int) (*domain.Equipment, error) {
	return s.getResult, s.getErr
}

func (s *stubEquipmentService) GetByInventoryNumber(_ context.Context, _ string) (*domain.Equipment, error) {
	return s.getByInv, s.getByInvErr
}

func (s *stubEquipmentService) ListActive(_ context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	s.lastFilter = filter
	return s.listActive, s.listErr
}

func (s *stubEquipmentService) ListInactive(_ context.Context, filter domain.EquipmentFilter) ([]*domain.Equipment, error) {
	s.lastFilter = filter
	return s.listInactive, s.listErr
}

func (s *stubEquipmentService) UpdateByID(_ context.Context, _ int, up domain.EquipmentUpdate) (*domain.Equipment, error) {
	s.lastUpdate = up
	return s.updateResult, s.updateErr
}

func (s *stubEquipmentService) UpdateByInventoryNumber(_ context.Context, _ string, up domain.EquipmentUpdate) (*domain.Equipment, error) {
	s.lastUpdate = up
	return s.updateByInv, s.updateInvErr
}

func (s *stubEquipmentService) DeleteByID(_ context.Context, _ int) (*domain.Equipment, error) {
	return &domain.Equipment{}, s.deleteErr
}

func (s *stubEquipmentService) DeleteByInventoryNumber(_ context.Context, _ string) (*domain.Equipment, error) {
	return &domain.Equipment{}, s.deleteInvErr
}

// discardEquipmentLogger — логгер, отбрасывающий записи (общий discardLogger
// объявлен в cards_test.go, здесь не используется, чтобы не полагаться на
// порядок файлов).
func discardEquipmentLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// activeEquipment — активное оборудование для тестов.
func activeEquipment() *domain.Equipment {
	eq, _ := domain.NewEquipment(domain.EquipmentInput{InventoryNumber: "ИТ00042"})
	eq.ID = 1
	return eq
}

// reqWithEquipmentParam внедряет chi route context с указанным path-параметром
// (при прямом вызове хендлера URLParam() пуст).
func reqWithEquipmentParam(req *http.Request, name, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(name, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestEquipmentHandler_Create(t *testing.T) {
	stub := &stubEquipmentService{createResult: activeEquipment()}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	body := `{"inventory_number":"ИТ00042","serial_number":"SN-1","nomenclature_id":5,` +
		`"model_name":"Ноутбук","manufacture_date":"2015-03-10","arrival_date":"2012-08-01",` +
		`"status":"broken","form_number":"Ф-1","location":"каб. 101","notes":"прим."}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/equipments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201 (тело: %s)", rr.Code, rr.Body)
	}
	if stub.lastInput.InventoryNumber != "ИТ00042" || stub.lastInput.Status != "broken" {
		t.Errorf("input = %+v", stub.lastInput)
	}
	if stub.lastInput.ManufactureDate == nil || stub.lastInput.ManufactureDate.Format(dateFormat) != "2015-03-10" {
		t.Errorf("ManufactureDate = %v", stub.lastInput.ManufactureDate)
	}
}

func TestEquipmentHandler_Create_InvalidBody(t *testing.T) {
	h := NewEquipmentHandler(&stubEquipmentService{}, discardEquipmentLogger())

	// Мусор вместо JSON.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/equipments", bytes.NewBufferString("{"))
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("мусорный JSON: status = %d, want 400", rr.Code)
	}

	// Некорректная дата.
	body := `{"inventory_number":"ИТ00042","arrival_date":"08.2012"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/equipments", bytes.NewBufferString(body))
	rr = httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("некорректная дата: status = %d, want 400", rr.Code)
	}
}

func TestEquipmentHandler_GetByID(t *testing.T) {
	stub := &stubEquipmentService{getResult: activeEquipment()}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	req := reqWithEquipmentParam(httptest.NewRequest(http.MethodGet, "/api/v1/equipments/1", nil), "id", "1")
	rr := httptest.NewRecorder()
	h.GetByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestEquipmentHandler_GetByID_Errors(t *testing.T) {
	cases := []struct {
		name     string
		param    string
		stub     *stubEquipmentService
		wantCode int
	}{
		{"невалидный id", "abc", &stubEquipmentService{}, http.StatusBadRequest},
		{"не найдено", "5", &stubEquipmentService{getErr: domain.ErrNotFound}, http.StatusNotFound},
		{"внутренняя ошибка", "1", &stubEquipmentService{getErr: errInternal}, http.StatusInternalServerError},
	}
	for _, c := range cases {
		h := NewEquipmentHandler(c.stub, discardEquipmentLogger())
		req := reqWithEquipmentParam(httptest.NewRequest(http.MethodGet, "/api/v1/equipments/"+c.param, nil), "id", c.param)
		rr := httptest.NewRecorder()
		h.GetByID(rr, req)
		if rr.Code != c.wantCode {
			t.Errorf("%s: status = %d, want %d", c.name, rr.Code, c.wantCode)
		}
	}
}

// errInternal — произвольная не-доменная ошибка для проверки ветки 500.
var errInternal = errors.New("db down")

func TestEquipmentHandler_GetByInventoryNumber(t *testing.T) {
	stub := &stubEquipmentService{getByInv: activeEquipment()}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	req := reqWithEquipmentParam(httptest.NewRequest(http.MethodGet, "/api/v1/equipments/inventory/ИТ00042", nil),
		"inventory_number", "ИТ00042")
	rr := httptest.NewRecorder()
	h.GetByInventoryNumber(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestEquipmentHandler_GetByInventoryNumber_InvalidNumber(t *testing.T) {
	h := NewEquipmentHandler(&stubEquipmentService{}, discardEquipmentLogger())

	req := reqWithEquipmentParam(httptest.NewRequest(http.MethodGet, "/api/v1/equipments/inventory/12345", nil),
		"inventory_number", "12345")
	rr := httptest.NewRecorder()
	h.GetByInventoryNumber(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEquipmentHandler_UpdateByID(t *testing.T) {
	stub := &stubEquipmentService{updateResult: activeEquipment()}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	// Частичное обновление: model_name + очистка nullable-даты пустой строкой.
	body := `{"model_name":"Новая модель","arrival_date":""}`
	req := reqWithEquipmentParam(
		httptest.NewRequest(http.MethodPatch, "/api/v1/equipments/1", bytes.NewBufferString(body)), "id", "1")
	rr := httptest.NewRecorder()
	h.UpdateByID(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (тело: %s)", rr.Code, rr.Body)
	}
	if stub.lastUpdate.ModelName == nil || *stub.lastUpdate.ModelName != "Новая модель" {
		t.Errorf("ModelName = %v", stub.lastUpdate.ModelName)
	}
	if stub.lastUpdate.ArrivalDate == nil || !stub.lastUpdate.ArrivalDate.IsZero() {
		t.Errorf("пустая дата должна обнулиться (сигнал очистки): %v", stub.lastUpdate.ArrivalDate)
	}
}

func TestEquipmentHandler_UpdateByID_InvalidStatus(t *testing.T) {
	h := NewEquipmentHandler(&stubEquipmentService{updateErr: domain.ErrInvalidStatus}, discardEquipmentLogger())

	body := `{"status":"нет"}`
	req := reqWithEquipmentParam(
		httptest.NewRequest(http.MethodPatch, "/api/v1/equipments/1", bytes.NewBufferString(body)), "id", "1")
	rr := httptest.NewRecorder()
	h.UpdateByID(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEquipmentHandler_UpdateByInventoryNumber(t *testing.T) {
	stub := &stubEquipmentService{updateByInv: activeEquipment()}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	body := `{"serial_number":"SN-9"}`
	req := reqWithEquipmentParam(
		httptest.NewRequest(http.MethodPatch, "/api/v1/equipments/inventory/ИТ00042", bytes.NewBufferString(body)),
		"inventory_number", "ИТ00042")
	rr := httptest.NewRecorder()
	h.UpdateByInventoryNumber(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if stub.lastUpdate.SerialNumber == nil || *stub.lastUpdate.SerialNumber != "SN-9" {
		t.Errorf("SerialNumber = %v", stub.lastUpdate.SerialNumber)
	}
}

func TestEquipmentHandler_Delete(t *testing.T) {
	h := NewEquipmentHandler(&stubEquipmentService{}, discardEquipmentLogger())

	req := reqWithEquipmentParam(httptest.NewRequest(http.MethodDelete, "/api/v1/equipments/1", nil), "id", "1")
	rr := httptest.NewRecorder()
	h.DeleteByID(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("по id: status = %d, want 204", rr.Code)
	}

	hInv := NewEquipmentHandler(&stubEquipmentService{deleteInvErr: domain.ErrNotFound}, discardEquipmentLogger())
	req = reqWithEquipmentParam(httptest.NewRequest(http.MethodDelete, "/api/v1/equipments/inventory/ИТ00042", nil),
		"inventory_number", "ИТ00042")
	rr = httptest.NewRecorder()
	hInv.DeleteByInventoryNumber(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("по номеру: status = %d, want 404", rr.Code)
	}
}

func TestEquipmentHandler_List(t *testing.T) {
	stub := &stubEquipmentService{listActive: []*domain.Equipment{activeEquipment()}}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/equipments?inventory_number=ИТ0004&status=exp&location=каб", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if stub.lastFilter.InventoryNumber == nil || *stub.lastFilter.InventoryNumber != "ИТ0004" {
		t.Errorf("filter.InventoryNumber = %v", stub.lastFilter.InventoryNumber)
	}
	if stub.lastFilter.Status == nil || *stub.lastFilter.Status != domain.StatusExp {
		t.Errorf("filter.Status = %v", stub.lastFilter.Status)
	}
	if stub.lastFilter.Location == nil || *stub.lastFilter.Location != "каб" {
		t.Errorf("filter.Location = %v", stub.lastFilter.Location)
	}
}

func TestEquipmentHandler_List_InvalidStatus(t *testing.T) {
	h := NewEquipmentHandler(&stubEquipmentService{}, discardEquipmentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/equipments?status=нет", nil)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestEquipmentHandler_ListInactive(t *testing.T) {
	stub := &stubEquipmentService{listInactive: []*domain.Equipment{activeEquipment()}}
	h := NewEquipmentHandler(stub, discardEquipmentLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/equipments/inactive", nil)
	rr := httptest.NewRecorder()
	h.ListInactive(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
