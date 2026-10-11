package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newClosedPool создаёт pgxpool, который не подключается к БД, но сразу
// закрывается: Ping() на таком пуле возвращает ошибку без сетевых вызовов.
func newClosedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@127.0.0.1:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New(): %v", err)
	}
	pool.Close()
	return pool
}

func TestNewHealthHandler(t *testing.T) {
	pool := newClosedPool(t)

	h := NewHealthHandler(pool)
	if h == nil {
		t.Fatal("NewHealthHandler() = nil")
	}
	if h.pool != pool {
		t.Error("NewHealthHandler() не сохранил переданный пул")
	}
}

func TestHealthHandler_Live(t *testing.T) {
	h := NewHealthHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr := httptest.NewRecorder()

	h.Live(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "Ok" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "Ok")
	}
}

func TestHealthHandler_Ready_DatabaseUnavailable(t *testing.T) {
	h := NewHealthHandler(newClosedPool(t))

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rr := httptest.NewRecorder()

	h.Ready(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if got := rr.Body.String(); got != "database not ready\n" {
		t.Errorf("body = %q, want %q", got, "database not ready\n")
	}
}
