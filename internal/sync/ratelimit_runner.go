package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	inner           Runner
	guard           BudgetChecker
	store           *cache.Store
	logger          *logger.Logger
	currentChunkSize int
	budget          *ratelimit.BudgetManager
	mu              sync.RWMutex
}

func NewRateLimitRunner(inner Runner, guard BudgetChecker, store *cache.Store, repo string) *RateLimitRunner {
	budget := guard.Budget()
	return &RateLimitRunner{
		inner:  inner,
		guard:  guard,
		store:  store,
		logger: logger.New("ratelimit_runner"),
		budget: budget,
	}
}

func (r *RateLimitRunner) Run(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) (runErr error) {
	// Local stop channel ensures each Run call controls its own watcher lifecycle.
	// Deferred close guarantees the watcher stops on every exit path, including early returns.
	stopCh := make(chan struct{})
	defer close(stopCh)

	filter := cache.PRFilter{Repo: repo}
	prs, err := r.store.ListPRs(filter)
	if err != nil {
		return fmt.Errorf("list PRs for budget estimation: %w", err)
	}
	totalPRs := len(prs)

	// Seed the current chunk size before making the first budget decision.
	r.updateChunkSize()

	// Calculate initial chunk size
	chunkSize := r.getChunkSize()

	// Check pause condition BEFORE starting the budget watcher goroutine.
	// This prevents a goroutine leak when we return early due to budget exhaustion.
	if r.guard.ShouldPause() {
		if r.budget != nil {
			r.logger.Warn("sync paused before batch due to rate limit budget",
				"repo", repo,
				"chunk_size", chunkSize,
				"remaining_budget", r.budget.Remaining(),
				"total_prs", totalPRs)
		}
		_, err := r.guard.CheckBudget(repo, totalPRs)
		return err
	}

	// Only start the budget watcher after confirming we won't return early
	r.startBudgetWatcher(stopCh)

	options := ratelimit.EstimateOptions{
		FetchFiles:   true,
		FetchReviews: true,
		PerPage:      100,
	}
	effectiveTotal := totalPRs
	if chunkSize > 0 && chunkSize < effectiveTotal {
		effectiveTotal = chunkSize
	}
	estimatedRequests := ratelimit.EstimateRequests(effectiveTotal, options)

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

	// Create context with chunk size
	ctx = context.WithValue(ctx, "chunk_size", chunkSize)

	// Run inner runner
	err = r.inner.Run(ctx, repo, emit)

	return err
}

func (r *RateLimitRunner) startBudgetWatcher(stopCh chan struct{}) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				r.updateChunkSize()
			}
		}
	}()
}

func (r *RateLimitRunner) updateChunkSize() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.budget == nil {
		r.currentChunkSize = 0
		return
	}

	remaining := r.budget.Remaining()
	switch {
	case remaining > 2000:
		r.currentChunkSize = 100
	case remaining > 500:
		r.currentChunkSize = 50
	case remaining > 200:
		r.currentChunkSize = 25
	default:
		r.currentChunkSize = 0
	}

	// Ensure we can afford the chunk size
	requestsNeeded := r.currentChunkSize * 3
	if !r.budget.CanAfford(requestsNeeded) {
		affordableChunk := (remaining - 200) / 3
		if affordableChunk < 10 {
			r.currentChunkSize = 0
		} else {
			r.currentChunkSize = affordableChunk
		}
	}
}

func (r *RateLimitRunner) getChunkSize() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentChunkSize
}