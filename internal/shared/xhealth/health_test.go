package xhealth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCheck struct {
	name string
	err  error
}

func (m *mockCheck) Name() string {
	return m.name
}

func (m *mockCheck) Check(_ context.Context) error {
	return m.err
}

func TestHealthRegistry_Register(t *testing.T) {
	registry := NewHealthRegistry()

	check := &mockCheck{name: "test"}
	registry.Register(check)

	names := registry.List()
	assert.Contains(t, names, "test")
}

func TestHealthRegistry_Check(t *testing.T) {
	registry := NewHealthRegistry()

	t.Run("successful check", func(t *testing.T) {
		check := &mockCheck{name: "healthy", err: nil}
		registry.Register(check)

		err := registry.Check(t.Context(), "healthy")
		require.NoError(t, err)
	})

	t.Run("failing check", func(t *testing.T) {
		expectedErr := errors.New("check failed")
		check := &mockCheck{name: "unhealthy", err: expectedErr}
		registry.Register(check)

		err := registry.Check(t.Context(), "unhealthy")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("check not found", func(t *testing.T) {
		err := registry.Check(t.Context(), "nonexistent")
		assert.ErrorIs(t, err, ErrCheckNotFound)
	})
}

func TestHealthRegistry_CheckAll(t *testing.T) {
	registry := NewHealthRegistry()

	healthyCheck := &mockCheck{name: "healthy", err: nil}
	unhealthyCheck := &mockCheck{name: "unhealthy", err: errors.New("failed")}

	registry.Register(healthyCheck)
	registry.Register(unhealthyCheck)

	results := registry.CheckAll(t.Context())

	assert.Len(t, results, 2)
	require.NoError(t, results["healthy"])
	assert.Error(t, results["unhealthy"])
}

func TestNewFuncCheck(t *testing.T) {
	called := false
	check := NewFuncCheck("custom", func(_ context.Context) error {
		called = true
		return nil
	})

	assert.Equal(t, "custom", check.Name())

	err := check.Check(t.Context())
	require.NoError(t, err)
	assert.True(t, called)
}
