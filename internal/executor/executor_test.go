package executor

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func mergeIntent(dryRun bool) types.ActionIntent {
	return types.ActionIntent{
		ID:             "intent-merge-101",
		Action:         types.ActionKindMerge,
		PRNumber:       101,
		Lane:           types.ActionLaneFastMerge,
		DryRun:         dryRun,
		PolicyProfile:  types.PolicyProfileAutonomous,
		Confidence:     0.95,
		Reasons:        []string{"test"},
		EvidenceRefs:   []string{"fixture"},
		IdempotencyKey: "owner/repo#101:merge",
	}
}

func TestExecutorDryRunDoesNotMutateFakeGitHub(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "abc"})
	exec := New(Config{Repo: "owner/repo", DryRun: true, PolicyProfile: types.PolicyProfileAutonomous}, fake, NewMemoryLedger())

	result, err := exec.ExecuteIntent(ctx, mergeIntent(true))
	if err != nil {
		t.Fatalf("execute dry-run merge: %v", err)
	}
	if !result.DryRun || result.Executed {
		t.Fatalf("dry-run result = %+v", result)
	}
	pr, ok := fake.GetPR(101)
	if !ok || pr.Merged {
		t.Fatalf("fake PR mutated in dry-run: %+v ok=%t", pr, ok)
	}
}

func TestExecutorPolicyBlocksWrites(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "abc"})
	intent := mergeIntent(false)

	advisory := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAdvisory}, fake, NewMemoryLedger())
	if _, err := advisory.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected advisory non-dry-run merge denial")
	}

	guarded := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileGuarded}, fake, NewMemoryLedger())
	if _, err := guarded.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected guarded merge denial")
	}
}

func TestExecutorAutonomousMergeMutatesAndRecordsIdempotency(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, ledger)

	result, err := exec.ExecuteIntent(ctx, mergeIntent(false))
	if err != nil {
		t.Fatalf("execute merge: %v", err)
	}
	if !result.Executed || result.DryRun || result.AlreadyExecuted {
		t.Fatalf("merge result = %+v", result)
	}
	pr, ok := fake.GetPR(101)
	if !ok || !pr.Merged || pr.State != "merged" {
		t.Fatalf("fake PR not merged: %+v ok=%t", pr, ok)
	}
	again, err := exec.ExecuteIntent(ctx, mergeIntent(false))
	if err != nil {
		t.Fatalf("execute duplicate: %v", err)
	}
	if !again.AlreadyExecuted || again.Executed {
		t.Fatalf("duplicate result = %+v", again)
	}
}

func TestExecutorGuardedAllowsCommentDryRunFalse(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 102, State: "open", Mergeable: false, HeadSHA: "def"})
	intent := types.ActionIntent{
		ID:             "intent-comment-102",
		Action:         types.ActionKindComment,
		PRNumber:       102,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileGuarded,
		IdempotencyKey: "owner/repo#102:comment",
		Reasons:        []string{"focused_review"},
	}
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileGuarded}, fake, NewMemoryLedger())
	result, err := exec.ExecuteIntent(ctx, intent)
	if err != nil {
		t.Fatalf("execute guarded comment: %v", err)
	}
	if !result.Executed {
		t.Fatalf("comment result = %+v", result)
	}
	comments := fake.Comments(102)
	if len(comments) != 1 {
		t.Fatalf("comments = %#v", comments)
	}
}

func TestExecutorRejectsUnknownAction(t *testing.T) {
	ctx := context.Background()
	exec := New(Config{Repo: "owner/repo", DryRun: true, PolicyProfile: types.PolicyProfileAdvisory}, NewFakeGitHub(), NewMemoryLedger())
	intent := mergeIntent(true)
	intent.Action = types.ActionKind("teleport")
	if _, err := exec.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected unknown action error")
	}
}
