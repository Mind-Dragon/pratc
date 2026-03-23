package planner

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPlanner_EmptyPool(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	planner := New(WithNow(now))

	prs := []types.PR{
		{Number: 1, Title: "Draft PR", IsDraft: true},
		{Number: 2, Title: "Conflict PR", Mergeable: "conflicting"},
		{Number: 3, Title: "CI Failing PR", CIStatus: "failure"},
	}

	result, err := planner.Plan(ctx, "test/repo", prs, 20, formula.ModeCombination)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	if result.Repo != "test/repo" {
		t.Errorf("result.Repo = %q, want %q", result.Repo, "test/repo")
	}
	if result.Target != 20 {
		t.Errorf("result.Target = %d, want %d", result.Target, 20)
	}
	if result.CandidatePoolSize != 0 {
		t.Errorf("result.CandidatePoolSize = %d, want 0", result.CandidatePoolSize)
	}
	if len(result.Selected) != 0 {
		t.Errorf("result.Selected should be empty, got %d items", len(result.Selected))
	}
	if len(result.Rejections) == 0 {
		t.Error("result.Rejections should not be empty")
	}
}

func TestPlanner_HappyPath(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	prs := []types.PR{
		{
			Number:       1,
			Title:        "Feature A",
			CIStatus:     "success",
			Mergeable:    "mergeable",
			ReviewStatus: "approved",
			BaseBranch:   "main",
			HeadBranch:   "feature-a",
			CreatedAt:    now.Add(-24 * time.Hour).Format(time.RFC3339),
			UpdatedAt:    now.Add(-24 * time.Hour).Format(time.RFC3339),
		},
		{
			Number:       2,
			Title:        "Feature B",
			CIStatus:     "success",
			Mergeable:    "mergeable",
			ReviewStatus: "approved",
			BaseBranch:   "main",
			HeadBranch:   "feature-b",
			CreatedAt:    now.Add(-48 * time.Hour).Format(time.RFC3339),
			UpdatedAt:    now.Add(-48 * time.Hour).Format(time.RFC3339),
		},
		{
			Number:       3,
			Title:        "Feature C",
			CIStatus:     "success",
			Mergeable:    "mergeable",
			ReviewStatus: "approved",
			BaseBranch:   "feature-a",
			HeadBranch:   "feature-c",
			CreatedAt:    now.Add(-12 * time.Hour).Format(time.RFC3339),
			UpdatedAt:    now.Add(-12 * time.Hour).Format(time.RFC3339),
		},
	}

	planner := New(WithNow(now))
	result, err := planner.Plan(ctx, "test/repo", prs, 2, formula.ModeCombination)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	if result.Repo != "test/repo" {
		t.Errorf("result.Repo = %q, want %q", result.Repo, "test/repo")
	}
	if result.Target != 2 {
		t.Errorf("result.Target = %d, want %d", result.Target, 2)
	}
	if result.CandidatePoolSize == 0 {
		t.Error("result.CandidatePoolSize should be > 0")
	}
	if len(result.Selected) == 0 {
		t.Error("result.Selected should not be empty")
	}
	if len(result.Ordering) == 0 {
		t.Error("result.Ordering should not be empty")
	}
	if result.Strategy != "formula+graph" {
		t.Errorf("result.Strategy = %q, want %q", result.Strategy, "formula+graph")
	}
}

func TestPlanner_DefaultTarget(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	prs := []types.PR{
		{Number: 1, Title: "PR 1", CIStatus: "success", Mergeable: "mergeable"},
	}

	planner := New(WithNow(now))
	result, err := planner.Plan(ctx, "test/repo", prs, 0, formula.ModeCombination)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	// Target 0 should default to 20
	if result.Target != 20 {
		t.Errorf("result.Target = %d, want 20 (default)", result.Target)
	}
}

func TestPlanner_Deduplication(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	// Create PRs that might be selected multiple times by formula engine
	prs := make([]types.PR, 5)
	for i := 1; i <= 5; i++ {
		prs[i-1] = types.PR{
			Number:     i,
			Title:      "PR " + string(rune('A'+i-1)),
			CIStatus:   "success",
			Mergeable:  "mergeable",
			BaseBranch: "main",
			HeadBranch: "feature-" + string(rune('a'+i-1)),
			CreatedAt:  now.Add(-time.Duration(i*24) * time.Hour).Format(time.RFC3339),
			UpdatedAt:  now.Add(-time.Duration(i*24) * time.Hour).Format(time.RFC3339),
		}
	}

	planner := New(WithNow(now))
	result, err := planner.Plan(ctx, "test/repo", prs, 3, formula.ModeWithReplacement)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	// Check for uniqueness in selected PRs
	seen := make(map[int]bool)
	for _, candidate := range result.Selected {
		if seen[candidate.PRNumber] {
			t.Errorf("Duplicate PR %d in result.Selected", candidate.PRNumber)
		}
		seen[candidate.PRNumber] = true
	}
}

func TestPlanner_RejectionTracking(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	prs := make([]types.PR, 10)
	for i := 1; i <= 10; i++ {
		prs[i-1] = types.PR{
			Number:     i,
			Title:      "PR " + string(rune('A'+i-1)),
			CIStatus:   "success",
			Mergeable:  "mergeable",
			BaseBranch: "main",
			HeadBranch: "feature-" + string(rune('a'+i-1)),
			CreatedAt:  now.Add(-time.Duration(i*24) * time.Hour).Format(time.RFC3339),
			UpdatedAt:  now.Add(-time.Duration(i*24) * time.Hour).Format(time.RFC3339),
		}
	}

	planner := New(WithNow(now))
	result, err := planner.Plan(ctx, "test/repo", prs, 3, formula.ModeCombination)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	// PRs not selected should be in rejections
	selectedNums := make(map[int]bool)
	for _, c := range result.Selected {
		selectedNums[c.PRNumber] = true
	}

	for _, rej := range result.Rejections {
		if rej.Reason == "not selected by strategy" {
			if selectedNums[rej.PRNumber] {
				t.Errorf("PR %d in both selected and rejections", rej.PRNumber)
			}
		}
	}
}

func TestPlanner_FilterPipelineIntegration(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	prs := []types.PR{
		{Number: 1, Title: "Valid PR", CIStatus: "success", Mergeable: "mergeable"},
		{Number: 2, Title: "Draft PR", IsDraft: true},
		{Number: 3, Title: "Conflict PR", Mergeable: "conflicting"},
		{Number: 4, Title: "CI Failing PR", CIStatus: "failure"},
		{Number: 5, Title: "Another Valid PR", CIStatus: "success", Mergeable: "mergeable"},
	}

	planner := New(WithNow(now))
	result, err := planner.Plan(ctx, "test/repo", prs, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("Plan() unexpected error: %v", err)
	}

	// Check that draft, conflict, and CI-failing PRs are rejected
	rejectionReasons := make(map[int]string)
	for _, rej := range result.Rejections {
		rejectionReasons[rej.PRNumber] = rej.Reason
	}

	if reason, ok := rejectionReasons[2]; !ok || reason != "draft" {
		t.Errorf("PR 2 should be rejected as draft, got reason %q", reason)
	}
	if reason, ok := rejectionReasons[3]; !ok || reason != "merge conflict" {
		t.Errorf("PR 3 should be rejected as merge conflict, got reason %q", reason)
	}
	if reason, ok := rejectionReasons[4]; !ok || reason != "ci failure" {
		t.Errorf("PR 4 should be rejected as ci failure, got reason %q", reason)
	}
}

// Test helper to verify planner can be constructed with default options
func TestNewPlanner_Defaults(t *testing.T) {
	planner := New()
	if planner == nil {
		t.Fatal("New() returned nil")
	}

	// Fields are lazy-initialized, so they may be nil until first use
	// but the planner should still be functional
	if planner.now == nil {
		t.Error("now function should be initialized")
	}
	if planner.validator == nil {
		t.Error("validator should be initialized")
	}
}
