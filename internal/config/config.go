package config

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	
	"github.com/ilyakaznacheev/cleanenv"
)

type server struct{
	Port            string        `env:"SERVER_PORT" env-default:"8080" toml:"port"`
	Host            string        `env:"SERVER_HOST" env-default:"localhost" toml:"host"`
	ReadTimeout     time.Duration `env:"SERVER_READ_TIMEOUT" env-default:"10s" toml:"read_timeout"`
	WriteTimeout    time.Duration `env:"SERVER_WRITE_TIMEOUT" env-default:"10s" toml:"write_timeout"`
	IdleTimeout     time.Duration `env:"SERVER_IDLE_TIMEOUT" env-default:"60s" toml:"idle_timeout"`
	ShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" env-default:"30s" toml:"shutdown_timeout"`
}

type database struct{
	// DSN имеет приоритет, если задан
	DSN string `env:"DATABASE_URL" toml:"dsn"`
	
	// Компоненты для сборки DSN
	Host     string `env:"DB_HOST" env-default:"localhost" toml:"host"`
	Port     string `env:"DB_PORT" env-default:"5432" toml:"port"`
	Database string `env:"DB_NAME" env-default:"inventory" toml:"database"`
	User     string `env:"DB_USER" env-default:"admin" toml:"user"`
	Password string `env:"DB_PASSWORD" env-default:"admin" toml:"password"`
	SSLMode  string `env:"DB_SSLMODE" env-default:"disable" toml:"sslmode"`
	
	MaxConnections  int32         `env:"DATABASE_MAX_CONNECTIONS" env-default:"25" toml:"max_connections"`
	MinConnections  int32         `env:"DATABASE_MIN_CONNECTIONS" env-default:"5" toml:"min_connections"`
	Timeout         time.Duration `env:"DATABASE_TIMEOUT" env-default:"30s" toml:"timeout"`
	MaxConnLifetime time.Duration `env:"DATABASE_MAX_CONN_LIFETIME" env-default:"1h" toml:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `env:"DATABASE_MAX_CONN_IDLE_TIME" env-default:"30m" toml:"max_conn_idle_time"`
	RunMigrations   bool          `env:"DATABASE_RUN_MIGRATIONS" env-default:"true" toml:"run_migrations"`
}

type app struct{
	Environment string `env:"APP_ENVIRONMENT" env-default:"development" toml:"environment"`
	Debug       bool   `env:"APP_DEBUG" env-default:"false" toml:"debug"`
	LogLevel    string `env:"APP_LOG_LEVEL" env-default:"info" toml:"log_level"`
	LogFormat   string `env:"APP_LOG_FORMAT" env-default:"terminal" toml:"log_format"`
}

type security struct {
	TrustedProxies []string `env:"SECURITY_TRUSTED_PROXIES" env-separator:"," toml:"trusted_proxies"`
	EnableCORS     bool     `env:"SECURITY_ENABLE_CORS" env-default:"false" toml:"enable_cors"`
	AllowedOrigins []string `env:"SECURITY_ALLOWED_ORIGINS" env-separator:"," toml:"allowed_origins"`
}

type Config struct {
	Server   server
	DB       database
	App      app
	Security security
}

// GetDSN возвращает строку подключения к БД.
// Если DSN задан напрямую - возвращает его, иначе собирает из компонентов.
func (d *database) GetDSN() string {
	if d.DSN != "" {
		return d.DSN
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Database, d.SSLMode)
}

// Load загружает конфигурацию из множественных источников в порядке приоритета:
// 1. Дефолтные значения (из тегов env-default)
// 2. Файл config.toml (путь задается через флаг -config)
// 3. Переменные окружения
// 4. Флаги командной строки (наивысший приоритет)
func Load() (*Config, error) {
	cfg := &Config{}
	
	// 1. Первый проход: извлекаем только флаг -config из os.Args
	configPath := parseConfigFlag()
	
	// 2. Читаем TOML файл (дефолты уже в тегах)
	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, cfg); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
			}
		}
	}
	
	// 3. Читаем переменные окружения (перезаписывают значения из TOML)
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to read environment variables: %w", err)
	}
	
	// 4. Второй проход: парсим остальные флаги.
	// Дефолты флагов = текущие значения cfg (из TOML/ENV),
	// поэтому незаданные флаги ничего не меняют,
	// а заданные - перезаписывают (наивысший приоритет).
	parseFlags(cfg)
	
	return cfg, nil
}

// parseConfigFlag извлекает значение флага -config из os.Args.
// Поддерживает формы: -config=path, -config path, --config path
func parseConfigFlag() string {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-config" || arg == "--config" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if strings.HasPrefix(arg, "-config=") {
			return strings.TrimPrefix(arg, "-config=")
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	return "config.toml"
}

// parseFlags регистрирует флаги командной строки с дефолтами из cfg
// и применяет заданные значения поверх конфигурации.
func parseFlags(cfg *Config) {
	// Database - только DSN или основные компоненты
	flag.StringVar(&cfg.DB.DSN, "db-dsn", cfg.DB.DSN, "PostgreSQL DSN (приоритет над компонентами)")
	flag.StringVar(&cfg.DB.Host, "db-host", cfg.DB.Host, "Database host")
	flag.StringVar(&cfg.DB.Port, "db-port", cfg.DB.Port, "Database port")
	flag.StringVar(&cfg.DB.Database, "db-name", cfg.DB.Database, "Database name")
	flag.StringVar(&cfg.DB.User, "db-user", cfg.DB.User, "Database user")
	flag.StringVar(&cfg.DB.Password, "db-password", cfg.DB.Password, "Database password")
	
	// Server - порт и хост
	flag.StringVar(&cfg.Server.Port, "port", cfg.Server.Port, "HTTP server port")
	flag.StringVar(&cfg.Server.Host, "host", cfg.Server.Host, "HTTP server host")
	
	// App - логирование и режим
	flag.StringVar(&cfg.App.Environment, "env", cfg.App.Environment, "Environment: development, staging, production")
	flag.BoolVar(&cfg.App.Debug, "debug", cfg.App.Debug, "Enable debug mode")
	flag.StringVar(&cfg.App.LogLevel, "log-level", cfg.App.LogLevel, "Log level: debug, info, warn, error")
	flag.StringVar(&cfg.App.LogFormat, "log-format", cfg.App.LogFormat, "Log format: text, terminal, json")
	
	// Флаг -config уже обработан в первом проходе, регистрируем повторно,
	// чтобы flag.Parse() не выдавал ошибку о неизвестном флаге
	flag.String("config", "config.toml", "Path to config file")
	
	flag.Parse()
}
