package config

import (
	"log/slog"
	"os"
	"strings"
)

func InitLogger(logLevel string) {
	logLevel = strings.ToUpper(logLevel)
	var level slog.Leveler
	switch logLevel {
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)
}
