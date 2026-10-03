package logger

import (
	"log/slog"
	"os"
)

func NewInfoLogger() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	infoLogger := slog.New(jsonHandler)
	return infoLogger
}

func NewWarnogger() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	warnLogger := slog.New(jsonHandler)
	return warnLogger
}

func NewErrorLogger() *slog.Logger {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	errorLogger := slog.New(jsonHandler)
	return errorLogger
}
