package xtestc_test

import (
	"context"
	"testing"
	"time"

	"github.com/raphoester/movie-reservation-system/internal/shared/xtestc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	r := xtestc.NewRedis()

	t.Cleanup(func() {
		if err := r.TearDownSuiteBg(); err != nil {
			t.Logf("failed tearing redis down: %v", err)
		}
	})

	err := r.Bootstrap(ctx)
	require.NoError(t, err)

	client := r.Client()

	err = client.Set(ctx, "key", "value", 0).Err()
	require.NoError(t, err)

	val, err := client.Get(ctx, "key").Result()
	require.NoError(t, err)
	assert.Equal(t, "value", val)

	r.SetupTest(t)

	exists, err := client.Exists(ctx, "key").Result()
	require.NoError(t, err)
	assert.EqualValues(t, 0, exists)
}

func TestRedis_Config(t *testing.T) {
	r := xtestc.BootstrapRedis(t)

	cfg := r.Config()
	assert.Equal(t, "tcp", cfg.Protocol)
	assert.NotEmpty(t, cfg.Host)
	assert.NotZero(t, cfg.Port)

	r.TearDownSuite(t)
}
