package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// loadChildEnvKey - маркер среды: внутри дочернего процесса тест напрямую
// вызывает Load(), который можно выполнить лишь один раз за прогон
// (parseFlags регистрирует флаги в глобальном flag.CommandLine).
const loadChildEnvKey = "CIT_INVENTORY_TEST_LOAD_CHILD"

func TestDatabase_GetDSN(t *testing.T) {
	tests := []struct {
		name string
		db   database
		want string
	}{
		{
			name: "прямой DSN имеет приоритет",
			db: database{
				DSN:      "postgres://custom:pass@db:5432/custom?sslmode=require",
				Host:     "ignored",
				Port:     "1111",
				Database: "ignored",
				User:     "ignored",
				Password: "ignored",
				SSLMode:  "disable",
			},
			want: "postgres://custom:pass@db:5432/custom?sslmode=require",
		},
		{
			name: "пустой DSN собирается из компонентов",
			db: database{
				Host:     "dbhost",
				Port:     "6543",
				Database: "mydb",
				User:     "myuser",
				Password: "mypassword",
				SSLMode:  "require",
			},
			want: "postgres://myuser:mypassword@dbhost:6543/mydb?sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.db.GetDSN(); got != tt.want {
				t.Errorf("GetDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseConfigFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "форма -config=path",
			args: []string{"server", "-config=/tmp/custom.toml"},
			want: "/tmp/custom.toml",
		},
		{
			name: "форма -config path",
			args: []string{"server", "-config", "/tmp/custom.toml"},
			want: "/tmp/custom.toml",
		},
		{
			name: "форма --config=path",
			args: []string{"server", "--config=/tmp/custom.toml"},
			want: "/tmp/custom.toml",
		},
		{
			name: "форма --config path",
			args: []string{"server", "--config", "/tmp/custom.toml"},
			want: "/tmp/custom.toml",
		},
		{
			name: "значение после других аргументов",
			args: []string{"server", "-debug", "-config", "/tmp/other.toml"},
			want: "/tmp/other.toml",
		},
		{
			name: "-config без значения в конце аргументов",
			args: []string{"server", "-config"},
			want: "",
		},
		{
			name: "без флага используется config.toml",
			args: []string{"server", "-debug"},
			want: "config.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			t.Cleanup(func() { os.Args = origArgs })

			os.Args = tt.args

			if got := parseConfigFlag(); got != tt.want {
				t.Errorf("parseConfigFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

// runLoadInSubprocess запускает текущий тестовый бинарник в дочернем процессе
// и выполняет child внутри него. Нужен потому, что Load() регистрирует флаги
// в глобальном flag.CommandLine и потому вызывается не более одного раза за прогон.
func runLoadInSubprocess(t *testing.T, extraEnv []string, child func(t *testing.T)) {
	t.Helper()

	if os.Getenv(loadChildEnvKey) == "1" {
		child(t)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}

	cmd := exec.Command(exe, "-test.run=^"+t.Name()+"$", "-test.v")
	cmd.Env = append(cleanTestEnv(), loadChildEnvKey+"=1")
	cmd.Env = append(cmd.Env, extraEnv...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("дочерний процесс завершился с ошибкой: %v\nвывод:\n%s", err, out)
	}
}

// cleanTestEnv возвращает чистое окружение без переменных приложения,
// чтобы тесты Load() были детерминированы независимо от окружения запускающего shell.
func cleanTestEnv() []string {
	var env []string
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	return env
}

// TestLoad_EnvOverridesFileAndFlagsOverrideEnv проверяет каскад приоритетов:
// дефолты < TOML-файл < переменные окружения < флаги командной строки.
func TestLoad_EnvOverridesFileAndFlagsOverrideEnv(t *testing.T) {
	runLoadInSubprocess(t,
		[]string{
			"SERVER_PORT=2222",
			"DB_NAME=fromenv",
		},
		func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.toml")

			tomlBody := strings.Join([]string{
				"[server]",
				`port = "1111"`,
				`host = "filehost"`,
				"",
				"[app]",
				`log_level = "warn"`,
				"",
				"[db]",
				`host = "dbfile"`,
				`database = "invfile"`,
				"",
			}, "\n")
			if err := os.WriteFile(configPath, []byte(tomlBody), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			os.Args = []string{"config.test", "-config=" + configPath, "-host=flaghost"}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}

			// env перезаписывает TOML
			if cfg.Server.Port != "2222" {
				t.Errorf("Server.Port = %q, want %q (env should override file)", cfg.Server.Port, "2222")
			}
			// флаг перезаписывает TOML (env для host не задавалось)
			if cfg.Server.Host != "flaghost" {
				t.Errorf("Server.Host = %q, want %q (flag should override file)", cfg.Server.Host, "flaghost")
			}
			// значение остаётся из TOML, если не задано ни в env, ни в флаге
			if cfg.App.LogLevel != "warn" {
				t.Errorf("App.LogLevel = %q, want %q (value from file)", cfg.App.LogLevel, "warn")
			}
			if cfg.DB.Host != "dbfile" {
				t.Errorf("DB.Host = %q, want %q (value from file)", cfg.DB.Host, "dbfile")
			}
			if cfg.DB.Database != "fromenv" {
				t.Errorf("DB.Database = %q, want %q (env should override file)", cfg.DB.Database, "fromenv")
			}
			// дефолт из тега env-default
			if cfg.DB.User != "admin" {
				t.Errorf("DB.User = %q, want %q (default)", cfg.DB.User, "admin")
			}
			if cfg.Server.ReadTimeout != 10*time.Second {
				t.Errorf("Server.ReadTimeout = %v, want %v (default)", cfg.Server.ReadTimeout, 10*time.Second)
			}
			if cfg.App.Debug {
				t.Errorf("App.Debug = true, want false (default)")
			}

			wantDSN := "postgres://admin:admin@dbfile:5432/fromenv?sslmode=disable"
			if got := cfg.DB.GetDSN(); got != wantDSN {
				t.Errorf("GetDSN() = %q, want %q", got, wantDSN)
			}
		},
	)
}

// TestLoad_MissingConfigFile проверяет, что отсутствие конфиг-файла не является
// ошибкой и Load() возвращает значения по умолчанию.
func TestLoad_MissingConfigFile(t *testing.T) {
	runLoadInSubprocess(t, nil, func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "does-not-exist.toml")
		os.Args = []string{"config.test", "-config=" + missing}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() unexpected error for missing file: %v", err)
		}

		if cfg.Server.Port != "8080" {
			t.Errorf("Server.Port = %q, want %q (default)", cfg.Server.Port, "8080")
		}
		if cfg.Server.Host != "localhost" {
			t.Errorf("Server.Host = %q, want %q (default)", cfg.Server.Host, "localhost")
		}
		if cfg.App.Environment != "development" {
			t.Errorf("App.Environment = %q, want %q (default)", cfg.App.Environment, "development")
		}
		if cfg.DB.MaxConnections != 25 {
			t.Errorf("DB.MaxConnections = %d, want %d (default)", cfg.DB.MaxConnections, 25)
		}
		wantDSN := "postgres://admin:admin@localhost:5432/inventory?sslmode=disable"
		if got := cfg.DB.GetDSN(); got != wantDSN {
			t.Errorf("GetDSN() = %q, want %q", got, wantDSN)
		}
	})
}
