# Конфигурация приложения

Приложение поддерживает загрузку конфигурации из нескольких источников с приоритетами.

## Порядок приоритета (от низшего к высшему)

1. **Дефолтные значения** - встроены в код (см. теги `env-default` в `/pkg/config/config.go`)
2. **Файл config.toml** - конфигурационный файл
3. **Переменные окружения** - системные переменные окружения
4. **Флаги командной строки** - параметры запуска (наивысший приоритет)

## Использование

### Вариант 1: Файл config.toml

```bash
# Скопируйте пример
cp config.toml.example config.toml

# Отредактируйте config.toml под свои нужды
vim config.toml

# Запустите приложение (по умолчанию ищет config.toml)
./app

# Или укажите другой путь
./app -config=/path/to/config.toml
```

### Вариант 2: Переменные окружения

```bash
# Скопируйте пример
cp .env.example .env

# Отредактируйте .env
vim .env

# Запустите приложение (cleanenv автоматически загрузит .env)
./app
```

### Вариант 3: Системные переменные окружения

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=inventory
export SERVER_PORT=8080

./app
```

### Вариант 4: Флаги командной строки

```bash
./app \
  -db-host=localhost \
  -db-port=5432 \
  -db-name=inventory \
  -port=8080 \
  -log-level=debug
```

### Комбинирование источников

Можно комбинировать разные источники. Например:

```bash
# Базовая конфигурация из config.toml
# Секреты из переменных окружения
export DB_PASSWORD=super_secret_password

# Переопределение порта через флаг
./app -config=config.toml -port=9000
```

## Доступные флаги

```
Database:
  -db-dsn string       PostgreSQL DSN (приоритет над компонентами)
  -db-host string      Database host
  -db-port string      Database port
  -db-name string      Database name
  -db-user string      Database user
  -db-password string  Database password

Server:
  -port string         HTTP server port
  -host string         HTTP server host

Application:
  -env string          Environment: development, staging, production
  -debug               Enable debug mode
  -log-level string    Log level: debug, info, warn, error
  -log-format string   Log format: text, terminal, json
```

Полный список флагов: `./app -help`

## Переменные окружения

### Server
- `SERVER_PORT` - порт HTTP сервера (default: `8080`)
- `SERVER_HOST` - хост HTTP сервера (default: `localhost`)
- `SERVER_READ_TIMEOUT` - таймаут чтения (default: `10s`)
- `SERVER_WRITE_TIMEOUT` - таймаут записи (default: `10s`)
- `SERVER_IDLE_TIMEOUT` - таймаут idle соединения (default: `60s`)
- `SERVER_SHUTDOWN_TIMEOUT` - таймаут graceful shutdown (default: `30s`)

### Database

**Вариант 1: Полный DSN**
- `DATABASE_URL` - полная строка подключения PostgreSQL
  ```
  postgres://user:password@host:port/dbname?sslmode=disable
  ```

**Вариант 2: Компоненты (если DATABASE_URL не задан)**
- `DB_HOST` - хост БД (default: `localhost`)
- `DB_PORT` - порт БД (default: `5432`)
- `DB_NAME` - имя БД (default: `inventory`)
- `DB_USER` - пользователь БД (default: `admin`)
- `DB_PASSWORD` - пароль БД (default: `admin`)
- `DB_SSLMODE` - режим SSL (default: `disable`)

**Connection Pool**
- `DATABASE_MAX_CONNECTIONS` - максимум соединений (default: `25`)
- `DATABASE_MIN_CONNECTIONS` - минимум соединений (default: `5`)
- `DATABASE_TIMEOUT` - таймаут подключения (default: `30s`)
- `DATABASE_MAX_CONN_LIFETIME` - максимальное время жизни соединения (default: `1h`)
- `DATABASE_MAX_CONN_IDLE_TIME` - максимальное время idle соединения (default: `30m`)
- `DATABASE_RUN_MIGRATIONS` - запускать миграции при старте (default: `true`)

### Application
- `APP_ENVIRONMENT` - окружение: `development`, `staging`, `production` (default: `development`)
- `APP_DEBUG` - режим отладки (default: `false`)
- `APP_LOG_LEVEL` - уровень логирования: `debug`, `info`, `warn`, `error` (default: `info`)
- `APP_LOG_FORMAT` - формат логов: `text`, `terminal`, `json` (default: `terminal`)

### Security
- `SECURITY_TRUSTED_PROXIES` - доверенные прокси (через запятую)
- `SECURITY_ENABLE_CORS` - включить CORS (default: `false`)
- `SECURITY_ALLOWED_ORIGINS` - разрешенные origins для CORS (через запятую)

## Примеры использования

### Development (локальная разработка)

```bash
cp .env.example .env
# Отредактируйте .env при необходимости
./app -debug
```

### Production (Docker)

```yaml
# docker-compose.yml
services:
  app:
    image: myapp:latest
    environment:
      APP_ENVIRONMENT: production
      APP_LOG_FORMAT: json
      DATABASE_URL: ${DATABASE_URL}
      SERVER_PORT: 8080
```

### Production (Kubernetes)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  APP_ENVIRONMENT: production
  SERVER_PORT: "8080"
  DB_HOST: postgres-service
  DB_PORT: "5432"
  DB_NAME: inventory
---
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
type: Opaque
stringData:
  DB_USER: admin
  DB_PASSWORD: super_secret_password
```

## Получение строки подключения к БД в коде

```go
package main

import (
    "log"
    "yourproject/pkg/config"
)

func main() {
    // Load() автоматически загружает config.toml (можно изменить через флаг -config)
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    
    // Получаем строку подключения
    // Если задан DATABASE_URL - вернет его
    // Иначе соберет из компонентов (DB_HOST, DB_PORT, etc.)
    dsn := cfg.DB.GetDSN()
    
    log.Println("Connecting to:", dsn)
}
```

## Best Practices

1. **Никогда не коммитьте секреты** - добавьте `.env` и `config.toml` в `.gitignore`
2. **Используйте .env для локальной разработки** - удобно и быстро
3. **Используйте переменные окружения для production** - безопаснее и проще управлять
4. **Используйте config.toml для сложных конфигураций** - когда много параметров
5. **Храните секреты отдельно** - используйте системы управления секретами (Vault, AWS Secrets Manager, K8s Secrets)
