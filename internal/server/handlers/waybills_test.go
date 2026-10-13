package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
)

// stubWaybillService — заглушка WaybillService для тестов хендлеров без БД.
type stubWaybillService struct {
	createResult *domain.Waybill
	createErr    error
	getResult    *domain.Waybill
	getErr       error
	listItems    []*domain.Waybill
	listErr      error

	lastFilter   domain.WaybillFilter
	lastName     string
	lastNumber   string
	lastIssueNil bool // Create получил nil-дату
}

func (s *stubWaybillService) Create(_ context.Context, name, number string, issueDate *time.Time) (*domain.Waybill, error) {
	s.lastName = name
	s.lastNumber = number
	s.lastIssueNil = issueDate == nil
	return s.createResult, s.createErr
}

func (s *stubWaybillService) Get(_ context.Context, _ int) (*domain.Waybill, error) {
	return s.getResult, s.getErr
}

func (s *stubWaybillService) List(_ context.Context, filter domain.WaybillFilter) ([]*domain.Waybill, error) {
	s.lastFilter = filter
	return s.listItems, s.listErr
}

// reqWithID определён в nomenclatures_test.go (общий пакет handlers).

func waybill123() *domain.Waybill {
	num := "123/2026"
	date := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	return &domain.Waybill{ID: 1, Name: "Путевой лист", Number: &num, IssueDate: &date}
}

func TestWaybillHandler_Create(t *testing.T) {
	stub := &stubWaybillService{createResult: waybill123()}
	h := NewWaybillHandler(stub, discardLogger())

	body := `{"name":"Путевой лист","number":"123/2026","issue_date":"2026-10-13"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/waybills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if stub.lastName != "Путевой лист" || stub.lastNumber != "123/2026" || stub.lastIssueNil {
		t.Errorf("сервис получил name=%q number=%q issueNil=%v", stub.lastName, stub.lastNumber, stub.lastIssueNil)
	}
}

func TestWaybillHandler_Create_OptionalFieldsOmitted(t *testing.T) {
	stub := &stubWaybillService{createResult: &domain.Waybill{ID: 2, Name: "Без опций"}}
	h := NewWaybillHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/waybills", bytes.NewBufferString(`{"name":"Без опций"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if stub.lastNumber != "" || !stub.lastIssueNil {
		t.Errorf("number=%q issueNil=%v, want пустой и nil", stub.lastNumber, stub.lastIssueNil)
	}
}

func TestWaybillHandler_Create_InvalidIssueDate(t *testing.T) {
	h := NewWaybillHandler(&stubWaybillService{}, discardLogger())

	body := `{"name":"X","issue_date":"13.10.2026"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/waybills", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestWaybillHandler_Create_Duplicate(t *testing.T) {
	stub := &stubWaybillService{createErr: domain.ErrAlreadyExists}
	h := NewWaybillHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/waybills", bytes.NewBufferString(`{"name":"X","number":"1/2026"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rr.Code)
	}
}

func TestWaybillHandler_List(t *testing.T) {
	stub := &stubWaybillService{listItems: []*domain.Waybill{waybill123()}}
	h := NewWaybillHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/waybills", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got []waybillResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].Number == nil || *got[0].Number != "123/2026" {
		t.Errorf("список = %+v", got)
	}
	if got[0].IssueDate == nil || *got[0].IssueDate != "2026-10-13" {
		t.Errorf("issue_date = %v, want 2026-10-13", got[0].IssueDate)
	}
}

func TestWaybillHandler_List_FilterFromQuery(t *testing.T) {
	stub := &stubWaybillService{}
	h := NewWaybillHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/waybills?name=лист&number=123&issue_date_from=2026-10-01&issue_date_to=2026-10-31", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	f := stub.lastFilter
	if f.Name == nil || *f.Name != "лист" || f.Number == nil || *f.Number != "123" {
		t.Errorf("строковые фильтры: %+v", f)
	}
	if f.IssueDateFrom == nil || f.IssueDateTo == nil {
		t.Fatal("границы дат = nil")
	}
	if got := f.IssueDateFrom.Format("2006-01-02"); got != "2026-10-01" {
		t.Errorf("from = %s", got)
	}
	if got := f.IssueDateTo.Format("2006-01-02"); got != "2026-10-31" {
		t.Errorf("to = %s", got)
	}
}

func TestWaybillHandler_List_FromAfterTo(t *testing.T) {
	h := NewWaybillHandler(&stubWaybillService{}, discardLogger())

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/waybills?issue_date_from=2026-10-31&issue_date_to=2026-10-01", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestWaybillHandler_List_InvalidDate(t *testing.T) {
	h := NewWaybillHandler(&stubWaybillService{}, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/waybills?issue_date_from=garbage", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestWaybillHandler_Get(t *testing.T) {
	stub := &stubWaybillService{getResult: waybill123()}
	h := NewWaybillHandler(stub, discardLogger())

	req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/waybills/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestWaybillHandler_Get_NotFound(t *testing.T) {
	stub := &stubWaybillService{getErr: domain.ErrNotFound}
	h := NewWaybillHandler(stub, discardLogger())

	req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/waybills/999", nil), "999")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestWaybillHandler_Get_InvalidID(t *testing.T) {
	h := NewWaybillHandler(&stubWaybillService{}, discardLogger())

	for _, raw := range []string{"abc", "0"} {
		req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/waybills/"+raw, nil), raw)
		rr := httptest.NewRecorder()

		h.Get(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("id=%q: status = %d, want 400", raw, rr.Code)
		}
	}
}

func TestWaybillHandler_InternalError(t *testing.T) {
	stub := &stubWaybillService{getErr: errors.New("db down")}
	h := NewWaybillHandler(stub, discardLogger())

	req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/waybills/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
