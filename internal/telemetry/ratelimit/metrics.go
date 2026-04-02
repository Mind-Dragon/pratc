// Package ratelimit provides atomic counters for tracking GitHub API rate limit events.
package ratelimit

import (
	"fmt"
	"sync/atomic"
)

// Metrics holds atomic counters for rate limit telemetry.
// All counters are int64 and accessed atomically.
type Metrics struct {
	requestsTotal      atomic.Int64
	rateLimitHits      atomic.Int64
	secondaryLimitHits atomic.Int64
	retries            atomic.Int64
	budgetPauses       atomic.Int64
}

// MetricsSnapshot holds a point-in-time copy of the metrics.
// Values are copied at call time, making it safe to read without races.
type MetricsSnapshot struct {
	RequestsTotal      int64
	RateLimitHits      int64
	SecondaryLimitHits int64
	Retries            int64
	BudgetPauses       int64
}

// NewMetrics creates a new Metrics instance with all counters initialized to zero.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncRequests increments the total requests counter.
func (m *Metrics) IncRequests() {
	m.requestsTotal.Add(1)
}

// IncRateLimitHit increments the rate limit hits counter.
func (m *Metrics) IncRateLimitHit() {
	m.rateLimitHits.Add(1)
}

// IncSecondaryLimitHit increments the secondary rate limit hits counter.
func (m *Metrics) IncSecondaryLimitHit() {
	m.secondaryLimitHits.Add(1)
}

// IncRetry increments the retry counter.
func (m *Metrics) IncRetry() {
	m.retries.Add(1)
}

// IncBudgetPause increments the budget pauses counter.
func (m *Metrics) IncBudgetPause() {
	m.budgetPauses.Add(1)
}

// Snapshot returns a MetricsSnapshot with current values copied.
// The returned snapshot is safe to read without synchronization.
func (m *Metrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		RequestsTotal:      m.requestsTotal.Load(),
		RateLimitHits:      m.rateLimitHits.Load(),
		SecondaryLimitHits: m.secondaryLimitHits.Load(),
		Retries:            m.retries.Load(),
		BudgetPauses:       m.budgetPauses.Load(),
	}
}

// Reset atomically sets all counters to zero.
func (m *Metrics) Reset() {
	m.requestsTotal.Store(0)
	m.rateLimitHits.Store(0)
	m.secondaryLimitHits.Store(0)
	m.retries.Store(0)
	m.budgetPauses.Store(0)
}

// String returns a human-readable representation of the current metrics.
func (m *Metrics) String() string {
	snap := m.Snapshot()
	return fmt.Sprintf("Metrics: requests=%d rate_limit_hits=%d secondary_limit_hits=%d retries=%d budget_pauses=%d",
		snap.RequestsTotal,
		snap.RateLimitHits,
		snap.SecondaryLimitHits,
		snap.Retries,
		snap.BudgetPauses,
	)
}
