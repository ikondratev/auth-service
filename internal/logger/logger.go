package logger

import (
	"log/slog"
	"os"
)

func New(e string) *slog.Logger {
	level := slog.LevelInfo

	if e == "dev" {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler)
}
