package executor

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

func TestWorkerDryRunE2EDoesNotWriteAndReachesVerified(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-dry-e2e", PRNumber: 701, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, PriorityScore: 0.7, Confidence: 0.9, ReasonTrail: []string{"comment"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindComment}, IdempotencyKey: "owner/repo#701:work_item"}
	intent := types.ActionIntent{ID: "intent-dry-e2e", WorkItemID: item.ID, Action: types.ActionKindComment, PRNumber: 701, Lane: types.ActionLaneFocusedReview, DryRun: true, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"dry-run comment"}, IdempotencyKey: "owner/repo#701:comment"}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 701, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-dry", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: false}, q, fake, ledger)
	if processed, err := worker.ProcessOnce(ctx); err != nil || !processed {
		t.Fatalf("process once: processed=%t err=%v", processed, err)
	}
	if fake.HasWritten() {
		t.Fatalf("dry-run worker wrote to GitHub: writes=%d log=%+v", fake.WriteCount(), fake.Log())
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateVerified {
		t.Fatalf("state = %s, want verified", finished.State)
	}
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !hasLedgerTransition(history, "executed") {
		t.Fatalf("dry-run missing execution proof transition: %+v", history)
	}
}

func TestWorkerLiveFakeE2EWritesAndVerifies(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-live-e2e", PRNumber: 702, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, PriorityScore: 0.7, Confidence: 0.9, ReasonTrail: []string{"comment"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindComment}, IdempotencyKey: "owner/repo#702:work_item"}
	intent := types.ActionIntent{ID: "intent-live-e2e", WorkItemID: item.ID, Action: types.ActionKindComment, PRNumber: 702, Lane: types.ActionLaneFocusedReview, DryRun: false, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"live fake comment"}, IdempotencyKey: "owner/repo#702:comment"}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 702, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-live", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: true}, q, fake, ledger)
	if processed, err := worker.ProcessOnce(ctx); err != nil || !processed {
		t.Fatalf("process once: processed=%t err=%v", processed, err)
	}
	if !fake.HasWritten() || fake.WriteCount() != 1 {
		t.Fatalf("live fake worker writes=%d log=%+v", fake.WriteCount(), fake.Log())
	}
	comments, err := fake.GetComments(ctx, "owner/repo", 702)
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 1 || comments[0].Body != "live fake comment" {
		t.Fatalf("comments = %+v", comments)
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateVerified {
		t.Fatalf("state = %s, want verified", finished.State)
	}
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !hasLedgerTransition(history, "executed") || hasLedgerTransition(history, "failed") {
		t.Fatalf("bad live ledger history: %+v", history)
	}
}
