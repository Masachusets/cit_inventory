package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"warn", "warn", slog.LevelWarn},
		{"warning - синоним", "warning", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"info", "info", slog.LevelInfo},
		{"пустая строка - дефолт info", "", slog.LevelInfo},
		{"неизвестный уровень - дефолт info", "trace", slog.LevelInfo},
		{"регистр не важен", "WARN", slog.LevelWarn},
		{"регистр не важен (debug)", "DEBUG", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLogLevel(tt.level); got != tt.want {
				t.Errorf("getLogLevel(%q) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}

func TestSetupLogger_LevelFromLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		logLevel string
		enabled  []slog.Level
		disabled []slog.Level
	}{
		{
			name:     "warn пропускает warn и error",
			debug:    false,
			logLevel: "warn",
			enabled:  []slog.Level{slog.LevelWarn, slog.LevelError},
			disabled: []slog.Level{slog.LevelInfo, slog.LevelDebug},
		},
		{
			name:     "error пропускает только error",
			debug:    false,
			logLevel: "error",
			enabled:  []slog.Level{slog.LevelError},
			disabled: []slog.Level{slog.LevelWarn, slog.LevelInfo, slog.LevelDebug},
		},
		{
			name:     "debug пропускает всё",
			debug:    false,
			logLevel: "debug",
			enabled:  []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError},
			disabled: nil,
		},
		{
			name:     "пустой уровень - info по умолчанию",
			debug:    false,
			logLevel: "",
			enabled:  []slog.Level{slog.LevelInfo, slog.LevelWarn, slog.LevelError},
			disabled: []slog.Level{slog.LevelDebug},
		},
		{
			name:     "debug=true принудительно включает debug",
			debug:    true,
			logLevel: "error",
			enabled:  []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError},
			disabled: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := SetupLogger(tt.debug, tt.logLevel, "text")
			if log == nil {
				t.Fatal("SetupLogger() = nil")
			}

			h := log.Handler()
			ctx := context.Background()

			for _, level := range tt.enabled {
				if !h.Enabled(ctx, level) {
					t.Errorf("уровень %v должен логироваться", level)
				}
			}
			for _, level := range tt.disabled {
				if h.Enabled(ctx, level) {
					t.Errorf("уровень %v не должен логироваться", level)
				}
			}
		})
	}
}

func TestSetupLogger_Format(t *testing.T) {
	tests := []struct {
		name       string
		logFormat  string
		wantJSON   bool
		wantString string
	}{
		{"json даёт JSONHandler", "json", true, "*slog.JSONHandler"},
		{"JSON в верхнем регистре", "JSON", true, "*slog.JSONHandler"},
		{"text даёт TextHandler", "text", false, "*slog.TextHandler"},
		{"terminal даёт TextHandler", "terminal", false, "*slog.TextHandler"},
		{"неизвестный формат даёт TextHandler", "unknown", false, "*slog.TextHandler"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := SetupLogger(false, "info", tt.logFormat)

			switch tt.wantJSON {
			case true:
				if _, ok := log.Handler().(*slog.JSONHandler); !ok {
					t.Fatalf("handler = %T, want *slog.JSONHandler", log.Handler())
				}
			default:
				if _, ok := log.Handler().(*slog.TextHandler); !ok {
					t.Fatalf("handler = %T, want *slog.TextHandler", log.Handler())
				}
			}
		})
	}
}

func TestSetupLogger_SetsDefaultLogger(t *testing.T) {
	log := SetupLogger(false, "info", "text")

	if slog.Default() != log {
		t.Error("SetupLogger() должен устанавливать логгер через slog.SetDefault")
	}
}
