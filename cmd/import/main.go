package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Masachusets/cit_inventory/internal/adapters/postgres"
	"github.com/Masachusets/cit_inventory/internal/config"
	"github.com/Masachusets/cit_inventory/internal/importer"
	"github.com/Masachusets/cit_inventory/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Флаги импорта регистрируются до config.Load(): там вызывается flag.Parse,
	// поэтому -excel/-ums подхватываются на общих условиях с остальными флагами.
	excelPath := flag.String("excel", "excel",
		"Путь к Excel-файлу номенклатурной таблицы или к директории с ними")
	umsPath := flag.String("ums", "excel/UMS.xlsx",
		"Путь к UMS.xlsx (карточки КУ Ф-111) или директория с ним; пустая строка — пропустить импорт карточек")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.SetupLogger(cfg.App.Debug, cfg.App.LogLevel, cfg.App.LogFormat)

	// Остановка по Ctrl+C / SIGTERM (например, при долгом импорте).
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

	return importer.New(pool, log, *excelPath, *umsPath).Run(ctx)
}
