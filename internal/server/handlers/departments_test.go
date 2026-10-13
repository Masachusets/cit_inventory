package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/go-chi/chi/v5"
)

// stubDepartmentService — заглушка DepartmentService для тестов хендлеров без БД.
type stubDepartmentService struct {
	createResult *domain.Department
	createErr    error
	getResult    *domain.Department
	getErr       error
	listActive   []*domain.Department
	listInactive []*domain.Department
	listErr      error
	updateResult *domain.Department
	updateErr    error
	deactResult  *domain.Department
	deactErr     error

	lastFilter domain.DepartmentFilter
	lastName   string
}

func (s *stubDepartmentService) Create(_ context.Context, _ int, name string) (*domain.Department, error) {
	s.lastName = name
	return s.createResult, s.createErr
}

func (s *stubDepartmentService) Get(_ context.Context, _ int) (*domain.Department, error) {
	return s.getResult, s.getErr
}

func (s *stubDepartmentService) ListActive(_ context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	s.lastFilter = filter
	return s.listActive, s.listErr
}

func (s *stubDepartmentService) ListInactive(_ context.Context, filter domain.DepartmentFilter) ([]*domain.Department, error) {
	s.lastFilter = filter
	return s.listInactive, s.listErr
}

func (s *stubDepartmentService) UpdateName(_ context.Context, _ int, name string) (*domain.Department, error) {
	s.lastName = name
	return s.updateResult, s.updateErr
}

func (s *stubDepartmentService) Deactivate(_ context.Context, _ int) (*domain.Department, error) {
	return s.deactResult, s.deactErr
}

// reqWithCode внедряет chi route context с path-параметром {code}.
func reqWithCode(req *http.Request, code string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", code)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func activeDepartment() *domain.Department {
	d, _ := domain.NewDepartment(301, "опк Гомель")
	return d
}

func TestDepartmentHandler_Create(t *testing.T) {
	stub := &stubDepartmentService{createResult: activeDepartment()}
	h := NewDepartmentHandler(stub, discardLogger())

	body := `{"code":301,"name":"опк Гомель"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/departments", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if stub.lastName != "опк Гомель" {
		t.Errorf("сервис получил name=%q", stub.lastName)
	}
}

func TestDepartmentHandler_Create_Duplicate(t *testing.T) {
	stub := &stubDepartmentService{createErr: domain.ErrAlreadyExists}
	h := NewDepartmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/departments", bytes.NewBufferString(`{"code":100,"name":"X"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rr.Code)
	}
}

func TestDepartmentHandler_List(t *testing.T) {
	stub := &stubDepartmentService{listActive: []*domain.Department{activeDepartment()}}
	h := NewDepartmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/departments", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got []departmentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if len(got) != 1 || got[0].Code != 301 || !got[0].IsActive {
		t.Errorf("список = %+v", got)
	}
}

func TestDepartmentHandler_List_FilterFromQuery(t *testing.T) {
	stub := &stubDepartmentService{}
	h := NewDepartmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/departments?name=гомель&created_at_from=2026-10-01&created_at_to=2026-10-31", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if stub.lastFilter.Name == nil || *stub.lastFilter.Name != "гомель" {
		t.Errorf("filter.Name = %v", stub.lastFilter.Name)
	}
	if stub.lastFilter.CreatedAtFrom == nil || stub.lastFilter.CreatedAtTo == nil {
		t.Fatal("границы CreatedAtFrom/CreatedAtTo = nil")
	}
	if got := stub.lastFilter.CreatedAtFrom.Format("2006-01-02"); got != "2026-10-01" {
		t.Errorf("filter.CreatedAtFrom = %s, want 2026-10-01", got)
	}
	if got := stub.lastFilter.CreatedAtTo.Format("2006-01-02"); got != "2026-10-31" {
		t.Errorf("filter.CreatedAtTo = %s, want 2026-10-31", got)
	}
}

func TestDepartmentHandler_List_InvalidCreatedAt(t *testing.T) {
	h := NewDepartmentHandler(&stubDepartmentService{}, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/departments?created_at_from=13.10.2026", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestDepartmentHandler_List_FromAfterTo(t *testing.T) {
	h := NewDepartmentHandler(&stubDepartmentService{}, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/departments?created_at_from=2026-10-31&created_at_to=2026-10-01", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestDepartmentHandler_ListInactive(t *testing.T) {
	stub := &stubDepartmentService{listInactive: []*domain.Department{activeDepartment()}}
	h := NewDepartmentHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/departments/inactive", nil)
	rr := httptest.NewRecorder()

	h.ListInactive(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got []departmentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("список = %+v", got)
	}
}

func TestDepartmentHandler_Get(t *testing.T) {
	stub := &stubDepartmentService{getResult: activeDepartment()}
	h := NewDepartmentHandler(stub, discardLogger())

	req := reqWithCode(httptest.NewRequest(http.MethodGet, "/api/v1/departments/301", nil), "301")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got departmentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if got.Code != 301 || got.Name != "опк Гомель" {
		t.Errorf("ответ = %+v", got)
	}
	if len(got.CreatedAt) != len("2006-01-02") {
		t.Errorf("created_at = %q, want YYYY-MM-DD", got.CreatedAt)
	}
}

func TestDepartmentHandler_Get_NotFound(t *testing.T) {
	stub := &stubDepartmentService{getErr: domain.ErrNotFound}
	h := NewDepartmentHandler(stub, discardLogger())

	req := reqWithCode(httptest.NewRequest(http.MethodGet, "/api/v1/departments/999", nil), "999")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestDepartmentHandler_Get_InvalidCode(t *testing.T) {
	h := NewDepartmentHandler(&stubDepartmentService{}, discardLogger())

	for _, raw := range []string{"abc", "0"} {
		req := reqWithCode(httptest.NewRequest(http.MethodGet, "/api/v1/departments/"+raw, nil), raw)
		rr := httptest.NewRecorder()

		h.Get(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("code=%q: status = %d, want 400", raw, rr.Code)
		}
	}
}

func TestDepartmentHandler_Update(t *testing.T) {
	dept := activeDepartment()
	stub := &stubDepartmentService{updateResult: dept}
	h := NewDepartmentHandler(stub, discardLogger())

	req := reqWithCode(httptest.NewRequest(http.MethodPatch, "/api/v1/departments/301",
		bytes.NewBufferString(`{"name":"Новое Имя"}`)), "301")
	rr := httptest.NewRecorder()

	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if stub.lastName != "Новое Имя" {
		t.Errorf("сервис получил name=%q", stub.lastName)
	}
}

func TestDepartmentHandler_Deactivate(t *testing.T) {
	dept := activeDepartment()
	dept.Deactivate()
	stub := &stubDepartmentService{deactResult: dept}
	h := NewDepartmentHandler(stub, discardLogger())

	req := reqWithCode(httptest.NewRequest(http.MethodPost, "/api/v1/departments/301/deactivate", nil), "301")
	rr := httptest.NewRecorder()

	h.Deactivate(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got departmentResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if got.IsActive {
		t.Errorf("ответ = %+v, want is_active=false", got)
	}
}

func TestDepartmentHandler_InternalError(t *testing.T) {
	stub := &stubDepartmentService{getErr: errors.New("db down")}
	h := NewDepartmentHandler(stub, discardLogger())

	req := reqWithCode(httptest.NewRequest(http.MethodGet, "/api/v1/departments/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
