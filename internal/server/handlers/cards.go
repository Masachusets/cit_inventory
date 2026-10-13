package handlers

import (
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// maxRequestBody — предохранитель от слишком больших тел запроса (1 МБ хватит
// с запасом: карточка содержит всего номер и ФИО).
const maxRequestBody = 1 << 20

// timeFormat — формат RFC3339 для временных меток в JSON.
const timeFormat = "2006-01-02T15:04:05Z07:00"

// CardHandler — HTTP-слой для работы с карточками учёта. Зависит только от
// интерфейса CardService, поэтому легко тестируется подменной реализацией.
type CardHandler struct {
	svc service.CardService
	log *slog.Logger
}

// NewCardHandler создаёт обработчик карточек.
func NewCardHandler(svc service.CardService, log *slog.Logger) *CardHandler {
	return &CardHandler{svc: svc, log: log}
}

// createCardRequest — тело POST /api/v1/cards.
type createCardRequest struct {
	Number   int    `json:"number"`
	FullName string `json:"full_name"`
}

// updateCardRequest — тело PATCH /api/v1/cards/{number}.
type updateCardRequest struct {
	FullName string `json:"full_name"`
}

// cardResponse — JSON-представление карточки. Поле deleted_at сериализуется
// как ISO-строка либо null.
type cardResponse struct {
	Number    int     `json:"number"`
	FullName  string  `json:"full_name"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	DeletedAt *string `json:"deleted_at"`
}

// toCardResponse преобразует доменную модель в JSON-ответ.
func toCardResponse(c *domain.Card) cardResponse {
	resp := cardResponse{
		Number:    c.Number,
		FullName:  c.FullName,
		IsActive:  c.IsActive,
		CreatedAt: c.CreatedAt.Format(timeFormat),
		UpdatedAt: c.UpdatedAt.Format(timeFormat),
	}
	if c.DeletedAt != nil {
		s := c.DeletedAt.Format(timeFormat)
		resp.DeletedAt = &s
	}
	return resp
}

// toCardResponses конвертирует срез доменных карточек в срез JSON-ответов.
func toCardResponses(cards []*domain.Card) []cardResponse {
	resp := make([]cardResponse, 0, len(cards))
	for _, c := range cards {
		resp = append(resp, toCardResponse(c))
	}
	return resp
}

// Create обрабатывает POST /api/v1/cards.
func (h *CardHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createCardRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	card, err := h.svc.Create(r.Context(), req.Number, req.FullName)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, toCardResponse(card))
}

// List обрабатывает GET /api/v1/cards — только активные карточки.
func (h *CardHandler) List(w http.ResponseWriter, r *http.Request) {
	cards, err := h.svc.ListActive(r.Context())
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toCardResponses(cards))
}

// ListInactive обрабатывает GET /api/v1/cards/inactive — только неактивные.
func (h *CardHandler) ListInactive(w http.ResponseWriter, r *http.Request) {
	cards, err := h.svc.ListInactive(r.Context())
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toCardResponses(cards))
}

// Get обрабатывает GET /api/v1/cards/{number} — карточка в любом статусе.
func (h *CardHandler) Get(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseNumber(w, r)
	if !ok {
		return
	}

	card, err := h.svc.Get(r.Context(), number)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toCardResponse(card))
}

// Update обрабатывает PATCH /api/v1/cards/{number} — изменение только ФИО.
func (h *CardHandler) Update(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseNumber(w, r)
	if !ok {
		return
	}

	var req updateCardRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid JSON body")
		return
	}

	card, err := h.svc.UpdateName(r.Context(), number, req.FullName)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toCardResponse(card))
}

// Deactivate обрабатывает POST /api/v1/cards/{number}/deactivate.
func (h *CardHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseNumber(w, r)
	if !ok {
		return
	}

	card, err := h.svc.Deactivate(r.Context(), number)
	if err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, toCardResponse(card))
}

// Delete обрабатывает DELETE /api/v1/cards/{number} — физическое удаление.
func (h *CardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	number, ok := h.parseNumber(w, r)
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), number); err != nil {
		h.respondDomainError(w, r, err)
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

// parseNumber извлекает номер карточки из пути и валидирует его.
// При ошибке записывает ответ 400 и возвращает ok=false.
func (h *CardHandler) parseNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "number")
	number, err := strconv.Atoi(raw)
	if err != nil || number <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid card number in path")
		return 0, false
	}
	return number, true
}

// respondDomainError маппит доменные ошибки на HTTP-статусы:
//   - ErrNotFound           → 404;
//   - ErrAlreadyExists      → 409;
//   - ErrStillActive        → 409;
//   - ErrInUse              → 409;
//   - ErrInvalidNumber/Name → 400;
//   - прочее                → 500 (+ логирование).
func (h *CardHandler) respondDomainError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, r, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists),
		errors.Is(err, domain.ErrStillActive),
		errors.Is(err, domain.ErrInUse):
		respondError(w, r, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidNumber), errors.Is(err, domain.ErrInvalidName):
		respondError(w, r, http.StatusBadRequest, err.Error())
	default:
		h.log.Error("card handler error", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
	}
}

// decodeJSON читает и декодирует тело запроса (с лимитом размера).
func decodeJSON(r *http.Request, out any) error {
	if r.Body == nil {
		return errors.New("empty body")
	}
	defer func() { _ = r.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
