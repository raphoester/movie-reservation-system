package xhealth

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"time"
)

type StatusChangeHandler func(name string, healthy bool, err error)

type Monitor struct {
	registry       *HealthRegistry
	logger         *slog.Logger
	onStatusChange StatusChangeHandler
	checkTimeout   time.Duration
	lastStatus     map[string]bool
	lastStatusMu   sync.RWMutex
}

type MonitorConfig struct {
	CheckTimeout   time.Duration
	OnStatusChange StatusChangeHandler
}

func NewMonitor(registry *HealthRegistry, logger *slog.Logger, config MonitorConfig) *Monitor {
	if config.CheckTimeout == 0 {
		config.CheckTimeout = 5 * time.Second
	}

	return &Monitor{
		lastStatusMu:   sync.RWMutex{},
		registry:       registry,
		logger:         logger,
		onStatusChange: config.OnStatusChange,
		checkTimeout:   config.CheckTimeout,
		lastStatus:     make(map[string]bool),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	checks := m.registry.List()

	var wg sync.WaitGroup

	for _, name := range checks {
		wg.Go(func() {
			m.monitorCheck(ctx, name)
		})
	}

	wg.Wait()
}

func (m *Monitor) monitorCheck(ctx context.Context, name string) {
	m.registry.mu.RLock()
	check, exists := m.registry.checks[name]
	m.registry.mu.RUnlock()

	if !exists {
		m.logger.Error("health check not found", "name", name)
		return
	}

	m.registry.mu.RLock()
	cfg, exists := m.registry.configs[name]
	m.registry.mu.RUnlock()

	if !exists {
		cfg = defaultCheckConfig()
	}

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	m.logger.Info("starting health check monitor", "name", name, "interval", cfg)

	m.runCheck(ctx, check)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("stopping health check monitor", "name", name)
			return
		case <-ticker.C:
			m.runCheck(ctx, check)
		}
	}
}

func (m *Monitor) runCheck(ctx context.Context, check HealthCheck) {
	ctx, cancel := context.WithTimeout(ctx, m.checkTimeout)
	defer cancel()

	name := check.Name()
	err := check.Check(ctx)
	healthy := err == nil

	m.lastStatusMu.RLock()
	lastHealthy, hadPreviousStatus := m.lastStatus[name]
	m.lastStatusMu.RUnlock()

	m.lastStatusMu.Lock()
	m.lastStatus[name] = healthy
	m.lastStatusMu.Unlock()

	if healthy {
		m.logger.Debug("health check passed", "name", name)
	} else {
		m.logger.Error("health check failed", "name", name, "error", err)
	}

	if m.onStatusChange != nil && hadPreviousStatus && lastHealthy != healthy {
		m.onStatusChange(name, healthy, err)
	}
}

func (m *Monitor) Status() map[string]bool {
	m.lastStatusMu.RLock()
	defer m.lastStatusMu.RUnlock()

	status := make(map[string]bool, len(m.lastStatus))
	maps.Copy(status, m.lastStatus)

	return status
}
