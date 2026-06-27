package xconfigs_test

import (
	"context"
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvSecretManager(t *testing.T) {
	ctx := t.Context()

	t.Run("AccessSecretIfEligible", func(t *testing.T) {
		resolveEnvSecret := func(t *testing.T, identifier string) (string, error) {
			t.Helper()
			return xconfigs.NewEnvSecretManager().AccessSecretIfEligible(ctx, identifier)
		}

		t.Run("should resolve a set environment variable", func(t *testing.T) {
			t.Setenv("MY_SECRET_KEY", "super-secret-value")

			val, err := resolveEnvSecret(t, "env://MY_SECRET_KEY")
			require.NoError(t, err)
			assert.Equal(t, "super-secret-value", val)
		})

		t.Run("should return ErrNotEligible for non-env:// identifier", func(t *testing.T) {
			_, err := resolveEnvSecret(t, "awssm://some-secret:key")
			require.Error(t, err)
			assert.ErrorIs(t, err, xconfigs.ErrNotEligible)
		})

		t.Run("should return ErrNotEligible for plain string", func(t *testing.T) {
			_, err := resolveEnvSecret(t, "just-a-plain-value")
			require.Error(t, err)
			assert.ErrorIs(t, err, xconfigs.ErrNotEligible)
		})

		t.Run("should return ErrEnvVarNotFound when variable is not set", func(t *testing.T) {
			val, err := resolveEnvSecret(t, "env://SURELY_NOT_SET_XYZ_12345")
			require.Error(t, err)
			assert.ErrorIs(t, err, xconfigs.ErrEnvVarNotFound)
			assert.Empty(t, val)
		})
	})

	t.Run("AccessSecretCallbackIfEligible", func(t *testing.T) {
		createCallback := func(t *testing.T, identifier string) (func(context.Context) (string, error), error) {
			t.Helper()
			return xconfigs.NewEnvSecretManager().AccessSecretCallbackIfEligible(identifier)
		}

		t.Run("should return ErrNotEligible for non-env:// identifier", func(t *testing.T) {
			_, err := createCallback(t, "awssm://some-secret:key")
			require.Error(t, err)
			assert.ErrorIs(t, err, xconfigs.ErrNotEligible)
		})

		t.Run("should return a callback that resolves the variable lazily", func(t *testing.T) {
			t.Setenv("LAZY_ENV_VAR", "initial-value")

			cb, err := createCallback(t, "env://LAZY_ENV_VAR")
			require.NoError(t, err)
			require.NotNil(t, cb)

			val, err := cb(ctx)
			require.NoError(t, err)
			assert.Equal(t, "initial-value", val)

			t.Setenv("LAZY_ENV_VAR", "updated-value")

			val, err = cb(ctx)
			require.NoError(t, err)
			assert.Equal(t, "updated-value", val)
		})

		t.Run("callback should return ErrEnvVarNotFound when variable is not set", func(t *testing.T) {
			cb, err := createCallback(t, "env://SURELY_NOT_SET_ABC_99999")
			require.NoError(t, err)
			require.NotNil(t, cb)

			val, err := cb(ctx)
			require.Error(t, err)
			assert.ErrorIs(t, err, xconfigs.ErrEnvVarNotFound)
			assert.Empty(t, val)
		})
	})
}
