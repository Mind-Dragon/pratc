// Package ratelimit provides budget tracking for GitHub API rate limits.
package ratelimit

import (
	"fmt"
	"sync"
	"time"
)

// BudgetManager tracks GitHub API rate limit budget and helps make decisions
// about when to pause sync operations.
type BudgetManager struct {
	Limit         int
	RemainingVal  int
	ResetTime     time.Time
	LastUpdated   time.Time
	reserveBuffer int
	resetBuffer   time.Duration
	reservedCount int
	metrics       *Metrics
	mu            sync.RWMutex
}

// Option is a functional option for configuring BudgetManager.
type Option func(*BudgetManager)

// WithRateLimit sets the total rate limit (default: 5000).
func WithRateLimit(limit int) Option {
	return func(b *BudgetManager) {
		b.Limit = limit
		if b.RemainingVal > limit {
			b.RemainingVal = limit
		}
	}
}

// WithReserveBuffer sets the minimum requests to keep in reserve (default: 200).
func WithReserveBuffer(buffer int) Option {
	return func(b *BudgetManager) {
		b.reserveBuffer = buffer
	}
}

// WithResetBuffer sets the additional buffer time to wait after rate limit reset (default: 15s).
func WithResetBuffer(seconds int) Option {
	return func(b *BudgetManager) {
		b.resetBuffer = time.Duration(seconds) * time.Second
	}
}

// ReserveBuffer returns the configured reserve buffer.
func (b *BudgetManager) ReserveBuffer() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.reserveBuffer
}

// WithMetrics sets the metrics collector for telemetry tracking.
func WithMetrics(metrics *Metrics) Option {
	return func(b *BudgetManager) {
		b.metrics = metrics
	}
}

// NewBudgetManager creates a BudgetManager with the provided options.
// Defaults: Limit=5000, Remaining=5000, ResetTime=now+1hour, reserveBuffer=200, resetBuffer=15s.
func NewBudgetManager(opts ...Option) *BudgetManager {
	now := time.Now()
	bm := &BudgetManager{
		Limit:         5000,
		RemainingVal:  5000,
		ResetTime:     now.Add(1 * time.Hour),
		LastUpdated:   now,
		reserveBuffer: 200,
		resetBuffer:   15 * time.Second,
		reservedCount: 0,
	}

	for _, opt := range opts {
		opt(bm)
	}

	return bm
}

// Remaining returns the current remaining requests (accounting for reservations).
func (b *BudgetManager) Remaining() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.RemainingVal - b.reservedCount
}

// ResetAt returns the rate limit reset time.
func (b *BudgetManager) ResetAt() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.ResetTime
}

// ResetBufferSeconds returns the reset buffer duration in seconds.
func (b *BudgetManager) ResetBufferSeconds() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return int(b.resetBuffer.Seconds())
}

// ShouldPause returns true if the budget is exhausted and operations should pause.
// Returns true if Remaining() <= reserveBuffer.
// When returning true, increments the budgetPauses metrics counter if metrics are configured.
func (b *BudgetManager) ShouldPause() bool {
	shouldPause := b.WouldPause()

	if shouldPause && b.metrics != nil {
		b.metrics.IncBudgetPause()
	}

	return shouldPause
}

// WouldPause returns true when the current budget is at or below the reserve buffer.
// Unlike ShouldPause, it does not emit metrics.
func (b *BudgetManager) WouldPause() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return (b.RemainingVal - b.reservedCount) <= b.reserveBuffer
}

// Reserve attempts to reserve N requests from the budget.
// Returns an error if there are insufficient remaining requests or if the reservation
// would leave the budget below the reserve buffer.
func (b *BudgetManager) Reserve(count int) error {
	if count < 0 {
		return fmt.Errorf("cannot reserve negative requests: %d", count)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	available := b.RemainingVal - b.reservedCount
	if available < count {
		return fmt.Errorf("insufficient budget: requested %d, available %d (reserved %d)", count, available, b.reservedCount)
	}

	// Check if reservation would leave us below reserve buffer (allow exact match)
	if (available-count) < b.reserveBuffer && count != available {
		return fmt.Errorf("reservation would exceed reserve buffer: requested %d, would leave %d (reserve %d)", count, available-count, b.reserveBuffer)
	}

	b.reservedCount += count
	return nil
}

// Release releases N previously reserved requests back to the budget.
func (b *BudgetManager) Release(count int) {
	if count < 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if count > b.reservedCount {
		count = b.reservedCount
	}
	b.reservedCount -= count
}

// EstimatedCompletionTime calculates the estimated time to complete the remaining work.
// Assumes requests are made at a rate of 1 per second (conservative estimate for GitHub API).
// Returns 0 if there's no remaining work or sufficient budget.
// If budget is insufficient, includes wait time until reset plus reset buffer.
func (b *BudgetManager) EstimatedCompletionTime(remainingWork int) time.Duration {
	if remainingWork <= 0 {
		return 0
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	available := b.RemainingVal - b.reservedCount

	// If we have enough budget, estimate based on 1 req/sec rate
	if available >= remainingWork {
		return time.Duration(remainingWork) * time.Second
	}

	// Not enough budget - need to wait for reset
	waitDuration := time.Until(b.ResetTime)
	waitDuration = max(waitDuration, 0)

	// Add reset buffer and time to complete remaining work after reset
	totalTime := waitDuration + b.resetBuffer + time.Duration(remainingWork)*time.Second
	return totalTime
}

// CanAfford returns true if there are enough remaining requests to cover
// the requested amount while maintaining the reserve buffer.
func (b *BudgetManager) CanAfford(requests int) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	available := b.RemainingVal - b.reservedCount
	// At exact reserve boundary with no requests, we can afford
	if requests == 0 && available >= b.reserveBuffer {
		return true
	}
	return (available - requests) > b.reserveBuffer
}

// RecordResponse updates the budget state from GitHub API response headers.
// remaining: the X-RateLimit-Remaining header value
// resetEpoch: the X-RateLimit-Reset header value (Unix epoch seconds)
func (b *BudgetManager) RecordResponse(remaining int, resetEpoch int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.RemainingVal = remaining
	b.ResetTime = time.Unix(resetEpoch, 0)
	b.LastUpdated = time.Now()
}

// WaitDuration returns the duration to wait until the rate limit resets.
// Returns 0 if there is sufficient remaining budget (above or at reserve).
// Returns positive duration if budget is below reserve, including reset buffer.
func (b *BudgetManager) WaitDuration() time.Duration {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if (b.RemainingVal - b.reservedCount) > b.reserveBuffer {
		return 0
	}

	wait := time.Until(b.ResetTime)
	wait = max(wait, 0)

	return wait + b.resetBuffer
}

// String returns a human-readable summary of the budget state.
func (b *BudgetManager) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	resetIn := time.Until(b.ResetTime)
	resetIn = max(resetIn, 0)

	minutes := int(resetIn.Minutes())
	seconds := int(resetIn.Seconds()) % 60

	var resetStr string
	if minutes > 0 {
		resetStr = fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		resetStr = fmt.Sprintf("%ds", seconds)
	}

	return fmt.Sprintf("Budget: %d/%d remaining (%d reserved), resets in %s",
		b.RemainingVal-b.reservedCount, b.Limit, b.reservedCount, resetStr)
}
