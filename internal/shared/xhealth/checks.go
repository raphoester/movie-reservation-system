package xhealth

import (
	"context"
	"fmt"
	"time"

	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
	"github.com/redis/go-redis/v9"
)

const (
	DefaultCheckInterval = 10 * time.Second
)

type PostgresCheck struct {
	name string
	db   *xpg.Postgres
}

func NewPostgresCheck(name string, db *xpg.Postgres) *PostgresCheck {
	return &PostgresCheck{
		name: name,
		db:   db,
	}
}

func NewSinglePostgresCheck(db *xpg.Postgres) *PostgresCheck {
	return NewPostgresCheck("postgres", db)
}

func (c *PostgresCheck) Name() string {
	return c.name
}

func (c *PostgresCheck) Check(ctx context.Context) error {
	if err := c.db.SQLClient().PingContext(ctx); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}
	return nil
}

type RedisCheck struct {
	name   string
	client redis.UniversalClient
}

func NewRedisCheck(name string, client redis.UniversalClient) *RedisCheck {
	return &RedisCheck{
		name:   name,
		client: client,
	}
}

func (c *RedisCheck) Name() string {
	return c.name
}

func (c *RedisCheck) Check(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}
