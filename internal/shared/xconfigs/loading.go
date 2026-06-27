package xconfigs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/go-viper/mapstructure/v2"
	"github.com/raphoester/movie-reservation-system/assets"
	"github.com/raphoester/movie-reservation-system/internal/shared/xaws"
	"github.com/raphoester/movie-reservation-system/internal/shared/xcolls"
	"github.com/raphoester/movie-reservation-system/internal/shared/xenv"
	"github.com/raphoester/movie-reservation-system/internal/shared/xvalidate"
	"github.com/spf13/viper"
)

var validEnvs = xcolls.NewSet("dev", "staging", "prod", "local", "docker")

var (
	// ErrNotEligible is returned by a SecretManager when the identifier does not
	// match the scheme it handles (e.g. a plain value passed to an awssm:// manager).
	ErrNotEligible        = errors.New("secret manager not eligible for this identifier")
	ErrInvalidEnvironment = errors.New("invalid ENVIRONMENT value")
	ErrSecretResolution   = errors.New("secret resolution failed")
	ErrConfigValidation   = errors.New("config validation failed")
	ErrCustomValidation   = errors.New("custom config validation failed")
)

type LoadOption func(loadParams) loadParams

type loadParams struct {
	secretManagers []SecretManager
	sharedConfigFS fs.FS
}

// defaultAWSConfig returns an AWS config for secret resolution.
// For cloud environments (dev/staging/prod), it first tries to obtain credentials via the local
// AWS SSO profile (useful when running cloud configs locally). Falls back to the SDK default
// chain (IMDS / task roles) for actual cloud deployments where the AWS CLI is unavailable.
func defaultAWSConfig(ctx context.Context) aws.Config {
	env := xenv.Environment()
	if env == "dev" || env == "staging" || env == "prod" {
		if cfg, err := xaws.ConfigForEnv(ctx, env); err == nil {
			return *cfg
		}
	}
	cfg, _ := awsconfig.LoadDefaultConfig(ctx)
	return cfg
}

func (l loadParams) withDefaultsWhereNeeded() loadParams {
	// If there are no defined secret managers, default to AWS Secrets Manager.
	// We do NOT want ANY secret manager when the config is only being validated; that way it can be run in environments
	// where credentials are not available (e.g. local dev). As a result, the secret:// strings will be left as-is.
	if len(l.secretManagers) == 0 && !xenv.IsValidateConfigOnly() {
		l.secretManagers = []SecretManager{
			NewEnvSecretManager(),
			NewAWSSecretManager(NewAwsCache(NewAWSClientAdapter(secretsmanager.NewFromConfig(defaultAWSConfig(context.Background()))))),
		}
	}

	if l.sharedConfigFS == nil {
		l.sharedConfigFS = assets.SharedConfigs()
	}

	return l
}

func mustObtainCurrentPath() string {
	path, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("could not obtain current path: %v", err))
	}
	return path
}

func mustObtainDebugInfo() string {
	path := mustObtainCurrentPath()
	env := xenv.Environment()
	return fmt.Sprintf("current path: %s, environment: %s", path, env)
}

// loadCombinedYAML returns a pre-rendered config from disk (via RENDERED_CONFIG env var) if
// one is available, otherwise falls back to assembling it from the embedded shared configs
// and the service's on-disk config file.
func loadCombinedYAML(sharedFS fs.FS, env string, serviceConfigPath string) (string, error) {
	if path := xenv.RenderedConfigPath(); path != "" {
		data, err := os.ReadFile(filepath.Clean(path)) //nolint:gosec // G703: RENDERED_CONFIG is set by the operator (Dockerfile / docker-compose), not by end-user input
		if err == nil && len(data) > 0 {
			return string(data), nil
		}
		// File missing or empty — fall through to the embedded path silently.
	}
	return BuildCombinedYAML(sharedFS, env, serviceConfigPath)
}

// BuildCombinedYAML merges base, env-shared, and service configs into one YAML string.
// Exported so tooling commands can parse configs without triggering full secret resolution.
func BuildCombinedYAML(sharedFS fs.FS, env string, serviceConfigPath string) (string, error) {
	var parts []string

	base, err := fs.ReadFile(sharedFS, "base.yaml")
	if err != nil {
		return "", fmt.Errorf("could not read shared base config: %w", err)
	}
	parts = append(parts, string(base))

	envFile, err := fs.ReadFile(sharedFS, env+".yaml")
	if err != nil {
		return "", fmt.Errorf("could not read shared %s config: %w", env, err)
	}
	parts = append(parts, string(envFile))

	svcConfig, err := os.ReadFile(serviceConfigPath)
	if err != nil {
		return "", fmt.Errorf("could not read service config file %q: %w", serviceConfigPath, err)
	}

	parts = append(parts, string(svcConfig))
	return strings.Join(parts, "\n"), nil
}

func LoadConfig(config AppConfig, opts ...LoadOption) error {
	var lp loadParams
	for _, opt := range opts {
		lp = opt(lp)
	}
	lp = lp.withDefaultsWhereNeeded()

	env := xenv.Environment()
	if !validEnvs.Contains(env) {
		return fmt.Errorf("%w: %q", ErrInvalidEnvironment, env)
	}

	serviceConfigPath := filepath.Join("configs", filepath.Clean(env)+".yaml")
	if !strings.HasPrefix(serviceConfigPath, "configs"+string(filepath.Separator)) {
		return fmt.Errorf("%w: %q", ErrInvalidEnvironment, env)
	}

	combined, err := loadCombinedYAML(lp.sharedConfigFS, env, serviceConfigPath)
	if err != nil {
		return fmt.Errorf("could not build combined yaml for %s: %w", mustObtainDebugInfo(), err)
	}

	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.SetConfigType("yaml")

	if err := v.ReadConfig(strings.NewReader(combined)); err != nil {
		return fmt.Errorf("could not read combined yaml for %s: %w", mustObtainDebugInfo(), err)
	}

	decodeHook := viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			resolveSecretManager(lp.secretManagers),
			resolveLazyString(),
		),
	)

	if err := v.Unmarshal(config, decodeHook); err != nil {
		return fmt.Errorf("could not unmarshal config for %s: %w", mustObtainDebugInfo(), err)
	}

	config.SetEnvironment(env)

	if err := xvalidate.NewValidator().Struct(config); err != nil {
		return fmt.Errorf("%w: %s %v", ErrConfigValidation, mustObtainDebugInfo(), err.Error())
	}

	if validator, ok := config.(interface{ Validate() error }); ok {
		if err := validator.Validate(); err != nil {
			return fmt.Errorf("%w: %s %v", ErrCustomValidation, mustObtainDebugInfo(), err.Error())
		}
	}

	// returning a sentinel error would be cleaner, but will require changes to all callers.
	// this is probably a better solution.
	if xenv.IsValidateConfigOnly() {
		os.Exit(0)
	}

	return nil
}
