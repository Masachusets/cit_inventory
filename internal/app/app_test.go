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

	"github.com/Masachusets/cit_inventory/internal/config"
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
