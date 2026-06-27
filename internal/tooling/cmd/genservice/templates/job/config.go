// {{.Header}}

package main

import (
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

type Config struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	Logger                     xlog.Config `validate:"required"`
}

func (c *Config) GetLoggerConfig() xlog.Config {
	return c.Logger
}
