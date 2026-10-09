package handlers

import (
	"encoding/json/v2"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

type errorResponce struct {
	Error     string `json:"error"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.MarshalWrite(w, data); err != nil {
			slog.Error("encode responce", "error", err)
		}
	}
}

func respondError(w http.ResponseWriter, r *http.Request, status int, message string) {
	respondJSON(w, status, errorResponce{
		Error: http.StatusText(status),
		Message: message,
		RequestID: chimw.GetReqID(r.Context()),
	})
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusNotFound, "route not found")
}

func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, http.StatusMethodNotAllowed, "method not allowed")
}
