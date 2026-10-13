package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Masachusets/cit_inventory/internal/repository"
	"github.com/Masachusets/cit_inventory/internal/server/handlers"
	"github.com/Masachusets/cit_inventory/internal/server/middleware"
	"github.com/Masachusets/cit_inventory/internal/service"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(log *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.Logger(log))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	health := handlers.NewHealthHandler(pool)
	r.Get("/health/live", health.Live)
	r.Get("/health/ready", health.Ready)

	// Бизнес-слой: собираем цепочку зависимостей (composition root).
	// pool может быть nil в тестах — репозиторий создаётся лениво, обращения
	// к БД в health-проверках и роутинге не происходят.
	cardRepo := repository.NewCardRepo(pool)
	cardSvc := service.NewCardService(cardRepo)
	cards := handlers.NewCardHandler(cardSvc, log)

	r.Route("/api/v1", func(r chi.Router) {
		// Статический маршрут /cards/inactive объявлен ДО параметрического
		// /cards/{number}, иначе chi перехватил бы "inactive" как {number}.
		r.Get("/cards/inactive", cards.ListInactive)
		r.Get("/cards", cards.List)
		r.Post("/cards", cards.Create)

		r.Get("/cards/{number}", cards.Get)
		r.Patch("/cards/{number}", cards.Update)
		r.Post("/cards/{number}/deactivate", cards.Deactivate)
		r.Delete("/cards/{number}", cards.Delete)
	})

	r.NotFound(handlers.NotFound)
	r.MethodNotAllowed(handlers.MethodNotAllowed)

	return r
}
