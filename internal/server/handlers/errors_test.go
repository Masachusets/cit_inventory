package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

type errorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func decodeError(t *testing.T, body []byte) errorResponse {
	t.Helper()

	var resp errorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("не удалось разобрать JSON-ответ %q: %v", body, err)
	}
	return resp
}

func TestRespondJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	respondJSON(rr, http.StatusCreated, data)

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusCreated)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var got map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("тело ответа не является валидным JSON: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("JSON key = %q, want %q", got["key"], "value")
	}
}

func TestRespondJSON_NilData(t *testing.T) {
	rr := httptest.NewRecorder()

	respondJSON(rr, http.StatusNoContent, nil)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
	if rr.Body.Len() != 0 {
		t.Errorf("тело = %q, want пустое", rr.Body.String())
	}
}

func TestRespondError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), chimw.RequestIDKey, "req-123"))

	rr := httptest.NewRecorder()

	respondError(rr, req, http.StatusBadRequest, "bad request")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	resp := decodeError(t, rr.Body.Bytes())
	if resp.Error != http.StatusText(http.StatusBadRequest) {
		t.Errorf("error = %q, want %q", resp.Error, http.StatusText(http.StatusBadRequest))
	}
	if resp.Message != "bad request" {
		t.Errorf("message = %q, want %q", resp.Message, "bad request")
	}
	if resp.RequestID != "req-123" {
		t.Errorf("request_id = %q, want %q", resp.RequestID, "req-123")
	}
}

func TestNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()

	NotFound(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}

	resp := decodeError(t, rr.Body.Bytes())
	if resp.Error != "Not Found" {
		t.Errorf("error = %q, want %q", resp.Error, "Not Found")
	}
	if resp.Message != "route not found" {
		t.Errorf("message = %q, want %q", resp.Message, "route not found")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/health/live", nil)
	rr := httptest.NewRecorder()

	MethodNotAllowed(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}

	resp := decodeError(t, rr.Body.Bytes())
	if resp.Error != "Method Not Allowed" {
		t.Errorf("error = %q, want %q", resp.Error, "Method Not Allowed")
	}
	if resp.Message != "method not allowed" {
		t.Errorf("message = %q, want %q", resp.Message, "method not allowed")
	}
}
