package main

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"log/slog"
	"os"

	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/raphoester/movie-reservation-system/internal/shared/xlog"
	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
)

const MigrationsDirEnvKey = "MIGRATIONS_DIR"

func main() {
	if err := run(context.TODO()); err != nil {
		xlog.QuickNew().Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg := &Config{}
	if err := xconfigs.LoadConfig(cfg); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := xlog.New(cfg.Logger)

	migrationsDir := os.Getenv(MigrationsDirEnvKey)
	if migrationsDir == "" {
		return fmt.Errorf("environment variable %q not set", MigrationsDirEnvKey)
	}

	if migrationsDir == "__all__" {
		for key, pgConfig := range cfg.Postgres {
			exists, err := migrationDirExists(key)
			if err != nil {
				return fmt.Errorf("check migration dir %q: %w", key, err)
			}
			if !exists {
				logger.Info("skipping: no migration directory found for postgres config", "key", key)
				continue
			}
			pgConfig := pgConfig
			if err := migrateOne(ctx, logger, &pgConfig, key); err != nil {
				return err
			}
		}
		return nil
	}

	pgConfig, err := cfg.getPostgresForMigrationDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("get Postgres config for migration dir: %w", err)
	}
	return migrateOne(ctx, logger, pgConfig, migrationsDir)
}

func migrationDirExists(dir string) (bool, error) {
	migrationsFS := assets.Migrations()
	entries, err := iofs.ReadDir(migrationsFS, dir)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read migrations dir from embedded assets: %w", err)
	}
	return len(entries) > 0, nil
}

func migrateOne(ctx context.Context, logger *slog.Logger, pgConfig *xpg.LazyLoadedConfig, migrationsDir string) error {
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

	logger.Info("running migrations", "db", pg.DBName(), "migrationsDir", migrationsDir)

	if err := pg.Migrate(migrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	logger.Info("migrations completed successfully", "migrationsDir", migrationsDir)

	return nil
}
