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

// TestNewRouter_UIRoutes проверяет регистрацию страниц веб-интерфейса:
// маршрут должен существовать (не 404). Статус может быть 500 — в тестах
// pool=nil и обращения к БД не происходят/падают, важен факт роутинга.
func TestNewRouter_UIRoutes(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/ui"},
		{http.MethodGet, "/ui/"},
		{http.MethodGet, "/ui/equipments"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code == http.StatusNotFound && strings.HasPrefix(rr.Header().Get("Content-Type"), "text/html") {
				t.Errorf("%s %s вернул HTML-404 — маршрут страницы не зарегистрирован", tt.method, tt.path)
			}
			if rr.Code == http.StatusMethodNotAllowed {
				t.Errorf("метод %s не разрешён для %s", tt.method, tt.path)
			}
		})
	}
}

// TestNewRouter_UIHome проверяет главную страницу веб-интерфейса: статус,
// HTML-контент-тип и подключённый htmx.
func TestNewRouter_UIHome(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	for _, want := range []string{
		"<title>Главная · CIT Inventory</title>",
		`src="/static/htmx.min.js"`,
		"Оборудование",
	} {
		if !strings.Contains(rr.Body.String(), want) {
			t.Errorf("тело не содержит %q", want)
		}
	}
}

// TestNewRouter_UI404IsHTML проверяет, что несуществующий маршрут внутри /ui
// отвечает HTML-страницей 404, а не JSON-ошибкой API.
func TestNewRouter_UI404IsHTML(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/ui/no/such/page", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html (внутри /ui — HTML-404)", ct)
	}
	if !strings.Contains(rr.Body.String(), "/ui/no/such/page") {
		t.Errorf("тело не содержит запрошенный путь")
	}
}

// TestNewRouter_Static проверяет раздачу статики из go:embed: htmx и CSS
// доступны по /static/* с корректным контент-типом.
func TestNewRouter_Static(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	tests := []struct {
		path       string
		wantStatus int
		wantCT     string
	}{
		{"/static/htmx.min.js", http.StatusOK, "text/javascript"},
		{"/static/app.css", http.StatusOK, "text/css"},
		{"/static/no-such-file.js", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantCT != "" {
				if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, tt.wantCT) {
					t.Errorf("Content-Type = %q, want prefix %q", ct, tt.wantCT)
				}
			}
		})
	}
}

// TestNewRouter_RootRedirect проверяет, что корень перенаправляет в /ui.
func TestNewRouter_RootRedirect(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusFound)
	}
	if loc := rr.Header().Get("Location"); loc != "/ui" {
		t.Errorf("Location = %q, want %q", loc, "/ui")
	}
}

// TestNewRouter_APINotFoundStillJSON защищает от регрессии: маршруты вне /ui
// по-прежнему получают JSON-404 (API-совместимость не должна пострадать
// от веб-интерфейса).
func TestNewRouter_APINotFoundStillJSON(t *testing.T) {
	r := newTestRouter(t, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/no/such/route", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
