package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// WaybillHandler — HTTP-слой для работы с путевыми листами.
// Зависит только от интерфейса WaybillService.
type WaybillHandler struct {
	svc service.WaybillService
	log *slog.Logger
}

// NewWaybillHandler создаёт обработчик путевых листов.
func NewWaybillHandler(svc service.WaybillService, log *slog.Logger) *WaybillHandler {
	return &WaybillHandler{svc: svc, log: log}
}

// createWaybillRequest — тело POST /api/v1/waybills.
// number и issue_date опциональны: пустые значения и null → NULL в БД.
type createWaybillRequest struct {
	Name      string  `json:"name"`
	Number    *string `json:"number"`
	IssueDate *string `json:"issue_date"`
}

// waybillResponse — JSON-представление путевого листа. number и issue_date
// могут быть null.
type waybillResponse struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Number    *string `json:"number"`
	IssueDate *string `json:"issue_date"`
}

// toWaybillResponse преобразует доменную модель в JSON-ответ.
// issue_date — DATE-колонка, сериализуется как YYYY-MM-DD либо null.
func toWaybillResponse(wb *domain.Waybill) waybillResponse {
	resp := waybillResponse{ID: wb.ID, Name: wb.Name, Number: wb.Number}
	if wb.IssueDate != nil {
		s := wb.IssueDate.Format(dateFormat)
		resp.IssueDate = &s
	}
	return resp
}

// toWaybillResponses конвертирует срез доменных моделей в срез JSON-ответов.
func toWaybillResponses(items []*domain.Waybill) []waybillResponse {
	resp := make([]waybillResponse, 0, len(items))
	for _, wb := range items {
		resp = append(resp, toWaybillResponse(wb))
	}
	return resp
}

// parseWaybillFilter извлекает фильтр из query-параметров
// ?name=...&number=...&issue_date_from=YYYY-MM-DD&issue_date_to=YYYY-MM-DD.
// Пустые или отсутствующие параметры не фильтруют соответствующее поле;
// некорректная дата или from > to — 400.
func parseWaybillFilter(w http.ResponseWriter, r *http.Request) (domain.WaybillFilter, bool) {
	var f domain.WaybillFilter
	q := r.URL.Query()
	if v := q.Get("name"); v != "" {
		f.Name = &v
	}
	if v := q.Get("number"); v != "" {
		f.Number = &v
	}
	from, ok := parseDateParam(w, r, "issue_date_from")
	if !ok {
		return f, false
	}
	f.IssueDateFrom = from
	to, ok := parseDateParam(w, r, "issue_date_to")
	if !ok {
		return f, false
	}
	f.IssueDateTo = to
	if from != nil && to != nil && from.After(*to) {
		respondError(w, r, http.StatusBadRequest, "issue_date_from must not be after issue_date_to")
		return f, false
	}
	return f, true
}

// List обрабатывает GET /api/v1/waybills — список с фильтрацией.
func (h *WaybillHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseWaybillFilter(w, r)
	if !ok {
		return
	}

	items, err := h.svc.List(r.Context(), filter)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toWaybillResponses(items))
}

// Get обрабатывает GET /api/v1/waybills/{id}.
func (h *WaybillHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	wb, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toWaybillResponse(wb))
}

// Create обрабатывает POST /api/v1/waybills.
func (h *WaybillHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createWaybillRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	issueDate, ok := parseOptionalDate(w, r, req.IssueDate, "issue_date")
	if !ok {
		return
	}

	wb, err := h.svc.Create(r.Context(), req.Name, derefReqString(req.Number), issueDate)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, toWaybillResponse(wb))
}

// parseID извлекает идентификатор из пути и валидирует его.
// При ошибке записывает ответ 400 и возвращает ok=false.
func (h *WaybillHandler) parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid id in path")
		return 0, false
	}
	return id, true
}

// respondDomainError маппит доменные ошибки на HTTP-статусы:
//   - ErrNotFound      → 404;
//   - ErrAlreadyExists → 409;
//   - ErrInvalidID/Name→ 400;
//   - прочее           → 500 (+ логирование).
func (h *WaybillHandler) respondDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		respondError(w, r, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidID), errors.Is(err, domain.ErrInvalidName):
		respondError(w, r, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("waybill handler error", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}

// parseOptionalDate парсит опциональную дату YYYY-MM-DD из тела запроса.
// nil/пустая строка → (nil, true); мусор → ошибка 400.
func parseOptionalDate(w http.ResponseWriter, r *http.Request, raw *string, field string) (*time.Time, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	t, err := time.Parse(dateFormat, *raw)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid "+field+", want YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}

// derefReqString возвращает значение опциональной строки или "" (пустой
// номер домен приводит к nil — NULL в БД).
func derefReqString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
