package executor

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestExecutorMergeUsesPayloadOptionsAndPreconditions(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 301, State: "open", Mergeable: true, HeadSHA: "abc123", BaseBranch: "release/v2"})
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, NewMemoryLedger())

	intent := types.ActionIntent{
		ID:             "intent-merge-payload",
		Action:         types.ActionKindMerge,
		PRNumber:       301,
		Lane:           types.ActionLaneFastMerge,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAutonomous,
		IdempotencyKey: "owner/repo#301:merge",
		Payload: map[string]any{
			"merge_method":      "rebase",
			"commit_title":      "Merge PR 301",
			"commit_message":    "Verified by prATC",
			"expected_head_sha": "abc123",
			"allowed_branches":  []any{"main", "release/v2"},
		},
	}

	if _, err := exec.ExecuteIntent(ctx, intent); err != nil {
		t.Fatalf("execute merge: %v", err)
	}
	log := fake.Log()
	if len(log) != 1 || log[0].Action != "merge" {
		t.Fatalf("log = %+v", log)
	}
	opts := log[0].MergeOptions
	if opts.MergeMethod != "rebase" || opts.CommitTitle != "Merge PR 301" || opts.CommitMessage != "Verified by prATC" {
		t.Fatalf("merge options = %+v", opts)
	}
}

func TestExecutorMergeRejectsChangedHeadSHA(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 302, State: "open", Mergeable: true, HeadSHA: "new-sha"})
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, NewMemoryLedger())
	intent := mergeIntent(false)
	intent.PRNumber = 302
	intent.ID = "intent-merge-stale-sha"
	intent.IdempotencyKey = "owner/repo#302:merge"
	intent.Payload = map[string]any{"expected_head_sha": "old-sha"}

	if _, err := exec.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected stale head SHA preflight error")
	}
	if fake.WriteCount() != 0 {
		t.Fatalf("write count = %d, want 0", fake.WriteCount())
	}
}

func TestExecutorCloseRequiresReason(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 401, State: "open", Mergeable: false, HeadSHA: "abc"})
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, NewMemoryLedger())
	intent := types.ActionIntent{ID: "intent-close-no-reason", Action: types.ActionKindClose, PRNumber: 401, DryRun: false, PolicyProfile: types.PolicyProfileAutonomous, IdempotencyKey: "owner/repo#401:close"}

	if _, err := exec.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected close without reason to be rejected")
	}
	if fake.WriteCount() != 0 {
		t.Fatalf("write count = %d, want 0", fake.WriteCount())
	}
	history, err := exec.ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("ledger history: %v", err)
	}
	if !hasLedgerTransition(history, "failed") {
		t.Fatalf("missing failed ledger transition: %+v", history)
	}
}

func TestExecutorCloseCommentsBeforeCloseAndVerifiesComment(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 402, State: "open", Mergeable: false, HeadSHA: "abc"})
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, NewMemoryLedger())
	intent := types.ActionIntent{
		ID:             "intent-close-with-comment",
		Action:         types.ActionKindClose,
		PRNumber:       402,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAutonomous,
		IdempotencyKey: "owner/repo#402:close",
		Reasons:        []string{"duplicate of #99"},
		Payload:        map[string]any{"comment": "Closing as duplicate of #99"},
	}

	if _, err := exec.ExecuteIntent(ctx, intent); err != nil {
		t.Fatalf("execute close: %v", err)
	}
	log := fake.Log()
	if len(log) != 2 || log[0].Action != "comment" || log[1].Action != "close" {
		t.Fatalf("expected comment before close, got %+v", log)
	}
	if fake.WriteCount() != 2 {
		t.Fatalf("write count = %d, want 2", fake.WriteCount())
	}
}
