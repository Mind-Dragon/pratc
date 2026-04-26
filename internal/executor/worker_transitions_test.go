package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

func TestWorkerExecutionFailureFailsClaimedItem(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-exec-fail", PRNumber: 501, Lane: types.ActionLaneFastMerge, State: types.ActionWorkItemStateProposed, PriorityScore: 0.9, Confidence: 0.9, ReasonTrail: []string{"merge"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindMerge}, IdempotencyKey: "owner/repo#501:work_item"}
	intent := types.ActionIntent{ID: "intent-exec-fail", WorkItemID: item.ID, Action: types.ActionKindMerge, PRNumber: 501, Lane: types.ActionLaneFastMerge, DryRun: false, PolicyProfile: types.PolicyProfileAutonomous, IdempotencyKey: "owner/repo#501:merge", Payload: map[string]any{"expected_head_sha": "abc"}}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 501, State: "open", Mergeable: false, HeadSHA: "abc"})
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-test", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: true}, q, fake, NewMemoryLedger())

	processed, err := worker.ProcessOnce(ctx)
	if err == nil || !strings.Contains(err.Error(), "preflight check failed") {
		t.Fatalf("expected preflight failure, got processed=%t err=%v", processed, err)
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateFailed || finished.LeaseState.ClaimedBy != "" {
		t.Fatalf("finished = %+v, want failed with cleared lease", finished)
	}
}

func TestWorkerSuccessfulLiveAttemptHasQueueTransitionTrail(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-verified-trail", PRNumber: 502, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, PriorityScore: 0.8, Confidence: 0.9, ReasonTrail: []string{"comment"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindComment}, IdempotencyKey: "owner/repo#502:work_item"}
	intent := types.ActionIntent{ID: "intent-verified-comment", WorkItemID: item.ID, Action: types.ActionKindComment, PRNumber: 502, Lane: types.ActionLaneFocusedReview, DryRun: false, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"review comment"}, IdempotencyKey: "owner/repo#502:comment"}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 502, State: "open", Mergeable: true, HeadSHA: "abc"})
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-test", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: true}, q, fake, NewMemoryLedger())
	if processed, err := worker.ProcessOnce(ctx); err != nil || !processed {
		t.Fatalf("process once: processed=%t err=%v", processed, err)
	}

	ledger, err := q.GetExecutorLedger(20)
	if err != nil {
		t.Fatalf("ledger: %v", err)
	}
	want := []string{"claimable->claimed", "claimed->preflighted", "preflighted->approved_for_execution", "approved_for_execution->executed", "executed->verified"}
	for _, expected := range want {
		if !hasQueueTransition(ledger.Transitions, item.ID, expected) {
			t.Fatalf("missing transition %s in %+v", expected, ledger.Transitions)
		}
	}
}

func hasQueueTransition(transitions []workqueue.ActionTransition, itemID, pair string) bool {
	for _, tr := range transitions {
		if tr.WorkItemID == itemID && tr.FromState+"->"+tr.ToState == pair {
			return true
		}
	}
	return false
}
