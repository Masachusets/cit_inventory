package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/go-chi/chi/v5"
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

// DepartmentsCreate — GET /ui/departments/create: пустая модальная форма создания подразделения.
func (h *PageHandler) DepartmentsCreate(w http.ResponseWriter, r *http.Request) {
	data := web.DepartmentModalData{
		ActionURL: "/ui/departments/create",
	}
	h.render(w, r, http.StatusOK, web.DepartmentCreateModal(data))
}

// DepartmentsCreateSubmit — POST /ui/departments/create: создание подразделения.
// При succès — HX-Redirect на /ui/departments.
// При ошибке — перерисовка модалки с сообщением об ошибке.
func (h *PageHandler) DepartmentsCreateSubmit(w http.ResponseWriter, r *http.Request) {
	codeStr := strings.TrimSpace(r.FormValue("code"))
	name := strings.TrimSpace(r.FormValue("name"))

	code, err := strconv.Atoi(codeStr)
	if err != nil || code <= 0 {
		h.render(w, r, http.StatusOK, web.DepartmentCreateModal(web.DepartmentModalData{
			Name:      name,
			Error:     "Код подразделения должен быть положительным числом.",
			ActionURL: "/ui/departments/create",
		}))
		return
	}

	_, err = h.departments.Create(r.Context(), code, name)
	if err != nil {
		h.render(w, r, http.StatusOK, web.DepartmentCreateModal(web.DepartmentModalData{
			Code:      code,
			Name:      name,
			Error:     domainErrorMessage(err),
			ActionURL: "/ui/departments/create",
		}))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/departments")
	w.WriteHeader(http.StatusOK)
}

// DepartmentsEdit — GET /ui/departments/{code}/edit: модальная форма редактирования подразделения.
func (h *PageHandler) DepartmentsEdit(w http.ResponseWriter, r *http.Request) {
	code, ok := parseDepartmentCode(w, r)
	if !ok {
		return
	}

	dept, err := h.departments.Get(r.Context(), code)
	if err != nil {
		h.render(w, r, http.StatusOK, web.DepartmentEditModal(web.DepartmentModalData{
			Code:      code,
			Error:     "Подразделение не найдено.",
			ActionURL: "/ui/departments/" + strconv.Itoa(code),
		}))
		return
	}

	h.render(w, r, http.StatusOK, web.DepartmentEditModal(web.DepartmentModalData{
		Code:      int(dept.Code),
		Name:      dept.Name,
		ActionURL: "/ui/departments/" + strconv.Itoa(code),
	}))
}

// DepartmentsUpdateSubmit — POST /ui/departments/{code}: обновление подразделения.
// При succès — HX-Redirect на /ui/departments.
// При ошибке — перерисовка модалки с сообщением об ошибке.
func (h *PageHandler) DepartmentsUpdateSubmit(w http.ResponseWriter, r *http.Request) {
	code, ok := parseDepartmentCode(w, r)
	if !ok {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))

	_, err := h.departments.UpdateName(r.Context(), code, name)
	if err != nil {
		h.render(w, r, http.StatusOK, web.DepartmentEditModal(web.DepartmentModalData{
			Code:      code,
			Name:      name,
			Error:     domainErrorMessage(err),
			ActionURL: "/ui/departments/" + strconv.Itoa(code),
		}))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/departments")
	w.WriteHeader(http.StatusOK)
}

// parseDepartmentCode извлекает и валидирует код подразделения из пути chi.
func parseDepartmentCode(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "code")
	code, err := strconv.Atoi(raw)
	if err != nil || code <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid department code in path")
		return 0, false
	}
	return code, true
}
