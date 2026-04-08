package sync

import (
	"context"
	"fmt"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

type BudgetChecker interface {
	CheckBudget(repo string, estimatedRequests int) (int, error)
	ShouldPause() bool
	Budget() *ratelimit.BudgetManager
}

type RateLimitRunner struct {
	inner  Runner
	guard  BudgetChecker
	store  *cache.Store
	logger *logger.Logger
}

func NewRateLimitRunner(inner Runner, guard BudgetChecker, store *cache.Store, repo string) *RateLimitRunner {
	return &RateLimitRunner{
		inner:  inner,
		guard:  guard,
		store:  store,
		logger: logger.New("ratelimit_runner"),
	}
}

func (r *RateLimitRunner) Run(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
	filter := cache.PRFilter{Repo: repo}
	prs, err := r.store.ListPRs(filter)
	if err != nil {
		return fmt.Errorf("list PRs for budget estimation: %w", err)
	}
	totalPRs := len(prs)
	budget := r.guard.Budget()

	if r.guard.ShouldPause() {
		if budget != nil {
			chunkSize := CalculateChunkSize(budget)
			r.logger.Warn("sync paused before batch due to rate limit budget",
				"repo", repo,
				"chunk_size", chunkSize,
				"remaining_budget", budget.Remaining(),
				"total_prs", totalPRs)
		}
		_, err := r.guard.CheckBudget(repo, totalPRs)
		return err
	}

	options := ratelimit.EstimateOptions{
		FetchFiles:   true,
		FetchReviews: true,
		PerPage:      100,
	}
	estimatedRequests := ratelimit.EstimateRequests(totalPRs, options)
	chunkSize := totalPRs
	if budget != nil {
		chunkSize = CalculateChunkSize(budget)
	}
	if totalPRs > 0 && chunkSize <= 0 {
		_, err := r.guard.CheckBudget(repo, estimatedRequests)
		return err
	}

	if _, err := r.guard.CheckBudget(repo, estimatedRequests); err != nil {
		return err
	}

	if chunkSize < totalPRs {
		r.logger.Warn("partial sync due to rate limit budget constraints",
			"repo", repo,
			"total_prs", totalPRs,
			"chunk_size", chunkSize,
			"estimated_requests", estimatedRequests)
	}

	return r.inner.Run(ctx, repo, emit)
}
