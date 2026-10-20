package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/go-chi/chi/v5"
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

// EquipmentsCreate — GET /ui/equipments/create: модальная форма создания
// оборудования. Отдаётся фрагмент (htmx вставляет его в #equipment-modal).
func (h *PageHandler) EquipmentsCreate(w http.ResponseWriter, r *http.Request) {
	nomenclatures, err := h.nomenclatures.List(r.Context(), domain.NomenclatureFilter{})
	if err != nil {
		h.respondListError(w, r, "list nomenclatures", err)
		return
	}
	data := web.EquipmentModalData{
		ActionURL:     equipmentCreateActionURL,
		Nomenclatures: nomenclatures,
		Status:        string(domain.StatusExp),
	}
	h.render(w, r, http.StatusOK, web.EquipmentCreateModal(data))
}

// equipmentCreateActionURL — URL отправки формы создания оборудования.
const equipmentCreateActionURL = "/ui/equipments/create"

// equipmentEditActionURL — URL отправки формы редактирования оборудования.
func equipmentEditActionURL(inventoryNumber string) string {
	return "/ui/equipments/" + inventoryNumber
}

// parseFormNomenclatureID разбирает выбранный id номенклатуры из формы.
// Пустое или некорректное значение — ok == false: номенклатура обязательна
// при создании и редактировании оборудования.
func parseFormNomenclatureID(value string) (int, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	id, err := strconv.Atoi(v)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// parseFormDate разбирает дату формы (формат YYYY-MM-DD). Пустое значение —
// nil (дата не задана); некорректный формат — ошибка.
func parseFormDate(value string) (*time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, fmt.Errorf("%w: %q (ожидается YYYY-MM-DD)", domain.ErrInvalidField, v)
	}
	return &t, nil
}

// parseFormUpdateDate разбирает дату формы для частичного обновления.
// Пустое значение — нулевое время (в EquipmentUpdate это означает «очистить
// колонку»), некорректный формат — ошибка.
func parseFormUpdateDate(value string) (*time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		zero := time.Time{}
		return &zero, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, fmt.Errorf("%w: %q (ожидается YYYY-MM-DD)", domain.ErrInvalidField, v)
	}
	return &t, nil
}

// formatDateForInput — дата → YYYY-MM-DD для input[type=date]; nil или
// нулевое время — пустая строка.
func formatDateForInput(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// ptrStr возвращает указатель на обрезанную строку. Пустая строка — тоже
// указатель: в EquipmentUpdate это означает очистку колонки, а не «не менять».
func ptrStr(v string) *string {
	s := strings.TrimSpace(v)
	return &s
}

// derefStr возвращает значение указателя либо пустую строку.
func derefStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// equipmentModalError перерисовывает модальную форму с сообщением об ошибке,
// сохраняя введённые пользователем значения. mode — "create" или "edit".
func (h *PageHandler) equipmentModalError(
	w http.ResponseWriter,
	r *http.Request,
	mode string,
	actionURL string,
	message string,
) {
	nomenclatures, err := h.nomenclatures.List(r.Context(), domain.NomenclatureFilter{})
	if err != nil {
		h.respondListError(w, r, "list nomenclatures", err)
		return
	}

	data := web.EquipmentModalData{
		InventoryNumber: strings.TrimSpace(r.FormValue("inventory_number")),
		SerialNumber:    strings.TrimSpace(r.FormValue("serial_number")),
		ModelName:       strings.TrimSpace(r.FormValue("model_name")),
		ManufactureDate: strings.TrimSpace(r.FormValue("manufacture_date")),
		ArrivalDate:     strings.TrimSpace(r.FormValue("arrival_date")),
		Status:          strings.TrimSpace(r.FormValue("status")),
		FormNumber:      strings.TrimSpace(r.FormValue("form_number")),
		Location:        strings.TrimSpace(r.FormValue("location")),
		Notes:           strings.TrimSpace(r.FormValue("notes")),
		Nomenclatures:   nomenclatures,
		ActionURL:       actionURL,
		IsEdit:          mode == "edit",
		Error:           message,
	}
	if id, ok := parseFormNomenclatureID(r.FormValue("nomenclature_id")); ok {
		data.NomenclatureID = id
	}
	if data.Status == "" {
		data.Status = string(domain.StatusExp)
	}

	if mode == "edit" {
		h.render(w, r, http.StatusOK, web.EquipmentEditModal(data))
		return
	}
	h.render(w, r, http.StatusOK, web.EquipmentCreateModal(data))
}

// EquipmentsCreateSubmit — POST /ui/equipments/create: создание оборудования
// и закрепление его на складе (новое оборудование поступает на склад).
func (h *PageHandler) EquipmentsCreateSubmit(w http.ResponseWriter, r *http.Request) {
	inventoryNumber := strings.TrimSpace(r.FormValue("inventory_number"))
	if err := domain.ValidateInventoryNumber(inventoryNumber); err != nil {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL, domainErrorMessage(err))
		return
	}

	nomenclatureID, ok := parseFormNomenclatureID(r.FormValue("nomenclature_id"))
	if !ok {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL,
			"Выберите номенклатуру.")
		return
	}

	manufactureDate, err := parseFormDate(r.FormValue("manufacture_date"))
	if err != nil {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL, domainErrorMessage(err))
		return
	}
	arrivalDate, err := parseFormDate(r.FormValue("arrival_date"))
	if err != nil {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL, domainErrorMessage(err))
		return
	}

	status := strings.TrimSpace(r.FormValue("status"))
	if _, err := domain.ParseEquipmentStatus(status); err != nil {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL,
			"Некорректный статус оборудования.")
		return
	}

	eq, err := h.equipments.Create(r.Context(), domain.EquipmentInput{
		InventoryNumber: inventoryNumber,
		SerialNumber:    strings.TrimSpace(r.FormValue("serial_number")),
		NomenclatureID:  &nomenclatureID,
		ModelName:       strings.TrimSpace(r.FormValue("model_name")),
		ManufactureDate: manufactureDate,
		ArrivalDate:     arrivalDate,
		Status:          status,
		FormNumber:      strings.TrimSpace(r.FormValue("form_number")),
		Location:        strings.TrimSpace(r.FormValue("location")),
		Notes:           strings.TrimSpace(r.FormValue("notes")),
	})
	if err != nil {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL, domainErrorMessage(err))
		return
	}

	if _, err := h.assignments.Create(r.Context(), domain.AssignmentInput{
		EquipmentID: eq.ID,
		TargetType:  string(domain.TargetWarehouse),
	}); err != nil {
		h.equipmentModalError(w, r, "create", equipmentCreateActionURL,
			"Оборудование создано, но закрепить его на складе не удалось: "+domainErrorMessage(err))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/equipments")
	w.WriteHeader(http.StatusOK)
}

// EquipmentsEdit — GET /ui/equipments/{inventory_number}/edit: модальная форма
// редактирования оборудования.
func (h *PageHandler) EquipmentsEdit(w http.ResponseWriter, r *http.Request) {
	inventoryNumber := chi.URLParam(r, "inventory_number")
	if err := domain.ValidateInventoryNumber(inventoryNumber); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid inventory number in path, want ИТxxxxx")
		return
	}

	eq, err := h.equipments.GetByInventoryNumber(r.Context(), inventoryNumber)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.equipmentModalError(w, r, "edit", equipmentEditActionURL(inventoryNumber),
				domainErrorMessage(err))
			return
		}
		h.respondListError(w, r, "get equipment", err)
		return
	}

	nomenclatures, err := h.nomenclatures.List(r.Context(), domain.NomenclatureFilter{})
	if err != nil {
		h.respondListError(w, r, "list nomenclatures", err)
		return
	}

	nomenclatureID := 0
	if eq.NomenclatureID != nil {
		nomenclatureID = *eq.NomenclatureID
	}

	data := web.EquipmentModalData{
		InventoryNumber: eq.InventoryNumber,
		SerialNumber:    derefStr(eq.SerialNumber),
		NomenclatureID:  nomenclatureID,
		ModelName:       derefStr(eq.ModelName),
		ManufactureDate: formatDateForInput(eq.ManufactureDate),
		ArrivalDate:     formatDateForInput(eq.ArrivalDate),
		Status:          string(eq.Status),
		FormNumber:      derefStr(eq.FormNumber),
		Location:        derefStr(eq.Location),
		Notes:           derefStr(eq.Notes),
		Nomenclatures:   nomenclatures,
		ActionURL:       equipmentEditActionURL(inventoryNumber),
		IsEdit:          true,
	}
	h.render(w, r, http.StatusOK, web.EquipmentEditModal(data))
}

// EquipmentsUpdateSubmit — POST /ui/equipments/{inventory_number}: обновление
// оборудования.
func (h *PageHandler) EquipmentsUpdateSubmit(w http.ResponseWriter, r *http.Request) {
	inventoryNumber := chi.URLParam(r, "inventory_number")
	if err := domain.ValidateInventoryNumber(inventoryNumber); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid inventory number in path, want ИТxxxxx")
		return
	}

	nomenclatureID, ok := parseFormNomenclatureID(r.FormValue("nomenclature_id"))
	if !ok {
		h.equipmentModalError(w, r, "edit", equipmentEditActionURL(inventoryNumber),
			"Выберите номенклатуру.")
		return
	}

	manufactureDate, err := parseFormUpdateDate(r.FormValue("manufacture_date"))
	if err != nil {
		h.equipmentModalError(w, r, "edit", equipmentEditActionURL(inventoryNumber),
			domainErrorMessage(err))
		return
	}
	arrivalDate, err := parseFormUpdateDate(r.FormValue("arrival_date"))
	if err != nil {
		h.equipmentModalError(w, r, "edit", equipmentEditActionURL(inventoryNumber),
			domainErrorMessage(err))
		return
	}

	status := strings.TrimSpace(r.FormValue("status"))
	if _, err := domain.ParseEquipmentStatus(status); err != nil {
		h.equipmentModalError(w, r, "edit", equipmentEditActionURL(inventoryNumber),
			"Некорректный статус оборудования.")
		return
	}

	up := domain.EquipmentUpdate{
		SerialNumber:    ptrStr(r.FormValue("serial_number")),
		NomenclatureID:  &nomenclatureID,
		ModelName:       ptrStr(r.FormValue("model_name")),
		ManufactureDate: manufactureDate,
		ArrivalDate:     arrivalDate,
		Status:          &status,
		FormNumber:      ptrStr(r.FormValue("form_number")),
		Location:        ptrStr(r.FormValue("location")),
		Notes:           ptrStr(r.FormValue("notes")),
	}

	if _, err := h.equipments.UpdateByInventoryNumber(r.Context(), inventoryNumber, up); err != nil {
		h.equipmentModalError(w, r, "edit", equipmentEditActionURL(inventoryNumber),
			domainErrorMessage(err))
		return
	}

	w.Header().Set("HX-Redirect", "/ui/equipments")
	w.WriteHeader(http.StatusOK)
}
