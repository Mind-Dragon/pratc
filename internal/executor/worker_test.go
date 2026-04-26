package executor

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

func TestWorkerExecutesPersistedIntent(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir()+"/queue.db", workqueue.WithNow(func() time.Time {
		return time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	}))
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{
		ID:             "wi-101",
		PRNumber:       101,
		Lane:           types.ActionLaneFocusedReview,
		State:          types.ActionWorkItemStateProposed,
		PriorityScore:  0.8,
		Confidence:     0.9,
		ReasonTrail:    []string{"needs comment"},
		EvidenceRefs:   []string{"fixture"},
		AllowedActions: []types.ActionKind{types.ActionKindComment},
		IdempotencyKey: "owner/repo#101:work_item",
	}
	intent := types.ActionIntent{
		ID:             "intent-101-comment",
		WorkItemID:     item.ID,
		Action:         types.ActionKindComment,
		PRNumber:       101,
		Lane:           types.ActionLaneFocusedReview,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileGuarded,
		Confidence:     0.9,
		Reasons:        []string{"worker comment"},
		EvidenceRefs:   []string{"fixture"},
		Preconditions:  []types.ActionPreflight{},
		IdempotencyKey: "owner/repo#101:comment",
		CreatedAt:      "2026-04-26T10:00:00Z",
	}
	plan := types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}
	if err := q.EnqueueActionPlan(ctx, plan); err != nil {
		t.Fatalf("enqueue plan: %v", err)
	}

	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "abc123"})
	worker := NewWorker(WorkerConfig{
		Repo:         "owner/repo",
		WorkerID:     "worker-test",
		LeaseTTL:     time.Minute,
		PollInterval: time.Millisecond,
		Concurrency:  1,
		Live:         true,
	}, q, fake, NewMemoryLedger())

	processed, err := worker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected worker to process one item")
	}
	comments := fake.Comments(101)
	if len(comments) != 1 || comments[0].Body != "worker comment" {
		t.Fatalf("comments = %+v", comments)
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get finished item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateVerified {
		t.Fatalf("state = %s, want verified", finished.State)
	}
}

func TestWorkerSkipsDryRunIntentWithoutMutation(t *testing.T) {
	ctx := context.Background()
	q, err := workqueue.OpenSQLite(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	t.Cleanup(func() { _ = q.Close() })

	item := types.ActionWorkItem{ID: "wi-102", PRNumber: 102, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, PriorityScore: 0.8, Confidence: 0.9, ReasonTrail: []string{"dry"}, EvidenceRefs: []string{"fixture"}, AllowedActions: []types.ActionKind{types.ActionKindComment}, IdempotencyKey: "owner/repo#102:work_item"}
	intent := types.ActionIntent{ID: "intent-102-comment", WorkItemID: item.ID, Action: types.ActionKindComment, PRNumber: 102, Lane: types.ActionLaneFocusedReview, DryRun: true, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"dry comment"}, EvidenceRefs: []string{"fixture"}, Preconditions: []types.ActionPreflight{}, IdempotencyKey: "owner/repo#102:comment", CreatedAt: "2026-04-26T10:00:00Z"}
	if err := q.EnqueueActionPlan(ctx, types.ActionPlan{Repo: "owner/repo", WorkItems: []types.ActionWorkItem{item}, ActionIntents: []types.ActionIntent{intent}}); err != nil {
		t.Fatalf("enqueue plan: %v", err)
	}
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 102, State: "open", Mergeable: true, HeadSHA: "abc123"})
	worker := NewWorker(WorkerConfig{Repo: "owner/repo", WorkerID: "worker-test", LeaseTTL: time.Minute, PollInterval: time.Millisecond, Concurrency: 1, Live: true}, q, fake, NewMemoryLedger())

	processed, err := worker.ProcessOnce(ctx)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected worker to process one item")
	}
	if got := fake.WriteCount(); got != 0 {
		t.Fatalf("write count = %d, want 0", got)
	}
	finished, err := q.Get(ctx, item.ID)
	if err != nil {
		t.Fatalf("get finished item: %v", err)
	}
	if finished.State != types.ActionWorkItemStateVerified {
		t.Fatalf("state = %s, want verified", finished.State)
	}
}
