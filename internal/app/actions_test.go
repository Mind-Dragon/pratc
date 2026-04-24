package app

import (
	"context"
	"testing"
	"time"

	actionpkg "github.com/jeffersonnunn/pratc/internal/actions"
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

func TestActionsRejectsEmptyReasonTrail(t *testing.T) {
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

	for _, intent := range plan.ActionIntents {
		if len(intent.Reasons) == 0 {
			t.Fatalf("intent %s for PR #%d has empty reasons", intent.ID, intent.PRNumber)
		}
	}
}

func TestActionsBuildsCompleteActionIntents(t *testing.T) {
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

	for _, intent := range plan.ActionIntents {
		if intent.PolicyProfile == "" {
			t.Fatalf("intent %s missing policy profile", intent.ID)
		}
		if intent.Confidence <= 0 {
			t.Fatalf("intent %s invalid confidence %f", intent.ID, intent.Confidence)
		}
		if len(intent.Reasons) == 0 {
			t.Fatalf("intent %s missing reasons", intent.ID)
		}
		if len(intent.EvidenceRefs) == 0 {
			t.Fatalf("intent %s missing evidence refs", intent.ID)
		}
		if intent.Preconditions == nil {
			t.Fatalf("intent %s has nil preconditions", intent.ID)
		}
		if intent.IdempotencyKey == "" {
			t.Fatalf("intent %s missing idempotency key", intent.ID)
		}
	}
}

func TestActionIntentsFromDecisionAddsCompletenessFallbacks(t *testing.T) {
	decision := actionpkg.LaneDecision{
		PRNumber:       42,
		Lane:           types.ActionLaneFocusedReview,
		Confidence:     0.70,
		AllowedActions: []types.ActionKind{types.ActionKindComment},
	}
	gate := actionpkg.PolicyGateResult{
		Profile:         types.PolicyProfileAdvisory,
		DryRun:          true,
		ProposedActions: []types.ActionKind{types.ActionKindComment},
	}

	intents := actionIntentsFromDecision("owner/repo", "run-1", "2026-04-24T09:00:00Z", decision, gate, true)
	if len(intents) != 1 {
		t.Fatalf("intent count = %d, want 1", len(intents))
	}
	intent := intents[0]
	if len(intent.Reasons) == 0 {
		t.Fatal("expected fallback reason")
	}
	if len(intent.EvidenceRefs) == 0 {
		t.Fatal("expected fallback evidence ref")
	}
	if intent.Preconditions == nil {
		t.Fatal("expected non-nil preconditions slice")
	}
	if intent.PolicyProfile != types.PolicyProfileAdvisory {
		t.Fatalf("policy profile = %q, want advisory", intent.PolicyProfile)
	}
}
