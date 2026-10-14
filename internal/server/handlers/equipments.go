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

// EquipmentHandler — HTTP-слой для работы с оборудованием. Зависит только от
// интерфейса EquipmentService, поэтому легко тестируется подменной реализацией.
type EquipmentHandler struct {
	svc service.EquipmentService
	log *slog.Logger
}

// NewEquipmentHandler создаёт обработчик оборудования.
func NewEquipmentHandler(svc service.EquipmentService, log *slog.Logger) *EquipmentHandler {
	return &EquipmentHandler{svc: svc, log: log}
}

// createEquipmentRequest — тело POST /api/v1/equipments.
// inventory_number обязателен; остальные поля опциональны: null/отсутствие
// и пустая строка → NULL в БД. Даты — YYYY-MM-DD.
type createEquipmentRequest struct {
	InventoryNumber string  `json:"inventory_number"`
	SerialNumber    *string `json:"serial_number"`
	NomenclatureID  *int    `json:"nomenclature_id"`
	ModelName       *string `json:"model_name"`
	ManufactureDate *string `json:"manufacture_date"`
	ArrivalDate     *string `json:"arrival_date"`
	Status          *string `json:"status"`
	FormNumber      *string `json:"form_number"`
	Location        *string `json:"location"`
	Notes           *string `json:"notes"`
}

// updateEquipmentRequest — тело PATCH /api/v1/equipments/{id|inventory_number}.
// Частичное обновление: отсутствующее поле не меняется; пустая строка для
// nullable-текста и nomenclature_id <= 0 очищают колонку (NULL); пустая
// строка даты очищает колонку (NULL).
type updateEquipmentRequest struct {
	SerialNumber    *string `json:"serial_number"`
	NomenclatureID  *int    `json:"nomenclature_id"`
	ModelName       *string `json:"model_name"`
	ManufactureDate *string `json:"manufacture_date"`
	ArrivalDate     *string `json:"arrival_date"`
	Status          *string `json:"status"`
	FormNumber      *string `json:"form_number"`
	Location        *string `json:"location"`
	Notes           *string `json:"notes"`
}

// equipmentResponse — JSON-представление оборудования. Nullable-поля и
// deleted_at сериализуются как значение либо null; DATE-колонки — YYYY-MM-DD,
// технические метки — RFC3339.
type equipmentResponse struct {
	ID              int     `json:"id"`
	InventoryNumber string  `json:"inventory_number"`
	SerialNumber    *string `json:"serial_number"`
	NomenclatureID  *int    `json:"nomenclature_id"`
	ModelName       *string `json:"model_name"`
	ManufactureDate *string `json:"manufacture_date"`
	ArrivalDate     *string `json:"arrival_date"`
	Status          string  `json:"status"`
	FormNumber      *string `json:"form_number"`
	Location        *string `json:"location"`
	Notes           *string `json:"notes"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	DeletedAt       *string `json:"deleted_at"`
}

// toEquipmentResponse преобразует доменную модель в JSON-ответ.
func toEquipmentResponse(eq *domain.Equipment) equipmentResponse {
	resp := equipmentResponse{
		ID:              eq.ID,
		InventoryNumber: eq.InventoryNumber,
		SerialNumber:    eq.SerialNumber,
		NomenclatureID:  eq.NomenclatureID,
		ModelName:       eq.ModelName,
		Status:          string(eq.Status),
		FormNumber:      eq.FormNumber,
		Location:        eq.Location,
		Notes:           eq.Notes,
		CreatedAt:       eq.CreatedAt.Format(timeFormat),
		UpdatedAt:       eq.UpdatedAt.Format(timeFormat),
	}
	if eq.ManufactureDate != nil {
		s := eq.ManufactureDate.Format(dateFormat)
		resp.ManufactureDate = &s
	}
	if eq.ArrivalDate != nil {
		s := eq.ArrivalDate.Format(dateFormat)
		resp.ArrivalDate = &s
	}
	if eq.DeletedAt != nil {
		s := eq.DeletedAt.Format(timeFormat)
		resp.DeletedAt = &s
	}
	return resp
}

// toEquipmentResponses конвертирует срез доменных моделей в срез JSON-ответов.
func toEquipmentResponses(items []*domain.Equipment) []equipmentResponse {
	resp := make([]equipmentResponse, 0, len(items))
	for _, eq := range items {
		resp = append(resp, toEquipmentResponse(eq))
	}
	return resp
}

// parseEquipmentFilter извлекает фильтр из query-параметров
// ?inventory_number=...&serial_number=...&model_name=...&status=...&location=...
// Пустые или отсутствующие параметры не фильтруют соответствующее поле;
// некорректный статус — 400.
func parseEquipmentFilter(w http.ResponseWriter, r *http.Request) (domain.EquipmentFilter, bool) {
	var f domain.EquipmentFilter
	q := r.URL.Query()
	if v := q.Get("inventory_number"); v != "" {
		f.InventoryNumber = &v
	}
	if v := q.Get("serial_number"); v != "" {
		f.SerialNumber = &v
	}
	if v := q.Get("model_name"); v != "" {
		f.ModelName = &v
	}
	if v := q.Get("location"); v != "" {
		f.Location = &v
	}
	if v := q.Get("status"); v != "" {
		st, err := domain.ParseEquipmentStatus(v)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "invalid status, want one of: exp, exp_int, exp_sp, broken, written_off")
			return f, false
		}
		f.Status = &st
	}
	return f, true
}

// List обрабатывает GET /api/v1/equipments — только активные записи
// (deleted_at IS NULL) с фильтрацией.
func (h *EquipmentHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseEquipmentFilter(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListActive(r.Context(), filter)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toEquipmentResponses(items))
}

// ListInactive обрабатывает GET /api/v1/equipments/inactive — только удалённые
// (soft delete) записи с фильтрацией.
func (h *EquipmentHandler) ListInactive(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseEquipmentFilter(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListInactive(r.Context(), filter)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toEquipmentResponses(items))
}

// Create обрабатывает POST /api/v1/equipments.
func (h *EquipmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createEquipmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	manufactureDate, ok := parseOptionalDate(w, r, req.ManufactureDate, "manufacture_date")
	if !ok {
		return
	}
	arrivalDate, ok := parseOptionalDate(w, r, req.ArrivalDate, "arrival_date")
	if !ok {
		return
	}
	var status string
	if req.Status != nil {
		status = *req.Status
	}

	eq, err := h.svc.Create(r.Context(), domain.EquipmentInput{
		InventoryNumber: req.InventoryNumber,
		SerialNumber:    derefReqString(req.SerialNumber),
		NomenclatureID:  req.NomenclatureID,
		ModelName:       derefReqString(req.ModelName),
		ManufactureDate: manufactureDate,
		ArrivalDate:     arrivalDate,
		Status:          status,
		FormNumber:      derefReqString(req.FormNumber),
		Location:        derefReqString(req.Location),
		Notes:           derefReqString(req.Notes),
	})
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, toEquipmentResponse(eq))
}

// GetByID обрабатывает GET /api/v1/equipments/{id} — в любом статусе.
func (h *EquipmentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	eq, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toEquipmentResponse(eq))
}

// GetByInventoryNumber обрабатывает GET /api/v1/equipments/inventory/{inventory_number}
// — в любом статусе.
func (h *EquipmentHandler) GetByInventoryNumber(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseInventoryNumber(w, r)
	if !ok {
		return
	}

	eq, err := h.svc.GetByInventoryNumber(r.Context(), number)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toEquipmentResponse(eq))
}

// UpdateByID обрабатывает PATCH /api/v1/equipments/{id} — частичное обновление.
func (h *EquipmentHandler) UpdateByID(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	var req updateEquipmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	up, ok := buildEquipmentUpdate(w, r, req)
	if !ok {
		return
	}

	eq, err := h.svc.UpdateByID(r.Context(), id, up)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toEquipmentResponse(eq))
}

// UpdateByInventoryNumber обрабатывает PATCH /api/v1/equipments/inventory/{inventory_number}.
func (h *EquipmentHandler) UpdateByInventoryNumber(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseInventoryNumber(w, r)
	if !ok {
		return
	}

	var req updateEquipmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}
	up, ok := buildEquipmentUpdate(w, r, req)
	if !ok {
		return
	}

	eq, err := h.svc.UpdateByInventoryNumber(r.Context(), number, up)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toEquipmentResponse(eq))
}

// DeleteByID обрабатывает DELETE /api/v1/equipments/{id} — soft delete
// (заполнение deleted_at). Идемпотентен.
func (h *EquipmentHandler) DeleteByID(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	if _, err := h.svc.DeleteByID(r.Context(), id); err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// DeleteByInventoryNumber обрабатывает DELETE /api/v1/equipments/inventory/{inventory_number}
// — soft delete по инвентарному номеру. Идемпотентен.
func (h *EquipmentHandler) DeleteByInventoryNumber(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseInventoryNumber(w, r)
	if !ok {
		return
	}

	if _, err := h.svc.DeleteByInventoryNumber(r.Context(), number); err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// parseID извлекает идентификатор из пути и валидирует его.
// При ошибке записывает ответ 400 и возвращает ok=false.
func (h *EquipmentHandler) parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid id in path")
		return 0, false
	}
	return id, true
}

// parseInventoryNumber извлекает инвентарный номер из пути и валидирует
// по формату (ИТxxxxx). При ошибке записывает ответ 400 и возвращает ok=false.
func (h *EquipmentHandler) parseInventoryNumber(w http.ResponseWriter, r *http.Request) (string, bool) {
	number := chi.URLParam(r, "inventory_number")
	if err := domain.ValidateInventoryNumber(number); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid inventory number in path, want ИТxxxxx")
		return "", false
	}
	return number, true
}

// buildEquipmentUpdate преобразует тело PATCH в доменное обновление.
// Пустая строка nullable-даты и nomenclature_id <= 0 очищают колонку (NULL);
// мусор в дате — 400.
func buildEquipmentUpdate(w http.ResponseWriter, r *http.Request, req updateEquipmentRequest) (domain.EquipmentUpdate, bool) {
	var up domain.EquipmentUpdate
	up.SerialNumber = req.SerialNumber
	up.NomenclatureID = req.NomenclatureID
	up.ModelName = req.ModelName
	up.Status = req.Status
	up.FormNumber = req.FormNumber
	up.Location = req.Location
	up.Notes = req.Notes

	manufactureDate, ok := parseUpdateDate(w, r, req.ManufactureDate, "manufacture_date")
	if !ok {
		return up, false
	}
	up.ManufactureDate = manufactureDate
	arrivalDate, ok := parseUpdateDate(w, r, req.ArrivalDate, "arrival_date")
	if !ok {
		return up, false
	}
	up.ArrivalDate = arrivalDate
	return up, true
}

// parseUpdateDate парсит опциональную дату YYYY-MM-DD из тела PATCH-запроса.
// nil → (nil, true) — не менять; пустая строка → нулевое время (сигнал
// «очистить колонку» в домене); мусор → ошибка 400.
func parseUpdateDate(w http.ResponseWriter, r *http.Request, raw *string, field string) (*time.Time, bool) {
	if raw == nil {
		return nil, true
	}
	if *raw == "" {
		zero := time.Time{}
		return &zero, true
	}
	t, err := time.Parse(dateFormat, *raw)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid "+field+", want YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}

// respondDomainError маппит доменные ошибки на HTTP-статусы:
//   - ErrNotFound                    → 404;
//   - ErrAlreadyExists               → 409;
//   - ErrInvalidID/InventoryNumber/  → 400;
//     Status/Field/Nomenclature
//   - прочее                         → 500 (+ логирование).
func (h *EquipmentHandler) respondDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		respondError(w, r, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidID),
		errors.Is(err, domain.ErrInvalidInventoryNumber),
		errors.Is(err, domain.ErrInvalidStatus),
		errors.Is(err, domain.ErrInvalidField),
		errors.Is(err, domain.ErrInvalidNomenclature):
		respondError(w, r, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("equipment handler error", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}
