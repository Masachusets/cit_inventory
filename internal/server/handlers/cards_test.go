package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/go-chi/chi/v5"
)

// stubCardService — заглушка CardService для тестов хендлеров без БД.
// Позволяет задать возвращаемые значения и нужную ошибку.
type stubCardService struct {
	createResult *domain.Card
	createErr    error
	getResult    *domain.Card
	getErr       error
	listActive   []*domain.Card
	listInactive []*domain.Card
	listErr      error
	updateResult *domain.Card
	updateErr    error
	deactResult  *domain.Card
	deactErr     error
	deleteErr    error

	// счётчики для проверки, что хендлер передал правильные аргументы
	lastCreateNumber int
	lastCreateName   string
	lastUpdateNumber int
	lastUpdateName   string
	lastFilter       domain.CardFilter
}

func (s *stubCardService) Create(_ context.Context, number int, fullName string) (*domain.Card, error) {
	s.lastCreateNumber = number
	s.lastCreateName = fullName
	return s.createResult, s.createErr
}

func (s *stubCardService) Get(_ context.Context, _ int) (*domain.Card, error) {
	return s.getResult, s.getErr
}

func (s *stubCardService) ListActive(_ context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	s.lastFilter = filter
	return s.listActive, s.listErr
}

func (s *stubCardService) ListInactive(_ context.Context, filter domain.CardFilter) ([]*domain.Card, error) {
	s.lastFilter = filter
	return s.listInactive, s.listErr
}

func (s *stubCardService) UpdateName(_ context.Context, number int, fullName string) (*domain.Card, error) {
	s.lastUpdateNumber = number
	s.lastUpdateName = fullName
	return s.updateResult, s.updateErr
}

func (s *stubCardService) Deactivate(_ context.Context, _ int) (*domain.Card, error) {
	return s.deactResult, s.deactErr
}

func (s *stubCardService) Delete(_ context.Context, _ int) error {
	return s.deleteErr
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func activeCard() *domain.Card {
	c, _ := domain.NewCard(42, "Иванов Иван")
	return c
}

// reqWithNumber внедряет chi route context с path-параметром {number},
// т.к. при прямом вызове хендлера (без роутера) URLParam() пуст.
func reqWithNumber(req *http.Request, number string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("number", number)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCardHandler_Create(t *testing.T) {
	stub := &stubCardService{createResult: activeCard()}
	h := NewCardHandler(stub, discardLogger())

	body := `{"number":42,"full_name":"Иванов Иван"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if stub.lastCreateNumber != 42 || stub.lastCreateName != "Иванов Иван" {
		t.Errorf("сервис получил number=%d name=%q", stub.lastCreateNumber, stub.lastCreateName)
	}
}

func TestCardHandler_Create_InvalidJSON(t *testing.T) {
	h := NewCardHandler(&stubCardService{}, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cards", bytes.NewBufferString("{bad"))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestCardHandler_Create_Duplicate(t *testing.T) {
	stub := &stubCardService{createErr: domain.ErrAlreadyExists}
	h := NewCardHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/cards", bytes.NewBufferString(`{"number":1,"full_name":"X"}`))
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rr.Code)
	}
}

func TestCardHandler_List(t *testing.T) {
	stub := &stubCardService{listActive: []*domain.Card{activeCard()}}
	h := NewCardHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got []cardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if len(got) != 1 || got[0].Number != 42 {
		t.Errorf("список = %+v", got)
	}
}

func TestCardHandler_List_FilterFromQuery(t *testing.T) {
	stub := &stubCardService{}
	h := NewCardHandler(stub, discardLogger())

	// На /cards доступны full_name и диапазоны created_at/updated_at.
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/cards?full_name=иван&created_at_from=2026-10-01&created_at_to=2026-10-31"+
			"&updated_at_from=2026-01-01&updated_at_to=2026-12-31", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	f := stub.lastFilter
	if f.FullName == nil || *f.FullName != "иван" {
		t.Errorf("filter.FullName = %v", f.FullName)
	}
	if f.CreatedAtFrom == nil || f.CreatedAtFrom.Format("2006-01-02") != "2026-10-01" {
		t.Errorf("filter.CreatedAtFrom = %v, want 2026-10-01", f.CreatedAtFrom)
	}
	if f.CreatedAtTo == nil || f.CreatedAtTo.Format("2006-01-02") != "2026-10-31" {
		t.Errorf("filter.CreatedAtTo = %v, want 2026-10-31", f.CreatedAtTo)
	}
	if f.UpdatedAtFrom == nil || f.UpdatedAtFrom.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("filter.UpdatedAtFrom = %v, want 2026-01-01", f.UpdatedAtFrom)
	}
	if f.UpdatedAtTo == nil || f.UpdatedAtTo.Format("2006-01-02") != "2026-12-31" {
		t.Errorf("filter.UpdatedAtTo = %v, want 2026-12-31", f.UpdatedAtTo)
	}
	// Фильтр deleted_at на /cards не разбирается — поля остаются nil.
	if f.DeletedAtFrom != nil || f.DeletedAtTo != nil {
		t.Errorf("filter.DeletedAtFrom/To = %v/%v, want nil на /cards", f.DeletedAtFrom, f.DeletedAtTo)
	}
}

// TestCardHandler_List_DeletedAtRejected проверяет, что на /cards параметры
// deleted_at_* отсекаются как 400: у активных карточек deleted_at всегда NULL,
// фильтр по нему доступен только на /cards/inactive.
func TestCardHandler_List_DeletedAtRejected(t *testing.T) {
	h := NewCardHandler(&stubCardService{}, discardLogger())

	for _, q := range []string{
		"?deleted_at_from=2026-01-01",
		"?deleted_at_to=2026-12-31",
		"?deleted_at_from=2026-01-01&deleted_at_to=2026-12-31",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cards"+q, nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rr.Code)
		}
	}
}

func TestCardHandler_List_InvalidDate(t *testing.T) {
	h := NewCardHandler(&stubCardService{}, discardLogger())

	for _, q := range []string{
		"?created_at_from=13.10.2026",
		"?updated_at_to=not-a-date",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cards"+q, nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rr.Code)
		}
	}
}

func TestCardHandler_List_FromAfterTo(t *testing.T) {
	h := NewCardHandler(&stubCardService{}, discardLogger())

	for _, q := range []string{
		"?created_at_from=2026-10-31&created_at_to=2026-10-01",
		"?updated_at_from=2026-12-31&updated_at_to=2026-01-01",
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/cards"+q, nil)
		rr := httptest.NewRecorder()

		h.List(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", q, rr.Code)
		}
	}
}

func TestCardHandler_ListInactive_FilterFromQuery(t *testing.T) {
	stub := &stubCardService{}
	h := NewCardHandler(stub, discardLogger())

	// На /cards/inactive доступен и фильтр deleted_at.
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/cards/inactive?full_name=петр&deleted_at_from=2026-01-01&deleted_at_to=2026-12-31", nil)
	rr := httptest.NewRecorder()

	h.ListInactive(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if stub.lastFilter.FullName == nil || *stub.lastFilter.FullName != "петр" {
		t.Errorf("filter.FullName = %v", stub.lastFilter.FullName)
	}
	if stub.lastFilter.DeletedAtFrom == nil || stub.lastFilter.DeletedAtFrom.Format("2006-01-02") != "2026-01-01" {
		t.Errorf("filter.DeletedAtFrom = %v, want 2026-01-01", stub.lastFilter.DeletedAtFrom)
	}
	if stub.lastFilter.DeletedAtTo == nil || stub.lastFilter.DeletedAtTo.Format("2006-01-02") != "2026-12-31" {
		t.Errorf("filter.DeletedAtTo = %v, want 2026-12-31", stub.lastFilter.DeletedAtTo)
	}
}

func TestCardHandler_ListInactive_DeletedAtFromAfterTo(t *testing.T) {
	h := NewCardHandler(&stubCardService{}, discardLogger())

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/cards/inactive?deleted_at_from=2026-12-31&deleted_at_to=2026-01-01", nil)
	rr := httptest.NewRecorder()

	h.ListInactive(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestCardHandler_Get_NotFound(t *testing.T) {
	stub := &stubCardService{getErr: domain.ErrNotFound}
	h := NewCardHandler(stub, discardLogger())

	req := reqWithNumber(httptest.NewRequest(http.MethodGet, "/api/v1/cards/999", nil), "999")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestCardHandler_Delete_Active(t *testing.T) {
	stub := &stubCardService{deleteErr: domain.ErrStillActive}
	h := NewCardHandler(stub, discardLogger())

	req := reqWithNumber(httptest.NewRequest(http.MethodDelete, "/api/v1/cards/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rr.Code)
	}
}

func TestCardHandler_Delete_OK(t *testing.T) {
	h := NewCardHandler(&stubCardService{}, discardLogger())

	req := reqWithNumber(httptest.NewRequest(http.MethodDelete, "/api/v1/cards/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Delete(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
}

func TestCardHandler_InternalError(t *testing.T) {
	stub := &stubCardService{getErr: errors.New("db down")}
	h := NewCardHandler(stub, discardLogger())

	req := reqWithNumber(httptest.NewRequest(http.MethodGet, "/api/v1/cards/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

// TestCardHandler_InvalidNumber проверяет, что некорректный номер в пути
// отсекается ещё до обращения к сервису (400).
func TestCardHandler_InvalidNumber(t *testing.T) {
	stub := &stubCardService{}
	h := NewCardHandler(stub, discardLogger())

	req := reqWithNumber(httptest.NewRequest(http.MethodGet, "/api/v1/cards/abc", nil), "abc")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}
