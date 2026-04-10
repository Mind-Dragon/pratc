package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	gh "github.com/jeffersonnunn/pratc/internal/github"
)

type RateLimitProvider interface {
	GetRateLimit(context.Context) (int, error)
	GetResetTime(context.Context) (time.Time, error)
}

type GitHubRateLimitSource interface {
	RateLimitStatus() (gh.RateLimitStatus, error)
}

type GitHubRateLimitProvider struct {
	Source GitHubRateLimitSource
}

func (p GitHubRateLimitProvider) GetRateLimit(context.Context) (int, error) {
	if p.Source == nil {
		return 0, fmt.Errorf("rate limit source is required")
	}
	status, err := p.Source.RateLimitStatus()
	if err != nil {
		return 0, err
	}
	return status.Remaining, nil
}

func (p GitHubRateLimitProvider) GetResetTime(context.Context) (time.Time, error) {
	if p.Source == nil {
		return time.Time{}, fmt.Errorf("rate limit source is required")
	}
	status, err := p.Source.RateLimitStatus()
	if err != nil {
		return time.Time{}, err
	}
	return status.ResetAt, nil
}

type QueueStore interface {
	Enqueue(repo string) (cache.SyncJob, error)
	Dequeue(repo string) (cache.SyncJob, bool, error)
	Peek(repo string) (cache.SyncJob, bool, error)
	MarkComplete(jobID string, syncedAt time.Time) error
	MarkFailed(jobID string, message string) error
}

type CacheQueueStore struct {
	store *cache.Store
}

func NewCacheQueueStore(store *cache.Store) CacheQueueStore {
	return CacheQueueStore{store: store}
}

func (s CacheQueueStore) Enqueue(repo string) (cache.SyncJob, error) {
	if s.store == nil {
		return cache.SyncJob{}, fmt.Errorf("cache store is required")
	}
	return s.store.CreateSyncJob(repo)
}

func (s CacheQueueStore) Dequeue(repo string) (cache.SyncJob, bool, error) {
	if s.store == nil {
		return cache.SyncJob{}, false, fmt.Errorf("cache store is required")
	}
	job, ok, err := s.store.ResumeSyncJob(repo)
	if err != nil || !ok {
		return job, ok, err
	}
	return job, true, nil
}

func (s CacheQueueStore) Peek(repo string) (cache.SyncJob, bool, error) {
	if s.store == nil {
		return cache.SyncJob{}, false, fmt.Errorf("cache store is required")
	}
	job, err := s.store.GetPausedSyncJobByRepo(repo)
	if err != nil {
		return cache.SyncJob{}, false, nil
	}
	return job, true, nil
}

func (s CacheQueueStore) MarkComplete(jobID string, syncedAt time.Time) error {
	if s.store == nil {
		return fmt.Errorf("cache store is required")
	}
	return s.store.MarkSyncJobComplete(jobID, syncedAt)
}

func (s CacheQueueStore) MarkFailed(jobID string, message string) error {
	if s.store == nil {
		return fmt.Errorf("cache store is required")
	}
	return s.store.MarkSyncJobFailed(jobID, message)
}

type QueuePolicy interface {
	NextBatch(ctx context.Context, store QueueStore, provider RateLimitProvider, repo string) ([]cache.SyncJob, error)
	ShouldWait(ctx context.Context, provider RateLimitProvider) (bool, error)
	WaitDuration(ctx context.Context, provider RateLimitProvider) (time.Duration, error)
}

type QueueRunner struct {
	store    QueueStore
	policy   QueuePolicy
	provider RateLimitProvider
	runFunc  func(context.Context, cache.SyncJob) error
}

func NewQueueRunner(store QueueStore, policy QueuePolicy, provider RateLimitProvider, runFunc func(context.Context, cache.SyncJob) error) *QueueRunner {
	return &QueueRunner{store: store, policy: policy, provider: provider, runFunc: runFunc}
}

func (r *QueueRunner) Run(ctx context.Context, repo string) error {
	if r == nil {
		return fmt.Errorf("queue runner is required")
	}
	if r.store == nil {
		return fmt.Errorf("queue store is required")
	}
	if r.policy == nil {
		return fmt.Errorf("queue policy is required")
	}
	if r.provider == nil {
		return fmt.Errorf("rate limit provider is required")
	}

	shouldWait, err := r.policy.ShouldWait(ctx, r.provider)
	if err != nil {
		return err
	}
	if shouldWait {
		wait, err := r.policy.WaitDuration(ctx, r.provider)
		if err != nil {
			return err
		}
		if wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}

	batch, err := r.policy.NextBatch(ctx, r.store, r.provider, repo)
	if err != nil {
		return err
	}
	for _, job := range batch {
		if r.runFunc == nil {
			if err := r.store.MarkComplete(job.ID, time.Now().UTC()); err != nil {
				return err
			}
			continue
		}
		if err := r.runFunc(ctx, job); err != nil {
			if markErr := r.store.MarkFailed(job.ID, err.Error()); markErr != nil {
				return fmt.Errorf("mark failed: %w", markErr)
			}
			return err
		}
		if err := r.store.MarkComplete(job.ID, time.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

type SingleRepoQueuePolicy struct {
	reserveBuffer int
	resetBuffer   time.Duration
	now           func() time.Time
}

type QueuePolicyOption func(*SingleRepoQueuePolicy)

func WithQueueReserveBuffer(buffer int) QueuePolicyOption {
	return func(p *SingleRepoQueuePolicy) {
		p.reserveBuffer = buffer
	}
}

func WithReserveBuffer(buffer int) QueuePolicyOption {
	return WithQueueReserveBuffer(buffer)
}

func WithQueueResetBuffer(d time.Duration) QueuePolicyOption {
	return func(p *SingleRepoQueuePolicy) {
		p.resetBuffer = d
	}
}

func WithResetBuffer(d time.Duration) QueuePolicyOption {
	return WithQueueResetBuffer(d)
}

func WithQueueNow(now func() time.Time) QueuePolicyOption {
	return func(p *SingleRepoQueuePolicy) {
		p.now = now
	}
}

func NewSingleRepoQueuePolicy(opts ...QueuePolicyOption) *SingleRepoQueuePolicy {
	policy := &SingleRepoQueuePolicy{
		reserveBuffer: 200,
		resetBuffer:   15 * time.Second,
		now:           func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(policy)
	}
	return policy
}

func (p *SingleRepoQueuePolicy) ShouldWait(ctx context.Context, provider RateLimitProvider) (bool, error) {
	remaining, err := provider.GetRateLimit(ctx)
	if err != nil {
		return false, err
	}
	return remaining <= p.reserveBuffer, nil
}

func (p *SingleRepoQueuePolicy) WaitDuration(ctx context.Context, provider RateLimitProvider) (time.Duration, error) {
	resetAt, err := provider.GetResetTime(ctx)
	if err != nil {
		return 0, err
	}
	now := p.now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	wait := resetAt.Sub(now())
	if wait < 0 {
		wait = 0
	}
	return wait + p.resetBuffer, nil
}

func (p *SingleRepoQueuePolicy) NextBatch(ctx context.Context, store QueueStore, provider RateLimitProvider, repo string) ([]cache.SyncJob, error) {
	shouldWait, err := p.ShouldWait(ctx, provider)
	if err != nil {
		return nil, err
	}
	if shouldWait {
		return nil, nil
	}
	dequeued, dequeuedOK, err := store.Dequeue(repo)
	if err != nil {
		return nil, err
	}
	if dequeuedOK {
		return []cache.SyncJob{dequeued}, nil
	}
	job, ok, err := store.Peek(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return []cache.SyncJob{job}, nil
}

// PRQueueItem represents a single PR in the sync queue
type PRQueueItem struct {
	PRNumber    int
	Priority    int       // Higher = process first
	Attempts    int
	LastError   error
	NextAttempt time.Time // When this item is eligible for processing
	ETag        string    // For conditional requests
}

// PRQueue is a thread-safe priority queue with rate limit awareness
type PRQueue struct {
	items       []PRQueueItem
	minBackoff  time.Duration
	store       *cache.Store
	mu          sync.Mutex
}

// NewPRQueue creates a new PR queue
func NewPRQueue(store *cache.Store, minBackoff time.Duration) *PRQueue {
	if minBackoff <= 0 {
		minBackoff = 5 * time.Minute
	}
	return &PRQueue{
		items:      make([]PRQueueItem, 0),
		minBackoff: minBackoff,
		store:      store,
	}
}

// Enqueue adds a PR to the queue
func (q *PRQueue) Enqueue(item PRQueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Insert in priority order (higher priority first)
	insertIdx := len(q.items)
	for i, existing := range q.items {
		if item.Priority > existing.Priority {
			insertIdx = i
			break
		}
	}

	// Insert at the found position
	q.items = append(q.items, PRQueueItem{})
	copy(q.items[insertIdx+1:], q.items[insertIdx:])
	q.items[insertIdx] = item
}

// Dequeue removes and returns the highest priority item that's ready to process
func (q *PRQueue) Dequeue() (PRQueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for i, item := range q.items {
		if now.After(item.NextAttempt) || now.Equal(item.NextAttempt) {
			// Remove this item from queue
			q.items = append(q.items[:i], q.items[i+1:]...)
			return item, true
		}
	}

	return PRQueueItem{}, false
}

// Process processes all items in the queue with the given processor function
func (q *PRQueue) Process(ctx context.Context, processor func(item PRQueueItem) error) error {
	for {
		item, ok := q.Dequeue()
		if !ok {
			// No items ready
			break
		}

		err := processor(item)
		if err != nil {
			// Check if it's a rate limit error
			if isRateLimitError(err) {
				// Re-queue with 5-minute backoff
				item.Attempts++
				item.LastError = err
				item.NextAttempt = time.Now().Add(q.minBackoff)
				q.Enqueue(item)
			} else {
				// Re-queue with exponential backoff
				item.Attempts++
				item.LastError = err
				backoff := time.Duration(1<<uint(item.Attempts)) * time.Minute
				if backoff > 30*time.Minute {
					backoff = 30 * time.Minute
				}
				item.NextAttempt = time.Now().Add(backoff)
				q.Enqueue(item)
			}
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// Len returns the number of items in the queue
func (q *PRQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Helper function to detect rate limit errors
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "rate limit") || contains(errStr, "403") || contains(errStr, "429")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
