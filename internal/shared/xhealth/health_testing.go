package xhealth

import (
	"context"
	"sync"
)

func (r *HealthRegistry) Check(ctx context.Context, name string) error {
	r.mu.RLock()
	check, exists := r.checks[name]
	r.mu.RUnlock()

	if !exists {
		return ErrCheckNotFound
	}

	return check.Check(ctx)
}

func (r *HealthRegistry) CheckAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	checks := make([]HealthCheck, 0, len(r.checks))
	for _, check := range r.checks {
		checks = append(checks, check)
	}
	r.mu.RUnlock()

	results := make(map[string]error)
	resultsMu := sync.Mutex{}

	var wg sync.WaitGroup
	for _, check := range checks {
		wg.Go(func() {
			err := check.Check(ctx)
			resultsMu.Lock()
			results[check.Name()] = err
			resultsMu.Unlock()
		})
	}
	wg.Wait()

	return results
}
