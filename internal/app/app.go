package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Masachusets/cit_inventory/internal/server"
	"github.com/Masachusets/cit_inventory/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// defaultShutdownTimeout применяется, если в конфиге не задан Server.ShutdownTimeout.
const defaultShutdownTimeout = 15 * time.Second

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// shutdownTimeout возвращает таймаут graceful shutdown из конфига.
// Если значение не задано (нулевое/отрицательное) — используется defaultShutdownTimeout.
func shutdownTimeout(cfg *config.Config) time.Duration {
	if cfg.Server.ShutdownTimeout > 0 {
		return cfg.Server.ShutdownTimeout
	}
	return defaultShutdownTimeout
}

// runApp инициализирует и запускает HTTP-сервер StockWise.
func Run(ctx context.Context, cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	// create HTTP-server
	srv := server.NewServer(cfg, log, pool)

	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", srv.Addr, err)
	}

	log.Info("http server listening", "addr", listener.Addr().String())

	srvErrCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(listener); err != nil {
			srvErrCh <- err
		}
	}()

	// Ожидание сигнала или ошибки сервера
	select {
	case <-ctx.Done():
		log.Info("received shutdown signal", "reason", ctx.Err())
	case err := <-srvErrCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
		log.Info("server stopped unexpectedly (graceful)")
	}

	// Graceful shutdown
	timeout := shutdownTimeout(cfg)
	log.Info("starting graceful shutdown", "timeout", timeout)

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		timeout,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info("server stopped gracefully")

	return nil
}
