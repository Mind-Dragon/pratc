// Package data provides data models and storage abstractions for the monitor package.
package data

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type subscriber struct {
	ch     chan DataUpdate
	closed atomic.Bool
}

type Broadcaster struct {
	mu               sync.RWMutex
	subscribers      map[chan DataUpdate]*subscriber
	store            *Store
	rateLimitFetcher *RateLimitFetcher
	timelineAgg      *TimelineAggregator
	stopCh           chan struct{}
	done             chan struct{}
	running          bool
	inFlight         sync.WaitGroup
	actionPlan       *types.ActionPlan
	actionPlanHash   int
}

func NewBroadcaster(store *Store, rateLimitFetcher *RateLimitFetcher, timelineAgg *TimelineAggregator) *Broadcaster {
	return &Broadcaster{
		subscribers:      make(map[chan DataUpdate]*subscriber),
		store:            store,
		rateLimitFetcher: rateLimitFetcher,
		timelineAgg:      timelineAgg,
	}
}

// SetActionPlan stores the provided ActionPlan and broadcasts it to subscribers.
// If the plan is unchanged (by hash), no broadcast occurs.
func (b *Broadcaster) SetActionPlan(plan *types.ActionPlan) {
	b.mu.Lock()
	defer b.mu.Unlock()
	hash := hashActionPlan(plan)
	if b.actionPlanHash == hash && b.actionPlan != nil && plan != nil {
		return
	}
	b.actionPlan = plan
	b.actionPlanHash = hash
	// Broadcast immediately
	go b.broadcast(DataUpdate{
		Timestamp:  time.Now(),
		ActionPlan: plan,
	})
}

func (b *Broadcaster) Subscribe() chan DataUpdate {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan DataUpdate, 64)
	sub := &subscriber{ch: ch}
	b.subscribers[ch] = sub
	return ch
}

func (b *Broadcaster) Unsubscribe(ch chan DataUpdate) {
	b.mu.Lock()
	sub, ok := b.subscribers[ch]
	if ok {
		delete(b.subscribers, ch)
		sub.closed.Store(true)
	}
	b.mu.Unlock()

	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func (b *Broadcaster) Start(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return
	}
	b.running = true
	b.stopCh = make(chan struct{})
	b.done = make(chan struct{})
	stopCh := b.stopCh
	done := b.done

	go b.run(ctx, stopCh, done)
}

func (b *Broadcaster) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.running = false
	stopCh := b.stopCh
	done := b.done
	subs := b.subscribers
	b.subscribers = make(map[chan DataUpdate]*subscriber)
	b.mu.Unlock()

	if stopCh != nil {
		close(stopCh)
	}
	if done != nil {
		<-done
	}

	b.inFlight.Wait()

	for _, sub := range subs {
		close(sub.ch)
	}
}

func (b *Broadcaster) run(ctx context.Context, stopCh chan struct{}, done chan struct{}) {
	defer close(done)

	var lastJobsHash int
	var lastRateLimitHash int
	var lastTimelineHash int
	var lastActionPlanHash int

	jobsTicker := time.NewTicker(2 * time.Second)
	rateLimitTicker := time.NewTicker(10 * time.Second)
	timelineTicker := time.NewTicker(30 * time.Second)
	actionPlanTicker := time.NewTicker(30 * time.Second)

	defer func() {
		jobsTicker.Stop()
		rateLimitTicker.Stop()
		timelineTicker.Stop()
		actionPlanTicker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-jobsTicker.C:
			b.pollAndBroadcastJobs(ctx, &lastJobsHash)
		case <-rateLimitTicker.C:
			b.pollAndBroadcastRateLimit(ctx, &lastRateLimitHash)
		case <-timelineTicker.C:
			b.pollAndBroadcastTimeline(ctx, &lastTimelineHash)
		case <-actionPlanTicker.C:
			b.pollAndBroadcastActionPlan(ctx, &lastActionPlanHash)
		}
	}
}

func (b *Broadcaster) pollAndBroadcastJobs(ctx context.Context, lastHash *int) {
	jobs := b.store.GetActiveJobs()
	hash := hashJobs(jobs)
	if hash != *lastHash {
		*lastHash = hash
		b.broadcast(DataUpdate{
			Timestamp: time.Now(),
			SyncJobs:  jobs,
		})
	}
}

func (b *Broadcaster) pollAndBroadcastRateLimit(ctx context.Context, lastHash *int) {
	rl, err := b.rateLimitFetcher.Fetch(ctx)
	if err != nil {
		return
	}
	hash := hashRateLimit(rl)
	if hash != *lastHash {
		*lastHash = hash
		update := DataUpdate{
			Timestamp: time.Now(),
			RateLimit: rl,
		}
		b.mu.RLock()
		hasExisting := len(b.subscribers) > 0
		b.mu.RUnlock()
		if hasExisting {
			b.broadcast(update)
		}
	}
}

func (b *Broadcaster) pollAndBroadcastTimeline(ctx context.Context, lastHash *int) {
	buckets := b.timelineAgg.GetTimeline(4)
	hash := hashTimeline(buckets)
	if hash != *lastHash {
		*lastHash = hash
		update := DataUpdate{
			Timestamp:       time.Now(),
			ActivityBuckets: buckets,
		}
		b.broadcast(update)
	}
}

func (b *Broadcaster) pollAndBroadcastActionPlan(ctx context.Context, lastHash *int) {
	b.mu.RLock()
	plan := b.actionPlan
	hash := b.actionPlanHash
	b.mu.RUnlock()
	if plan == nil {
		return
	}
	if hash != *lastHash {
		*lastHash = hash
		b.broadcast(DataUpdate{
			Timestamp:  time.Now(),
			ActionPlan: plan,
		})
	}
}

func (b *Broadcaster) broadcast(update DataUpdate) {
	b.mu.Lock()
	subs := make([]*subscriber, 0, len(b.subscribers))
	for _, sub := range b.subscribers {
		subs = append(subs, sub)
	}
	b.mu.Unlock()

	b.inFlight.Add(1)
	defer b.inFlight.Done()

	for _, sub := range subs {
		if sub.closed.Load() {
			continue
		}
		select {
		case sub.ch <- update:
		default:
		}
	}
}

func hashJobs(jobs []SyncJobView) int {
	h := 0
	for _, job := range jobs {
		h ^= hashStrings(job.ID, job.Repo, job.Status)
	}
	return h
}

func hashRateLimit(rl RateLimitView) int {
	return rl.Remaining ^ rl.Total ^ int(rl.ResetTime.Unix())
}

func hashTimeline(buckets []ActivityBucket) int {
	h := 0
	for _, b := range buckets {
		h ^= int(b.TimeWindow.Unix()) ^ b.JobCount ^ b.RequestCount
	}
	return h
}

func hashStrings(strs ...string) int {
	h := 0
	for _, s := range strs {
		for _, c := range s {
			h ^= int(c)
		}
	}
	return h
}

func hashActionPlan(plan *types.ActionPlan) int {
	if plan == nil {
		return 0
	}
	h := hashStrings(plan.RunID, plan.Repo, string(plan.PolicyProfile))
	for _, lane := range plan.Lanes {
		h ^= hashStrings(string(lane.Lane))
		h ^= lane.Count
	}
	for _, wi := range plan.WorkItems {
		h ^= hashStrings(wi.ID, string(wi.Lane), string(wi.State))
		h ^= int(wi.PRNumber)
	}
	return h
}
