package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NomenclatureHandler — HTTP-слой чтения справочника номенклатуры.
// Зависит только от интерфейса NomenclatureService.
type NomenclatureHandler struct {
	svc service.NomenclatureService
	log *slog.Logger
}

// NewNomenclatureHandler создаёт обработчик номенклатуры.
func NewNomenclatureHandler(svc service.NomenclatureService, log *slog.Logger) *NomenclatureHandler {
	return &NomenclatureHandler{svc: svc, log: log}
}

// nomenclatureResponse — JSON-представление записи номенклатуры.
type nomenclatureResponse struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// toNomenclatureResponse преобразует доменную модель в JSON-ответ.
func toNomenclatureResponse(n *domain.Nomenclature) nomenclatureResponse {
	return nomenclatureResponse{ID: n.ID, Code: n.Code, Name: n.Name}
}

// toNomenclatureResponses конвертирует срез доменных моделей в срез JSON-ответов.
func toNomenclatureResponses(items []*domain.Nomenclature) []nomenclatureResponse {
	resp := make([]nomenclatureResponse, 0, len(items))
	for _, n := range items {
		resp = append(resp, toNomenclatureResponse(n))
	}
	return resp
}

// parseNomenclatureFilter извлекает фильтр из query-параметров ?code=...&name=...
// Пустые или отсутствующие параметры не фильтруют соответствующее поле.
func parseNomenclatureFilter(r *http.Request) domain.NomenclatureFilter {
	var f domain.NomenclatureFilter
	q := r.URL.Query()
	if v := q.Get("code"); v != "" {
		f.Code = &v
	}
	if v := q.Get("name"); v != "" {
		f.Name = &v
	}
	return f
}

// List обрабатывает GET /api/v1/nomenclatures — список с фильтрацией.
func (h *NomenclatureHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), parseNomenclatureFilter(r))
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toNomenclatureResponses(items))
}

// Get обрабатывает GET /api/v1/nomenclatures/{id}.
func (h *NomenclatureHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	item, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toNomenclatureResponse(item))
}

// parseID извлекает идентификатор из пути и валидирует его.
// При ошибке записывает ответ 400 и возвращает ok=false.
func (h *NomenclatureHandler) parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid id in path")
		return 0, false
	}
	return id, true
}

// respondDomainError маппит доменные ошибки на HTTP-статусы:
//   - ErrNotFound  → 404;
//   - ErrInvalidID → 400;
//   - прочее       → 500 (+ логирование).
func (h *NomenclatureHandler) respondDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidID):
		respondError(w, r, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("nomenclature handler error", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}
