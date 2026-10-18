package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/go-chi/chi/v5"
)

// Cards — GET /ui/cards: список активных карточек с фильтром по ФИО.
// Для htmx-запроса — только фрагмент #cards-table.
func (h *PageHandler) Cards(w http.ResponseWriter, r *http.Request) {
	values := web.CardsFilterValues{FullName: filterValue(r, "full_name")}

	var filter domain.CardFilter
	if values.FullName != "" {
		filter.FullName = &values.FullName
	}

	items, err := h.cards.ListActive(r.Context(), filter)
	if h.respondListError(w, r, "list cards", err) {
		return
	}

	data := web.CardsTableData{Items: items, Filter: values}
	if isHXRequest(r) {
		h.render(w, r, http.StatusOK, web.CardsTable(data))
		return
	}
	h.render(w, r, http.StatusOK, web.CardsListPage(data))
}

// CardsCreate — GET /ui/cards/create: пустая модальная форма создания карточки.
func (h *PageHandler) CardsCreate(w http.ResponseWriter, r *http.Request) {
	data := web.CardModalData{
		ActionURL: "/ui/cards/create",
	}
	h.render(w, r, http.StatusOK, web.CardCreateModal(data))
}

// CardsCreateSubmit — POST /ui/cards/create: создание карточки.
// При succès — HX-Redirect на /ui/cards (htmx делает редирект).
// При ошибке — перерисовка модалки с сообщением об ошибке.
func (h *PageHandler) CardsCreateSubmit(w http.ResponseWriter, r *http.Request) {
	numberStr := strings.TrimSpace(r.FormValue("number"))
	fullName := strings.TrimSpace(r.FormValue("full_name"))

	number, err := strconv.Atoi(numberStr)
	if err != nil || number <= 0 {
		h.render(w, r, http.StatusOK, web.CardCreateModal(web.CardModalData{
			FullName:  fullName,
			Error:     "Номер карточки должен быть положительным числом.",
			ActionURL: "/ui/cards/create",
		}))
		return
	}

	_, err = h.cards.Create(r.Context(), number, fullName)
	if err != nil {
		h.render(w, r, http.StatusOK, web.CardCreateModal(web.CardModalData{
			Number:    number,
			FullName:  fullName,
			Error:     domainErrorMessage(err),
			ActionURL: "/ui/cards/create",
		}))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/cards")
	w.WriteHeader(http.StatusOK)
}

// CardsEdit — GET /ui/cards/{number}/edit: модальная форма редактирования карточки.
func (h *PageHandler) CardsEdit(w http.ResponseWriter, r *http.Request) {
	number, ok := parseCardNumber(w, r)
	if !ok {
		return
	}

	card, err := h.cards.Get(r.Context(), number)
	if err != nil {
		h.render(w, r, http.StatusOK, web.CardEditModal(web.CardModalData{
			Number:    number,
			Error:     "Карточка не найдена.",
			ActionURL: "/ui/cards/" + strconv.Itoa(number),
		}))
		return
	}

	h.render(w, r, http.StatusOK, web.CardEditModal(web.CardModalData{
		Number:    card.Number,
		FullName:  card.FullName,
		ActionURL: "/ui/cards/" + strconv.Itoa(number),
	}))
}

// CardsUpdateSubmit — POST /ui/cards/{number}: обновление карточки.
// При succès — HX-Redirect на /ui/cards.
// При ошибке — перерисовка модалки с сообщением об ошибке.
func (h *PageHandler) CardsUpdateSubmit(w http.ResponseWriter, r *http.Request) {
	number, ok := parseCardNumber(w, r)
	if !ok {
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))

	_, err := h.cards.UpdateName(r.Context(), number, fullName)
	if err != nil {
		h.render(w, r, http.StatusOK, web.CardEditModal(web.CardModalData{
			Number:    number,
			FullName:  fullName,
			Error:     domainErrorMessage(err),
			ActionURL: "/ui/cards/" + strconv.Itoa(number),
		}))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/cards")
	w.WriteHeader(http.StatusOK)
}

// parseCardNumber извлекает и валидирует номер карточки из пути chi.
func parseCardNumber(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "number")
	number, err := strconv.Atoi(raw)
	if err != nil || number <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid card number in path")
		return 0, false
	}
	return number, true
}
