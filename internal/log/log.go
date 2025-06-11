package log

import (
	"context"
	"io"
	"log/slog"
	"os"
)

var (
	defaultLogger = slog.New(NewCustomHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

type Logger struct {
	*slog.Logger
}

type Format int

type CustomHandler struct {
	slog.Handler
}

func NewCustomHandler(w io.Writer, opts *slog.HandlerOptions) *CustomHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{
				Key:   a.Key,
				Value: slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05")),
			}
		}
		return a
	}
	return &CustomHandler{
		Handler: slog.NewTextHandler(w, opts),
	}
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.Handler.Handle(ctx, r)
}

func SetDefault(logger *slog.Logger) {
	defaultLogger = logger
}

func Default() *slog.Logger {
	return defaultLogger
}

func SetLevel(level slog.Level) {
	defaultLogger = slog.New(NewCustomHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

func Disable() {
	if defaultLogger != nil {
		defaultLogger = slog.New(NewCustomHandler(io.Discard, nil))
	}
}

func Enable() {
	if defaultLogger != nil {
		defaultLogger = slog.New(NewCustomHandler(os.Stdout, &slog.HandlerOptions{
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
