package xtestc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/raphoester/movie-reservation-system/internal/shared/xpg"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const postgresImage = "postgres:18.3-alpine3.23"

func BootstrapPostgres(t *testing.T, migrationsDir string) *Postgres {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	postgres := NewPostgres(migrationsDir)

	err := postgres.Bootstrap(ctx)
	require.NoError(t, err, "failed to bootstrap postgres test container")

	return postgres
}

func NewPostgres(migrationsDir string) *Postgres {
	return &Postgres{
		migrationsDir: migrationsDir,
		config:        xpg.Config{},
		client:        nil,
		container:     nil,
	}
}

type Postgres struct {
	migrationsDir string

	config    xpg.Config
	client    *xpg.Postgres
	container testcontainers.Container
}

func (p *Postgres) Client() *xpg.Postgres {
	return p.client
}

// NewClientWithPool creates and connects a new xpg.Postgres against the running container
// with the given pool configuration applied.
func (p *Postgres) NewClientWithPool(pool xpg.PoolConfig) (*xpg.Postgres, error) {
	cfg := p.config
	cfg.Pool = pool
	client := xpg.New(cfg)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect with pool config: %w", err)
	}
	return client, nil
}

func (p *Postgres) SetupTest(t *testing.T) {
	t.Helper()
	err := p.client.Purge(t.Context())
	require.NoError(t, err)
}

// SetupTestExcept truncates all user tables except the listed ones.
// Use for tables holding static reference data seeded by migrations (e.g. lookup tables)
// that must survive between tests.
func (p *Postgres) SetupTestExcept(t *testing.T, excludedTables ...string) {
	t.Helper()
	err := p.client.PurgeExcept(t.Context(), excludedTables...)
	require.NoError(t, err)
}

func (p *Postgres) TearDownSuite(t *testing.T) {
	t.Helper()
	_ = p.client.Close()
	go func() {
		if err := p.container.Terminate(context.WithoutCancel(t.Context())); err != nil {
			fmt.Printf("warning: failed to terminate postgres container: %v\n", err)
		}
	}()
}

func (p *Postgres) TearDownSuiteBg() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = p.client.Close()

	if err := p.container.Terminate(ctx); err != nil {
		fmt.Printf("failed to terminate container: %v\n", err)
	}
}

// Config returns the connection configuration for the running container.
// Useful when callers need host/port to load data via external tools (e.g. psql).
func (p *Postgres) Config() xpg.Config {
	return p.config
}

func (p *Postgres) Bootstrap(ctx context.Context) error {
	if err := p.bootstrapContainer(ctx); err != nil {
		return err
	}
	if err := p.client.Migrate(p.migrationsDir); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// bootstrapContainer starts the container and establishes the connection without running
// migrations. Called by both Bootstrap and BootstrapPostgresRaw.
func (p *Postgres) bootstrapContainer(ctx context.Context) error {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        postgresImage,
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_PASSWORD": "postgres",
			},
			WaitingFor: wait.ForLog("PostgreSQL init process complete; ready for start up."),
		},
		Started: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	endpoint, err := container.PortEndpoint(ctx, "5432", "")
	if err != nil {
		return fmt.Errorf("failed to get container endpoint: %w", err)
	}

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("failed to split host and port: %w", err)
	}

	cfg := xpg.Config{
		Host:     host,
		Port:     port,
		User:     "postgres",
		Password: "postgres",
		DBName:   "postgres",
		SSLMode:  "disable",
	}

	client := xpg.New(cfg)

	if err := backoff.Retry(func() error {
		return client.Connect()
	}, backoff.NewExponentialBackOff(func(off *backoff.ExponentialBackOff) {
		off.InitialInterval = 500 * time.Millisecond
		off.MaxElapsedTime = 2 * time.Minute
		deadline, ok := ctx.Deadline()
		if ok {
			off.MaxElapsedTime = time.Until(deadline)
		}
	})); err != nil {
		return fmt.Errorf("failed to connect postgres client: %w", err)
	}

	p.config = cfg
	p.client = client
	p.container = container

	return nil
}
