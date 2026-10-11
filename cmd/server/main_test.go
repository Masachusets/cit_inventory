package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// runChildEnvKey - маркер дочернего процесса: run() вызывает config.Load(),
// который можно выполнить лишь один раз за прогон тестов.
const runChildEnvKey = "CIT_INVENTORY_TEST_RUN_CHILD"

func cleanEnv(extra ...string) []string {
	var env []string
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "LANG"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	return append(env, extra...)
}

// TestRun_DatabaseUnavailable запускает run() в дочернем процессе с DSN,
// ведущим на закрытый порт: приложение должно завершиться ошибкой подключения.
func TestRun_DatabaseUnavailable(t *testing.T) {
	if os.Getenv(runChildEnvKey) == "1" {
		// Дочерний процесс: выполняем run() напрямую.
		os.Args = []string{"server.test", "-config="}

		err := run()
		if err == nil {
			t.Fatal("run() error = nil, want database connection error")
		}
		if !strings.Contains(err.Error(), "error db connect") {
			t.Errorf("err = %q, want it to contain %q", err.Error(), "error db connect")
		}
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable(): %v", err)
	}

	cmd := exec.Command(exe, "-test.run=^TestRun_DatabaseUnavailable$", "-test.v")
	cmd.Env = cleanEnv(
		runChildEnvKey+"=1",
		"DATABASE_URL=postgres://user:pass@127.0.0.1:1/db?sslmode=disable",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("дочерний процесс завершился с ошибкой: %v\nвывод:\n%s", err, out)
	}
}

// TestMain_Binary запускает cmd/server как настоящий бинарник: при недоступной
// БД main() должен вывести сообщение "fatal" и завершиться с кодом 1.
func TestMain_Binary(t *testing.T) {
	exe, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain не найден: %v", err)
	}

	cmd := exec.Command(exe, "run", "./cmd/server")
	cmd.Dir = "../.."
	cmd.Env = cleanEnv(
		"DATABASE_URL=postgres://user:pass@127.0.0.1:1/db?sslmode=disable",
	)

	out, err := cmd.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("go run ./cmd/server: err = %v, want exit status 1\nвывод:\n%s", err, out)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1\nвывод:\n%s", exitErr.ExitCode(), out)
	}
	if !strings.Contains(string(out), "fatal:") {
		t.Errorf("вывод не содержит %q:\n%s", "fatal:", out)
	}
}
