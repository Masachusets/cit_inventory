package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/a-h/templ"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// PageHandler — HTML-хендлеры веб-интерфейса (/ui): серверный рендеринг
// templ-компонентов вместо JSON. Зависит от интерфейсов сервисов, поэтому
// тестируется подменной реализацией.
type PageHandler struct {
	equipments service.EquipmentService
	log        *slog.Logger
}

// NewPageHandler создаёт обработчик страниц веб-интерфейса.
func NewPageHandler(equipments service.EquipmentService, log *slog.Logger) *PageHandler {
	return &PageHandler{equipments: equipments, log: log}
}

// render пишет HTML-ответ компонента с заданным статусом.
// Ошибка рендера логируется — ответ к этому моменту уже начат, отвечать
// 500 поздно.
func (h *PageHandler) render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		h.log.Error("render page",
			"error", err,
			"path", r.URL.Path,
			"request_id", chimw.GetReqID(r.Context()))
	}
}

// Home — GET /ui: главная страница со списком разделов.
func (h *PageHandler) Home(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, web.Home())
}

// PageNotFound — HTML-404 внутри /ui: страница вместо JSON-ошибки API.
func (h *PageHandler) PageNotFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, web.PageNotFound(r.URL.Path))
}

// Equipments — GET /ui/equipments: список активного оборудования с фильтрацией.
// Для обычного запроса — полная страница; для htmx (HX-Request: true) — только
// фрагмент #equipment-table, подменяемый в DOM без перезагрузки.
func (h *PageHandler) Equipments(w http.ResponseWriter, r *http.Request) {
	values := parseEquipmentFilterValues(r)

	items, err := h.equipments.ListActive(r.Context(), equipmentFilterFrom(values))
	if err != nil {
		h.log.Error("list equipments",
			"error", err,
			"request_id", chimw.GetReqID(r.Context()))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	data := web.EquipmentTableData{Items: items, Filter: values}
	if r.Header.Get("HX-Request") == "true" {
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
		InventoryNumber: strings.TrimSpace(q.Get("inventory_number")),
		ModelName:       strings.TrimSpace(q.Get("model_name")),
		Location:        strings.TrimSpace(q.Get("location")),
		CardFullName:    strings.TrimSpace(q.Get("card_full_name")),
		DepartmentName:  strings.TrimSpace(q.Get("department_name")),
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
