# Быстрый старт: Конфигурация

## Способ 1: Переменные окружения (рекомендуется для начала)

```bash
# 1. Скопируйте пример
cp .env.example .env

# 2. Отредактируйте .env
vim .env

# 3. Запустите приложение
go run cmd/server/main.go
```

## Способ 2: Файл config.toml

```bash
# 1. Скопируйте пример
cp config.toml.example config.toml

# 2. Отредактируйте config.toml
vim config.toml

# 3. Запустите приложение (автоматически загрузит config.toml)
go run cmd/server/main.go

# Или укажите другой путь через флаг -config
go run cmd/server/main.go -config=/path/to/custom.toml
```

## Способ 3: Флаги командной строки

```bash
go run cmd/server/main.go \
  -db-host=localhost \
  -db-name=inventory \
  -db-user=admin \
  -db-password=secret \
  -port=8080
```

## Использование в коде

```go
package main

import (
    "log"
    "yourproject/pkg/config"
)

func main() {
    // Загрузка конфигурации
    // Load() автоматически читает config.toml (путь можно изменить через флаг -config)
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    
    // Использование
    println("Server:", cfg.Server.Host + ":" + cfg.Server.Port)
    println("Database:", cfg.DB.GetDSN())
    println("Environment:", cfg.App.Environment)
}
```

## Приоритет настроек (от низшего к высшему)

1. Дефолтные значения в коде
2. config.toml
3. Переменные окружения
4. Флаги командной строки ← **наивысший приоритет**

## Переменные базы данных

**Вариант А: Полный DSN (приоритет)**
```bash
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=disable
```

**Вариант Б: Компоненты**
```bash
DB_HOST=localhost
DB_PORT=5432
DB_NAME=inventory
DB_USER=admin
DB_PASSWORD=secret
DB_SSLMODE=disable
```

## Полная документация

См. [CONFIG.md](./CONFIG.md)
