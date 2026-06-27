package main

import (
	"fmt"
	"strings"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
)

type Config struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	Logger                     xlog.Config                     `validate:"required"`
	Postgres                   map[string]xpg.LazyLoadedConfig `validate:"required"`
}

func (c *Config) GetLoggerConfig() xlog.Config {
	return c.Logger
}

func (c *Config) Validate() error {
	if len(c.Postgres) == 0 {
		return fmt.Errorf("at least one Postgres configuration is required")
	}

	return nil
}

func (c *Config) getPostgresForMigrationDir(dir string) (*xpg.LazyLoadedConfig, error) {
	key := strings.ReplaceAll(dir, "/", "_")
	cfg, ok := c.Postgres[key]
	if !ok {
		return nil, fmt.Errorf("no postgres config found for migration dir %q (looked up key %q)", dir, key)
	}

	return &cfg, nil
}
