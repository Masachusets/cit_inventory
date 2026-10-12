package importer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

// filePattern — шаблон имени номенклатурной таблицы: «Номенклатурная таблица MM.YYYY.xlsx».
var filePattern = regexp.MustCompile(`^Номенклатурная таблица (\d{1,2})\.(\d{4})\.xlsx$`)

// fileVersion — версия файла, извлечённая из имени (месяц и год).
type fileVersion struct {
	Month int
	Year  int
}

// Importer импортирует номенклатурную таблицу из Excel-файла в БД.
type Importer struct {
	pool    *pgxpool.Pool
	log     *slog.Logger
	srcPath string
}

// New создаёт Importer. srcPath — путь к Excel-файлу либо к директории,
// в которой автоматически выбирается файл с максимальной версией MM.YYYY.
func New(pool *pgxpool.Pool, log *slog.Logger, srcPath string) *Importer {
	return &Importer{pool: pool, log: log, srcPath: srcPath}
}

// Run выполняет импорт: выбор файла → парсинг → синхронизация с БД.
func (imp *Importer) Run(ctx context.Context) error {
	path, version, err := resolveFile(imp.srcPath)
	if err != nil {
		return fmt.Errorf("resolve excel file: %w", err)
	}
	imp.log.Info("excel file selected", "path", path, "version", version)

	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open excel file %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	records, warnings, err := ParseNomenclature(f)
	if err != nil {
		return fmt.Errorf("parse nomenclature: %w", err)
	}
	for _, w := range warnings {
		imp.log.Warn("parse warning", "sheet", w.Sheet, "row", w.Row, "message", w.Message)
	}
	imp.log.Info("nomenclature parsed", "records", len(records), "warnings", len(warnings))

	stats, err := Sync(ctx, imp.pool, records)
	if err != nil {
		return fmt.Errorf("sync nomenclatures: %w", err)
	}

	imp.log.Info("import finished",
		"added", stats.Added,
		"updated", stats.Updated,
		"deleted", stats.Deleted,
		"unresolved", len(stats.Unresolved),
	)
	if len(stats.Unresolved) > 0 {
		imp.log.Warn("codes not deleted: referenced by equipments",
			"codes", strings.Join(stats.Unresolved, ", "))
	}

	return nil
}

// resolveFile возвращает путь к файлу и его версию.
// Если srcPath — файл, используется он (имя по шаблону желательно, но не обязательно).
// Если директория — ищутся файлы «Номенклатурная таблица MM.YYYY.xlsx»
// и выбирается самый свежий.
func resolveFile(srcPath string) (string, fileVersion, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", fileVersion{}, err
	}
	if !info.IsDir() {
		v, _ := parseVersion(filepath.Base(srcPath))
		return srcPath, v, nil
	}

	entries, err := os.ReadDir(srcPath)
	if err != nil {
		return "", fileVersion{}, err
	}

	var (
		bestPath string
		bestVer  fileVersion
	)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		v, ok := parseVersion(e.Name())
		if !ok {
			continue
		}
		if bestPath == "" || newer(v, bestVer) {
			bestPath = filepath.Join(srcPath, e.Name())
			bestVer = v
		}
	}
	if bestPath == "" {
		return "", fileVersion{}, fmt.Errorf(
			"в директории %s не найден файл вида «Номенклатурная таблица MM.YYYY.xlsx»", srcPath)
	}
	return bestPath, bestVer, nil
}

// parseVersion извлекает месяц и год из имени файла по шаблону.
func parseVersion(name string) (fileVersion, bool) {
	m := filePattern.FindStringSubmatch(name)
	if m == nil {
		return fileVersion{}, false
	}
	month, err := strconv.Atoi(m[1])
	if err != nil || month < 1 || month > 12 {
		return fileVersion{}, false
	}
	year, err := strconv.Atoi(m[2])
	if err != nil {
		return fileVersion{}, false
	}
	return fileVersion{Month: month, Year: year}, true
}

// newer возвращает true, если версия a новее версии b.
func newer(a, b fileVersion) bool {
	if a.Year != b.Year {
		return a.Year > b.Year
	}
	return a.Month > b.Month
}

// String форматирует версию как MM.YYYY (для логов).
func (v fileVersion) String() string {
	if v.Month == 0 && v.Year == 0 {
		return "неизвестна"
	}
	return fmt.Sprintf("%02d.%04d", v.Month, v.Year)
}
