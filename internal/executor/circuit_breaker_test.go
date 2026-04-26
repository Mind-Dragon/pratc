package executor

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

func TestCircuitBreakerEnforcesPerRepoAndGlobalLimits(t *testing.T) {
	breaker := NewMutationCircuitBreaker(CircuitBreakerConfig{MaxGlobal: 2, MaxPerRepo: 1})

	releaseA, err := breaker.Acquire("owner/repo")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer releaseA()

	if _, err := breaker.Acquire("owner/repo"); err == nil {
		t.Fatal("expected per-repo denial for second same-repo live mutation")
	}

	releaseB, err := breaker.Acquire("owner/other")
	if err != nil {
		t.Fatalf("second repo acquire: %v", err)
	}
	defer releaseB()

	if _, err := breaker.Acquire("owner/third"); err == nil {
		t.Fatal("expected global denial once max global live mutations are in flight")
	}

	status := breaker.Status()
	if status.InFlightGlobal != 2 || status.InFlightByRepo["owner/repo"] != 1 || status.InFlightByRepo["owner/other"] != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestWorkerCircuitBreakerDenialFailsItemAndRecordsLedger(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-denied", PRNumber: 201, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, PriorityScore: 0.8, Confidence: 0.9, ReasonTrail: []string{"comment"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindComment}, IdempotencyKey: "owner/repo#201:work_item"}
	intent := types.ActionIntent{ID: "intent-denied-comment", WorkItemID: item.ID, Action: types.ActionKindComment, PRNumber: 201, Lane: types.ActionLaneFocusedReview, DryRun: false, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"denied comment"}, EvidenceRefs: []string{"fixture"}, IdempotencyKey: "owner/repo#201:comment", CreatedAt: "2026-04-26T10:00:00Z"}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	breaker := NewMutationCircuitBreaker(CircuitBreakerConfig{MaxGlobal: 1, MaxPerRepo: 1})
	release, err := breaker.Acquire("owner/repo")
	if err != nil {
		t.Fatalf("pre-acquire breaker slot: %v", err)
	}
	defer release()

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 201, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-test", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: true, CircuitBreaker: breaker}, q, fake, ledger)

	processed, err := worker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected item to be processed")
	}
	if fake.WriteCount() != 0 {
		t.Fatalf("write count = %d, want 0", fake.WriteCount())
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateFailed || finished.LeaseState.ClaimedBy != "" {
		t.Fatalf("finished = %+v, want failed with cleared lease", finished)
	}
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("ledger history: %v", err)
	}
	if !hasLedgerTransition(history, "circuit_denied") {
		t.Fatalf("ledger history missing circuit_denied: %+v", history)
	}
}

func TestWorkerDryRunIntentBypassesCircuitBreaker(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-dry-breaker", PRNumber: 202, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, PriorityScore: 0.8, Confidence: 0.9, ReasonTrail: []string{"dry"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindComment}, IdempotencyKey: "owner/repo#202:work_item"}
	intent := types.ActionIntent{ID: "intent-dry-comment", WorkItemID: item.ID, Action: types.ActionKindComment, PRNumber: 202, Lane: types.ActionLaneFocusedReview, DryRun: true, PolicyProfile: types.PolicyProfileAdvisory, Reasons: []string{"dry comment"}, EvidenceRefs: []string{"fixture"}, IdempotencyKey: "owner/repo#202:comment", CreatedAt: "2026-04-26T10:00:00Z"}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	breaker := NewMutationCircuitBreaker(CircuitBreakerConfig{MaxGlobal: 1, MaxPerRepo: 1})
	release, err := breaker.Acquire("owner/repo")
	if err != nil {
		t.Fatalf("pre-acquire breaker slot: %v", err)
	}
	defer release()

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 202, State: "open", Mergeable: true, HeadSHA: "abc"})
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-test", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: true, CircuitBreaker: breaker}, q, fake, NewMemoryLedger())

	processed, err := worker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected item to be processed")
	}
	if fake.WriteCount() != 0 {
		t.Fatalf("write count = %d, want 0", fake.WriteCount())
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateVerified {
		t.Fatalf("state = %s, want verified", finished.State)
	}
}

func hasLedgerTransition(history []types.TransitionRecord, name string) bool {
	for _, record := range history {
		if record.Transition == name {
			return true
		}
	}
	return false
}
