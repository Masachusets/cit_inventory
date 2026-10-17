package handlers

import (
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
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
