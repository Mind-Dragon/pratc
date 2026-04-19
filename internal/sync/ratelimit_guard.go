package sync

import (
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

type RateLimitGuard struct {
	budget  *ratelimit.BudgetManager
	metrics *ratelimit.Metrics
	store   *cache.Store
	jobID   string
	log     *logger.Logger
}

func NewRateLimitGuard(budget *ratelimit.BudgetManager, metrics *ratelimit.Metrics, store *cache.Store, jobID string) RateLimitGuard {
	return RateLimitGuard{
		budget:  budget,
		metrics: metrics,
		store:   store,
		jobID:   jobID,
		log:     logger.New("ratelimit_guard"),
	}
}

func (g RateLimitGuard) CheckBudget(repo string, estimatedRequests int) (int, error) {
	g.metrics.IncRequests()
	if g.log != nil {
		g.log.Info("budget gate evaluated", "repo", repo, "estimated_requests", estimatedRequests, "remaining_budget", g.budget.Remaining())
	}

	if g.budget.CanAfford(estimatedRequests) {
		chunkSize := ratelimit.CalculateChunkSize(1000000, g.budget.Remaining(), 200, ratelimit.WithRequestsPerPR(1))
		if g.log != nil {
			g.log.Info("budget gate passed", "repo", repo, "chunk_size", chunkSize, "remaining_budget", g.budget.Remaining())
		}
		return chunkSize, nil
	}

	nextScheduledAt := g.budget.ResetTime.Add(15 * time.Second)
	if g.log != nil {
		g.log.Warn("budget gate paused sync", "repo", repo, "estimated_requests", estimatedRequests, "remaining_budget", g.budget.Remaining(), "next_scheduled_at", nextScheduledAt.Format(time.RFC3339))
	}
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
