package executor

import (
	"fmt"
	"sync"
)

// CircuitBreakerConfig controls fail-closed live mutation concurrency limits.
type CircuitBreakerConfig struct {
	MaxGlobal  int
	MaxPerRepo int
}

// CircuitBreakerStatus exposes current live mutation breaker state for operator surfaces.
type CircuitBreakerStatus struct {
	MaxGlobal      int            `json:"max_global"`
	MaxPerRepo     int            `json:"max_per_repo"`
	InFlightGlobal int            `json:"in_flight_global"`
	InFlightByRepo map[string]int `json:"in_flight_by_repo"`
}

// MutationCircuitBreaker guards live GitHub mutations from unbounded concurrent writes.
type MutationCircuitBreaker struct {
	mu     sync.Mutex
	cfg    CircuitBreakerConfig
	global int
	byRepo map[string]int
}

// NewMutationCircuitBreaker creates a fail-closed mutation circuit breaker.
func NewMutationCircuitBreaker(cfg CircuitBreakerConfig) *MutationCircuitBreaker {
	if cfg.MaxGlobal <= 0 {
		cfg.MaxGlobal = 1
	}
	if cfg.MaxPerRepo <= 0 {
		cfg.MaxPerRepo = 1
	}
	return &MutationCircuitBreaker{cfg: cfg, byRepo: map[string]int{}}
}

// Acquire reserves one live mutation slot for repo. The returned release function is idempotent.
func (b *MutationCircuitBreaker) Acquire(repo string) (func(), error) {
	if b == nil {
		return func() {}, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if repo == "" {
		return nil, fmt.Errorf("circuit breaker denied live mutation: repo is required")
	}
	if b.global >= b.cfg.MaxGlobal {
		return nil, fmt.Errorf("circuit breaker denied live mutation for %s: global limit exceeded (%d/%d)", repo, b.global, b.cfg.MaxGlobal)
	}
	if b.byRepo[repo] >= b.cfg.MaxPerRepo {
		return nil, fmt.Errorf("circuit breaker denied live mutation for %s: repo limit exceeded (%d/%d)", repo, b.byRepo[repo], b.cfg.MaxPerRepo)
	}
	b.global++
	b.byRepo[repo]++
	released := false
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if released {
			return
		}
		released = true
		if b.global > 0 {
			b.global--
		}
		if b.byRepo[repo] > 1 {
			b.byRepo[repo]--
		} else {
			delete(b.byRepo, repo)
		}
	}, nil
}

// Status returns a snapshot of breaker state.
func (b *MutationCircuitBreaker) Status() CircuitBreakerStatus {
	if b == nil {
		return CircuitBreakerStatus{InFlightByRepo: map[string]int{}}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	byRepo := make(map[string]int, len(b.byRepo))
	for repo, count := range b.byRepo {
		byRepo[repo] = count
	}
	return CircuitBreakerStatus{MaxGlobal: b.cfg.MaxGlobal, MaxPerRepo: b.cfg.MaxPerRepo, InFlightGlobal: b.global, InFlightByRepo: byRepo}
}
