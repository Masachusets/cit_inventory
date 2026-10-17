package handlers

import (
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
)

// Equipments — GET /ui/equipments: список активного оборудования с фильтрацией.
// Для обычного запроса — полная страница; для htmx (HX-Request: true) — только
// фрагмент #equipment-table, подменяемый в DOM без перезагрузки.
func (h *PageHandler) Equipments(w http.ResponseWriter, r *http.Request) {
	values := parseEquipmentFilterValues(r)

	items, err := h.equipments.ListActive(r.Context(), equipmentFilterFrom(values))
	if h.respondListError(w, r, "list equipments", err) {
		return
	}

	data := web.EquipmentTableData{Items: items, Filter: values}
	if isHXRequest(r) {
		h.render(w, r, http.StatusOK, web.EquipmentTable(data))
		return
	}
	h.render(w, r, http.StatusOK, web.EquipmentListPage(data))
}

// parseEquipmentFilterValues извлекает значения фильтра списка оборудования
// из query-параметров формы /ui/equipments. Некорректный статус молча
// игнорируется: в UI-форме статус — select только с валидными значениями.
func parseEquipmentFilterValues(r *http.Request) web.EquipmentFilterValues {
	q := r.URL.Query()
	v := web.EquipmentFilterValues{
		InventoryNumber: filterValue(r, "inventory_number"),
		ModelName:       filterValue(r, "model_name"),
		Location:        filterValue(r, "location"),
		CardFullName:    filterValue(r, "card_full_name"),
		DepartmentName:  filterValue(r, "department_name"),
	}
	if st, err := domain.ParseEquipmentStatus(q.Get("status")); err == nil {
		v.Status = string(st)
	}
	return v
}

// equipmentFilterFrom преобразует значения формы в доменный фильтр: пустая
// строка — nil-поле (не фильтровать по полю).
func equipmentFilterFrom(v web.EquipmentFilterValues) domain.EquipmentFilter {
	var f domain.EquipmentFilter
	if v.InventoryNumber != "" {
		f.InventoryNumber = &v.InventoryNumber
	}
	if v.ModelName != "" {
		f.ModelName = &v.ModelName
	}
	if v.Location != "" {
		f.Location = &v.Location
	}
	if v.CardFullName != "" {
		f.CardFullName = &v.CardFullName
	}
	if v.DepartmentName != "" {
		f.DepartmentName = &v.DepartmentName
	}
	if v.Status != "" {
		st := domain.EquipmentStatus(v.Status)
		f.Status = &st
	}
	return f
}
