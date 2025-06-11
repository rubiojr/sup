package log

import (
	"io"
	"log/slog"
	"os"
)

var (
	defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

type Logger struct {
	*slog.Logger
}

type Format int

func SetDefault(logger *slog.Logger) {
	defaultLogger = logger
}

func Default() *slog.Logger {
	return defaultLogger
}

func SetLevel(level slog.Level) {
	defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

func Disable() {
	if defaultLogger != nil {
		defaultLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

func Enable() {
	if defaultLogger != nil {
		defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}
}

func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}
