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

// dateFormat — формат даты DATE-колонок в JSON и query-параметрах.
const dateFormat = "2006-01-02"

// DepartmentHandler — HTTP-слой для работы с справочником подразделений.
// Зависит только от интерфейса DepartmentService.
type DepartmentHandler struct {
	svc service.DepartmentService
	log *slog.Logger
}

// NewDepartmentHandler создаёт обработчик подразделений.
func NewDepartmentHandler(svc service.DepartmentService, log *slog.Logger) *DepartmentHandler {
	return &DepartmentHandler{svc: svc, log: log}
}

// createDepartmentRequest — тело POST /api/v1/departments.
type createDepartmentRequest struct {
	Code int    `json:"code"`
	Name string `json:"name"`
}

// updateDepartmentRequest — тело PATCH /api/v1/departments/{code}.
type updateDepartmentRequest struct {
	Name string `json:"name"`
}

// departmentResponse — JSON-представление подразделения.
type departmentResponse struct {
	Code      int16  `json:"code"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// toDepartmentResponse преобразует доменную модель в JSON-ответ.
// created_at — DATE-колонка, сериализуется как YYYY-MM-DD.
func toDepartmentResponse(d *domain.Department) departmentResponse {
	return departmentResponse{
		Code:      d.Code,
		Name:      d.Name,
		IsActive:  d.IsActive,
		CreatedAt: d.CreatedAt.Format(dateFormat),
	}
}

// toDepartmentResponses конвертирует срез доменных моделей в срез JSON-ответов.
func toDepartmentResponses(items []*domain.Department) []departmentResponse {
	resp := make([]departmentResponse, 0, len(items))
	for _, d := range items {
		resp = append(resp, toDepartmentResponse(d))
	}
	return resp
}

// parseDepartmentFilter извлекает фильтр из query-параметров
// ?name=...&created_at_from=YYYY-MM-DD&created_at_to=YYYY-MM-DD.
// Пустые или отсутствующие параметры не фильтруют соответствующее поле;
// некорректная дата или from > to — 400.
func parseDepartmentFilter(w http.ResponseWriter, r *http.Request) (domain.DepartmentFilter, bool) {
	var f domain.DepartmentFilter
	q := r.URL.Query()
	if v := q.Get("name"); v != "" {
		f.Name = &v
	}
	from, ok := parseDateParam(w, r, "created_at_from")
	if !ok {
		return f, false
	}
	f.CreatedAtFrom = from
	to, ok := parseDateParam(w, r, "created_at_to")
	if !ok {
		return f, false
	}
	f.CreatedAtTo = to
	if from != nil && to != nil && from.After(*to) {
		respondError(w, r, http.StatusBadRequest, "created_at_from must not be after created_at_to")
		return f, false
	}
	return f, true
}

// parseDateParam парсит query-параметр с датой YYYY-MM-DD.
// Пустой/отсутствующий параметр → (nil, true); мусор → ошибка 400.
func parseDateParam(w http.ResponseWriter, r *http.Request, param string) (*time.Time, bool) {
	v := r.URL.Query().Get(param)
	if v == "" {
		return nil, true
	}
	t, err := time.Parse(dateFormat, v)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid "+param+", want YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}

// List обрабатывает GET /api/v1/departments — только активные.
func (h *DepartmentHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseDepartmentFilter(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListActive(r.Context(), filter)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toDepartmentResponses(items))
}

// ListInactive обрабатывает GET /api/v1/departments/inactive — только неактивные.
func (h *DepartmentHandler) ListInactive(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseDepartmentFilter(w, r)
	if !ok {
		return
	}

	items, err := h.svc.ListInactive(r.Context(), filter)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toDepartmentResponses(items))
}

// Get обрабатывает GET /api/v1/departments/{code}.
func (h *DepartmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	code, ok := h.parseCode(w, r)
	if !ok {
		return
	}

	dept, err := h.svc.Get(r.Context(), code)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toDepartmentResponse(dept))
}

// Create обрабатывает POST /api/v1/departments.
func (h *DepartmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDepartmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	dept, err := h.svc.Create(r.Context(), req.Code, req.Name)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, toDepartmentResponse(dept))
}

// Update обрабатывает PATCH /api/v1/departments/{code} — изменение только name.
func (h *DepartmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	code, ok := h.parseCode(w, r)
	if !ok {
		return
	}

	var req updateDepartmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	dept, err := h.svc.UpdateName(r.Context(), code, req.Name)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toDepartmentResponse(dept))
}

// Deactivate обрабатывает POST /api/v1/departments/{code}/deactivate.
func (h *DepartmentHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	code, ok := h.parseCode(w, r)
	if !ok {
		return
	}

	dept, err := h.svc.Deactivate(r.Context(), code)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toDepartmentResponse(dept))
}

// parseCode извлекает код подразделения из пути и валидирует положительность.
// При ошибке записывает ответ 400 и возвращает ok=false.
func (h *DepartmentHandler) parseCode(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "code")
	code, err := strconv.Atoi(raw)
	if err != nil || code <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid department code in path")
		return 0, false
	}
	return code, true
}

// respondDomainError маппит доменные ошибки на HTTP-статусы:
//   - ErrNotFound                → 404;
//   - ErrAlreadyExists           → 409;
//   - ErrInvalidCode/InvalidName → 400;
//   - прочее                     → 500 (+ логирование).
func (h *DepartmentHandler) respondDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		respondError(w, r, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCode), errors.Is(err, domain.ErrInvalidName):
		respondError(w, r, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("department handler error", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}
