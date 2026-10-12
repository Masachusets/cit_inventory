package logger

import (
	"log/slog"
	"os"
	"strings"
)

// SetupLogger configures slog based on provided flags.
// It respects debug, logLevel and logFormat.
func SetupLogger(debug bool, logLevel string, logFormat string) *slog.Logger {
	level := getLogLevel(logLevel)

	// If debug is enabled, cap level to debug (most verbose)
	if debug {
		level = slog.LevelDebug
	}

	handlerOpts := &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	}

	var handler slog.Handler
	if strings.ToLower(logFormat) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	}

	log := slog.New(handler)

	// Make it the default logger so slog.Info/... use our config
	slog.SetDefault(log)

	return log
}

func getLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
