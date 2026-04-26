package executor_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/executor"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestExecutorWithSQLiteLedger(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	
	// Create a fake GitHub mutator
	fakeMutator := &FakeGitHubMutator{}
	
	// Create executor with SQLite ledger
	exec := executor.New(
		executor.Config{
			Repo:          "owner/repo",
			DryRun:        true,
			PolicyProfile: types.PolicyProfileAutonomous,
		},
		fakeMutator,
		ledger,
	)
	
	// Test executing an intent
	intent := types.ActionIntent{
		ID:             "test-intent-123",
		Action:         types.ActionKindMerge,
		PRNumber:       42,
		IdempotencyKey: "test-key-123",
		DryRun:         true,
	}
	
	result, err := exec.ExecuteIntent(context.Background(), intent)
	if err != nil {
		t.Fatalf("Failed to execute intent: %v", err)
	}
	
	if result.AlreadyExecuted {
		t.Error("Expected intent to be executed for the first time")
	}
	
	if result.Executed {
		t.Error("Expected dry run to not execute")
	}
	
	// Verify the ledger has the transitions
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	expectedTransitions := []string{"proposed", "preflighted", "executed"}
	if len(history) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions, got %d", len(expectedTransitions), len(history))
	}
	
	for i, expected := range expectedTransitions {
		if history[i].Transition != expected {
			t.Errorf("Expected transition %q at position %d, got %q", expected, i, history[i].Transition)
		}
	}
	
	// Test executing the same intent again (idempotency)
	result2, err := exec.ExecuteIntent(context.Background(), intent)
	if err != nil {
		t.Fatalf("Failed to execute intent again: %v", err)
	}
	
	if !result2.AlreadyExecuted {
		t.Error("Expected intent to be marked as already executed")
	}
	
	// Verify the ledger still has only 3 transitions (not duplicated)
	history2, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(history2) != 3 {
		t.Errorf("Expected 3 transitions after idempotent execution, got %d", len(history2))
	}
}

func TestExecutorCrashRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	// First session: create executor and start execution
	store1, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	
	ledger1 := store1.ExecutorLedger()
	fakeMutator := &FakeGitHubMutator{}
	
	exec1 := executor.New(
		executor.Config{
			Repo:          "owner/repo",
			DryRun:        true,
			PolicyProfile: types.PolicyProfileAutonomous,
		},
		fakeMutator,
		ledger1,
	)
	
	intent := types.ActionIntent{
		ID:             "crash-test-intent",
		Action:         types.ActionKindMerge,
		PRNumber:       42,
		IdempotencyKey: "crash-test-key",
		DryRun:         true,
	}
	
	// Execute the intent
	result, err := exec1.ExecuteIntent(context.Background(), intent)
	if err != nil {
		t.Fatalf("Failed to execute intent: %v", err)
	}
	
	if result.AlreadyExecuted {
		t.Error("Expected intent to be executed for the first time")
	}
	
	// Simulate crash by closing the database
	store1.Close()
	
	// Second session: reopen and verify data persists
	store2, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to reopen database: %v", err)
	}
	defer store2.Close()
	
	ledger2 := store2.ExecutorLedger()
	
	// Verify the transitions persisted
	history, err := ledger2.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("Failed to get history after recovery: %v", err)
	}
	
	expectedTransitions := []string{"proposed", "preflighted", "executed"}
	if len(history) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions after recovery, got %d", len(expectedTransitions), len(history))
	}
	
	for i, expected := range expectedTransitions {
		if history[i].Transition != expected {
			t.Errorf("Expected transition %q at position %d, got %q", expected, i, history[i].Transition)
		}
	}
	
	// Create a new executor with the recovered ledger
	exec2 := executor.New(
		executor.Config{
			Repo:          "owner/repo",
			DryRun:        true,
			PolicyProfile: types.PolicyProfileAutonomous,
		},
		fakeMutator,
		ledger2,
	)
	
	// Execute the same intent again (should be idempotent)
	result2, err := exec2.ExecuteIntent(context.Background(), intent)
	if err != nil {
		t.Fatalf("Failed to execute intent after recovery: %v", err)
	}
	
	if !result2.AlreadyExecuted {
		t.Error("Expected intent to be marked as already executed after recovery")
	}
}

func TestExecutorPreflightFailureRecording(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	
	// Create a mutator that will fail preflight checks
	failingMutator := &FailingGitHubMutator{
		failReason: "PR is not open",
	}
	
	exec := executor.New(
		executor.Config{
			Repo:          "owner/repo",
			DryRun:        false,
			PolicyProfile: types.PolicyProfileAutonomous,
		},
		failingMutator,
		ledger,
	)
	
	intent := types.ActionIntent{
		ID:             "preflight-fail-test",
		Action:         types.ActionKindMerge,
		PRNumber:       42,
		IdempotencyKey: "preflight-fail-key",
		DryRun:         false,
	}
	
	// Execute the intent (should fail preflight)
	_, err = exec.ExecuteIntent(context.Background(), intent)
	if err == nil {
		t.Fatal("Expected preflight check to fail")
	}
	
	// Verify the ledger has the proposed and failed transitions
	history, err := ledger.GetHistory(intent.IdempotencyKey)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	expectedTransitions := []string{"proposed", "failed"}
	if len(history) != len(expectedTransitions) {
		t.Fatalf("Expected %d transitions after preflight failure, got %d", len(expectedTransitions), len(history))
	}
	
	for i, expected := range expectedTransitions {
		if history[i].Transition != expected {
			t.Errorf("Expected transition %q at position %d, got %q", expected, i, history[i].Transition)
		}
	}
	
	// Verify the failed transition has error information
	if history[1].PreflightSnapshot == "" {
		t.Error("Expected failed transition to have error snapshot")
	}
	
	// Verify IsExecuted returns false for failed intent
	if ledger.IsExecuted(intent.IdempotencyKey) {
		t.Error("Expected IsExecuted to return false for failed intent")
	}
}

func TestExecutorLedgerBackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()
	
	ledger := store.ExecutorLedger()
	
	// Test backward compatibility with old Record method
	result := types.ExecutionResult{
		IntentID: "backward-compat-test",
		Action:   types.ActionKindMerge,
		PRNumber: 42,
		Executed: true,
		Result:   "merged",
	}
	
	err = ledger.Record("backward-compat-key", result)
	if err != nil {
		t.Fatalf("Failed to record using old Record method: %v", err)
	}
	
	// Verify IsExecuted works
	if !ledger.IsExecuted("backward-compat-key") {
		t.Error("Expected IsExecuted to return true after Record")
	}
	
	// Verify GetHistory works
	history, err := ledger.GetHistory("backward-compat-key")
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}
	
	if len(history) != 1 {
		t.Fatalf("Expected 1 transition, got %d", len(history))
	}
	
	if history[0].Transition != "executed" {
		t.Errorf("Expected transition 'executed', got %q", history[0].Transition)
	}
}

// FakeGitHubMutator is a test double for GitHubMutator
type FakeGitHubMutator struct{}

func (f *FakeGitHubMutator) Merge(ctx context.Context, repo string, prNumber int, opts executor.MergeOptions, dryRun bool) (executor.MergeResult, error) {
	return executor.MergeResult{Merged: !dryRun, SHA: "abc123", AlreadyMerged: false}, nil
}

func (f *FakeGitHubMutator) Close(ctx context.Context, repo string, prNumber int, reason string, dryRun bool) error {
	return nil
}

func (f *FakeGitHubMutator) AddComment(ctx context.Context, repo string, prNumber int, body string, dryRun bool) error {
	return nil
}

func (f *FakeGitHubMutator) AddLabels(ctx context.Context, repo string, prNumber int, labels []string, dryRun bool) error {
	return nil
}

func (f *FakeGitHubMutator) ApplyFix(ctx context.Context, repo string, prNumber int, patch string, dryRun bool) (executor.ApplyFixResult, error) {
	return executor.ApplyFixResult{Applied: !dryRun, NewBranch: "fix-branch"}, nil
}

func (f *FakeGitHubMutator) GetPRState(ctx context.Context, repo string, prNumber int) (executor.PRState, error) {
	return executor.PRState{Number: prNumber, State: "open", HeadSHA: "abc123", BaseBranch: "main", Mergeable: true, CIStatus: "success"}, nil
}

func (f *FakeGitHubMutator) GetHeadSHA(ctx context.Context, repo string, prNumber int) (string, error) {
	return "abc123", nil
}

func (f *FakeGitHubMutator) GetBaseBranch(ctx context.Context, repo string, prNumber int) (string, error) {
	return "main", nil
}

func (f *FakeGitHubMutator) GetCIStatus(ctx context.Context, repo string, prNumber int) (string, error) {
	return "success", nil
}

func (f *FakeGitHubMutator) GetMergeable(ctx context.Context, repo string, prNumber int) (bool, error) {
	return true, nil
}

func (f *FakeGitHubMutator) GetRequiredReviews(ctx context.Context, repo string, prNumber int) (bool, error) {
	return true, nil
}

func (f *FakeGitHubMutator) GetRateLimitRemaining(ctx context.Context) (int, error) {
	return 100, nil
}

func (f *FakeGitHubMutator) GetComments(ctx context.Context, repo string, prNumber int) ([]executor.Comment, error) {
	return []executor.Comment{{Body: "test comment"}}, nil
}

func (f *FakeGitHubMutator) GetLabels(ctx context.Context, repo string, prNumber int) ([]string, error) {
	return []string{"pratc-action"}, nil
}

// FailingGitHubMutator is a test double that fails preflight checks
type FailingGitHubMutator struct {
	failReason string
}

func (f *FailingGitHubMutator) Merge(ctx context.Context, repo string, prNumber int, opts executor.MergeOptions, dryRun bool) (executor.MergeResult, error) {
	return executor.MergeResult{}, nil
}

func (f *FailingGitHubMutator) Close(ctx context.Context, repo string, prNumber int, reason string, dryRun bool) error {
	return nil
}

func (f *FailingGitHubMutator) AddComment(ctx context.Context, repo string, prNumber int, body string, dryRun bool) error {
	return nil
}

func (f *FailingGitHubMutator) AddLabels(ctx context.Context, repo string, prNumber int, labels []string, dryRun bool) error {
	return nil
}

func (f *FailingGitHubMutator) ApplyFix(ctx context.Context, repo string, prNumber int, patch string, dryRun bool) (executor.ApplyFixResult, error) {
	return executor.ApplyFixResult{}, nil
}

func (f *FailingGitHubMutator) GetPRState(ctx context.Context, repo string, prNumber int) (executor.PRState, error) {
	return executor.PRState{Number: prNumber, State: "closed", HeadSHA: "abc123", BaseBranch: "main", Mergeable: true, CIStatus: "success"}, nil
}

func (f *FailingGitHubMutator) GetHeadSHA(ctx context.Context, repo string, prNumber int) (string, error) {
	return "abc123", nil
}

func (f *FailingGitHubMutator) GetBaseBranch(ctx context.Context, repo string, prNumber int) (string, error) {
	return "main", nil
}

func (f *FailingGitHubMutator) GetCIStatus(ctx context.Context, repo string, prNumber int) (string, error) {
	return "success", nil
}

func (f *FailingGitHubMutator) GetMergeable(ctx context.Context, repo string, prNumber int) (bool, error) {
	return true, nil
}

func (f *FailingGitHubMutator) GetRequiredReviews(ctx context.Context, repo string, prNumber int) (bool, error) {
	return true, nil
}

func (f *FailingGitHubMutator) GetRateLimitRemaining(ctx context.Context) (int, error) {
	return 100, nil
}

func (f *FailingGitHubMutator) GetComments(ctx context.Context, repo string, prNumber int) ([]executor.Comment, error) {
	return []executor.Comment{}, nil
}

func (f *FailingGitHubMutator) GetLabels(ctx context.Context, repo string, prNumber int) ([]string, error) {
	return []string{}, nil
}
