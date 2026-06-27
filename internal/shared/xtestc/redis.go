package xtestc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/raphoester/movie-reservation-system/internal/shared/xmessaging"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const redisImage = "redis:8-alpine"

func BootstrapRedis(t *testing.T) *Redis {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	r := NewRedis()

	err := r.Bootstrap(ctx)
	require.NoError(t, err, "failed to bootstrap redis test container")

	return r
}

func NewRedis() *Redis {
	return &Redis{
		config:    xmessaging.RedisConfig{},
		client:    nil,
		container: nil,
	}
}

type Redis struct {
	config    xmessaging.RedisConfig
	client    *redis.Client
	container testcontainers.Container
}

func (r *Redis) Client() redis.UniversalClient {
	return r.client
}

// Config returns the connection configuration for the running container.
func (r *Redis) Config() xmessaging.RedisConfig {
	return r.config
}

func (r *Redis) SetupTest(t *testing.T) {
	t.Helper()
	err := r.client.FlushAll(t.Context()).Err()
	require.NoError(t, err)
}

func (r *Redis) TearDownSuite(t *testing.T) {
	t.Helper()
	if r.client != nil {
		_ = r.client.Close()
	}
	go func() {
		if r.container != nil {
			if err := r.container.Terminate(context.WithoutCancel(t.Context())); err != nil {
				fmt.Printf("warning: failed to terminate redis container: %v\n", err)
			}
		}
	}()
}

func (r *Redis) TearDownSuiteBg() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return r.teardown(ctx)
}

func (r *Redis) teardown(ctx context.Context) error {
	if r.client != nil {
		_ = r.client.Close()
	}

	if r.container != nil {
		if err := r.container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate container: %w", err)
		}
	}

	return nil
}

func (r *Redis) Bootstrap(ctx context.Context) error {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        redisImage,
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections tcp"),
		},
		Started: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start redis container: %w", err)
	}

	// Terminate the container on any subsequent error so callers never leak it.
	terminate := func(cause error) error {
		tctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(tctx)
		return cause
	}

	endpoint, err := container.PortEndpoint(ctx, "6379", "")
	if err != nil {
		return terminate(fmt.Errorf("failed to get container endpoint: %w", err))
	}

	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return terminate(fmt.Errorf("failed to split host and port: %w", err))
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return terminate(fmt.Errorf("failed to parse port: %w", err))
	}

	cfg := xmessaging.RedisConfig{
		Protocol: "tcp",
		Host:     host,
		Port:     port,
	}

	client := redis.NewClient(&redis.Options{
		Network: cfg.Protocol,
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
	})

	bo := backoff.WithContext(
		backoff.NewExponentialBackOff(func(off *backoff.ExponentialBackOff) {
			off.InitialInterval = 500 * time.Millisecond
			off.MaxElapsedTime = 2 * time.Minute
			if deadline, ok := ctx.Deadline(); ok {
				off.MaxElapsedTime = time.Until(deadline)
			}
		}),
		ctx,
	)
	if err := backoff.Retry(func() error {
		return client.Ping(ctx).Err()
	}, bo); err != nil {
		_ = client.Close()
		return terminate(fmt.Errorf("failed to connect redis client: %w", err))
	}

	r.config = cfg
	r.client = client
	r.container = container

	return nil
}
