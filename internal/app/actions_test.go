package app

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/testutil"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestActionsBuildsAdvisoryActionPlan(t *testing.T) {
	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	fixedNow := func() time.Time { return time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC) }
	service := NewService(Config{Now: fixedNow, IncludeReview: true})

	plan, err := service.Actions(context.Background(), manifest.Repo, ActionOptions{PolicyProfile: types.PolicyProfileAdvisory})
	if err != nil {
		t.Fatalf("Actions: %v", err)
	}
	if plan.SchemaVersion != "2.0" {
		t.Fatalf("schema version = %q, want 2.0", plan.SchemaVersion)
	}
	if plan.Repo != manifest.Repo {
		t.Fatalf("repo = %q, want %q", plan.Repo, manifest.Repo)
	}
	if plan.PolicyProfile != types.PolicyProfileAdvisory {
		t.Fatalf("policy = %q, want advisory", plan.PolicyProfile)
	}
	if plan.CorpusSnapshot.TotalPRs == 0 {
		t.Fatal("expected corpus snapshot total PRs")
	}
	if len(plan.WorkItems) == 0 {
		t.Fatal("expected non-empty action work items")
	}
	if len(plan.Lanes) == 0 {
		t.Fatal("expected non-empty lane summaries")
	}
	laneTotal := 0
	for _, lane := range plan.Lanes {
		if lane.Count == 0 {
			t.Fatalf("lane %s has zero count", lane.Lane)
		}
		laneTotal += lane.Count
	}
	if laneTotal != len(plan.WorkItems) {
		t.Fatalf("lane total = %d, work items = %d", laneTotal, len(plan.WorkItems))
	}
	for _, item := range plan.WorkItems {
		if item.ID == "" || item.IdempotencyKey == "" {
			t.Fatalf("work item missing stable ids: %+v", item)
		}
		if item.Lane == "" {
			t.Fatalf("work item %s missing lane", item.ID)
		}
		if len(item.ReasonTrail) == 0 {
			t.Fatalf("work item %s missing reason trail", item.ID)
		}
	}
	for _, intent := range plan.ActionIntents {
		if !intent.DryRun {
			t.Fatalf("advisory intent %s is not dry_run", intent.ID)
		}
	}
	if len(plan.Audit.Checks) < 2 {
		t.Fatalf("expected lane/advisory audit checks, got %+v", plan.Audit.Checks)
	}
	for _, check := range plan.Audit.Checks {
		if check.Status != "pass" && check.Status != "passed" {
			t.Fatalf("audit check %s status=%s reason=%s", check.Name, check.Status, check.Reason)
		}
	}
}

func TestActionsLaneFilter(t *testing.T) {
	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	service := NewService(Config{Now: func() time.Time { return time.Date(2026, 4, 24, 9, 30, 0, 0, time.UTC) }, IncludeReview: true})

	plan, err := service.Actions(context.Background(), manifest.Repo, ActionOptions{PolicyProfile: types.PolicyProfileAdvisory, LaneFilter: string(types.ActionLaneFocusedReview)})
	if err != nil {
		t.Fatalf("Actions focused_review: %v", err)
	}
	for _, item := range plan.WorkItems {
		if item.Lane != types.ActionLaneFocusedReview {
			t.Fatalf("filtered plan contains lane %s", item.Lane)
		}
	}
	for _, summary := range plan.Lanes {
		if summary.Lane != types.ActionLaneFocusedReview {
			t.Fatalf("filtered summary contains lane %s", summary.Lane)
		}
	}
}

func TestActionsRejectsInvalidLaneFilter(t *testing.T) {
	service := NewService(Config{})
	_, err := service.Actions(context.Background(), "owner/repo", ActionOptions{LaneFilter: "not_a_lane"})
	if err == nil {
		t.Fatal("expected invalid lane filter error")
	}
}
