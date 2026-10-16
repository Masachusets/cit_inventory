package handlers

import (
	"log/slog"
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/web"
	"github.com/a-h/templ"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// PageHandler — HTML-хендлеры веб-интерфейса (/ui): серверный рендеринг
// templ-компонентов вместо JSON. Зависит только от шаблонов web, бизнес-слоем
// (service) пользоваться начнёт на следующих этапах.
type PageHandler struct {
	log *slog.Logger
}

// NewPageHandler создаёт обработчик страниц веб-интерфейса.
func NewPageHandler(log *slog.Logger) *PageHandler {
	return &PageHandler{log: log}
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
