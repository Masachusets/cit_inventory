# Changelog: Улучшенная система конфигурации

## Дата: 2026-10-07

### ✅ Что изменилось

#### 1. Обновлен `/internal/config/config.go`

**API изменения:**
- ❌ **УДАЛЕНО**: `func New() *Config`
- ✅ **ДОБАВЛЕНО**: `func Load() (*Config, error)`
  - Теперь без аргументов
  - Путь к config.toml задается через флаг `-config` (default: `config.toml`)
  - Возвращает ошибку при проблемах с загрузкой

**Структура database:**
- ❌ **УДАЛЕНО**: `URL string`
- ✅ **ДОБАВЛЕНО**: 
  - `DSN string` - полная строка подключения (приоритет)
  - `Host string` - хост БД
  - `Port string` - порт БД
  - `Database string` - имя БД
  - `User string` - пользователь
  - `Password string` - пароль
  - `SSLMode string` - режим SSL
- ✅ **ДОБАВЛЕНО**: `func (d *database) GetDSN() string` - получение строки подключения

**Теги структур:**
- Все поля теперь имеют теги `env`, `toml` и `env-default`
- Поддержка переменных окружения для всех полей

#### 2. Порядок загрузки конфигурации

```
1. Дефолтные значения (env-default теги)
2. Файл config.toml (по умолчанию или через -config)
3. Переменные окружения
4. Флаги командной строки (наивысший приоритет)
```

#### 3. Новые файлы

- ✅ `config.toml.example` - пример TOML конфигурации
- ✅ `.env.example` - обновлен с полным набором переменных
- ✅ `CONFIG.md` - подробная документация
- ✅ `QUICKSTART_CONFIG.md` - быстрый старт
- ✅ `internal/config/example_test.go` - примеры использования

#### 4. Зависимости

- ✅ Добавлена: `github.com/ilyakaznacheev/cleanenv v1.5.0`

#### 5. Безопасность

- ✅ `.gitignore` обновлен: добавлены `.env` и `config.toml`

---

## 🔄 Миграция кода

### Было:
```go
cfg := config.New()
dsn := cfg.DB.URL
```

### Стало:
```go
cfg, err := config.Load()
if err != nil {
    log.Fatal(err)
}
dsn := cfg.DB.GetDSN()
```

---

## 🚀 Примеры использования

### Базовое использование
```go
import "github.com/Masachusets/cit_inventory/internal/config"

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    
    // Использование
    dsn := cfg.DB.GetDSN()
    port := cfg.Server.Port
}
```

### Запуск с разными конфигурациями

```bash
# Использовать config.toml из текущей директории
./app

# Указать другой путь к конфигу
./app -config=/etc/myapp/config.toml

# Переопределить через переменные окружения
export DB_HOST=prod.db.example.com
export DB_PASSWORD=secret
./app

# Переопределить через флаги
./app -db-host=localhost -port=9000 -debug
```

### Настройка базы данных

**Вариант 1: Полный DSN (рекомендуется для production)**
```bash
export DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=require"
```

**Вариант 2: Компоненты (удобно для development)**
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=inventory
export DB_USER=admin
export DB_PASSWORD=secret
```

---

## 📋 Доступные флаги

```
-config string
    Path to config file (default "config.toml")

Database:
  -db-dsn string
      PostgreSQL DSN (приоритет над компонентами)
  -db-host string
      Database host
  -db-port string
      Database port
  -db-name string
      Database name
  -db-user string
      Database user
  -db-password string
      Database password

Server:
  -port string
      HTTP server port
  -host string
      HTTP server host

Application:
  -env string
      Environment: development, staging, production
  -debug
      Enable debug mode
  -log-level string
      Log level: debug, info, warn, error
  -log-format string
      Log format: text, terminal, json
```

---

## 🎯 Следующие шаги

1. Обновить код, использующий `config.New()` → `config.Load()`
2. Заменить `cfg.DB.URL` → `cfg.DB.GetDSN()`
3. Скопировать `.env.example` в `.env` и настроить
4. (Опционально) Создать `config.toml` из `config.toml.example`

---

## 📚 Документация

- [CONFIG.md](./CONFIG.md) - Полная документация
- [QUICKSTART_CONFIG.md](./QUICKSTART_CONFIG.md) - Быстрый старт
- [config.toml.example](./config.toml.example) - Пример TOML конфигурации
- [.env.example](./.env.example) - Пример переменных окружения
