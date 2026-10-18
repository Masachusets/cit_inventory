package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/go-chi/chi/v5"
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

// WaybillsCreate — GET /ui/waybills/create: пустая модальная форма создания накладной.
func (h *PageHandler) WaybillsCreate(w http.ResponseWriter, r *http.Request) {
	data := web.WaybillModalData{
		ActionURL: "/ui/waybills/create",
	}
	h.render(w, r, http.StatusOK, web.WaybillCreateModal(data))
}

// WaybillsCreateSubmit — POST /ui/waybills/create: создание накладной.
// При succès — HX-Redirect на /ui/waybills.
// При ошибке — перерисовка модалки с сообщением об ошибке.
func (h *PageHandler) WaybillsCreateSubmit(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	number := strings.TrimSpace(r.FormValue("number"))
	issueDateStr := strings.TrimSpace(r.FormValue("issue_date"))

	var issueDate *time.Time
	if issueDateStr != "" {
		d, err := time.Parse("2006-01-02", issueDateStr)
		if err != nil {
			h.render(w, r, http.StatusOK, web.WaybillCreateModal(web.WaybillModalData{
				Name:      name,
				Number:    number,
				IssueDate: issueDateStr,
				Error:     "Некорректная дата документа (ожидается YYYY-MM-DD).",
				ActionURL: "/ui/waybills/create",
			}))
			return
		}
		issueDate = &d
	}

	_, err := h.waybills.Create(r.Context(), name, number, issueDate)
	if err != nil {
		h.render(w, r, http.StatusOK, web.WaybillCreateModal(web.WaybillModalData{
			Name:      name,
			Number:    number,
			IssueDate: issueDateStr,
			Error:     domainErrorMessage(err),
			ActionURL: "/ui/waybills/create",
		}))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/waybills")
	w.WriteHeader(http.StatusOK)
}

// WaybillsEdit — GET /ui/waybills/{id}/edit: модальная форма редактирования накладной.
func (h *PageHandler) WaybillsEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseWaybillID(w, r)
	if !ok {
		return
	}

	wb, err := h.waybills.Get(r.Context(), id)
	if err != nil {
		h.render(w, r, http.StatusOK, web.WaybillEditModal(web.WaybillModalData{
			ID:        id,
			Error:     "Накладная не найдена.",
			ActionURL: "/ui/waybills/" + strconv.Itoa(id),
		}))
		return
	}

	issueDate := ""
	if wb.IssueDate != nil {
		issueDate = wb.IssueDate.Format("2006-01-02")
	}

	number := ""
	if wb.Number != nil {
		number = *wb.Number
	}

	h.render(w, r, http.StatusOK, web.WaybillEditModal(web.WaybillModalData{
		ID:        wb.ID,
		Name:      wb.Name,
		Number:    number,
		IssueDate: issueDate,
		ActionURL: "/ui/waybills/" + strconv.Itoa(id),
	}))
}

// WaybillsUpdateSubmit — POST /ui/waybills/{id}: обновление накладной.
// При succès — HX-Redirect на /ui/waybills.
// При ошибке — перерисовка модалки с сообщением об ошибке.
func (h *PageHandler) WaybillsUpdateSubmit(w http.ResponseWriter, r *http.Request) {
	id, ok := parseWaybillID(w, r)
	if !ok {
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	number := strings.TrimSpace(r.FormValue("number"))
	issueDateStr := strings.TrimSpace(r.FormValue("issue_date"))

	var issueDate *time.Time
	if issueDateStr != "" {
		d, err := time.Parse("2006-01-02", issueDateStr)
		if err != nil {
			h.render(w, r, http.StatusOK, web.WaybillEditModal(web.WaybillModalData{
				ID:        id,
				Name:      name,
				Number:    number,
				IssueDate: issueDateStr,
				Error:     "Некорректная дата документа (ожидается YYYY-MM-DD).",
				ActionURL: "/ui/waybills/" + strconv.Itoa(id),
			}))
			return
		}
		issueDate = &d
	}

	_, err := h.waybills.Update(r.Context(), id, name, number, issueDate)
	if err != nil {
		h.render(w, r, http.StatusOK, web.WaybillEditModal(web.WaybillModalData{
			ID:        id,
			Name:      name,
			Number:    number,
			IssueDate: issueDateStr,
			Error:     domainErrorMessage(err),
			ActionURL: "/ui/waybills/" + strconv.Itoa(id),
		}))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/waybills")
	w.WriteHeader(http.StatusOK)
}

// parseWaybillID извлекает и валидирует идентификатор накладной из пути chi.
func parseWaybillID(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		respondError(w, r, http.StatusBadRequest, "invalid waybill id in path")
		return 0, false
	}
	return id, true
}
