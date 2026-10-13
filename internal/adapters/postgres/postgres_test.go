package postgres

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/pkg/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.DB.MaxConnections = 2
	cfg.DB.MinConnections = 0
	cfg.DB.MaxConnLifetime = time.Minute
	cfg.DB.MaxConnIdleTime = 30 * time.Second
	return cfg
}

func TestNew_InvalidDSN(t *testing.T) {
	cfg := testConfig()
	cfg.DB.DSN = "://definitely-not-a-dsn"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := New(ctx, cfg, testLogger())

	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("New() error = nil, want parse error")
	}
	if pool != nil {
		t.Errorf("pool = %v, want nil", pool)
	}
	if !strings.Contains(err.Error(), "failed to parse database URL") {
		t.Errorf("err = %q, want it to contain %q", err.Error(), "failed to parse database URL")
	}
}

func TestNew_ConnectionRefused(t *testing.T) {
	cfg := testConfig()
	// Порт 1 на loopback, как правило, закрыт: соединение будет отклонено
	// практически мгновенно без внешней зависимости от БД.
	cfg.DB.DSN = "postgres://user:pass@127.0.0.1:1/db?sslmode=disable"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := New(ctx, cfg, testLogger())

	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("New() error = nil, want ping error")
	}
	if pool != nil {
		t.Errorf("pool = %v, want nil after failed ping", pool)
	}
	if !strings.Contains(err.Error(), "failed to ping database") {
		t.Errorf("err = %q, want it to contain %q", err.Error(), "failed to ping database")
	}
}
