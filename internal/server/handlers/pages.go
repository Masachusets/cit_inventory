package handlers

import (
	"errors"
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
// тестируется подменной реализацией. Методы страниц разнесены по файлам:
// pages_equipments.go, pages_cards.go, pages_departments.go,
// pages_nomenclatures.go, pages_waybills.go.
type PageHandler struct {
	equipments    service.EquipmentService
	cards         service.CardService
	departments   service.DepartmentService
	nomenclatures service.NomenclatureService
	waybills      service.WaybillService
	assignments   service.AssignmentService
	log           *slog.Logger
}

// NewPageHandler создаёт обработчик страниц веб-интерфейса.
func NewPageHandler(
	equipments service.EquipmentService,
	cards service.CardService,
	departments service.DepartmentService,
	nomenclatures service.NomenclatureService,
	waybills service.WaybillService,
	assignments service.AssignmentService,
	log *slog.Logger,
) *PageHandler {
	return &PageHandler{
		equipments:    equipments,
		cards:         cards,
		departments:   departments,
		nomenclatures: nomenclatures,
		waybills:      waybills,
		assignments:   assignments,
		log:           log,
	}
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

// respondListError общая обработка ошибки сервиса при загрузке списка:
// логирование + ответ 500 без утечки деталей. Возвращает true, если ошибка
// была (вызывающий должен прекратить обработку).
func (h *PageHandler) respondListError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	if err == nil {
		return false
	}
	h.log.Error(op,
		"error", err,
		"request_id", chimw.GetReqID(r.Context()))
	http.Error(w, "internal server error", http.StatusInternalServerError)
	return true
}

// isHXRequest определяет запрос от htmx: для него отдаётся только фрагмент
// вместо полной страницы.
func isHXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// filterValue извлекает значение текстового поля фильтра из query с обрезкой
// краевых пробелов; пустое/отсутствующее — "".
func filterValue(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

// domainErrorMessage маппит доменные ошибки на пользовательские сообщения.
func domainErrorMessage(err error) string {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return "Запись не найдена."
	case errors.Is(err, domain.ErrAlreadyExists):
		return "Запись с таким значением уже существует."
	case errors.Is(err, domain.ErrInvalidNumber):
		return "Некорректный номер записи."
	case errors.Is(err, domain.ErrInvalidName):
		return "Некорректное наименование."
	case errors.Is(err, domain.ErrInvalidCode):
		return "Некорректный код подразделения."
	case errors.Is(err, domain.ErrStillActive):
		return "Нельзя выполнить действие над активной записью."
	case errors.Is(err, domain.ErrInUse):
		return "Запись используется другим оборудованием."
	case errors.Is(err, domain.ErrInvalidID):
		return "Некорректный идентификатор."
	case errors.Is(err, domain.ErrInvalidInventoryNumber):
		return "Некорректный инвентарный номер (ожидается ИТxxxxx)."
	case errors.Is(err, domain.ErrInvalidNomenclature):
		return "Некорректная ссылка на номенклатуру."
	case errors.Is(err, domain.ErrInvalidStatus):
		return "Некорректный статус оборудования."
	case errors.Is(err, domain.ErrInvalidField):
		return "Некорректное значение поля."

	default:
		return "Внутренняя ошибка сервера."
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
