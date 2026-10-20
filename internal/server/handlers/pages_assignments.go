package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masachusets/cit_inventory/internal/domain"
	"github.com/Masachusets/cit_inventory/internal/repository"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EquipmentAssignHandler — HTTP-обработчик изменения закрепления оборудования.
type EquipmentAssignHandler struct {
	svc            service.AssignmentService
	equipments     service.EquipmentService
	cards          service.CardService
	departments    service.DepartmentService
	waybills       service.WaybillService
	assignmentRepo repository.AssignmentRepository
	waybillRepo    repository.WaybillRepository
	pool           *pgxpool.Pool
	log            *slog.Logger
}

// NewEquipmentAssignHandler создаёт обработчик изменения закрепления.
func NewEquipmentAssignHandler(
	svc service.AssignmentService,
	equipments service.EquipmentService,
	cards service.CardService,
	departments service.DepartmentService,
	waybills service.WaybillService,
	assignmentRepo repository.AssignmentRepository,
	waybillRepo repository.WaybillRepository,
	pool *pgxpool.Pool,
	log *slog.Logger,
) *EquipmentAssignHandler {
	return &EquipmentAssignHandler{
		svc:            svc,
		equipments:     equipments,
		cards:          cards,
		departments:    departments,
		waybills:       waybills,
		assignmentRepo: assignmentRepo,
		waybillRepo:    waybillRepo,
		pool:           pool,
		log:            log,
	}
}

// render пишет HTML-ответ компонента с заданным статусом.
func (h *EquipmentAssignHandler) render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		h.log.Error("render page",
			"error", err,
			"path", r.URL.Path,
			"request_id", chimw.GetReqID(r.Context()))
	}
}

// respondListError общая обработка ошибки сервиса при загрузке списка.
func (h *EquipmentAssignHandler) respondListError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	if err == nil {
		return false
	}
	h.log.Error(op,
		"error", err,
		"request_id", chimw.GetReqID(r.Context()))
	http.Error(w, "internal server error", http.StatusInternalServerError)
	return true
}

// EquipmentsAssign — GET /ui/equipments/{inventory_number}/assign.
// Отдаёт модальную форму изменения закрепления.
func (h *EquipmentAssignHandler) EquipmentsAssign(w http.ResponseWriter, r *http.Request) {
	inventoryNumber := chi.URLParam(r, "inventory_number")
	if err := domain.ValidateInventoryNumber(inventoryNumber); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid inventory number, want ИТxxxxx")
		return
	}

	eq, err := h.equipments.GetByInventoryNumber(r.Context(), inventoryNumber)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.renderAssignForm(w, r, inventoryNumber, string(domain.TargetEmployee), web.EquipmentAssignModalData{
				InventoryNumber: inventoryNumber,
				TargetType:      string(domain.TargetEmployee),
				Error:           "Оборудование не найдено.",
				ActionURL:       assignActionURL(inventoryNumber),
			})
			return
		}
		h.respondListError(w, r, "get equipment", err)
		return
	}

	if eq.DeletedAt != nil {
		h.renderAssignForm(w, r, inventoryNumber, string(domain.TargetEmployee), web.EquipmentAssignModalData{
			InventoryNumber: inventoryNumber,
			TargetType:      string(domain.TargetEmployee),
			Error:           "Нельзя изменять закрепление для списанного оборудования.",
			ActionURL:       assignActionURL(inventoryNumber),
		})
		return
	}

	// Загружаем справочники для формы.
	cards, err := h.cards.ListActive(r.Context(), domain.CardFilter{})
	if err != nil {
		h.respondListError(w, r, "list cards", err)
		return
	}
	departments, err := h.departments.ListActive(r.Context(), domain.DepartmentFilter{})
	if err != nil {
		h.respondListError(w, r, "list departments", err)
		return
	}
	waybills, err := h.waybills.List(r.Context(), domain.WaybillFilter{})
	if err != nil {
		h.respondListError(w, r, "list waybills", err)
		return
	}

	data := web.EquipmentAssignModalData{
		InventoryNumber: inventoryNumber,
		TargetType:      string(domain.TargetEmployee),
		ActionURL:       assignActionURL(inventoryNumber),
		Cards:           cards,
		Departments:     departments,
		Waybills:        waybills,
		IsNewWaybill:    false,
	}
	h.render(w, r, http.StatusOK, web.EquipmentAssignForm(data))
}

// EquipmentsAssignSubmit — POST /ui/equipments/{inventory_number}/assign.
// Обрабатывает отправку формы изменения закрепления.
func (h *EquipmentAssignHandler) EquipmentsAssignSubmit(w http.ResponseWriter, r *http.Request) {
	inventoryNumber := chi.URLParam(r, "inventory_number")
	if err := domain.ValidateInventoryNumber(inventoryNumber); err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid inventory number, want ИТxxxxx")
		return
	}

	eq, err := h.equipments.GetByInventoryNumber(r.Context(), inventoryNumber)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.renderAssignForm(w, r, inventoryNumber, string(domain.TargetEmployee), web.EquipmentAssignModalData{
				InventoryNumber: inventoryNumber,
				TargetType:      string(domain.TargetEmployee),
				Error:           "Оборудование не найдено.",
				ActionURL:       assignActionURL(inventoryNumber),
			})
			return
		}
		h.respondListError(w, r, "get equipment", err)
		return
	}

	if eq.DeletedAt != nil {
		h.renderAssignForm(w, r, inventoryNumber, string(domain.TargetEmployee), web.EquipmentAssignModalData{
			InventoryNumber: inventoryNumber,
			TargetType:      string(domain.TargetEmployee),
			Error:           "Нельзя изменять закрепление для списанного оборудования.",
			ActionURL:       assignActionURL(inventoryNumber),
		})
		return
	}

	// Парсим форму.
	targetType := strings.TrimSpace(r.FormValue("target_type"))
	cardNumberStr := strings.TrimSpace(r.FormValue("card_number"))
	departmentCodeStr := strings.TrimSpace(r.FormValue("department_code"))
	waybillMode := strings.TrimSpace(r.FormValue("waybill_mode"))
	waybillIDStr := strings.TrimSpace(r.FormValue("waybill_id"))
	waybillName := strings.TrimSpace(r.FormValue("waybill_name"))
	waybillNumber := strings.TrimSpace(r.FormValue("waybill_number"))
	waybillIssueDateStr := strings.TrimSpace(r.FormValue("waybill_issue_date"))
	operatorComment := strings.TrimSpace(r.FormValue("operator_comment"))

	// Загружаем справочники для повторной отрисовки при ошибке.
	cards, err := h.cards.ListActive(r.Context(), domain.CardFilter{})
	if err != nil {
		h.respondListError(w, r, "list cards", err)
		return
	}
	departments, err := h.departments.ListActive(r.Context(), domain.DepartmentFilter{})
	if err != nil {
		h.respondListError(w, r, "list departments", err)
		return
	}
	waybills, err := h.waybills.List(r.Context(), domain.WaybillFilter{})
	if err != nil {
		h.respondListError(w, r, "list waybills", err)
		return
	}

	// Валидация цели закрепления.
	tt, err := domain.ParseAssignmentTarget(targetType)
	if err != nil {
		h.renderAssignForm(w, r, inventoryNumber, targetType, web.EquipmentAssignModalData{
			InventoryNumber:  inventoryNumber,
			TargetType:       targetType,
			Error:            "Некорректная цель закрепления.",
			ActionURL:        assignActionURL(inventoryNumber),
			Cards:            cards,
			Departments:      departments,
			Waybills:         waybills,
			IsNewWaybill:     waybillMode == "new",
			WaybillName:      waybillName,
			WaybillNumber:    waybillNumber,
			WaybillIssueDate: waybillIssueDateStr,
			OperatorComment:  operatorComment,
		})
		return
	}

	var cardNumber *int
	var departmentCode *int16
	var waybillID *int

	switch tt {
	case domain.TargetEmployee:
		if cardNumberStr == "" {
			h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
				InventoryNumber:  inventoryNumber,
				TargetType:       string(tt),
				Error:            "Для цели «сотрудник» укажите номер карточки.",
				ActionURL:        assignActionURL(inventoryNumber),
				Cards:            cards,
				Departments:      departments,
				Waybills:         waybills,
				IsNewWaybill:     waybillMode == "new",
				WaybillName:      waybillName,
				WaybillNumber:    waybillNumber,
				WaybillIssueDate: waybillIssueDateStr,
				OperatorComment:  operatorComment,
			})
			return
		}
		cn, err := strconv.Atoi(cardNumberStr)
		if err != nil || cn <= 0 {
			h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
				InventoryNumber:  inventoryNumber,
				TargetType:       string(tt),
				Error:            "Некорректный номер карточки.",
				ActionURL:        assignActionURL(inventoryNumber),
				Cards:            cards,
				Departments:      departments,
				Waybills:         waybills,
				IsNewWaybill:     waybillMode == "new",
				WaybillName:      waybillName,
				WaybillNumber:    waybillNumber,
				WaybillIssueDate: waybillIssueDateStr,
				OperatorComment:  operatorComment,
			})
			return
		}
		cardNumber = &cn

	case domain.TargetDepartment:
		if departmentCodeStr == "" {
			h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
				InventoryNumber:  inventoryNumber,
				TargetType:       string(tt),
				Error:            "Для цели «подразделение» выберите подразделение.",
				ActionURL:        assignActionURL(inventoryNumber),
				Cards:            cards,
				Departments:      departments,
				Waybills:         waybills,
				IsNewWaybill:     waybillMode == "new",
				WaybillName:      waybillName,
				WaybillNumber:    waybillNumber,
				WaybillIssueDate: waybillIssueDateStr,
				OperatorComment:  operatorComment,
			})
			return
		}
		dc, err := strconv.Atoi(departmentCodeStr)
		if err != nil || dc <= 0 {
			h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
				InventoryNumber:  inventoryNumber,
				TargetType:       string(tt),
				Error:            "Некорректный код подразделения.",
				ActionURL:        assignActionURL(inventoryNumber),
				Cards:            cards,
				Departments:      departments,
				Waybills:         waybills,
				IsNewWaybill:     waybillMode == "new",
				WaybillName:      waybillName,
				WaybillNumber:    waybillNumber,
				WaybillIssueDate: waybillIssueDateStr,
				OperatorComment:  operatorComment,
			})
			return
		}
		dc16 := int16(dc)
		departmentCode = &dc16

	case domain.TargetWarehouse:
		// нет дополнительных ссылок
	}

	// Накладная — только для подразделения/склада; для сотрудника её не указывают.
	var issueDate *time.Time
	if tt != domain.TargetEmployee {
		if waybillIssueDateStr != "" {
			d, err := time.Parse("2006-01-02", waybillIssueDateStr)
			if err != nil {
				h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
					InventoryNumber:  inventoryNumber,
					TargetType:       string(tt),
					Error:            "Некорректная дата документа (ожидается YYYY-MM-DD).",
					ActionURL:        assignActionURL(inventoryNumber),
					Cards:            cards,
					Departments:      departments,
					Waybills:         waybills,
					IsNewWaybill:     waybillMode == "new",
					WaybillName:      waybillName,
					WaybillNumber:    waybillNumber,
					WaybillIssueDate: waybillIssueDateStr,
					OperatorComment:  operatorComment,
				})
				return
			}
			issueDate = &d
		}

		if waybillMode == "new" {
			if waybillName == "" {
				h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
					InventoryNumber:  inventoryNumber,
					TargetType:       string(tt),
					Error:            "Для новой накладной укажите наименование.",
					ActionURL:        assignActionURL(inventoryNumber),
					Cards:            cards,
					Departments:      departments,
					Waybills:         waybills,
					IsNewWaybill:     true,
					WaybillName:      waybillName,
					WaybillNumber:    waybillNumber,
					WaybillIssueDate: waybillIssueDateStr,
					OperatorComment:  operatorComment,
				})
				return
			}
		} else if waybillMode == "existing" && waybillIDStr != "" && waybillIDStr != "0" {
			wid, err := strconv.Atoi(waybillIDStr)
			if err != nil || wid <= 0 {
				h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
					InventoryNumber: inventoryNumber,
					TargetType:      string(tt),
					Error:           "Некорректный идентификатор накладной.",
					ActionURL:       assignActionURL(inventoryNumber),
					Cards:           cards,
					Departments:     departments,
					Waybills:        waybills,
					IsNewWaybill:    false,
					WaybillID:       0,
				})
				return
			}
			waybillID = &wid
		}
	}

	// Выполняем всё в одной транзакции.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		h.log.Error("begin transaction", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	// 1. Закрываем предыдущее активное закрепление.
	if err := h.assignmentRepo.CloseActiveForEquipmentWithTx(r.Context(), tx, eq.ID); err != nil {
		h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
			InventoryNumber:  inventoryNumber,
			TargetType:       string(tt),
			Error:            domainErrorMessage(err),
			ActionURL:        assignActionURL(inventoryNumber),
			Cards:            cards,
			Departments:      departments,
			Waybills:         waybills,
			IsNewWaybill:     waybillMode == "new",
			WaybillName:      waybillName,
			WaybillNumber:    waybillNumber,
			WaybillIssueDate: waybillIssueDateStr,
			OperatorComment:  operatorComment,
		})
		return
	}

	// 2. При необходимости создаём новую накладную (только для не-employee целей).
	if waybillMode == "new" && tt != domain.TargetEmployee {
		wb, err := domain.NewWaybill(waybillName, waybillNumber, issueDate)
		if err != nil {
			h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
				InventoryNumber:  inventoryNumber,
				TargetType:       string(tt),
				Error:            domainErrorMessage(err),
				ActionURL:        assignActionURL(inventoryNumber),
				Cards:            cards,
				Departments:      departments,
				Waybills:         waybills,
				IsNewWaybill:     true,
				WaybillName:      waybillName,
				WaybillNumber:    waybillNumber,
				WaybillIssueDate: waybillIssueDateStr,
				OperatorComment:  operatorComment,
			})
			return
		}
		wid, err := h.waybillRepo.CreateWithIDWithTx(r.Context(), tx, wb)
		if err != nil {
			h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
				InventoryNumber:  inventoryNumber,
				TargetType:       string(tt),
				Error:            domainErrorMessage(err),
				ActionURL:        assignActionURL(inventoryNumber),
				Cards:            cards,
				Departments:      departments,
				Waybills:         waybills,
				IsNewWaybill:     true,
				WaybillName:      waybillName,
				WaybillNumber:    waybillNumber,
				WaybillIssueDate: waybillIssueDateStr,
				OperatorComment:  operatorComment,
			})
			return
		}
		waybillID = &wid
	}

	// 3. Создаём новое закрепление.
	assignedAt := time.Now()
	deptCodeInt := func() *int {
		if departmentCode != nil {
			v := int(*departmentCode)
			return &v
		}
		return nil
	}()

	a, err := domain.NewAssignment(domain.AssignmentInput{
		EquipmentID:     eq.ID,
		TargetType:      string(tt),
		CardNumber:      cardNumber,
		DepartmentCode:  deptCodeInt,
		WaybillID:       waybillID,
		AssignedAt:      &assignedAt,
		OperatorComment: operatorComment,
	})
	if err != nil {
		h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
			InventoryNumber:  inventoryNumber,
			TargetType:       string(tt),
			Error:            domainErrorMessage(err),
			ActionURL:        assignActionURL(inventoryNumber),
			Cards:            cards,
			Departments:      departments,
			Waybills:         waybills,
			IsNewWaybill:     waybillMode == "new",
			WaybillName:      waybillName,
			WaybillNumber:    waybillNumber,
			WaybillIssueDate: waybillIssueDateStr,
			OperatorComment:  operatorComment,
		})
		return
	}

	if err := h.assignmentRepo.CreateWithTx(r.Context(), tx, a); err != nil {
		h.renderAssignForm(w, r, inventoryNumber, string(tt), web.EquipmentAssignModalData{
			InventoryNumber:  inventoryNumber,
			TargetType:       string(tt),
			Error:            domainErrorMessage(err),
			ActionURL:        assignActionURL(inventoryNumber),
			Cards:            cards,
			Departments:      departments,
			Waybills:         waybills,
			IsNewWaybill:     waybillMode == "new",
			WaybillName:      waybillName,
			WaybillNumber:    waybillNumber,
			WaybillIssueDate: waybillIssueDateStr,
			OperatorComment:  operatorComment,
		})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		h.log.Error("commit transaction", "error", err, "request_id", chimw.GetReqID(r.Context()))
		respondError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("HX-Redirect", "/ui/equipments")
	w.WriteHeader(http.StatusOK)
}

// renderAssignForm — вспомогательный метод для отрисовки формы с ошибками.
func (h *EquipmentAssignHandler) renderAssignForm(w http.ResponseWriter, r *http.Request, inventoryNumber, targetType string, data web.EquipmentAssignModalData) {
	h.render(w, r, http.StatusOK, web.EquipmentAssignForm(data))
}

// assignActionURL возвращает URL для отправки формы.
func assignActionURL(inventoryNumber string) string {
	return "/ui/equipments/" + inventoryNumber + "/assign"
}

// EquipmentsAssignFields — GET /ui/equipments/{inventory_number}/assign/fields.
// Частичный фрагмент полей формы в зависимости от выбранной цели закрепления
// (htmx подменяет содержимое #assign-fields при переключении radio). Блок
// накладной входит в фрагмент и выводится только для department/warehouse;
// для employee накладная не показывается. Поля запроса (hx-include передаёт
// поля формы) позволяют сохранить введённые значения при переключении цели.
func (h *EquipmentAssignHandler) EquipmentsAssignFields(w http.ResponseWriter, r *http.Request) {
	inventoryNumber := chi.URLParam(r, "inventory_number")
	targetType := r.URL.Query().Get("target_type")
	tt, err := domain.ParseAssignmentTarget(targetType)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "invalid target_type, want employee / department / warehouse")
		return
	}

	data := web.EquipmentAssignModalData{
		InventoryNumber: inventoryNumber,
		TargetType:      targetType,
		// Сохраняем ранее введённые значения (hx-include передаёт поля формы).
		CardNumber: r.URL.Query().Get("card_number"),
	}

	switch tt {
	case domain.TargetEmployee:
		cards, err := h.cards.ListActive(r.Context(), domain.CardFilter{})
		if h.respondListError(w, r, "list cards", err) {
			return
		}
		data.Cards = cards
	case domain.TargetDepartment:
		departments, err := h.departments.ListActive(r.Context(), domain.DepartmentFilter{})
		if h.respondListError(w, r, "list departments", err) {
			return
		}
		data.Departments = departments
		if v := r.URL.Query().Get("department_code"); v != "" {
			if dc, err := strconv.Atoi(v); err == nil && dc > 0 {
				data.DepartmentCode = int16(dc)
			}
		}
	case domain.TargetWarehouse:
		// Дополнительные поля не требуются.
	}

	// Накладная — только для подразделения и склада.
	if tt != domain.TargetEmployee {
		if r.URL.Query().Get("waybill_mode") == "new" {
			data.IsNewWaybill = true
			data.WaybillName = r.URL.Query().Get("waybill_name")
			data.WaybillNumber = r.URL.Query().Get("waybill_number")
			data.WaybillIssueDate = r.URL.Query().Get("waybill_issue_date")
		} else {
			waybills, err := h.waybills.List(r.Context(), domain.WaybillFilter{})
			if h.respondListError(w, r, "list waybills", err) {
				return
			}
			data.Waybills = waybills
			if v := r.URL.Query().Get("waybill_id"); v != "" && v != "0" {
				if wid, err := strconv.Atoi(v); err == nil && wid > 0 {
					data.WaybillID = wid
				}
			}
		}
	}

	h.render(w, r, http.StatusOK, web.AssignTargetFieldsFragment(data))
}

// EquipmentsAssignWaybillFields — GET /ui/equipments/{inventory_number}/assign/waybill-fields.
// Частичный фрагмент полей накладной в зависимости от режима (existing / new);
// htmx подменяет содержимое #waybill-fields при переключении radio.
func (h *EquipmentAssignHandler) EquipmentsAssignWaybillFields(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode != "existing" && mode != "new" {
		respondError(w, r, http.StatusBadRequest, "invalid mode, want existing / new")
		return
	}

	data := web.EquipmentAssignModalData{IsNewWaybill: mode == "new"}

	if data.IsNewWaybill {
		data.WaybillName = r.URL.Query().Get("waybill_name")
		data.WaybillNumber = r.URL.Query().Get("waybill_number")
		data.WaybillIssueDate = r.URL.Query().Get("waybill_issue_date")
	} else {
		waybills, err := h.waybills.List(r.Context(), domain.WaybillFilter{})
		if h.respondListError(w, r, "list waybills", err) {
			return
		}
		data.Waybills = waybills
	}

	h.render(w, r, http.StatusOK, web.AssignWaybillFieldsFragment(data))
}
