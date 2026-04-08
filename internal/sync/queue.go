package sync

import (
	"context"
	"fmt"
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
