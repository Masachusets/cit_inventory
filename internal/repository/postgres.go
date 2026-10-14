package repository

// Файл содержит инфраструктуру, общую для всех pgx-репозиториев пакета.
// Конкретные реализации разнесены по файлам: card_repo.go, department_repo.go,
// nomenclature_repo.go, waybill_repo.go.

// pgUniqueViolation — код ошибки PostgreSQL при нарушении уникальности (PK).
const pgUniqueViolation = "23505"

// pgForeignKeyViolation — код ошибки PostgreSQL при нарушении внешнего ключа
// (ON DELETE RESTRICT): на карточку ссылается оборудование.
const pgForeignKeyViolation = "23503"

// cardScanner — то, что умеют и *pgx.Row, и pgx.Rows.
type cardScanner interface {
	Scan(dest ...any) error
}
