package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Masachusets/cit_inventory/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// type QueryTracer struct {
// 	logger *slog.Logger
// }

// func (t *QueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
// 	return ctx
// }

// func (t *QueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
// 	if data.Err != nil {
// 		t.logger.Error("database query failed",
// 			"error", data.Err,
// 			"sql", data.SQL,
// 		)
// 	}
// 	// Здесь можно добавить логику для логирования медленных запросов (например, > 100ms)
// }

// func (t *QueryTracer) TraceQueryStart(
//     ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
// ) context.Context {
//     t.logger.DebugContext(ctx, "query start", "sql", data.SQL)
//     return ctx
// }

// func (t *QueryTracer) TraceQueryEnd(
//     ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData,
// ) {
//     if data.Err != nil {
//         t.logger.ErrorContext(ctx, "query failed", "err", data.Err)
//         return
//     }
//     t.logger.DebugContext(ctx, "query done", "tag", data.CommandTag.String())
// }

func New(ctx context.Context, cfg *config.Config, logger *slog.Logger) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DB.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	poolConfig.MaxConns = cfg.DB.MaxConnections
	poolConfig.MinConns = cfg.DB.MinConnections
	poolConfig.MaxConnLifetime = cfg.DB.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.DB.MaxConnIdleTime
	
	// poolConfig.ConnConfig.Tracer = &QueryTracer{logger: logger}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx);err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connected successfully")

	return pool, nil
}