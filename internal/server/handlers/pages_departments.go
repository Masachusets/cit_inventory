package handlers

import (
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
)

// Departments — GET /ui/departments: список активных подразделений с фильтром
// по наименованию. Для htmx-запроса — только фрагмент #departments-table.
func (h *PageHandler) Departments(w http.ResponseWriter, r *http.Request) {
	values := web.DepartmentsFilterValues{Name: filterValue(r, "name")}

	var filter domain.DepartmentFilter
	if values.Name != "" {
		filter.Name = &values.Name
	}

	items, err := h.departments.ListActive(r.Context(), filter)
	if h.respondListError(w, r, "list departments", err) {
		return
	}

	data := web.DepartmentsTableData{Items: items, Filter: values}
	if isHXRequest(r) {
		h.render(w, r, http.StatusOK, web.DepartmentsTable(data))
		return
	}
	h.render(w, r, http.StatusOK, web.DepartmentsListPage(data))
}
