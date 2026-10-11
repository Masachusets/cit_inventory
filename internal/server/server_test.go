package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/internal/config"
)

func testServerConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.Port = "9090"
	cfg.Server.ReadTimeout = 5 * time.Second
	cfg.Server.WriteTimeout = 7 * time.Second
	cfg.Server.IdleTimeout = 90 * time.Second
	return cfg
}

func TestNewServer(t *testing.T) {
	log := slog.Default()

	srv := NewServer(testServerConfig(), log, nil)

	if srv == nil {
		t.Fatal("NewServer() = nil")
	}
	if srv.Addr != ":9090" {
		t.Errorf("Addr = %q, want %q", srv.Addr, ":9090")
	}
	if srv.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", srv.ReadTimeout, 5*time.Second)
	}
	if srv.WriteTimeout != 7*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", srv.WriteTimeout, 7*time.Second)
	}
	if srv.IdleTimeout != 90*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", srv.IdleTimeout, 90*time.Second)
	}
	if srv.Handler == nil {
		t.Error("Handler = nil, want router")
	}
	if srv.log != log {
		t.Error("логгер не был сохранён в Server")
	}
	if srv.pool != nil {
		t.Error("пул не был сохранён в Server (ожидался nil)")
	}
}

func TestNewServer_HandlerResponds(t *testing.T) {
	srv := NewServer(testServerConfig(), slog.Default(), nil)

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rr := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "Ok" {
		t.Errorf("body = %q, want %q", rr.Body.String(), "Ok")
	}
}
