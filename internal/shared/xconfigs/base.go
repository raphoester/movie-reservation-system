package xconfigs

import "github.com/raphoester/movie-reservation-system/internal/shared/xenv"

type ApplicationConfig struct {
	Name    string `validate:"required"`
	Version string `validate:"required"`

	environment string
}

type AppConfig interface {
	SetEnvironment(env string)

	IsLocal() bool
	Environment() string

	GetName() string
	GetVersion() string
}

func (c *ApplicationConfig) IsLocal() bool {
	return c.environment == xenv.LocalEnvironment
}

func (c *ApplicationConfig) Environment() string {
	return c.environment
}

func (c *ApplicationConfig) SetEnvironment(env string) {
	c.environment = env
}

func (c *ApplicationConfig) GetName() string {
	return c.Name
}

func (c *ApplicationConfig) GetVersion() string {
	return c.Version
}
