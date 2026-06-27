package xconfigs_test

import (
	"fmt"
	"testing"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLazyString(t *testing.T) {
	loadConfig := func(t *testing.T, yaml string, sms ...xconfigs.SecretManager) (testLazyConfig, error) {
		t.Helper()
		dir := writeConfigFile(t, yaml)
		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "local")
		cfg := testLazyConfig{}
		err := xconfigs.LoadConfig(&cfg, xconfigs.WithSecretManagers(sms...), xconfigs.WithSharedConfigFS(minimalSharedFS))
		if err != nil {
			return testLazyConfig{}, fmt.Errorf("failed to load config: %w", err)
		}
		return cfg, nil
	}

	t.Run("should populate LazyString from a secret manager", func(t *testing.T) {
		secretName := "name"
		secretValue := "value"
		sm := xconfigs.NewInMemorySecretManager(map[string]string{secretName: secretValue})

		cfg, err := loadConfig(t, "lazy: inmem://"+secretName, sm)
		require.NoError(t, err)

		got, err := cfg.Lazy.Value(t.Context())
		require.NoError(t, err)
		assert.Equal(t, secretValue, got)
	})

	t.Run("should populate LazyString from a plain string when no SM matches", func(t *testing.T) {
		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg, err := loadConfig(t, "lazy: plain-value", sm)
		require.NoError(t, err)

		got, err := cfg.Lazy.Value(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "plain-value", got)
	})

	t.Run("should not interfere with adjacent plain string fields", func(t *testing.T) {
		secretName := "my-secret"
		secretValue := "resolved"
		sm := xconfigs.NewInMemorySecretManager(map[string]string{secretName: secretValue})

		cfg, err := loadConfig(
			t,
			fmt.Sprintf(`lazy: inmem://%s
plain: just-a-string`, secretName),
			sm,
		)
		require.NoError(t, err)

		got, err := cfg.Lazy.Value(t.Context())
		require.NoError(t, err)
		assert.Equal(t, secretValue, got)
		assert.Equal(t, "just-a-string", cfg.Plain)
	})

	t.Run("should use the first eligible secret manager", func(t *testing.T) {
		secretName := "shared"
		first := xconfigs.NewInMemorySecretManager(map[string]string{secretName: "from-first"})
		second := xconfigs.NewInMemorySecretManager(map[string]string{secretName: "from-second"})

		cfg, err := loadConfig(t, "lazy: inmem://"+secretName, first, second)
		require.NoError(t, err)

		got, err := cfg.Lazy.Value(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "from-first", got)
	})

	t.Run("should populate LazyString from an unquoted integer in YAML", func(t *testing.T) {
		cfg, err := loadConfig(t, "lazy: 5432")
		require.NoError(t, err)

		got, err := cfg.Lazy.Value(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "5432", got)
	})

	t.Run("should resolve the value at call time, not at load time", func(t *testing.T) {
		secretName := "lazy-secret"
		sm := xconfigs.NewInMemorySecretManager(map[string]string{secretName: "initial-value"})

		cfg, err := loadConfig(t, "lazy: inmem://"+secretName, sm)
		require.NoError(t, err)

		sm.AddSecret(secretName, "updated-value")

		got, err := cfg.Lazy.Value(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "updated-value", got)
	})
}

var _ xconfigs.AppConfig = testLazyConfig{}

type testLazyConfig struct {
	Lazy  xconfigs.LazyString
	Plain string
}

func (t testLazyConfig) SetEnvironment(string) {}

func (t testLazyConfig) IsLocal() bool {
	return true
}

func (t testLazyConfig) Environment() string {
	return "local"
}

func (t testLazyConfig) GetName() string {
	panic("implement me")
}

func (t testLazyConfig) GetVersion() string {
	panic("implement me")
}
