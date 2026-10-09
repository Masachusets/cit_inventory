package server

import (
	"log/slog"
	"net/http"

	"github.com/Masachusets/cit_inventory/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	*http.Server
	log  *slog.Logger
	pool *pgxpool.Pool
}

func NewServer(cfg *config.Config, log *slog.Logger, pool *pgxpool.Pool) *Server {
	serv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      NewRouter(log, pool),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &Server{
		Server: serv,
		log:    log,
		pool:   pool,
	}
}
