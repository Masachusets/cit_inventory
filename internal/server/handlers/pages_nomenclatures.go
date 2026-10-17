package handlers

import (
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
)

// Nomenclatures — GET /ui/nomenclatures: справочник номенклатуры с фильтрами
// по коду и наименованию. Для htmx-запроса — только фрагмент
// #nomenclatures-table.
func (h *PageHandler) Nomenclatures(w http.ResponseWriter, r *http.Request) {
	values := web.NomenclaturesFilterValues{
		Code: filterValue(r, "code"),
		Name: filterValue(r, "name"),
	}

	var filter domain.NomenclatureFilter
	if values.Code != "" {
		filter.Code = &values.Code
	}
	if values.Name != "" {
		filter.Name = &values.Name
	}

	items, err := h.nomenclatures.List(r.Context(), filter)
	if h.respondListError(w, r, "list nomenclatures", err) {
		return
	}

	data := web.NomenclaturesTableData{Items: items, Filter: values}
	if isHXRequest(r) {
		h.render(w, r, http.StatusOK, web.NomenclaturesTable(data))
		return
	}
	h.render(w, r, http.StatusOK, web.NomenclaturesListPage(data))
}
