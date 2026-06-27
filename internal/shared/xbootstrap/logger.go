package xbootstrap

import (
	"log/slog"

	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

type config interface {
	IsLocal() bool
	Environment() string
	GetName() string
	GetVersion() string
	GetLoggerConfig() xlog.Config
}

func getLogger(cfg config) *slog.Logger {
	var baseLogger *slog.Logger
	if cfg.IsLocal() {
		baseLogger = xlog.NewForLocalConsole(cfg.GetLoggerConfig(), cfg.GetName())
	} else {
		baseLogger = xlog.New(cfg.GetLoggerConfig())
	}
	logger := baseLogger.
		With("environment", cfg.Environment()).
		With("app", cfg.GetName()).
		With("version", cfg.GetVersion())

	return logger
}
