package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Masachusets/cit_inventory/internal/adapter/postgres"
	"github.com/Masachusets/cit_inventory/internal/app"
	"github.com/Masachusets/cit_inventory/internal/config"
	"github.com/Masachusets/cit_inventory/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// read a configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to set a config: %w", err)
	}

	// defining a logger
	log := logger.SetupLogger(cfg.App.Debug, cfg.App.LogLevel, cfg.App.LogFormat)

	log.Info("configuration loaded",
	"debug", cfg.App.Debug,
	"log level", cfg.App.LogLevel,
	"log format", cfg.App.LogFormat,
)

	// creating a context, cancelled upon a signal
	ctx, stop := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer stop()

	// connection to the database (PostgreSQL)
	pool, err := postgres.New(ctx, cfg, log)
	if err != nil {
		log.Error("failed to initialize database",
		"error", err,
		"host", cfg.DB.Host,
		"database", cfg.DB.Database,
	)
		return fmt.Errorf("error db connect: %w", err)
	}
	defer pool.Close()



	if err := app.Run(ctx, cfg, log, pool); err != nil {
		return err
	}

	return nil
}
