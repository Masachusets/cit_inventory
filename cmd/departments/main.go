package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Masachusets/cit_inventory/internal/adapters/postgres"
	"github.com/Masachusets/cit_inventory/internal/config"
	"github.com/Masachusets/cit_inventory/internal/departments"
	"github.com/Masachusets/cit_inventory/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "departments import failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Флаг регистрируется до config.Load(): там вызывается flag.Parse.
	filePath := flag.String("file", "data/departments.txt",
		"Путь к текстовому файлу со списком подразделений («<код> <наименование>»)")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.SetupLogger(cfg.App.Debug, cfg.App.LogLevel, cfg.App.LogFormat)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	pool, err := postgres.New(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	data, err := os.ReadFile(*filePath)
	if err != nil {
		return fmt.Errorf("read departments file: %w", err)
	}

	depts, err := departments.Parse(strings.Split(string(data), "\n"))
	if err != nil {
		return fmt.Errorf("parse %s: %w", *filePath, err)
	}

	imported, err := departments.Insert(ctx, pool, depts)
	if err != nil {
		return fmt.Errorf("import departments: %w", err)
	}

	log.Info("departments import finished", "file", *filePath, "imported", imported)

	return nil
}
