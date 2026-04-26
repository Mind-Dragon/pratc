package e2e

import (
	"context"
	"fmt"
	"os"

	"github.com/jeffersonnunn/pratc/internal/executor"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// E2EHarness provides end-to-end test coverage for the guarded executor
type E2EHarness struct {
	mutator   *executor.FakeGitHub
	ledger    *executor.MemoryLedger
	repoPath  string
	tmpDir    string
}

// NewE2EHarness creates a new e2e test harness
func NewE2EHarness() *E2EHarness {
	tmpDir, err := os.MkdirTemp("", "pratc-e2e-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp dir: %v", err))
	}

	return &E2EHarness{
		mutator:  executor.NewFakeGitHub(),
		ledger:   executor.NewMemoryLedger(),
		repoPath: tmpDir,
		tmpDir:   tmpDir,
	}
}

// Cleanup removes the temporary directory
func (h *E2EHarness) Cleanup() error {
	if h.tmpDir == "" {
		return nil
	}
	err := os.RemoveAll(h.tmpDir)
	if err != nil {
		return fmt.Errorf("failed to remove temp dir: %w", err)
	}
	h.tmpDir = ""
	return nil
}

// TestCommentAction tests comment action end-to-end
func (h *E2EHarness) TestCommentAction() error {
	ctx := context.Background()

	// Setup PR
	h.mutator.UpsertPR(executor.FakePR{
		Number:    101,
		State:     "open",
		HeadSHA:   "abc123",
		Mergeable: true,
	})

	// Create executor
	cfg := executor.Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAutonomous,
	}
	exec := executor.New(cfg, h.mutator, h.ledger)

	// Execute comment action
	intent := types.ActionIntent{
		ID:             "test-comment-101",
		Action:         types.ActionKindComment,
		PRNumber:       101,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAutonomous,
		IdempotencyKey: "owner/repo#101:comment",
		Reasons:        []string{"test comment"},
	}

	result, err := exec.ExecuteIntent(ctx, intent)
	if err != nil {
		return fmt.Errorf("execute comment action failed: %w", err)
	}

	if !result.Executed {
		return fmt.Errorf("comment action should be executed")
	}

	if result.Result != "commented" {
		return fmt.Errorf("expected result 'commented', got %q", result.Result)
	}

	// Verify comment was added
	comments, err := h.mutator.GetComments(ctx, "owner/repo", 101)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}

	if len(comments) == 0 {
		return fmt.Errorf("comment was not added")
	}

	return nil
}

// TestLabelAction tests label action end-to-end
func (h *E2EHarness) TestLabelAction() error {
	ctx := context.Background()

	// Setup PR
	h.mutator.UpsertPR(executor.FakePR{
		Number:    102,
		State:     "open",
		HeadSHA:   "def456",
		Mergeable: true,
	})

	// Create executor
	cfg := executor.Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAutonomous,
	}
	exec := executor.New(cfg, h.mutator, h.ledger)

	// Execute label action
	intent := types.ActionIntent{
		ID:             "test-label-102",
		Action:         types.ActionKindLabel,
		PRNumber:       102,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAutonomous,
		IdempotencyKey: "owner/repo#102:label",
		Reasons:        []string{"test label"},
	}

	result, err := exec.ExecuteIntent(ctx, intent)
	if err != nil {
		return fmt.Errorf("execute label action failed: %w", err)
	}

	if !result.Executed {
		return fmt.Errorf("label action should be executed")
	}

	if result.Result != "labeled" {
		return fmt.Errorf("expected result 'labeled', got %q", result.Result)
	}

	// Verify label was added
	labels, err := h.mutator.GetLabels(ctx, "owner/repo", 102)
	if err != nil {
		return fmt.Errorf("failed to get labels: %w", err)
	}

	if len(labels) == 0 {
		return fmt.Errorf("label was not added")
	}

	return nil
}

// TestMergeAction tests merge action with preflight checks
func (h *E2EHarness) TestMergeAction() error {
	ctx := context.Background()

	// Setup PR
	h.mutator.UpsertPR(executor.FakePR{
		Number:    103,
		State:     "open",
		HeadSHA:   "ghi789",
		Mergeable: true,
	})

	// Create executor
	cfg := executor.Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAutonomous,
	}
	exec := executor.New(cfg, h.mutator, h.ledger)

	// Execute merge action
	intent := types.ActionIntent{
		ID:             "test-merge-103",
		Action:         types.ActionKindMerge,
		PRNumber:       103,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAutonomous,
		IdempotencyKey: "owner/repo#103:merge",
		Reasons:        []string{"test merge"},
	}

	result, err := exec.ExecuteIntent(ctx, intent)
	if err != nil {
		return fmt.Errorf("execute merge action failed: %w", err)
	}

	if !result.Executed {
		return fmt.Errorf("merge action should be executed")
	}

	if result.Result != "merged" {
		return fmt.Errorf("expected result 'merged', got %q", result.Result)
	}

	return nil
}

// TestAdvisoryModeNoWrites tests that advisory mode prevents GitHub writes
func (h *E2EHarness) TestAdvisoryModeNoWrites() error {
	ctx := context.Background()

	// Setup PR
	h.mutator.UpsertPR(executor.FakePR{
		Number:    104,
		State:     "open",
		HeadSHA:   "jkl012",
		Mergeable: true,
	})

	// Create executor with advisory policy
	cfg := executor.Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAdvisory,
	}
	exec := executor.New(cfg, h.mutator, h.ledger)

	// Execute comment action (should fail in advisory mode)
	intent := types.ActionIntent{
		ID:             "test-comment-104",
		Action:         types.ActionKindComment,
		PRNumber:       104,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAdvisory,
		IdempotencyKey: "owner/repo#104:comment",
		Reasons:        []string{"test comment"},
	}

	_, err := exec.ExecuteIntent(ctx, intent)
	if err == nil {
		return fmt.Errorf("expected error in advisory mode, got nil")
	}

	// Verify no comments were added
	comments, err := h.mutator.GetComments(ctx, "owner/repo", 104)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}

	if len(comments) > 0 {
		return fmt.Errorf("no comments should be added in advisory mode")
	}

	return nil
}
