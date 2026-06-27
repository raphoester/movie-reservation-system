package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
)

const DBNameEnvKey = "DB_NAME"

func main() {
	if err := run(context.TODO()); err != nil {
		xlog.QuickNew().Error("db creation failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := &Config{}
	if err := xconfigs.LoadConfig(cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := xlog.New(cfg.Logger)

	dbName := os.Getenv(DBNameEnvKey)
	if dbName == "" {
		return fmt.Errorf("environment variable %q not set", DBNameEnvKey)
	}

	if dbName == "__all__" {
		return createAllDBs(ctx, logger, cfg)
	}

	if err := createDB(ctx, logger, cfg, dbName); err != nil {
		return fmt.Errorf("failed to create database %q: %w", dbName, err)
	}

	return nil
}

func createAllDBs(ctx context.Context, logger *slog.Logger, cfg *Config) error {
	for name := range cfg.RootPostgres {
		if err := createDB(ctx, logger, cfg, name); err != nil {
			return fmt.Errorf("failed to create database %q: %w", name, err)
		}
	}
	return nil
}

func createDB(ctx context.Context, logger *slog.Logger, cfg *Config, dbName string) error {
	pgConfig, err := cfg.getPostgresForDBName(dbName)
	if err != nil {
		return fmt.Errorf("get Postgres config: %w", err)
	}

	pg, err := xpg.NewLazyLoaded(ctx, *pgConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize Postgres connection: %w", err)
	}

	defer func() {
		if err := pg.Close(); err != nil {
			logger.Error("failed to close Postgres connection", "error", err)
		}
	}()

	if err := pg.Connect(); err != nil {
		return fmt.Errorf("failed to connect to Postgres: %w", err)
	}

	logger.Info("creating database", "db", dbName)

	if err := pg.CreateDatabase(ctx, dbName); err != nil {
		if errors.Is(err, xpg.ErrDatabaseAlreadyExists) {
			logger.Info("database already exists, skipping", "db", dbName)
			return nil
		}
		return fmt.Errorf("failed to create database: %w", err)
	}

	logger.Info("database created successfully", "db", dbName)

	return nil
}
