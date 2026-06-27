// {{.Header}}

package main

import (
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xgrpcsrv"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
)

type Config struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	Server                     xgrpcsrv.Config `validate:"required"`
	Logger                     xlog.Config     `validate:"required"`
}

func (c *Config) GetGRPCServerConfig() xgrpcsrv.Config {
	return c.Server
}

func (c *Config) GetLoggerConfig() xlog.Config {
	return c.Logger
}
