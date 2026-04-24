// Package data provides data models and storage abstractions for the monitor package.
package data

import (
	"context"
	"strconv"
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
	actionPlanHash   uint64
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

	var lastJobsHash uint64
	var lastRateLimitHash uint64
	var lastTimelineHash uint64
	var lastActionPlanHash uint64
	var lastExecutorHash uint64

	jobsTicker := time.NewTicker(2 * time.Second)
	rateLimitTicker := time.NewTicker(10 * time.Second)
	timelineTicker := time.NewTicker(30 * time.Second)
	actionPlanTicker := time.NewTicker(30 * time.Second)
	executorTicker := time.NewTicker(10 * time.Second)

	defer func() {
		jobsTicker.Stop()
		rateLimitTicker.Stop()
		timelineTicker.Stop()
		actionPlanTicker.Stop()
		executorTicker.Stop()
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
		case <-executorTicker.C:
			b.pollAndBroadcastExecutorStats(ctx, &lastExecutorHash)
		}
	}
}

func (b *Broadcaster) pollAndBroadcastJobs(ctx context.Context, lastHash *uint64) {
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

func (b *Broadcaster) pollAndBroadcastRateLimit(ctx context.Context, lastHash *uint64) {
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

func (b *Broadcaster) pollAndBroadcastTimeline(ctx context.Context, lastHash *uint64) {
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

func (b *Broadcaster) pollAndBroadcastActionPlan(ctx context.Context, lastHash *uint64) {
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

func (b *Broadcaster) pollAndBroadcastExecutorStats(ctx context.Context, lastHash *uint64) {
	wq := b.store.GetWorkqueue()
	if wq == nil {
		return // no workqueue configured
	}

	// Build ExecutorState
	summary, err := wq.GetQueueSummary()
	if err != nil {
		return
	}
	ledger, err := wq.GetExecutorLedger(50)
	if err != nil {
		return
	}

	state := ExecutorState{
		PendingIntents:    summary.ByState["proposed"] + summary.ByState["claimable"],
		ClaimedItems:      summary.ByState["claimed"],
		InProgressItems:  summary.ByState["preflighted"] + summary.ByState["patched"] + summary.ByState["tested"] + summary.ByState["approved_for_execution"],
		CompletedItems:   summary.ByState["completed"],
		FailedItems:      summary.ByState["failed"] + summary.ByState["escalated"] + summary.ByState["canceled"],
		ProofBundleCount: len(ledger.ProofBundles),
	}

	// Also get corpus stats from cache
	totalPRs, lastSync, syncJobs, auditEntries, err := b.store.GetCorpusStats()
	if err != nil {
		return
	}
	corpus := CorpusStats{
		TotalPRs:       totalPRs,
		LastSync:       lastSync,
		SyncJobsActive: syncJobs,
		AuditEntries:   auditEntries,
	}

	// Also get recent audit ledger from cache
	auditEntriesList, err := b.store.GetAuditLedger(20)
	if err != nil {
		return
	}
	auditLedger := AuditLedger{
		Entries: make([]AuditLedgerEntry, len(auditEntriesList)),
	}
	for i, e := range auditEntriesList {
		auditLedger.Entries[i] = AuditLedgerEntry{
			Timestamp: e.Timestamp,
			Action:    e.Action,
			WorkItemID: "",
			PRNumber:  0,
			Reason:    e.Reason,
			Actor:     e.Actor,
		}
	}

	// Also get proof bundles from workqueue
	proofBundles, err := b.store.GetRecentProofBundles(10)
	if err != nil {
		return
	}

	// Combined hash for state + corpus + audit + proof
	h := hashExecutorState(state)
	h = hashCorpusStats(h, corpus)
	h = hashAuditLedger(h, auditLedger)
	h = hashProofBundles(h, proofBundles)

	if h != *lastHash {
		*lastHash = h
		update := DataUpdate{
			Timestamp:      time.Now(),
			ExecutorState:  state,
			CorpusStats:    corpus,
			AuditLedger:    auditLedger,
			ProofBundles:   proofBundles,
		}
		// Keep existing subscribers warm - merge with any existing update pattern
		b.mu.RLock()
		hasExisting := len(b.subscribers) > 0
		b.mu.RUnlock()
		if hasExisting {
			b.broadcast(update)
		}
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

func hashJobs(jobs []SyncJobView) uint64 {
	var h uint64
	for _, job := range jobs {
		h ^= hashStrings(job.ID, job.Repo, job.Status)
	}
	return h
}

func hashRateLimit(rl RateLimitView) uint64 {
	return uint64(rl.Remaining) ^ uint64(rl.Total) ^ uint64(rl.ResetTime.Unix())
}

func hashTimeline(buckets []ActivityBucket) uint64 {
	var h uint64
	for _, b := range buckets {
		h ^= uint64(b.TimeWindow.Unix()) ^ uint64(b.JobCount) ^ uint64(b.RequestCount)
	}
	return h
}

func hashStrings(strs ...string) uint64 {
	var h uint64
	for _, s := range strs {
		for _, c := range s {
			h ^= uint64(c)
		}
	}
	return h
}

func hashActionPlan(plan *types.ActionPlan) uint64 {
	if plan == nil {
		return 0
	}
	h := hashStrings(plan.RunID, plan.Repo, string(plan.PolicyProfile))
	for _, lane := range plan.Lanes {
		h ^= hashStrings(string(lane.Lane))
		h ^= uint64(lane.Count)
	}
	for _, wi := range plan.WorkItems {
		h ^= hashStrings(wi.ID, string(wi.Lane), string(wi.State))
		h ^= uint64(wi.PRNumber)
	}
	return h
}

// fnv1aInit is the initial hash value for FNV-1a.
const fnv1aInit uint64 = 14695981039346656037

// fnv1aUpdate updates the hash with the given string using FNV-1a algorithm.
func fnv1aUpdate(h uint64, s string) uint64 {
	const fnvPrime uint64 = 1099511628211
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

func hashExecutorState(s ExecutorState) uint64 {
	h := fnv1aInit
	h = fnv1aUpdate(h, strconv.Itoa(s.PendingIntents))
	h = fnv1aUpdate(h, strconv.Itoa(s.ClaimedItems))
	h = fnv1aUpdate(h, strconv.Itoa(s.InProgressItems))
	h = fnv1aUpdate(h, strconv.Itoa(s.CompletedItems))
	h = fnv1aUpdate(h, strconv.Itoa(s.FailedItems))
	h = fnv1aUpdate(h, strconv.Itoa(s.ProofBundleCount))
	return h
}

func hashCorpusStats(h uint64, s CorpusStats) uint64 {
	h = fnv1aUpdate(h, strconv.Itoa(s.TotalPRs))
	h = fnv1aUpdate(h, strconv.FormatInt(s.LastSync.Unix(), 10))
	h = fnv1aUpdate(h, strconv.Itoa(s.SyncJobsActive))
	h = fnv1aUpdate(h, strconv.Itoa(s.AuditEntries))
	return h
}

func hashAuditLedger(h uint64, s AuditLedger) uint64 {
	for _, e := range s.Entries {
		h = fnv1aUpdate(h, e.Action)
		h = fnv1aUpdate(h, e.WorkItemID)
		h = fnv1aUpdate(h, strconv.Itoa(e.PRNumber))
		h = fnv1aUpdate(h, e.Actor)
	}
	return h
}

func hashProofBundles(h uint64, refs []ProofBundleRef) uint64 {
	for _, r := range refs {
		h = fnv1aUpdate(h, r.ID)
		h = fnv1aUpdate(h, r.WorkItemID)
		h = fnv1aUpdate(h, strconv.Itoa(r.PRNumber))
		h = fnv1aUpdate(h, r.Summary)
		h = fnv1aUpdate(h, strconv.FormatInt(r.CreatedAt.Unix(), 10))
	}
	return h
}
