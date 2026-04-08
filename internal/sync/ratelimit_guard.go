package sync

import (
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

type RateLimitGuard struct {
	budget  *ratelimit.BudgetManager
	metrics *ratelimit.Metrics
	store   *cache.Store
	jobID   string
}

func NewRateLimitGuard(budget *ratelimit.BudgetManager, metrics *ratelimit.Metrics, store *cache.Store, jobID string) RateLimitGuard {
	return RateLimitGuard{
		budget:  budget,
		metrics: metrics,
		store:   store,
		jobID:   jobID,
	}
}

func (g RateLimitGuard) CheckBudget(repo string, estimatedRequests int) (int, error) {
	g.metrics.IncRequests()

	if g.budget.CanAfford(estimatedRequests) {
		chunkSize := ratelimit.CalculateChunkSize(1000000, g.budget.Remaining(), 200, ratelimit.WithRequestsPerPR(1))
		return chunkSize, nil
	}

	nextScheduledAt := g.budget.ResetTime.Add(15 * time.Second)

	if err := g.store.PauseSyncJob(g.jobID, nextScheduledAt, "rate limit budget exhausted"); err != nil {
		return 0, fmt.Errorf("pause sync job: %w", err)
	}

	g.metrics.IncBudgetPause()
	return 0, fmt.Errorf("rate limit budget exhausted")
}

func (g RateLimitGuard) ShouldPause() bool {
	if g.budget == nil {
		return false
	}
	return g.budget.WouldPause()
}

func (g RateLimitGuard) Budget() *ratelimit.BudgetManager {
	return g.budget
}
