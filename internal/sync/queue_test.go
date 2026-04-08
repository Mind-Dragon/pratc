package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	gh "github.com/jeffersonnunn/pratc/internal/github"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

type mockQueueStore struct {
	peekFn         func(repo string) (cache.SyncJob, bool, error)
	dequeueFn      func(repo string) (cache.SyncJob, bool, error)
	enqueueFn      func(repo string) (cache.SyncJob, error)
	markCompleteFn func(jobID string, syncedAt time.Time) error
	markFailedFn   func(jobID string, message string) error
}

func (m *mockQueueStore) Enqueue(repo string) (cache.SyncJob, error) {
	if m.enqueueFn != nil {
		return m.enqueueFn(repo)
	}
	return cache.SyncJob{}, nil
}

func (m *mockQueueStore) Dequeue(repo string) (cache.SyncJob, bool, error) {
	if m.dequeueFn != nil {
		return m.dequeueFn(repo)
	}
	return cache.SyncJob{}, false, nil
}

func (m *mockQueueStore) Peek(repo string) (cache.SyncJob, bool, error) {
	if m.peekFn != nil {
		return m.peekFn(repo)
	}
	return cache.SyncJob{}, false, nil
}

func (m *mockQueueStore) MarkComplete(jobID string, syncedAt time.Time) error {
	if m.markCompleteFn != nil {
		return m.markCompleteFn(jobID, syncedAt)
	}
	return nil
}

func (m *mockQueueStore) MarkFailed(jobID string, message string) error {
	if m.markFailedFn != nil {
		return m.markFailedFn(jobID, message)
	}
	return nil
}

type mockRateLimitProvider struct {
	remaining int
	resetAt   time.Time
	err       error
}

func (m mockRateLimitProvider) GetRateLimit(context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.remaining, nil
}

func (m mockRateLimitProvider) GetResetTime(context.Context) (time.Time, error) {
	if m.err != nil {
		return time.Time{}, m.err
	}
	return m.resetAt, nil
}

func TestSingleRepoQueuePolicyWaitsUntilReset(t *testing.T) {
	t.Parallel()

	policy := NewSingleRepoQueuePolicy(WithReserveBuffer(200), WithResetBuffer(15*time.Second))
	provider := mockRateLimitProvider{remaining: 150, resetAt: time.Now().Add(30 * time.Minute)}
	store := &mockQueueStore{}

	shouldWait, err := policy.ShouldWait(context.Background(), provider)
	if err != nil {
		t.Fatalf("ShouldWait returned error: %v", err)
	}
	if !shouldWait {
		t.Fatal("expected policy to wait when remaining budget is below reserve")
	}

	wait, err := policy.WaitDuration(context.Background(), provider)
	if err != nil {
		t.Fatalf("WaitDuration returned error: %v", err)
	}
	if wait < 30*time.Minute {
		t.Fatalf("expected wait duration to reach reset window, got %s", wait)
	}

	batch, err := policy.NextBatch(context.Background(), store, provider, "owner/repo")
	if err != nil {
		t.Fatalf("NextBatch returned error: %v", err)
	}
	if len(batch) != 0 {
		t.Fatalf("expected no batch while waiting, got %d items", len(batch))
	}
}

func TestSingleRepoQueuePolicyReturnsQueuedWorkAfterReset(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	policy := NewSingleRepoQueuePolicy(WithReserveBuffer(200), WithResetBuffer(15*time.Second))
	provider := mockRateLimitProvider{remaining: 1000, resetAt: now.Add(-time.Minute)}
	store := &mockQueueStore{peekFn: func(repo string) (cache.SyncJob, bool, error) {
		return cache.SyncJob{ID: "job-1", Repo: repo, Status: cache.SyncJobStatusPaused}, true, nil
	}}

	shouldWait, err := policy.ShouldWait(context.Background(), provider)
	if err != nil {
		t.Fatalf("ShouldWait returned error: %v", err)
	}
	if shouldWait {
		t.Fatal("expected policy not to wait when budget is available")
	}

	batch, err := policy.NextBatch(context.Background(), store, provider, "owner/repo")
	if err != nil {
		t.Fatalf("NextBatch returned error: %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("expected one queued job, got %d", len(batch))
	}
	if batch[0].ID != "job-1" {
		t.Fatalf("expected queued job-1, got %s", batch[0].ID)
	}
}

func TestQueueRunnerMarksFailures(t *testing.T) {
	t.Parallel()

	store := &mockQueueStore{dequeueFn: func(repo string) (cache.SyncJob, bool, error) {
		return cache.SyncJob{ID: "job-1", Repo: repo, Status: cache.SyncJobStatusPaused}, true, nil
	}}
	policy := NewSingleRepoQueuePolicy()
	provider := mockRateLimitProvider{remaining: 1000, resetAt: time.Now().Add(-time.Minute)}
	innerErr := errors.New("boom")
	innerCalled := false
	runner := NewQueueRunner(store, policy, provider, func(ctx context.Context, job cache.SyncJob) error {
		innerCalled = true
		return innerErr
	})

	err := runner.Run(context.Background(), "owner/repo")
	if !errors.Is(err, innerErr) {
		t.Fatalf("expected inner error, got %v", err)
	}
	if !innerCalled {
		t.Fatal("expected inner runner to be called")
	}
}

func TestGitHubRateLimitProviderBridgesClientBudget(t *testing.T) {
	t.Parallel()

	client := gh.NewClient(gh.Config{
		BudgetManager: ratelimit.NewBudgetManager(ratelimit.WithRateLimit(4200)),
	})
	provider := GitHubRateLimitProvider{Source: client}

	remaining, err := provider.GetRateLimit(context.Background())
	if err != nil {
		t.Fatalf("GetRateLimit returned error: %v", err)
	}
	if remaining != 4200 {
		t.Fatalf("expected remaining budget 4200, got %d", remaining)
	}

	resetAt, err := provider.GetResetTime(context.Background())
	if err != nil {
		t.Fatalf("GetResetTime returned error: %v", err)
	}
	if resetAt.IsZero() {
		t.Fatal("expected reset time to be populated")
	}
}
