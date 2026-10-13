package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Masachusets/cit_inventory/pkg/config"
)

func TestStatusWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	sw.WriteHeader(http.StatusCreated)

	if sw.status != http.StatusCreated {
		t.Errorf("status = %d, want %d", sw.status, http.StatusCreated)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("recorder code = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestStatusWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	n, err := sw.Write([]byte("data"))

	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 4 {
		t.Errorf("Write() n = %d, want %d", n, 4)
	}
	if rec.Body.String() != "data" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "data")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("recorder code = %d, want %d", rec.Code, http.StatusOK)
	}
}

func testRunConfig(port string) *config.Config {
	cfg := &config.Config{}
	cfg.Server.Port = port
	return cfg
}

func TestShutdownTimeout(t *testing.T) {
	tests := []struct {
		name string
		cfg  time.Duration
		want time.Duration
	}{
		{name: "значение из конфига", cfg: 30 * time.Second, want: 30 * time.Second},
		{name: "миллисекунды из конфига", cfg: 250 * time.Millisecond, want: 250 * time.Millisecond},
		{name: "не задано — дефолт", cfg: 0, want: defaultShutdownTimeout},
		{name: "отрицательное — дефолт", cfg: -time.Second, want: defaultShutdownTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testRunConfig("0")
			cfg.Server.ShutdownTimeout = tt.cfg

			if got := shutdownTimeout(cfg); got != tt.want {
				t.Errorf("shutdownTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// listenNotifyHandler сигнализирует через канал, когда сервер начал слушать адрес.
type listenNotifyHandler struct {
	slog.Handler
	ch   chan struct{}
	once sync.Once
}

func (h *listenNotifyHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "http server listening" {
		h.once.Do(func() { close(h.ch) })
	}
	return h.Handler.Handle(ctx, r)
}

func TestRun_GracefulShutdown(t *testing.T) {
	notify := &listenNotifyHandler{
		Handler: slog.NewTextHandler(io.Discard, nil),
		ch:      make(chan struct{}),
	}
	log := slog.New(notify)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		// Пул не используется до обращения к /health/ready, поэтому nil допустим.
		errCh <- Run(ctx, testRunConfig("0"), log, nil)
	}()

	select {
	case <-notify.ch:
		// сервер начал слушать
	case err := <-errCh:
		t.Fatalf("Run() завершился раньше запуска: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("сервер не начал слушать адрес за 10s")
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Run() после отмены ctx вернул %v, want nil", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run() не завершился за 20s после отмены контекста")
	}
}

// TestRun_ShutdownTimeoutFromConfig проверяет, что Run берёт таймаут
// graceful shutdown из cfg.Server.ShutdownTimeout, а не из зашитой константы.
func TestRun_ShutdownTimeoutFromConfig(t *testing.T) {
	notify := &listenNotifyHandler{
		Handler: slog.NewTextHandler(io.Discard, nil),
		ch:      make(chan struct{}),
	}
	log := slog.New(notify)

	// Ищем свободный порт и освобождаем его — займёт Run.
	probe, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("net.Listen(): %v", err)
	}
	port := strconv.Itoa(probe.Addr().(*net.TCPAddr).Port)
	if err := probe.Close(); err != nil {
		t.Fatalf("probe.Close(): %v", err)
	}

	cfg := testRunConfig(port)
	cfg.Server.ShutdownTimeout = 300 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- Run(ctx, cfg, log, nil) }()

	select {
	case <-notify.ch:
	case err := <-errCh:
		t.Fatalf("Run() завершился раньше запуска: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("сервер не начал слушать адрес за 10s")
	}

	// Незавершённый запрос: сервер блокируется на чтении заголовков,
	// соединение считается активным и Shutdown ждёт до таймаута из конфига.
	conn, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		t.Fatalf("net.Dial(): %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte("GET /health/live HTTP/1.1\r\n")); err != nil {
		t.Fatalf("conn.Write(): %v", err)
	}

	// Даём серверу принять соединение и начать читать запрос.
	time.Sleep(300 * time.Millisecond)

	start := time.Now()
	cancel()

	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("Run() error = nil, want deadline exceeded из cfg.Server.ShutdownTimeout")
		}
		if !strings.Contains(err.Error(), "server shutdown") {
			t.Errorf("err = %q, want it to contain %q", err.Error(), "server shutdown")
		}
		if elapsed < 100*time.Millisecond {
			t.Errorf("shutdown занял %v — похоже, таймаут из конфига не применён", elapsed)
		}
		if elapsed > 3*time.Second {
			t.Errorf("shutdown занял %v, want около %v", elapsed, cfg.Server.ShutdownTimeout)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run() не завершился за 10s после отмены контекста")
	}
}

func TestRun_ListenError(t *testing.T) {
	// Занимаем порт, чтобы Listen внутри Run завершился ошибкой.
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("net.Listen(): %v", err)
	}
	defer func() { _ = l.Close() }()

	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)

	runErr := Run(context.Background(), testRunConfig(port), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	if runErr == nil {
		t.Fatal("Run() error = nil, want listen error")
	}
	want := "listen on :" + port
	if !strings.Contains(runErr.Error(), want) {
		t.Errorf("err = %q, want it to contain %q", runErr.Error(), want)
	}
}
