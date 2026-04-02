// Package ratelimit provides budget tracking for GitHub API rate limits.
package ratelimit

import (
	"fmt"
	"time"
)

// BudgetManager tracks GitHub API rate limit budget and helps make decisions
// about when to pause sync operations.
type BudgetManager struct {
	Limit       int
	Remaining   int
	ResetTime   time.Time
	LastUpdated time.Time
}

// NewBudgetManager creates a BudgetManager with default values.
// Default: Limit=5000, Remaining=5000, ResetTime=now+1hour.
func NewBudgetManager() BudgetManager {
	now := time.Now()
	return BudgetManager{
		Limit:       5000,
		Remaining:   5000,
		ResetTime:   now.Add(1 * time.Hour),
		LastUpdated: now,
	}
}

// reserveBuffer is the minimum number of requests to keep in reserve.
const reserveBuffer = 200

// CanAfford returns true if there are enough remaining requests to cover
// the requested amount plus the reserve buffer.
// Returns false if Remaining < n + reserveBuffer.
func (b BudgetManager) CanAfford(requests int) bool {
	return b.Remaining >= requests+reserveBuffer
}

// RecordResponse updates the budget state from GitHub API response headers.
// remaining: the X-RateLimit-Remaining header value
// resetEpoch: the X-RateLimit-Reset header value (Unix epoch seconds)
func (b *BudgetManager) RecordResponse(remaining int, resetEpoch int64) {
	b.Remaining = remaining
	b.ResetTime = time.Unix(resetEpoch, 0)
	b.LastUpdated = time.Now()
}

// WaitDuration returns the duration to wait until the rate limit resets.
// Returns 0 if there is sufficient remaining budget (above reserve).
// Returns positive duration if budget is exhausted or below reserve.
func (b BudgetManager) WaitDuration() time.Duration {
	if b.Remaining >= reserveBuffer {
		return 0
	}
	wait := b.ResetTime.Sub(time.Now())
	if wait < 0 {
		return 0
	}
	return wait
}

// String returns a human-readable summary of the budget state.
func (b BudgetManager) String() string {
	resetIn := b.ResetTime.Sub(time.Now())
	if resetIn < 0 {
		resetIn = 0
	}

	minutes := int(resetIn.Minutes())
	seconds := int(resetIn.Seconds()) % 60

	var resetStr string
	if minutes > 0 {
		resetStr = fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		resetStr = fmt.Sprintf("%ds", seconds)
	}

	return fmt.Sprintf("Budget: %d/%d remaining, resets in %s", b.Remaining, b.Limit, resetStr)
}
