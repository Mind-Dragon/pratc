package executor

import (
	"context"
	"fmt"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type mutationErrorMutator struct{ *FakeGitHub }

func (m mutationErrorMutator) AddComment(ctx context.Context, repo string, prNumber int, body string, dryRun bool) error {
	return fmt.Errorf("mutation failed")
}

type verificationErrorMutator struct{ *FakeGitHub }

func (m verificationErrorMutator) GetComments(ctx context.Context, repo string, prNumber int) ([]Comment, error) {
	return []Comment{}, nil
}

func TestExecutorRecordsFailedTransitionOnMutationError(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 601, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileGuarded}, mutationErrorMutator{FakeGitHub: fake}, ledger)
	intent := types.ActionIntent{ID: "intent-mutation-error", Action: types.ActionKindComment, PRNumber: 601, DryRun: false, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"comment"}, IdempotencyKey: "owner/repo#601:comment"}

	if _, err := exec.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected mutation error")
	}
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !hasLedgerTransition(history, "failed") {
		t.Fatalf("missing failed transition: %+v", history)
	}
}

func TestExecutorRecordsFailedTransitionOnVerificationError(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 602, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileGuarded}, verificationErrorMutator{FakeGitHub: fake}, ledger)
	intent := types.ActionIntent{ID: "intent-verification-error", Action: types.ActionKindComment, PRNumber: 602, DryRun: false, PolicyProfile: types.PolicyProfileGuarded, Reasons: []string{"comment"}, IdempotencyKey: "owner/repo#602:comment"}

	if _, err := exec.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected verification error")
	}
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !hasLedgerTransition(history, "failed") {
		t.Fatalf("missing failed transition: %+v", history)
	}
}

func TestExecutorRecordsFailedTransitionOnPolicyDenial(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 603, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAdvisory}, fake, ledger)
	intent := types.ActionIntent{ID: "intent-policy-denied", Action: types.ActionKindComment, PRNumber: 603, DryRun: false, PolicyProfile: types.PolicyProfileAdvisory, Reasons: []string{"comment"}, IdempotencyKey: "owner/repo#603:comment"}

	if _, err := exec.ExecuteIntent(ctx, intent); err == nil {
		t.Fatal("expected policy denial")
	}
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if !hasLedgerTransition(history, "failed") {
		t.Fatalf("missing failed transition: %+v", history)
	}
}

func TestMemoryLedgerRecordTransitionUpsertsByIntentAndTransition(t *testing.T) {
	ledger := NewMemoryLedger()
	if err := ledger.RecordTransition("intent-dup", "executed", `{"result":"first"}`, nil); err != nil {
		t.Fatalf("first record: %v", err)
	}
	if err := ledger.RecordTransition("intent-dup", "executed", `{"result":"second"}`, nil); err != nil {
		t.Fatalf("second record: %v", err)
	}
	history, err := ledger.GetHistory("intent-dup")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(history) != 1 || history[0].PreflightSnapshot != `{"result":"second"}` {
		t.Fatalf("history = %+v, want one updated executed transition", history)
	}
}
