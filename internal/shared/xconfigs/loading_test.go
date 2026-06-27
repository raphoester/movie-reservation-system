package xconfigs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/raphoester/movie-reservation-system/internal/shared/xconfigs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	DbPassword                 string `validate:"required" mapstructure:"dbPassword"`
	APIKey                     string `validate:"required" mapstructure:"apiKey"`
	PlainValue                 string `validate:"required" mapstructure:"plainValue"`
}

type testConfigWithValidateMethod struct {
	xconfigs.ApplicationConfig  `validate:"required" mapstructure:",squash"`
	shouldReturnValidationError bool
}

func (c *testConfigWithValidateMethod) Validate() error {
	if c.shouldReturnValidationError {
		return fmt.Errorf("custom validation error")
	}
	return nil
}

func newConfigWithValidateMethod(withError bool) *testConfigWithValidateMethod {
	return &testConfigWithValidateMethod{
		shouldReturnValidationError: withError,
	}
}

// minimalSharedFS satisfies the requirement that base.yaml and the env-specific
// yaml must both exist. Content is intentionally empty — tests that need real
// anchor values build their own fstest.MapFS inline.
var minimalSharedFS = fstest.MapFS{
	"base.yaml":    &fstest.MapFile{Data: []byte("")},
	"local.yaml":   &fstest.MapFile{Data: []byte("")},
	"dev.yaml":     &fstest.MapFile{Data: []byte("")},
	"staging.yaml": &fstest.MapFile{Data: []byte("")},
	"prod.yaml":    &fstest.MapFile{Data: []byte("")},
	"docker.yaml":  &fstest.MapFile{Data: []byte("")},
}

func TestLoadConfig(t *testing.T) {
	t.Run("should resolve secrets via in-memory secret manager", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "inmem://prod/db-password"
apiKey: "inmem://prod/api-key"
plainValue: no-secret-here
`)

		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "local")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{
			"prod/db-password": "s3cret-db-pass",
			"prod/api-key":     "sk-12345",
		})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.Equal(t, "s3cret-db-pass", cfg.DbPassword)
		assert.Equal(t, "sk-12345", cfg.APIKey)
		assert.Equal(t, "no-secret-here", cfg.PlainValue)
		assert.Equal(t, "test-service", cfg.GetName())
		assert.Equal(t, "1.0.0", cfg.GetVersion())
		assert.True(t, cfg.IsLocal())
	})

	t.Run("should throw an error when a matching secret manager fails to return a valid value", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "inmem://prod/db-password"
apiKey: key
plainValue: val
`)

		chdir(t, dir)

		sm := xconfigs.NewInMemorySecretManager(map[string]string{
			"prod/other-secret": "resolved-value",
		})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.Error(t, err)
		assert.ErrorIs(t, err, xconfigs.ErrSecretResolution)
	})

	t.Run("should default to local environment when ENVIRONMENT is unset", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: pw
apiKey: key
plainValue: val
`)

		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.True(t, cfg.IsLocal())
		assert.Equal(t, "local", cfg.Environment())
	})

	t.Run("should reject invalid ENVIRONMENT value", func(t *testing.T) {
		dir := writeConfigFile(t, `{}`)

		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "invalid-env")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.Error(t, err)
		assert.ErrorIs(t, err, xconfigs.ErrInvalidEnvironment)
	})

	t.Run("should reject path traversal in ENVIRONMENT value", func(t *testing.T) {
		dir := writeConfigFile(t, `{}`)
		chdir(t, dir)

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		for _, malicious := range []string{
			"../../etc/passwd",
			"../secret",
			"dev/../../etc/shadow",
		} {
			t.Run(malicious, func(t *testing.T) {
				t.Setenv("ENVIRONMENT", malicious)

				cfg := &testConfig{}
				err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
				require.Error(t, err)
				assert.ErrorIs(t, err, xconfigs.ErrInvalidEnvironment)
			})
		}
	})

	t.Run("should return validation error for missing required fields", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
`)

		chdir(t, dir)

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.Error(t, err)
		assert.ErrorIs(t, err, xconfigs.ErrConfigValidation)
	})

	t.Run("should return an error if the config has a Validate method that returns an error", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: pw
apiKey: key
plainValue: val
`)

		chdir(t, dir)

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := newConfigWithValidateMethod(true)
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.Error(t, err)
		assert.ErrorIs(t, err, xconfigs.ErrCustomValidation)
	})

	t.Run("should succeed if the config has a Validate method that returns no error", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: pw
apiKey: key
plainValue: val
`)

		chdir(t, dir)

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := newConfigWithValidateMethod(false)
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)
	})

	t.Run("should apply value from the first applicable secret manager", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "inmem://shared-secret"
apiKey: key
plainValue: val
`)

		chdir(t, dir)

		first := xconfigs.NewInMemorySecretManager(map[string]string{
			"shared-secret": "from-first",
		})
		second := xconfigs.NewInMemorySecretManager(map[string]string{
			"shared-secret": "from-second",
		})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(first, second), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.Equal(t, "from-first", cfg.DbPassword)
	})

	t.Run("should load config for each valid non-dev environment", func(t *testing.T) {
		for _, env := range []string{"staging", "prod"} {
			t.Run(env, func(t *testing.T) {
				dir := writeConfigFileForEnv(t, env, `
name: test-service
version: "2.0.0"
dbPassword: pw
apiKey: key
plainValue: val
`)

				chdir(t, dir)
				t.Setenv("ENVIRONMENT", env)

				sm := xconfigs.NewInMemorySecretManager(map[string]string{})

				cfg := &testConfig{}
				err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
				require.NoError(t, err)

				assert.False(t, cfg.IsLocal())
				assert.Equal(t, env, cfg.Environment())
				assert.Equal(t, "test-service", cfg.GetName())
				assert.Equal(t, "2.0.0", cfg.GetVersion())
			})
		}
	})

	t.Run("should resolve secrets in non-dev environment", func(t *testing.T) {
		dir := writeConfigFileForEnv(t, "prod", `
name: prod-service
version: "1.0.0"
dbPassword: "inmem://prod/db-password"
apiKey: "inmem://prod/api-key"
plainValue: plain
`)

		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "prod")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{
			"prod/db-password": "prod-db-pass",
			"prod/api-key":     "prod-api-key",
		})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.Equal(t, "prod-db-pass", cfg.DbPassword)
		assert.Equal(t, "prod-api-key", cfg.APIKey)
		assert.Equal(t, "plain", cfg.PlainValue)
		assert.False(t, cfg.IsLocal())
		assert.Equal(t, "prod", cfg.Environment())
	})

	t.Run("should load value from environment variable via AutomaticEnv", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: from-yaml
apiKey: from-yaml
plainValue: val
`)

		chdir(t, dir)
		t.Setenv("DBPASSWORD", "from-env")
		t.Setenv("APIKEY", "from-env-too")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.Equal(t, "from-env", cfg.DbPassword)
		assert.Equal(t, "from-env-too", cfg.APIKey)
		assert.Equal(t, "val", cfg.PlainValue, "fields without a matching env var should keep the YAML value")
	})

	t.Run("should prefer AutomaticEnv over config file value", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: from-yaml
apiKey: key
plainValue: val
`)

		chdir(t, dir)
		t.Setenv("DBPASSWORD", "from-env")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.Equal(t, "from-env", cfg.DbPassword)
	})

	t.Run("should resolve env:// secrets from environment variables", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "env://DB_PASSWORD"
apiKey: "env://API_KEY"
plainValue: val
`)

		chdir(t, dir)
		t.Setenv("DB_PASSWORD", "from-env")
		t.Setenv("API_KEY", "from-env-too")

		sm := xconfigs.NewEnvSecretManager()

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.NoError(t, err)

		assert.Equal(t, "from-env", cfg.DbPassword)
		assert.Equal(t, "from-env-too", cfg.APIKey)
		assert.Equal(t, "val", cfg.PlainValue)
	})

	t.Run("should return validation error for missing required fields in non-dev environment", func(t *testing.T) {
		dir := writeConfigFileForEnv(t, "staging", `
name: staging-service
version: "1.0.0"
`)

		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "staging")

		sm := xconfigs.NewInMemorySecretManager(map[string]string{})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(cfg, xconfigs.WithSecretManagers(sm), xconfigs.WithSharedConfigFS(minimalSharedFS))
		require.Error(t, err)
		assert.ErrorIs(t, err, xconfigs.ErrConfigValidation)
	})
}

// TestResolutionOrder documents the priority chain for config value resolution:
//
//  1. AutomaticEnv (viper matches the YAML key name uppercased against env vars) — highest priority.
//     Resolved by viper before decode hooks run, so the decode hooks never see the original YAML string.
//  2. env:// scheme (EnvSecretManager) — explicit, self-documenting env var reference in YAML.
//     Only reached when AutomaticEnv did not override the key.
//  3. Other secret managers (e.g. inmem://, awssm://) in the order they are registered.
//     First manager whose scheme matches the identifier wins.
//  4. Plain YAML value — lowest priority, used when nothing else matches.
func TestResolutionOrder(t *testing.T) {
	t.Run("AutomaticEnv takes priority over a plain YAML value", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: yaml-value
apiKey: key
plainValue: val
`)
		chdir(t, dir)
		t.Setenv("DBPASSWORD", "auto-env-value")

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewInMemorySecretManager(map[string]string{})),
			xconfigs.WithSharedConfigFS(minimalSharedFS),
		)
		require.NoError(t, err)
		assert.Equal(t, "auto-env-value", cfg.DbPassword, "AutomaticEnv should win over the YAML value")
	})

	t.Run("AutomaticEnv takes priority over an env:// reference", func(t *testing.T) {
		// When the YAML key is `dbPassword`, viper looks for DBPASSWORD via AutomaticEnv.
		// If DBPASSWORD is set, that value is used and the decode hook never sees the
		// env:// string — so EXPLICIT_SECRET is never consulted.
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "env://EXPLICIT_SECRET"
apiKey: key
plainValue: val
`)
		chdir(t, dir)
		t.Setenv("DBPASSWORD", "auto-env-value")
		t.Setenv("EXPLICIT_SECRET", "explicit-env-value")

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewEnvSecretManager()),
			xconfigs.WithSharedConfigFS(minimalSharedFS),
		)
		require.NoError(t, err)
		assert.Equal(t, "auto-env-value", cfg.DbPassword, "AutomaticEnv (DBPASSWORD) should win over env://EXPLICIT_SECRET")
	})

	t.Run("env:// reference is resolved when AutomaticEnv does not match", func(t *testing.T) {
		// DBPASSWORD is not set, so AutomaticEnv does not override.
		// The decode hook sees the env:// string and EnvSecretManager resolves EXPLICIT_SECRET.
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "env://EXPLICIT_SECRET"
apiKey: key
plainValue: val
`)
		chdir(t, dir)
		t.Setenv("EXPLICIT_SECRET", "explicit-env-value")

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewEnvSecretManager()),
			xconfigs.WithSharedConfigFS(minimalSharedFS),
		)
		require.NoError(t, err)
		assert.Equal(t, "explicit-env-value", cfg.DbPassword, "env:// should resolve when AutomaticEnv does not match")
	})

	t.Run("first secret manager in the chain wins for a matching identifier", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "inmem://shared-secret"
apiKey: key
plainValue: val
`)
		chdir(t, dir)

		first := xconfigs.NewInMemorySecretManager(map[string]string{"shared-secret": "from-first"})
		second := xconfigs.NewInMemorySecretManager(map[string]string{"shared-secret": "from-second"})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(first, second),
			xconfigs.WithSharedConfigFS(minimalSharedFS),
		)
		require.NoError(t, err)
		assert.Equal(t, "from-first", cfg.DbPassword, "first eligible secret manager should win")
	})

	t.Run("second secret manager is tried when first is not eligible", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: "inmem://my-secret"
apiKey: key
plainValue: val
`)
		chdir(t, dir)

		// env:// manager is first but not eligible for inmem:// identifiers.
		envSM := xconfigs.NewEnvSecretManager()
		inmemSM := xconfigs.NewInMemorySecretManager(map[string]string{"my-secret": "from-inmem"})

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(envSM, inmemSM),
			xconfigs.WithSharedConfigFS(minimalSharedFS),
		)
		require.NoError(t, err)
		assert.Equal(t, "from-inmem", cfg.DbPassword, "second manager should resolve when first returns ErrNotEligible")
	})

	t.Run("plain YAML value is used when no secret manager matches and AutomaticEnv does not override", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: plain-yaml-value
apiKey: key
plainValue: val
`)
		chdir(t, dir)

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewInMemorySecretManager(map[string]string{})),
			xconfigs.WithSharedConfigFS(minimalSharedFS),
		)
		require.NoError(t, err)
		assert.Equal(t, "plain-yaml-value", cfg.DbPassword, "plain YAML value should be used as the last resort")
	})
}

type testHTTPConfig struct {
	xconfigs.ApplicationConfig `validate:"required" mapstructure:",squash"`
	Logger                     struct {
		Level string `validate:"required" mapstructure:"level"`
	} `validate:"required" mapstructure:"logger"`
	Server struct {
		Port      int    `validate:"required" mapstructure:"port"`
		URLPrefix string `mapstructure:"urlPrefix"`
	} `validate:"required" mapstructure:"server"`
}

func TestSharedConfigAnchors(t *testing.T) {
	t.Run("should resolve shared anchors from base and env files", func(t *testing.T) {
		sharedFS := fstest.MapFS{
			"base.yaml": &fstest.MapFile{
				Data: []byte("_http_server_defaults: &http_server_defaults\n  port: 9999\n"),
			},
			"local.yaml": &fstest.MapFile{
				Data: []byte("_logger_defaults: &logger_defaults\n  level: warn\n"),
			},
		}

		dir := writeConfigFile(t, `
name: my-service
version: "1.0.0"
logger:
  <<: *logger_defaults
server:
  <<: *http_server_defaults
  urlPrefix: /v1
`)

		chdir(t, dir)

		cfg := &testHTTPConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewInMemorySecretManager(map[string]string{})),
			xconfigs.WithSharedConfigFS(sharedFS),
		)
		require.NoError(t, err)

		assert.Equal(t, "warn", cfg.Logger.Level)
		assert.Equal(t, 9999, cfg.Server.Port)
		assert.Equal(t, "/v1", cfg.Server.URLPrefix)
	})

	t.Run("should return an error when shared config files are missing", func(t *testing.T) {
		dir := writeConfigFile(t, `
name: test-service
version: "1.0.0"
dbPassword: pw
apiKey: key
plainValue: val
`)

		chdir(t, dir)

		cfg := &testConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewInMemorySecretManager(map[string]string{})),
			xconfigs.WithSharedConfigFS(fstest.MapFS{}),
		)
		require.Error(t, err)
	})

	t.Run("should use env-specific anchor values for prod", func(t *testing.T) {
		sharedFS := fstest.MapFS{
			"base.yaml": &fstest.MapFile{
				Data: []byte("_http_server_defaults: &http_server_defaults\n  port: 3000\n"),
			},
			"prod.yaml": &fstest.MapFile{
				Data: []byte("_logger_defaults: &logger_defaults\n  level: info\n"),
			},
		}

		dir := writeConfigFileForEnv(t, "prod", `
name: prod-service
version: "1.0.0"
logger:
  <<: *logger_defaults
server:
  <<: *http_server_defaults
`)

		chdir(t, dir)
		t.Setenv("ENVIRONMENT", "prod")

		cfg := &testHTTPConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewInMemorySecretManager(map[string]string{})),
			xconfigs.WithSharedConfigFS(sharedFS),
		)
		require.NoError(t, err)

		assert.Equal(t, "info", cfg.Logger.Level)
		assert.Equal(t, 3000, cfg.Server.Port)
	})

	t.Run("should allow service config to override shared anchor values", func(t *testing.T) {
		sharedFS := fstest.MapFS{
			"base.yaml": &fstest.MapFile{
				Data: []byte("_http_server_defaults: &http_server_defaults\n  port: 3000\n"),
			},
			"local.yaml": &fstest.MapFile{
				Data: []byte("_logger_defaults: &logger_defaults\n  level: debug\n"),
			},
		}

		dir := writeConfigFile(t, `
name: my-service
version: "1.0.0"
logger:
  <<: *logger_defaults
  level: warn
server:
  <<: *http_server_defaults
  port: 8080
`)

		chdir(t, dir)

		cfg := &testHTTPConfig{}
		err := xconfigs.LoadConfig(
			cfg,
			xconfigs.WithSecretManagers(xconfigs.NewInMemorySecretManager(map[string]string{})),
			xconfigs.WithSharedConfigFS(sharedFS),
		)
		require.NoError(t, err)

		assert.Equal(t, "warn", cfg.Logger.Level)
		assert.Equal(t, 8080, cfg.Server.Port)
	})
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	return writeConfigFileForEnv(t, "local", content)
}

func writeConfigFileForEnv(t *testing.T, env string, content string) string {
	t.Helper()
	dir := t.TempDir()
	configDir := filepath.Join(dir, "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, env+".yaml"), []byte(content), 0o600))
	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	t.Chdir(dir)
}
