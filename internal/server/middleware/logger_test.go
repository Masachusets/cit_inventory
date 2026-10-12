package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	rw.WriteHeader(http.StatusForbidden)

	if rw.status != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rw.status, http.StatusForbidden)
	}
	if !rw.wroteHeader {
		t.Error("wroteHeader = false, want true")
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("recorder code = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestResponseWriter_WriteHeader_IgnoresSecondCall(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	rw.WriteHeader(http.StatusNotFound)
	rw.WriteHeader(http.StatusInternalServerError)

	if rw.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d (повторный WriteHeader должен игнорироваться)", rw.status, http.StatusNotFound)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("recorder code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestResponseWriter_Write_ImpliesOK(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec}

	n, err := rw.Write([]byte("hello"))

	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 5 {
		t.Errorf("Write() n = %d, want %d", n, 5)
	}
	if rw.status != http.StatusOK {
		t.Errorf("status = %d, want %d (Write без WriteHeader должен фиксировать 200)", rw.status, http.StatusOK)
	}
	if !rw.wroteHeader {
		t.Error("wroteHeader = false, want true")
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "hello")
	}
}

// newTestLogger возвращает логгер, пишущий в буфер, и сам буфер.
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

func TestLogger_CallsNextHandler(t *testing.T) {
	log, _ := newTestLogger()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodGet, "/tea", nil)
	rr := httptest.NewRecorder()

	Logger(log)(next).ServeHTTP(rr, req)

	if !called {
		t.Fatal("следующий обработчик не был вызван")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusTeapot)
	}
}

func TestLogger_LogsRequestAttributes(t *testing.T) {
	log, buf := newTestLogger()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("payload"))
	})

	req := httptest.NewRequest(http.MethodPost, "/items?limit=5", nil)
	req = req.WithContext(context.WithValue(req.Context(), chimw.RequestIDKey, "req-42"))
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("User-Agent", "test-agent")

	rr := httptest.NewRecorder()
	Logger(log)(next).ServeHTTP(rr, req)

	out := buf.String()

	for _, want := range []string{
		`msg="http request"`,
		"method=POST",
		"path=/items",
		`query="limit=5"`,
		"status=201",
		"bytes=7",
		`remote_addr=10.0.0.1:1234`,
		"user_agent=test-agent",
		"request_id=req-42",
		"level=INFO",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("лог не содержит %q;\nлог:\n%s", want, out)
		}
	}
}

func TestLogger_StatusBasedLevels(t *testing.T) {
	tests := []struct {
		name      string
		handler   func(w http.ResponseWriter, r *http.Request)
		wantLevel string
	}{
		{
			name:      "2xx логируется на уровне INFO",
			handler:   func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) },
			wantLevel: "level=INFO",
		},
		{
			name:      "4xx логируется на уровне WARN",
			handler:   func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) },
			wantLevel: "level=WARN",
		},
		{
			name:      "5xx логируется на уровне ERROR",
			handler:   func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
			wantLevel: "level=ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, buf := newTestLogger()

			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			rr := httptest.NewRecorder()

			Logger(log)(http.HandlerFunc(tt.handler)).ServeHTTP(rr, req)

			if !strings.Contains(buf.String(), tt.wantLevel) {
				t.Errorf("лог не содержит %q;\nлог:\n%s", tt.wantLevel, buf.String())
			}
		})
	}
}
