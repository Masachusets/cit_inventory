package handlers

import (
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
)

// Waybills — GET /ui/waybills: список накладных с фильтрами по наименованию
// и номеру. Для htmx-запроса — только фрагмент #waybills-table.
func (h *PageHandler) Waybills(w http.ResponseWriter, r *http.Request) {
	values := web.WaybillsFilterValues{
		Name:   filterValue(r, "name"),
		Number: filterValue(r, "number"),
	}

	var filter domain.WaybillFilter
	if values.Name != "" {
		filter.Name = &values.Name
	}
	if values.Number != "" {
		filter.Number = &values.Number
	}

	items, err := h.waybills.List(r.Context(), filter)
	if h.respondListError(w, r, "list waybills", err) {
		return
	}

	data := web.WaybillsTableData{Items: items, Filter: values}
	if isHXRequest(r) {
		h.render(w, r, http.StatusOK, web.WaybillsTable(data))
		return
	}
	h.render(w, r, http.StatusOK, web.WaybillsListPage(data))
}
