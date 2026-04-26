package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

// WorkerConfig controls the central executor worker that consumes persisted ActionIntents.
type WorkerConfig struct {
	Repo           string
	WorkerID       string
	LeaseTTL       time.Duration
	PollInterval   time.Duration
	Concurrency    int
	Live           bool
	CircuitBreaker *MutationCircuitBreaker
}

// Worker claims work items and executes their persisted intents through the central executor.
type Worker struct {
	cfg     WorkerConfig
	queue   *workqueue.Queue
	mutator GitHubMutator
	ledger  Ledger
	now     func() time.Time
}

func NewWorker(cfg WorkerConfig, queue *workqueue.Queue, mutator GitHubMutator, ledger Ledger) *Worker {
	if cfg.WorkerID == "" {
		cfg.WorkerID = "executor-worker"
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 10 * time.Minute
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Live && cfg.CircuitBreaker == nil {
		cfg.CircuitBreaker = NewMutationCircuitBreaker(CircuitBreakerConfig{MaxGlobal: 1, MaxPerRepo: 1})
	}
	if ledger == nil {
		ledger = NewMemoryLedger()
	}
	return &Worker{cfg: cfg, queue: queue, mutator: mutator, ledger: ledger, now: func() time.Time { return time.Now().UTC() }}
}

func (w *Worker) Run(ctx context.Context) error {
	if w.queue == nil {
		return fmt.Errorf("work queue is required")
	}
	if w.mutator == nil {
		return fmt.Errorf("github mutator is required")
	}
	errCh := make(chan error, w.cfg.Concurrency)
	for i := 0; i < w.cfg.Concurrency; i++ {
		idx := i
		go func() {
			for {
				select {
				case <-ctx.Done():
					errCh <- nil
					return
				default:
					if _, err := w.ProcessOnce(ctx); err != nil {
						errCh <- fmt.Errorf("worker %d: %w", idx, err)
						return
					}
					timer := time.NewTimer(w.cfg.PollInterval)
					select {
					case <-ctx.Done():
						timer.Stop()
						errCh <- nil
						return
					case <-timer.C:
					}
				}
			}
		}()
	}
	for i := 0; i < w.cfg.Concurrency; i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	if w.queue == nil {
		return false, fmt.Errorf("work queue is required")
	}
	if w.mutator == nil {
		return false, fmt.Errorf("github mutator is required")
	}
	items, err := w.queue.GetClaimable(ctx, w.cfg.Repo, "", 1)
	if err != nil {
		return false, err
	}
	if len(items) == 0 {
		return false, nil
	}
	item := items[0]
	claimed, err := w.queue.Claim(ctx, item.ID, w.cfg.WorkerID, w.cfg.LeaseTTL)
	if err != nil {
		return false, err
	}
	intents, err := w.queue.GetIntentsForWorkItem(ctx, w.cfg.Repo, claimed.ID)
	if err != nil {
		_ = w.fail(ctx, claimed, "load_intents_failed")
		return true, err
	}
	if len(intents) == 0 {
		_ = w.fail(ctx, claimed, "missing_action_intent")
		return true, fmt.Errorf("work item %s has no persisted action intents", claimed.ID)
	}
	current := claimed
	for _, intent := range intents {
		actualLiveWrite := w.cfg.Live && !intent.DryRun
		var release func()
		if actualLiveWrite {
			var err error
			release, err = w.cfg.CircuitBreaker.Acquire(w.cfg.Repo)
			if err != nil {
				snapshot := fmt.Sprintf(`{"intent_id":%q,"repo":%q,"error":%q}`, intent.ID, w.cfg.Repo, err.Error())
				_ = w.ledger.RecordTransition(intent.IdempotencyKey, "circuit_denied", snapshot, nil)
				_ = w.fail(ctx, current, "circuit_breaker_denied")
				return true, nil
			}
		}
		exec := New(Config{Repo: w.cfg.Repo, DryRun: !actualLiveWrite, PolicyProfile: intent.PolicyProfile}, w.mutator, w.ledger)
		result, err := exec.ExecuteIntent(ctx, intent)
		if release != nil {
			release()
		}
		if err != nil {
			_ = w.fail(ctx, current, result.Error)
			return true, err
		}
		if !result.Executed && !result.AlreadyExecuted && !result.DryRun {
			_ = w.fail(ctx, current, "intent_not_executed")
			return true, nil
		}
	}
	preflighted, err := w.queue.Transition(ctx, current.ID, w.cfg.WorkerID, current.State, types.ActionWorkItemStatePreflighted, "worker_preflighted")
	if err != nil {
		return true, err
	}
	approved, err := w.queue.Transition(ctx, preflighted.ID, w.cfg.WorkerID, preflighted.State, types.ActionWorkItemStateApprovedForExecution, "worker_approved")
	if err != nil {
		return true, err
	}
	executed, err := w.queue.Transition(ctx, approved.ID, w.cfg.WorkerID, approved.State, types.ActionWorkItemStateExecuted, "worker_executed")
	if err != nil {
		return true, err
	}
	_, err = w.queue.Transition(ctx, executed.ID, w.cfg.WorkerID, executed.State, types.ActionWorkItemStateVerified, "worker_verified")
	return true, err
}

func (w *Worker) fail(ctx context.Context, item types.ActionWorkItem, reason string) error {
	if reason == "" {
		reason = "worker_failed"
	}
	_, err := w.queue.Transition(ctx, item.ID, w.cfg.WorkerID, item.State, types.ActionWorkItemStateFailed, reason)
	return err
}
