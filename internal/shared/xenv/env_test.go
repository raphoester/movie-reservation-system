package xenv_test

import (
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironment_DefaultsToLocal(t *testing.T) {
	t.Setenv(xenv.KeyEnvironment, "")
	assert.Equal(t, "local", xenv.Environment())
}

func TestEnvironment_ReturnsSetValue(t *testing.T) {
	t.Setenv(xenv.KeyEnvironment, "staging")
	assert.Equal(t, "staging", xenv.Environment())
}

func TestIsLocal_TrueWhenLocal(t *testing.T) {
	t.Setenv(xenv.KeyEnvironment, "local")
	assert.True(t, xenv.IsLocal())
}

func TestIsLocal_TrueWhenUnset(t *testing.T) {
	t.Setenv(xenv.KeyEnvironment, "")
	assert.True(t, xenv.IsLocal())
}

func TestIsLocal_FalseWhenNonLocal(t *testing.T) {
	t.Setenv(xenv.KeyEnvironment, "prod")
	assert.False(t, xenv.IsLocal())
}

func TestRenderedConfigPath_EmptyWhenUnset(t *testing.T) {
	t.Setenv(xenv.KeyRenderedConfig, "")
	assert.Empty(t, xenv.RenderedConfigPath())
}

func TestRenderedConfigPath_ReturnsSetValue(t *testing.T) {
	t.Setenv(xenv.KeyRenderedConfig, "/tmp/config.yaml")
	assert.Equal(t, "/tmp/config.yaml", xenv.RenderedConfigPath())
}

func TestIsValidateConfigOnly_FalseWhenUnset(t *testing.T) {
	t.Setenv(xenv.KeyValidateConfigOnly, "")
	assert.False(t, xenv.IsValidateConfigOnly())
}

func TestIsValidateConfigOnly_TrueWhenOne(t *testing.T) {
	t.Setenv(xenv.KeyValidateConfigOnly, "1")
	require.True(t, xenv.IsValidateConfigOnly())
}

func TestIsValidateConfigOnly_FalseForOtherValues(t *testing.T) {
	t.Setenv(xenv.KeyValidateConfigOnly, "true")
	assert.False(t, xenv.IsValidateConfigOnly())
}
