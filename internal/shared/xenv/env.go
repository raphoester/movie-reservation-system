package xenv

import "os"

const (
	KeyEnvironment        = "ENVIRONMENT"
	KeyRenderedConfig     = "RENDERED_CONFIG"
	KeyValidateConfigOnly = "VALIDATE_CONFIG_ONLY"

	LocalEnvironment = "local"
)

// Environment returns the current deployment environment.
// Defaults to LocalEnvironment when the env var is unset.
func Environment() string {
	if v := os.Getenv(KeyEnvironment); v != "" {
		return v
	}
	return LocalEnvironment
}

// IsLocal reports whether the service is running in the local environment.
func IsLocal() bool {
	return Environment() == LocalEnvironment
}

// RenderedConfigPath returns the path to a pre-rendered config file, or "" if unset.
func RenderedConfigPath() string {
	return os.Getenv(KeyRenderedConfig)
}

// IsValidateConfigOnly reports whether the process should exit after config validation.
func IsValidateConfigOnly() bool {
	return os.Getenv(KeyValidateConfigOnly) == "1"
}
