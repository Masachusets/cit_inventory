package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Masachusets/cit_inventory/internal/server/handlers"
	"github.com/Masachusets/cit_inventory/internal/server/middleware"
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

	// r.Route("/api/v1", func (r chi.Router) {})

	r.NotFound(handlers.NotFound)
	r.MethodNotAllowed(handlers.MethodNotAllowed)

	return r
}
