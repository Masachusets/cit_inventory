package config_test

import (
	"fmt"
	"log"
	"os"

	"github.com/Masachusets/cit_inventory/pkg/config"
)

// mustSetenv устанавливает переменную окружения, завершая пример при ошибке.
func mustSetenv(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		log.Fatal(err)
	}
}

// Пример использования GetDSN() с DSN.
// Единственный запускаемый пример: Load() вызывается один раз за прогон,
// т.к. флаги регистрируются в глобальном flag только однократно.
func Example_getDSNWithDSN() {
	mustSetenv("DATABASE_URL", "postgres://user:pass@localhost:5432/mydb?sslmode=require")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// DATABASE_URL имеет приоритет
	fmt.Println(cfg.DB.GetDSN())
	// Output: postgres://user:pass@localhost:5432/mydb?sslmode=require
}

// Пример использования Load() - простейший случай.
// Без комментария Output - компилируется, но не выполняется.
func ExampleLoad() {
	// Load() автоматически:
	// 1. Читает config.toml (флаг -config, по умолчанию config.toml)
	// 2. Применяет переменные окружения
	// 3. Применяет флаги командной строки (наивысший приоритет)
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Server: %s:%s\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Database: %s\n", cfg.DB.GetDSN())
}

// Пример использования GetDSN() с компонентами.
// Без комментария Output - компилируется, но не выполняется.
func Example_getDSNWithComponents() {
	mustSetenv("DB_HOST", "localhost")
	mustSetenv("DB_PORT", "5432")
	mustSetenv("DB_NAME", "inventory")
	mustSetenv("DB_USER", "admin")
	mustSetenv("DB_PASSWORD", "secret")
	mustSetenv("DB_SSLMODE", "disable")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Собирается из компонентов, т.к. DATABASE_URL не задан
	fmt.Println(cfg.DB.GetDSN())
	// Формат: postgres://admin:secret@localhost:5432/inventory?sslmode=disable
}
