package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/go-chi/chi/v5"
)

// stubNomenclatureService — заглушка NomenclatureService для тестов хендлеров без БД.
type stubNomenclatureService struct {
	getResult *domain.Nomenclature
	getErr    error
	listItems []*domain.Nomenclature
	listErr   error

	lastFilter domain.NomenclatureFilter
}

func (s *stubNomenclatureService) Get(_ context.Context, _ int) (*domain.Nomenclature, error) {
	return s.getResult, s.getErr
}

func (s *stubNomenclatureService) List(_ context.Context, filter domain.NomenclatureFilter) ([]*domain.Nomenclature, error) {
	s.lastFilter = filter
	return s.listItems, s.listErr
}

// reqWithID внедряет chi route context с path-параметром {id}.
func reqWithID(req *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func laptop() *domain.Nomenclature {
	return &domain.Nomenclature{ID: 1, Code: "08.01.02.00.00/0041", Name: "Ноутбук Clevo 15"}
}

func TestNomenclatureHandler_List(t *testing.T) {
	stub := &stubNomenclatureService{listItems: []*domain.Nomenclature{laptop()}}
	h := NewNomenclatureHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got []nomenclatureResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].Code != "08.01.02.00.00/0041" {
		t.Errorf("список = %+v", got)
	}
}

func TestNomenclatureHandler_List_FilterFromQuery(t *testing.T) {
	stub := &stubNomenclatureService{}
	h := NewNomenclatureHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures?code=0041&name=ноутбук", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if stub.lastFilter.Code == nil || *stub.lastFilter.Code != "0041" {
		t.Errorf("filter.Code = %v, want 0041", stub.lastFilter.Code)
	}
	if stub.lastFilter.Name == nil || *stub.lastFilter.Name != "ноутбук" {
		t.Errorf("filter.Name = %v, want ноутбук", stub.lastFilter.Name)
	}
}

func TestNomenclatureHandler_List_EmptyFilterParamsIgnored(t *testing.T) {
	stub := &stubNomenclatureService{}
	h := NewNomenclatureHandler(stub, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures?code=&name=", nil)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if stub.lastFilter.Code != nil || stub.lastFilter.Name != nil {
		t.Errorf("пустые параметры не должны попадать в фильтр: %+v", stub.lastFilter)
	}
}

func TestNomenclatureHandler_Get(t *testing.T) {
	stub := &stubNomenclatureService{getResult: laptop()}
	h := NewNomenclatureHandler(stub, discardLogger())

	req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	var got nomenclatureResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if got.ID != 1 || got.Code != "08.01.02.00.00/0041" {
		t.Errorf("ответ = %+v", got)
	}
}

func TestNomenclatureHandler_Get_NotFound(t *testing.T) {
	stub := &stubNomenclatureService{getErr: domain.ErrNotFound}
	h := NewNomenclatureHandler(stub, discardLogger())

	req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures/999", nil), "999")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

func TestNomenclatureHandler_Get_InvalidID(t *testing.T) {
	stub := &stubNomenclatureService{}
	h := NewNomenclatureHandler(stub, discardLogger())

	for _, raw := range []string{"abc", "0", "-1"} {
		req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures/"+raw, nil), raw)
		rr := httptest.NewRecorder()

		h.Get(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("id=%q: status = %d, want 400", raw, rr.Code)
		}
	}
}

func TestNomenclatureHandler_InternalError(t *testing.T) {
	stub := &stubNomenclatureService{getErr: errors.New("db down")}
	h := NewNomenclatureHandler(stub, discardLogger())

	req := reqWithID(httptest.NewRequest(http.MethodGet, "/api/v1/nomenclatures/1", nil), "1")
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}
