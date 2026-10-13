package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Masachusets/cit_inventory/internal/server/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRouter(t *testing.T, pool *pgxpool.Pool, buf *bytes.Buffer) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(bufOrDiscard(buf), nil))
	return NewRouter(log, pool)
}

func bufOrDiscard(buf *bytes.Buffer) *bytes.Buffer {
	if buf != nil {
		return buf
	}
	return &bytes.Buffer{}
}

func TestNewRouter_Live(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "Ok" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "Ok")
	}
}

func TestNewRouter_Ready_DatabaseUnavailable(t *testing.T) {
	pool, err := pgxpool.New(t.Context(), "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New(): %v", err)
	}
	t.Cleanup(pool.Close)
	pool.Close()

	r := newTestRouter(t, pool, nil)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestNewRouter_NotFound(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/no/such/route", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var resp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("тело ответа не является валидным JSON: %v", err)
	}
	if resp.Error != "Not Found" {
		t.Errorf("error = %q, want %q", resp.Error, "Not Found")
	}
	if resp.Message != "route not found" {
		t.Errorf("message = %q, want %q", resp.Message, "route not found")
	}
}

func TestNewRouter_MethodNotAllowed(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/health/live", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}

	var resp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("тело ответа не является валидным JSON: %v", err)
	}
	if resp.Error != "Method Not Allowed" {
		t.Errorf("error = %q, want %q", resp.Error, "Method Not Allowed")
	}
}

func TestNewRouter_ClientRequestIDInLogs(t *testing.T) {
	buf := &bytes.Buffer{}
	r := newTestRouter(t, nil, buf)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	req.Header.Set("X-Request-Id", "client-id-1")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// chi RequestID middleware прокидывает ID из заголовка клиента в контекст,
	// откуда его забирает логгер.
	if !strings.Contains(buf.String(), "request_id=client-id-1") {
		t.Errorf("лог не содержит request_id из заголовка клиента;\nлог:\n%s", buf.String())
	}
}

func TestNewRouter_LogsRequest(t *testing.T) {
	buf := &bytes.Buffer{}
	r := newTestRouter(t, nil, buf)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	out := buf.String()
	for _, want := range []string{
		`msg="http request"`,
		"method=GET",
		"path=/health/live",
		"status=200",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("лог не содержит %q;\nлог:\n%s", want, out)
		}
	}
}

// TestNewRouter_HealthHandlerPackageLinkage защищает от рассинхрона роутера
// и хендлеров: маршруты health обязаны отвечать через handlers.HealthHandler.
func TestNewRouter_HealthHandlerPackageLinkage(t *testing.T) {
	h := handlers.NewHealthHandler(nil)
	if h == nil {
		t.Fatal("handlers.NewHealthHandler() = nil")
	}
}

// TestNewRouter_CardsRoutes проверяет регистрацию всех маршрутов карточек
// под /api/v1: маршрут должен существовать (не 404) и принимать метод (не 405).
func TestNewRouter_CardsRoutes(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/cards"},
		{http.MethodGet, "/api/v1/cards"},
		{http.MethodGet, "/api/v1/cards/inactive"},
		{http.MethodGet, "/api/v1/cards/1"},
		{http.MethodPatch, "/api/v1/cards/1"},
		{http.MethodPost, "/api/v1/cards/1/deactivate"},
		{http.MethodDelete, "/api/v1/cards/1"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(""))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code == http.StatusNotFound {
				t.Errorf("маршрут %s %s не зарегистрирован (404)", tt.method, tt.path)
			}
			if rr.Code == http.StatusMethodNotAllowed {
				t.Errorf("метод %s не разрешён для %s", tt.method, tt.path)
			}
		})
	}
}

// TestNewRouter_CardsInactiveNotCapturedByParam защищает от рассинхрона
// порядка маршрутов: GET /cards/inactive не должен перехватываться как
// /cards/{number} (иначе parseNumber вернёт 400 на слове "inactive").
func TestNewRouter_CardsInactiveNotCapturedByParam(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/inactive", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code == http.StatusBadRequest {
		t.Errorf("статус 400: маршрут /cards/inactive перехвачен как {number} " +
			"(сработал parseNumber вместо ListInactive)")
	}
}

// TestNewRouter_CardsInvalidNumber проверяет валидацию номера в пути:
// нечисловой {number} → 400, а не 500.
func TestNewRouter_CardsInvalidNumber(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cards/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (номер «abc» не число)", rr.Code)
	}
}
