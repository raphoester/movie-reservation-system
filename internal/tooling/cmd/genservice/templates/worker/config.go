// {{.Header}}

package main

import (
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xhttpsrv"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

type Config struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	Server                     xhttpsrv.Config `validate:"required"`
	Logger                     xlog.Config     `validate:"required"`
}

func (c *Config) GetHTTPServerConfig() xhttpsrv.Config {
	return c.Server
}

func (c *Config) GetLoggerConfig() xlog.Config {
	return c.Logger
}
