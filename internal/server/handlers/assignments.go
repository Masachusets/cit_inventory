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

// AssignmentHandler — HTTP-слой для работы с закреплениями оборудования.
// Зависит только от интерфейса AssignmentService.
type AssignmentHandler struct {
	svc service.AssignmentService
	log *slog.Logger
}

// NewAssignmentHandler создаёт обработчик закреплений.
func NewAssignmentHandler(svc service.AssignmentService, log *slog.Logger) *AssignmentHandler {
	return &AssignmentHandler{svc: svc, log: log}
}

// createAssignmentRequest — тело POST /api/v1/assignments.
// card_number/department_code/waybill_id обязаны согласовываться с target_type
// (CHECK chk_target); assigned_at опционален (null → DEFAULT NOW()),
// unassigned_at — «уже снят с закрепления».
type createAssignmentRequest struct {
	EquipmentID     int     `json:"equipment_id"`
	TargetType      string  `json:"target_type"`
	CardNumber      *int    `json:"card_number"`
	DepartmentCode  *int    `json:"department_code"`
	WaybillID       *int    `json:"waybill_id"`
	AssignedAt      *string `json:"assigned_at"`
	UnassignedAt    *string `json:"unassigned_at"`
	OperatorComment *string `json:"operator_comment"`
}

// updateAssignmentRequest — тело PATCH /api/v1/assignments/{id}.
// Частичное обновление: отсутствующее поле не меняется; card_number/
// department_code/waybill_id <= 0 очищают ссылку (NULL); пустая строка
// unassigned_at очищает колонку (NULL), пустая строка assigned_at — ошибка
// (колонка NOT NULL); пустая строка operator_comment очищает колонку.
type updateAssignmentRequest struct {
	TargetType      *string `json:"target_type"`
	CardNumber      *int    `json:"card_number"`
	DepartmentCode  *int    `json:"department_code"`
	WaybillID       *int    `json:"waybill_id"`
	AssignedAt      *string `json:"assigned_at"`
	UnassignedAt    *string `json:"unassigned_at"`
	OperatorComment *string `json:"operator_comment"`
}

// assignmentEquipmentDTO — вложенный объект оборудования в ответе.
type assignmentEquipmentDTO struct {
	ID              int    `json:"id"`
	InventoryNumber string `json:"inventory_number"`
}

// assignmentWaybillDTO — вложенный объект путевого листа (все поля waybills).
// number и issue_date — nullable-колонки, сериализуются как значение либо null.
type assignmentWaybillDTO struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Number    *string `json:"number"`
	IssueDate *string `json:"issue_date"`
}

// assignmentCardDTO — вложенный объект карточки (только поля закрепления).
type assignmentCardDTO struct {
	Number   int    `json:"number"`
	FullName string `json:"full_name"`
}

// assignmentDepartmentDTO — вложенный объект подразделения (поля закрепления).
type assignmentDepartmentDTO struct {
	Code int16  `json:"code"`
	Name string `json:"name"`
}

// assignmentResponse — JSON-представление закрепления со связанными
// сущностями. waybill/card/department могут быть null (не заданы / не
// применимы к типу субъекта); equipment заполнен всегда (FK NOT NULL).
type assignmentResponse struct {
	ID              int                      `json:"id"`
	TargetType      string                   `json:"target_type"`
	AssignedAt      string                   `json:"assigned_at"`
	UnassignedAt    *string                  `json:"unassigned_at"`
	OperatorComment *string                  `json:"operator_comment"`
	Equipment       *assignmentEquipmentDTO  `json:"equipment"`
	Waybill         *assignmentWaybillDTO    `json:"waybill"`
	Card            *assignmentCardDTO       `json:"card"`
	Department      *assignmentDepartmentDTO `json:"department"`
}

// toAssignmentResponse преобразует доменную модель в JSON-ответ.
// assigned_at/unassigned_at — TIMESTAMPTZ, сериализуются как RFC3339;
// issue_date — DATE, как YYYY-MM-DD либо null.
func toAssignmentResponse(a *domain.Assignment) assignmentResponse {
	resp := assignmentResponse{
		ID:              a.ID,
		TargetType:      string(a.TargetType),
		AssignedAt:      a.AssignedAt.Format(time.RFC3339),
		OperatorComment: a.OperatorComment,
	}
	if a.UnassignedAt != nil {
		s := a.UnassignedAt.Format(time.RFC3339)
		resp.UnassignedAt = &s
	}
	if a.Equipment != nil {
		resp.Equipment = &assignmentEquipmentDTO{
			ID:              a.Equipment.ID,
			InventoryNumber: a.Equipment.InventoryNumber,
		}
	}
	if a.Waybill != nil {
		resp.Waybill = &assignmentWaybillDTO{
			ID:     a.Waybill.ID,
			Name:   a.Waybill.Name,
			Number: a.Waybill.Number,
		}
		if a.Waybill.IssueDate != nil {
			s := a.Waybill.IssueDate.Format(dateFormat)
			resp.Waybill.IssueDate = &s
		}
	}
	if a.Card != nil {
		resp.Card = &assignmentCardDTO{Number: a.Card.Number, FullName: a.Card.FullName}
	}
	if a.Department != nil {
		resp.Department = &assignmentDepartmentDTO{Code: a.Department.Code, Name: a.Department.Name}
	}
	return resp
}

// toAssignmentResponses конвертирует срез доменных моделей в срез JSON-ответов.
func toAssignmentResponses(items []*domain.Assignment) []assignmentResponse {
	resp := make([]assignmentResponse, 0, len(items))
	for _, a := range items {
		resp = append(resp, toAssignmentResponse(a))
	}
	return resp
}

// parseAssignmentFilter извлекает фильтр из query-параметров:
//
//	?id-поля: equipment_id, card_number, department_code, waybill_id (точные);
//	строки: inventory_number, card_full_name, department_name, waybill_name,
//	        waybill_number, operator_comment (подстрока, ILIKE);
//	enum: target_type (employee|department|warehouse); bool: active;
//	даты: waybill_issue_date_from/to, assigned_at_from/to, unassigned_at_from/to.
//
// Пустые или отсутствующие параметры не фильтруют соответствующее поле;
// некорректное значение, дата или from > to — 400.
func parseAssignmentFilter(w http.ResponseWriter, r *http.Request) (domain.AssignmentFilter, bool) {
	var f domain.AssignmentFilter
	q := r.URL.Query()

	// Строковые подстроки (ILIKE).
	for _, p := range []struct {
		name string
		dst  **string
	}{
		{"inventory_number", &f.InventoryNumber},
		{"card_full_name", &f.CardFullName},
		{"department_name", &f.DepartmentName},
		{"waybill_name", &f.WaybillName},
		{"waybill_number", &f.WaybillNumber},
		{"operator_comment", &f.OperatorComment},
	} {
		if v := q.Get(p.name); v != "" {
			*p.dst = &v
		}
	}

	// Точные идентификаторы (положительные целые).
	for _, p := range []struct {
		name string
		dst  **int
	}{
		{"equipment_id", &f.EquipmentID},
		{"card_number", &f.CardNumber},
		{"department_code", &f.DepartmentCode},
		{"waybill_id", &f.WaybillID},
	} {
		v, ok := parseIntParam(w, r, p.name)
		if !ok {
			return f, false
		}
		if v != nil {
			*p.dst = v
		}
	}

	// Тип субъекта закрепления (enum).
	if v := q.Get("target_type"); v != "" {
		t, err := domain.ParseAssignmentTarget(v)
		if err != nil {
			respondError(w, r, http.StatusBadRequest,
				"invalid target_type, want employee|department|warehouse")
			return f, false
		}
		f.TargetType = &t
	}

	// Активность: active=true — ещё закреплены, active=false — сняты.
	if v := q.Get("active"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			respondError(w, r, http.StatusBadRequest, "invalid active, want true|false")
			return f, false
		}
		f.Active = &b
	}

	// Диапазоны включительных дат (YYYY-MM-DD) с проверкой from <= to.
	for _, dr := range []struct {
		suffix string
		from   **time.Time
		to     **time.Time
	}{
		{"waybill_issue_date", &f.WaybillIssueDateFrom, &f.WaybillIssueDateTo},
		{"assigned_at", &f.AssignedAtFrom, &f.AssignedAtTo},
		{"unassigned_at", &f.UnassignedAtFrom, &f.UnassignedAtTo},
	} {
		from, ok := parseDateParam(w, r, dr.suffix+"_from")
		if !ok {
			return f, false
		}
		to, ok := parseDateParam(w, r, dr.suffix+"_to")
		if !ok {
			return f, false
		}
		if from != nil && to != nil && from.After(*to) {
			respondError(w, r, http.StatusBadRequest,
				dr.suffix+"_from must not be after "+dr.suffix+"_to")
			return f, false
		}
		*dr.from = from
		*dr.to = to
	}
	return f, true
}

// parseIntParam парсит query-параметр с положительным целым.
// Пустой/отсутствующий → (nil, true); мусор или значение <= 0 → ошибка 400.
func parseIntParam(w http.ResponseWriter, r *http.Request, param string) (*int, bool) {
	v := r.URL.Query().Get(param)
	if v == "" {
		return nil, true
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid "+param+", want positive integer")
		return nil, false
	}
	return &n, true
}

// parseOptionalTimestamp парсит опциональную метку времени из тела запроса.
// nil/пустая строка → (nil, true) — не задано; YYYY-MM-DD → полночь UTC
// (удобный ввод даты), RFC3339 → момент времени; мусор → ошибка 400.
func parseOptionalTimestamp(w http.ResponseWriter, r *http.Request, raw *string, field string) (*time.Time, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	t, ok := parseTimestamp(w, r, *raw, field)
	return t, ok
}

// parseUpdateTimestamp парсит метку времени из тела PATCH-запроса.
// nil → (nil, true) — не менять; пустая строка → нулевое время (сигнал
// «очистить колонку» в домене, для assigned_at домен вернёт ошибку 400);
// иначе — как parseOptionalTimestamp.
func parseUpdateTimestamp(w http.ResponseWriter, r *http.Request, raw *string, field string) (*time.Time, bool) {
	if raw == nil {
		return nil, true
	}
	if *raw == "" {
		zero := time.Time{}
		return &zero, true
	}
	t, ok := parseTimestamp(w, r, *raw, field)
	return t, ok
}

// parseTimestamp разбирает строку как YYYY-MM-DD (полночь UTC) либо RFC3339.
// Мусор → ошибка 400 с именем поля.
func parseTimestamp(w http.ResponseWriter, r *http.Request, raw, field string) (*time.Time, bool) {
	if t, err := time.Parse(dateFormat, raw); err == nil {
		return &t, true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, true
	}
	respondError(w, r, http.StatusBadRequest, "invalid "+field+", want YYYY-MM-DD or RFC3339")
	return nil, false
}

// List обрабатывает GET /api/v1/assignments — список с фильтрацией.
func (h *AssignmentHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseAssignmentFilter(w, r)
	if !ok {
		return
	}

	items, err := h.svc.List(r.Context(), filter)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toAssignmentResponses(items))
}

// Get обрабатывает GET /api/v1/assignments/{id}.
func (h *AssignmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	a, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toAssignmentResponse(a))
}

// Create обрабатывает POST /api/v1/assignments.
func (h *AssignmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	assignedAt, ok := parseOptionalTimestamp(w, r, req.AssignedAt, "assigned_at")
	if !ok {
		return
	}
	unassignedAt, ok := parseOptionalTimestamp(w, r, req.UnassignedAt, "unassigned_at")
	if !ok {
		return
	}

	comment := ""
	if req.OperatorComment != nil {
		comment = *req.OperatorComment
	}

	a, err := h.svc.Create(r.Context(), domain.AssignmentInput{
		EquipmentID:     req.EquipmentID,
		TargetType:      req.TargetType,
		CardNumber:      req.CardNumber,
		DepartmentCode:  req.DepartmentCode,
		WaybillID:       req.WaybillID,
		AssignedAt:      assignedAt,
		UnassignedAt:    unassignedAt,
		OperatorComment: comment,
	})
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, toAssignmentResponse(a))
}

// Update обрабатывает PATCH /api/v1/assignments/{id} — частичное обновление.
func (h *AssignmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	var req updateAssignmentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	assignedAt, ok := parseUpdateTimestamp(w, r, req.AssignedAt, "assigned_at")
	if !ok {
		return
	}
	unassignedAt, ok := parseUpdateTimestamp(w, r, req.UnassignedAt, "unassigned_at")
	if !ok {
		return
	}

	a, err := h.svc.Update(r.Context(), id, domain.AssignmentUpdate{
		TargetType:      req.TargetType,
		CardNumber:      req.CardNumber,
		DepartmentCode:  req.DepartmentCode,
		WaybillID:       req.WaybillID,
		AssignedAt:      assignedAt,
		UnassignedAt:    unassignedAt,
		OperatorComment: req.OperatorComment,
	})
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toAssignmentResponse(a))
}

// Delete обрабатывает DELETE /api/v1/assignments/{id} — физическое удаление
// записи истории (таблица без deleted_at).
func (h *AssignmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}

	a, err := h.svc.Delete(r.Context(), id)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toAssignmentResponse(a))
}

// parseID извлекает идентификатор из пути и валидирует его.
// При ошибке записывает ответ 400 и возвращает ok=false.
func (h *AssignmentHandler) parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid id in path")
		return 0, false
	}
	return id, true
}

// respondDomainError маппит доменные ошибки на HTTP-статусы:
//   - ErrNotFound                         → 404;
//   - ErrInvalidID/Field/Target/Reference/ → 400;
//     Number/Name
//   - прочее                              → 500 (+ логирование).
func (h *AssignmentHandler) respondDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidID),
		errors.Is(err, domain.ErrInvalidField),
		errors.Is(err, domain.ErrInvalidTarget),
		errors.Is(err, domain.ErrInvalidReference),
		errors.Is(err, domain.ErrInvalidNumber),
		errors.Is(err, domain.ErrInvalidName):
		respondError(w, r, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("assignment handler error", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}
