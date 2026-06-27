package main

import (
	"fmt"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
)

type Config struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	Logger                     xlog.Config `validate:"required"`

	RootPostgres map[string]xpg.LazyLoadedConfig `validate:"required"`
}

func (c *Config) GetLoggerConfig() xlog.Config {
	return c.Logger
}

func (c *Config) Validate() error {
	if len(c.RootPostgres) == 0 {
		return fmt.Errorf("at least one RootPostgres configuration is required")
	}

	return nil
}

func (c *Config) getPostgresForDBName(dbName string) (*xpg.LazyLoadedConfig, error) {
	cfg, ok := c.RootPostgres[dbName]
	if !ok {
		return nil, fmt.Errorf("postgres config not found: %s", dbName)
	}

	return &cfg, nil
}
