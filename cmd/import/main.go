// Command import — единый CLI импорта данных в БД.
//
// Подкоманды:
//
//	(по умолчанию)  импорт из Excel: номенклатура (синхронизация) →
//	                карточки КУ Ф-111 → оборудование «Учет»
//	departments     одноразовая загрузка справочника подразделений из текстового файла
//	help            справка
//
// Общая часть всех подкоманд: config.Load() → logger → postgres pool → graceful shutdown.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Masachusets/cit_inventory/internal/adapters/postgres"
	"github.com/Masachusets/cit_inventory/internal/importer"
	"github.com/Masachusets/cit_inventory/pkg/config"
	"github.com/Masachusets/cit_inventory/pkg/departments"
	"github.com/Masachusets/cit_inventory/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// cmdImport — подкоманда импорта из Excel (также поведение по умолчанию).
	cmdImport = "import"
	// cmdDepartments — подкоманда импорта справочника подразделений.
	cmdDepartments = "departments"
	// cmdHelp — подкоманда вывода справки.
	cmdHelp = "help"
)

// usage — справка по подкомандам и флагам.
const usage = `Использование: import [подкоманда] [флаги]

Подкоманды:
  (по умолчанию)   импорт из Excel: номенклатура (синхронизация) → карточки КУ Ф-111 → оборудование «Учет»
  departments      одноразовая загрузка справочника подразделений из текстового файла
  help             эта справка

Примеры:
  import
  import -excel=data/excel -ums=data/excel/UMS.xlsx
  import departments -file=data/departments.txt
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cmd, err := parseCommand()
	if err != nil {
		return err
	}

	switch cmd {
	case cmdDepartments:
		return runDepartments()
	case cmdHelp:
		fmt.Print(usage)
		return nil
	default:
		return runImport()
	}
}

// parseCommand выделяет подкоманду из аргументов и удаляет её из os.Args,
// иначе flag.Parse() внутри config.Load() принял бы её за аргументы.
// Подкомандой считается только первый аргумент, не начинающийся с «-»,
// поэтому значение флага (например, «-excel excel») подкомандой не станет.
// Если подкоманда не задана, выполняется импорт из Excel.
func parseCommand() (string, error) {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		return cmdImport, nil
	}

	cmd := os.Args[1]
	switch cmd {
	case cmdImport, cmdDepartments, cmdHelp:
		// ёмкость 1 заставляет append скопировать срез, не затирая исходный os.Args.
		os.Args = append(os.Args[:1:1], os.Args[2:]...)
		return cmd, nil
	default:
		return "", fmt.Errorf("неизвестная подкоманда %q\n\n%s", cmd, usage)
	}
}

// cli — общая инфраструктура подкоманд: конфиг, логгер, контекст и пул БД.
type cli struct {
	cfg  *config.Config
	log  *slog.Logger
	ctx  context.Context
	pool *pgxpool.Pool
	stop context.CancelFunc
}

// newCLI выполняет подготовку, общую для всех подкоманд:
// config.Load() → logger → контекст с graceful-остановкой → пул PostgreSQL.
// При успехе вызывающий код обязан вызвать (*cli).close (обычно через defer).
func newCLI() (*cli, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	log := logger.SetupLogger(cfg.App.Debug, cfg.App.LogLevel, cfg.App.LogFormat)

	// Остановка по Ctrl+C / SIGTERM (например, при долгом импорте).
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	pool, err := postgres.New(ctx, cfg, log)
	if err != nil {
		stop()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return &cli{cfg: cfg, log: log, ctx: ctx, pool: pool, stop: stop}, nil
}

// close освобождает ресурсы: пул БД и контекст.
func (c *cli) close() {
	c.pool.Close()
	c.stop()
}

// runImport — импорт из Excel: номенклатурная таблица, карточки и оборудование.
func runImport() error {
	// Флаги импорта регистрируются до config.Load(): там вызывается flag.Parse,
	// поэтому -excel/-ums подхватываются на общих условиях с остальными флагами.
	excelPath := flag.String("excel", "data/excel",
		"Путь к Excel-файлу номенклатурной таблицы или к директории с ними")
	umsPath := flag.String("ums", "data/excel/UMS.xlsx",
		"Путь к UMS.xlsx (листы «КУ Ф-111» → cards и «Учет» → equipments) или директория с ним; пустая строка — пропустить импорт карточек и оборудования")

	c, err := newCLI()
	if err != nil {
		return err
	}
	defer c.close()

	return importer.New(c.pool, c.log, *excelPath, *umsPath).Run(c.ctx)
}

// runDepartments — одноразовая загрузка справочника подразделений из текстового файла.
func runDepartments() error {
	// Флаг регистрируется до config.Load(): там вызывается flag.Parse.
	filePath := flag.String("file", "data/departments.txt",
		"Путь к текстовому файлу со списком подразделений («<код> <наименование>»)")

	c, err := newCLI()
	if err != nil {
		return err
	}
	defer c.close()

	data, err := os.ReadFile(*filePath)
	if err != nil {
		return fmt.Errorf("read departments file: %w", err)
	}

	depts, err := departments.Parse(strings.Split(string(data), "\n"))
	if err != nil {
		return fmt.Errorf("parse %s: %w", *filePath, err)
	}

	imported, err := departments.Insert(c.ctx, c.pool, depts)
	if err != nil {
		return fmt.Errorf("import departments: %w", err)
	}

	c.log.Info("departments import finished", "file", *filePath, "imported", imported)

	return nil
}
