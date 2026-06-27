package xhealth

import (
	"context"
	"sync"
	"time"
)

type HealthCheck interface {
	Name() string
	Check(ctx context.Context) error
}

type HealthRegistry struct {
	mu      sync.RWMutex
	checks  map[string]HealthCheck
	configs map[string]checkConfig
}

func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{
		mu:      sync.RWMutex{},
		checks:  make(map[string]HealthCheck),
		configs: make(map[string]checkConfig),
	}
}

type CheckOption func(*checkConfig)

type checkConfig struct {
	interval time.Duration
}

func defaultCheckConfig() checkConfig {
	return checkConfig{
		interval: DefaultCheckInterval,
	}
}

func WithInterval(d time.Duration) CheckOption {
	return func(c *checkConfig) {
		c.interval = d
	}
}

func (r *HealthRegistry) Register(check HealthCheck, opts ...CheckOption) {
	cfg := defaultCheckConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.checks[check.Name()] = check
	r.configs[check.Name()] = cfg
}

func (r *HealthRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.checks))
	for name := range r.checks {
		names = append(names, name)
	}

	return names
}
