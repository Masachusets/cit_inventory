package web

import (
	"io/fs"
	"net/http"
)

// staticFS — содержимое static/ из Static (корень отдаётся по /static/*).
// Ошибка невозможна: каталог входит в go:embed.
var staticFS, _ = fs.Sub(Static, "static")

// StaticHandler отдаёт статические ресурсы веб-слоя (htmx, CSS)
// по префиксу /static/*.
func StaticHandler() http.Handler {
	return http.StripPrefix("/static/", http.FileServerFS(staticFS))
}
