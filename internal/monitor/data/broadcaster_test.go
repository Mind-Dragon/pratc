package data

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

type mockStore struct {
	activeJobs []SyncJobView
}

func (m *mockStore) GetActiveJobs() []SyncJobView {
	return m.activeJobs
}

type mockRateLimitFetcher struct {
	view RateLimitView
	err  error
}

func (m *mockRateLimitFetcher) Fetch(ctx context.Context) (RateLimitView, error) {
	return m.view, m.err
}

type mockTimelineAggregator struct {
	buckets []ActivityBucket
}

func (m *mockTimelineAggregator) GetTimeline(hours int) []ActivityBucket {
	return m.buckets
}

func TestBroadcasterSubscribe(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()

	b.mu.Lock()
	_, ok := b.subscribers[ch]
	b.mu.Unlock()

	if !ok {
		t.Error("Subscribe() should register channel in subscribers map")
	}
}

func TestBroadcasterUnsubscribe(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()

	b.Unsubscribe(ch)

	b.mu.Lock()
	_, ok := b.subscribers[ch]
	b.mu.Unlock()

	if ok {
		t.Error("Unsubscribe() should remove channel from subscribers map")
	}
}

func TestBroadcasterUnsubscribeNotSubscribed(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := make(chan DataUpdate)

	b.Unsubscribe(ch)
}

func TestBroadcasterMultipleSubscribers(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	ch3 := b.Subscribe()

	b.mu.Lock()
	count := len(b.subscribers)
	b.mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 subscribers, got %d", count)
	}

	b.Unsubscribe(ch1)
	b.Unsubscribe(ch2)
	b.Unsubscribe(ch3)

	b.mu.Lock()
	count = len(b.subscribers)
	b.mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribes, got %d", count)
	}
}

func TestBroadcasterStartStop(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()

	b.Start(ctx)

	b.mu.Lock()
	running := b.running
	b.mu.Unlock()

	if !running {
		t.Error("Start() should set running to true")
	}

	b.Stop()

	b.mu.Lock()
	running = b.running
	b.mu.Unlock()

	if running {
		t.Error("Stop() should set running to false")
	}
}

func TestBroadcasterStartTwice(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()

	b.Start(ctx)

	b.mu.Lock()
	running1 := b.running
	b.mu.Unlock()

	b.Start(ctx)

	b.mu.Lock()
	running2 := b.running
	b.mu.Unlock()

	if !running1 || !running2 {
		t.Error("broadcaster should be running after Start()")
	}

	b.Stop()
}

func TestBroadcasterBroadcast(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()
	_ = ch // channel registered via subscribe

	update := DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []SyncJobView{{ID: "job1", Repo: "owner/repo", Status: StatusActive}},
	}

	b.broadcast(update)

	select {
	case received := <-ch:
		if received.SyncJobs[0].ID != "job1" {
			t.Errorf("expected job1, got %s", received.SyncJobs[0].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("broadcast should have sent update to channel")
	}
}

func TestBroadcasterMultipleSubscribersReceiveSameData(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()
	ch3 := b.Subscribe()

	update := DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []SyncJobView{{ID: "job1", Repo: "owner/repo", Status: StatusActive}},
	}

	b.broadcast(update)

	for i, ch := range []chan DataUpdate{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.SyncJobs[0].ID != "job1" {
				t.Errorf("subscriber %d: expected job1, got %s", i, received.SyncJobs[0].ID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d should have received update", i)
		}
	}
}

func TestBroadcasterStopClosesAllChannels(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	ctx := context.Background()
	b.Start(ctx)
	b.Stop()

	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("channel 1 should be closed after Stop()")
		}
	default:
		t.Error("channel 1 should be closed and not hit default")
	}

	select {
	case _, ok := <-ch2:
		if ok {
			t.Error("channel 2 should be closed after Stop()")
		}
	default:
		t.Error("channel 2 should be closed and not hit default")
	}
}

func TestBroadcasterUnsubscribeRemovesChannel(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	b.Unsubscribe(ch1)

	b.mu.Lock()
	_, ch1StillSubscribed := b.subscribers[ch1]
	_, ch2StillSubscribed := b.subscribers[ch2]
	b.mu.Unlock()

	if ch1StillSubscribed {
		t.Error("ch1 should not be in subscribers after Unsubscribe")
	}

	if !ch2StillSubscribed {
		t.Error("ch2 should still be in subscribers")
	}

	update := DataUpdate{
		Timestamp: time.Now(),
		SyncJobs:  []SyncJobView{{ID: "job1"}},
	}

	b.broadcast(update)

	select {
	case <-ch1:
		t.Error("unsubscribed channel should not receive update")
	default:
	}

	select {
	case received := <-ch2:
		if received.SyncJobs[0].ID != "job1" {
			t.Errorf("expected job1, got %s", received.SyncJobs[0].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("active channel should receive update")
	}
}

func TestBroadcasterConcurrentAccess(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()
	b.Start(ctx)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := b.Subscribe()
			time.Sleep(10 * time.Millisecond)
			b.Unsubscribe(ch)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			update := DataUpdate{Timestamp: time.Now()}
			b.broadcast(update)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()

	b.Stop()
}

func TestHashJobs(t *testing.T) {
	jobs1 := []SyncJobView{
		{ID: "job1", Repo: "owner/repo", Status: StatusActive},
		{ID: "job2", Repo: "owner/repo2", Status: StatusPaused},
	}
	jobs2 := []SyncJobView{
		{ID: "job1", Repo: "owner/repo", Status: StatusActive},
		{ID: "job2", Repo: "owner/repo2", Status: StatusPaused},
	}
	jobs3 := []SyncJobView{
		{ID: "job1", Repo: "owner/repo", Status: StatusFailed},
		{ID: "job2", Repo: "owner/repo2", Status: StatusPaused},
	}

	hash1 := hashJobs(jobs1)
	hash2 := hashJobs(jobs2)
	hash3 := hashJobs(jobs3)

	if hash1 != hash2 {
		t.Error("expected same hash for identical jobs")
	}

	if hash1 == hash3 {
		t.Error("expected different hash for different jobs")
	}
}

func TestHashRateLimit(t *testing.T) {
	now := time.Now()

	rl1 := RateLimitView{Remaining: 100, Total: 5000, ResetTime: now}
	rl2 := RateLimitView{Remaining: 100, Total: 5000, ResetTime: now}
	rl3 := RateLimitView{Remaining: 200, Total: 5000, ResetTime: now}

	hash1 := hashRateLimit(rl1)
	hash2 := hashRateLimit(rl2)
	hash3 := hashRateLimit(rl3)

	if hash1 != hash2 {
		t.Error("expected same hash for identical rate limits")
	}

	if hash1 == hash3 {
		t.Error("expected different hash for different rate limits")
	}
}

func TestHashTimeline(t *testing.T) {
	now := time.Now()

	buckets1 := []ActivityBucket{
		{TimeWindow: now, JobCount: 5, RequestCount: 100},
		{TimeWindow: now.Add(15 * time.Minute), JobCount: 3, RequestCount: 50},
	}
	buckets2 := []ActivityBucket{
		{TimeWindow: now, JobCount: 5, RequestCount: 100},
		{TimeWindow: now.Add(15 * time.Minute), JobCount: 3, RequestCount: 50},
	}
	buckets3 := []ActivityBucket{
		{TimeWindow: now, JobCount: 5, RequestCount: 100},
		{TimeWindow: now.Add(15 * time.Minute), JobCount: 10, RequestCount: 50},
	}

	hash1 := hashTimeline(buckets1)
	hash2 := hashTimeline(buckets2)
	hash3 := hashTimeline(buckets3)

	if hash1 != hash2 {
		t.Error("expected same hash for identical timelines")
	}

	if hash1 == hash3 {
		t.Error("expected different hash for different timelines")
	}
}

func TestHashStrings(t *testing.T) {
	hash1 := hashStrings("a", "b", "c")
	hash2 := hashStrings("a", "b", "c")
	hash3 := hashStrings("a", "b", "d")

	if hash1 != hash2 {
		t.Error("expected same hash for identical strings")
	}

	if hash1 == hash3 {
		t.Error("expected different hash for different strings")
	}
}

func TestBroadcasterStopWhenNotRunning(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)

	b.Stop()

	b.mu.Lock()
	running := b.running
	b.mu.Unlock()

	if running {
		t.Error("expected running to be false after Stop on non-running broadcaster")
	}
}

func TestBroadcasterPollAndBroadcastJobs(t *testing.T) {
	cacheStore := newTestCacheStoreForBroadcaster(t)
	store := NewStore(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 10,
		TotalPRs:     20,
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	b := NewBroadcaster(store, nil, nil)
	ch := b.Subscribe()

	var lastHash int
	b.pollAndBroadcastJobs(context.Background(), &lastHash)

	select {
	case update := <-ch:
		if len(update.SyncJobs) != 1 {
			t.Errorf("expected 1 job, got %d", len(update.SyncJobs))
		}
		if update.SyncJobs[0].ID != job.ID {
			t.Errorf("expected job %s, got %s", job.ID, update.SyncJobs[0].ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected update to be broadcast")
	}
}

func TestBroadcasterPollAndBroadcastJobsNoChange(t *testing.T) {
	cacheStore := newTestCacheStoreForBroadcaster(t)
	store := NewStore(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 10,
		TotalPRs:     20,
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	b := NewBroadcaster(store, nil, nil)
	ch := b.Subscribe()

	var lastHash int
	b.pollAndBroadcastJobs(context.Background(), &lastHash)

	select {
	case <-ch:
	case <-time.After(50 * time.Millisecond):
		t.Error("expected first poll to broadcast")
	}

	b.pollAndBroadcastJobs(context.Background(), &lastHash)

	select {
	case <-ch:
		t.Error("expected no broadcast when hash unchanged")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcasterPollAndBroadcastRateLimit(t *testing.T) {
	fetcher := NewRateLimitFetcher("")
	fetcher.cached = &RateLimitView{
		Remaining: 4000,
		Total:     5000,
		ResetTime: time.Now().Add(1 * time.Hour),
	}
	fetcher.cacheUntil = time.Now().Add(1 * time.Hour)

	b := NewBroadcaster(nil, fetcher, nil)
	ch := b.Subscribe()

	var lastHash int
	b.pollAndBroadcastRateLimit(context.Background(), &lastHash)

	select {
	case update := <-ch:
		if update.RateLimit.Remaining != 4000 {
			t.Errorf("expected remaining 4000, got %d", update.RateLimit.Remaining)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected rate limit update to be broadcast")
	}
}

func TestBroadcasterPollAndBroadcastRateLimitError(t *testing.T) {
	fetcher := NewRateLimitFetcher("")
	fetcher.httpClient = &mockHTTPClient{
		doErr: errors.New("fetch error"),
	}

	b := NewBroadcaster(nil, fetcher, nil)
	ch := b.Subscribe()

	var lastHash int
	b.pollAndBroadcastRateLimit(context.Background(), &lastHash)

	select {
	case <-ch:
		t.Error("expected no broadcast on error")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestBroadcasterPollAndBroadcastTimeline(t *testing.T) {
	cacheStore := newTestCacheStoreForBroadcaster(t)
	timelineAgg := NewTimelineAggregator(cacheStore)

	b := NewBroadcaster(nil, nil, timelineAgg)
	ch := b.Subscribe()

	var lastHash int
	b.pollAndBroadcastTimeline(context.Background(), &lastHash)

	select {
	case update := <-ch:
		if len(update.ActivityBuckets) == 0 {
			t.Error("expected at least one bucket in timeline update")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected timeline update to be broadcast")
	}
}

func TestBroadcasterContextCancellation(t *testing.T) {
	cacheStore := newTestCacheStoreForBroadcaster(t)
	store := NewStore(cacheStore)

	b := NewBroadcaster(store, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())

	b.Start(ctx)

	b.mu.Lock()
	running := b.running
	doneCh := b.done
	b.mu.Unlock()

	if !running {
		t.Error("expected broadcaster to be running")
	}

	cancel()

	select {
	case <-doneCh:
	case <-time.After(500 * time.Millisecond):
		t.Error("expected broadcaster run loop to exit after context cancellation")
	}
}

func TestBroadcasterBroadcastToClosedSubscriber(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()

	b.Unsubscribe(ch)

	update := DataUpdate{Timestamp: time.Now()}

	done := make(chan struct{})
	go func() {
		b.broadcast(update)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("expected broadcast to complete even with closed subscriber")
	}
}

func TestBroadcasterMultipleStartCalls(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ctx := context.Background()

	b.Start(ctx)
	b.Start(ctx)
	b.Start(ctx)

	b.mu.Lock()
	running := b.running
	b.mu.Unlock()

	if !running {
		t.Error("expected broadcaster to be running")
	}

	b.Stop()
}

func TestBroadcasterBroadcastActionPlan(t *testing.T) {
	b := NewBroadcaster(nil, nil, nil)
	ch := b.Subscribe()

	plan := &types.ActionPlan{
		RunID:         "run-123",
		Repo:          "owner/repo",
		PolicyProfile: types.PolicyProfileAdvisory,
		Lanes: []types.ActionLaneSummary{
			{Lane: types.ActionLaneFastMerge, Count: 1},
		},
		WorkItems: []types.ActionWorkItem{
			{ID: "wi-1", PRNumber: 42, Lane: types.ActionLaneFastMerge, State: types.ActionWorkItemStateProposed},
		},
	}

	b.SetActionPlan(plan)

	select {
	case update := <-ch:
		if update.ActionPlan == nil {
			t.Error("expected ActionPlan in update")
		}
		if update.ActionPlan.RunID != plan.RunID {
			t.Errorf("expected RunID %s, got %s", plan.RunID, update.ActionPlan.RunID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("expected ActionPlan update to be broadcast")
	}
}

func newTestCacheStoreForBroadcaster(t *testing.T) *cache.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test-broadcaster.db")
	cacheStore, err := cache.Open(path)
	if err != nil {
		t.Fatalf("open cache store: %v", err)
	}
	t.Cleanup(func() {
		cacheStore.Close()
	})

	return cacheStore
}
