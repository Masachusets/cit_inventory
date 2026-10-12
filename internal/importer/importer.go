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

// Importer импортирует данные из Excel-файлов в БД:
// номенклатурную таблицу (srcPath) и карточки учёта UMS (umsPath).
type Importer struct {
	pool    *pgxpool.Pool
	log     *slog.Logger
	srcPath string
	umsPath string
}

// New создаёт Importer.
// srcPath — путь к номенклатурной таблице (файл либо директория,
// в которой выбирается файл с максимальной версией MM.YYYY);
// umsPath — путь к UMS.xlsx (файл либо директория; пустая строка —
// импорт карточек пропускается).
func New(pool *pgxpool.Pool, log *slog.Logger, srcPath, umsPath string) *Importer {
	return &Importer{pool: pool, log: log, srcPath: srcPath, umsPath: umsPath}
}

// Run выполняет импорт: номенклатурная таблица → карточки учёта → оборудование.
func (imp *Importer) Run(ctx context.Context) error {
	if err := imp.runNomenclature(ctx); err != nil {
		return err
	}
	if err := imp.runCards(ctx); err != nil {
		return err
	}
	return imp.runEquipment(ctx)
}

// runNomenclature: выбор файла → парсинг → синхронизация с БД.
func (imp *Importer) runNomenclature(ctx context.Context) error {
	path, version, err := resolveFile(imp.srcPath)
	if err != nil {
		return fmt.Errorf("resolve excel file: %w", err)
	}
	imp.log.Info("excel file selected", "kind", "nomenclature", "path", path, "version", version)

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

	imp.log.Info("nomenclature import finished",
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

// runCards: выбор UMS.xlsx → парсинг листа «КУ Ф-111» → вставка в cards.
// Импорт одноразовый: без синхронизации, повторный запуск упадёт на UNIQUE.
func (imp *Importer) runCards(ctx context.Context) error {
	if imp.umsPath == "" {
		imp.log.Info("cards import skipped: no UMS path")
		return nil
	}

	path, err := resolveUMSFile(imp.umsPath)
	if err != nil {
		return fmt.Errorf("resolve UMS file: %w", err)
	}
	imp.log.Info("excel file selected", "kind", "cards", "path", path)

	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open UMS file %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	res, err := ParseCards(f)
	if err != nil {
		return fmt.Errorf("parse cards: %w", err)
	}
	for _, w := range res.Warnings {
		imp.log.Warn("parse warning", "sheet", w.Sheet, "row", w.Row, "message", w.Message)
	}
	imp.log.Info("cards parsed",
		"records", len(res.Cards),
		"skipped_written_off", res.WrittenOff,
		"warnings", len(res.Warnings),
	)

	imported, err := ImportCards(ctx, imp.pool, res.Cards)
	if err != nil {
		return fmt.Errorf("import cards: %w", err)
	}

	imp.log.Info("cards import finished",
		"imported", imported,
		"skipped_written_off", res.WrittenOff,
	)

	return nil
}

// runEquipment: выбор UMS.xlsx → парсинг листа «Учет» → вставка в equipments
// и equipments_assignments. Импорт одноразовый: без синхронизации, повторный
// запуск упадёт на UNIQUE(inventory_number).
func (imp *Importer) runEquipment(ctx context.Context) error {
	if imp.umsPath == "" {
		imp.log.Info("equipment import skipped: no UMS path")
		return nil
	}

	path, err := resolveUMSFile(imp.umsPath)
	if err != nil {
		return fmt.Errorf("resolve UMS file: %w", err)
	}
	imp.log.Info("excel file selected", "kind", "equipment", "path", path)

	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open UMS file %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	res, err := ParseEquipment(f)
	if err != nil {
		return fmt.Errorf("parse equipment: %w", err)
	}
	for _, w := range res.Warnings {
		imp.log.Warn("parse warning", "sheet", w.Sheet, "row", w.Row, "message", w.Message)
	}
	imp.log.Info("equipment parsed", "records", len(res.Equipments), "warnings", len(res.Warnings))

	stats, warnings, err := ImportEquipment(ctx, imp.pool, res.Equipments)
	if err != nil {
		return fmt.Errorf("import equipment: %w", err)
	}
	for _, w := range warnings {
		imp.log.Warn("resolve warning", "sheet", w.Sheet, "row", w.Row, "message", w.Message)
	}

	imp.log.Info("equipment import finished",
		"equipments", stats.Equipments,
		"assignments", stats.Assignments,
		"assign_employee", stats.AssignEmployee,
		"assign_department", stats.AssignDepartment,
		"assign_warehouse", stats.AssignWarehouse,
		"warnings", stats.Warnings,
	)

	return nil
}

// resolveUMSFile возвращает путь к UMS.xlsx: сам файл либо директорию с ним.
func resolveUMSFile(srcPath string) (string, error) {
	info, err := os.Stat(srcPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return srcPath, nil
	}
	path := filepath.Join(srcPath, "UMS.xlsx")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("UMS.xlsx не найден в директории %s", srcPath)
	}
	return path, nil
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
