package app

import (
	"context"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/testutil"
)

// TestPlanOptions_CandidatePoolCapLimitsPlannerInput verifies that
// CandidatePoolCap limits the number of candidates fed to the planner.
func TestPlanOptions_CandidatePoolCapLimitsPlannerInput(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, DynamicTarget: DynamicTargetConfig{Enabled: false}})

	// First, get baseline pool size with no cap.
	baseline, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target: 20,
		Mode:   formula.ModeCombination,
	})
	if err != nil {
		t.Fatalf("baseline plan: %v", err)
	}
	if baseline.CandidatePoolSize == 0 {
		t.Skip("fixture has no candidates — cannot test cap")
	}

	// Now cap at 5.
	capped, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target:           20,
		Mode:             formula.ModeCombination,
		CandidatePoolCap: 5,
	})
	if err != nil {
		t.Fatalf("capped plan: %v", err)
	}

	// The capped plan should have fewer or equal candidates in its pool.
	if capped.CandidatePoolSize > 5 {
		t.Fatalf("expected candidate pool <= 5, got %d", capped.CandidatePoolSize)
	}

	// Should have rejections explaining the cap.
	capRejections := 0
	for _, r := range capped.Rejections {
		if r.Reason == "excluded by candidate_pool_cap" {
			capRejections++
		}
	}
	if capRejections == 0 {
		t.Fatal("expected at least one candidate_pool_cap rejection")
	}
}

// TestPlanOptions_ScoreMinFiltersLowScoreCandidates verifies that
// ScoreMin filters out candidates below the minimum score threshold.
func TestPlanOptions_ScoreMinFiltersLowScoreCandidates(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, DynamicTarget: DynamicTargetConfig{Enabled: false}})

	// Set a very high score_min that should filter most candidates.
	response, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target:  20,
		Mode:    formula.ModeCombination,
		ScoreMin: 0.99, // very high threshold
	})
	if err != nil {
		t.Fatalf("plan with score_min: %v", err)
	}

	// With such a high threshold, the pool should be reduced.
	// Check that score_min rejections exist.
	scoreMinRejections := 0
	for _, r := range response.Rejections {
		if r.Reason == "below score_min threshold" {
			scoreMinRejections++
		}
	}
	if scoreMinRejections == 0 {
		t.Fatal("expected at least one score_min rejection with threshold 0.99")
	}
}

// TestPlanOptions_StaleScoreThresholdFiltersStaleCandidates verifies that
// StaleScoreThreshold uses staleness scoring to exclude stale candidates.
func TestPlanOptions_StaleScoreThresholdFiltersStaleCandidates(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, DynamicTarget: DynamicTargetConfig{Enabled: false}})

	// Set a very low threshold — most PRs will be "stale".
	response, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target:              20,
		Mode:                formula.ModeCombination,
		StaleScoreThreshold: 0.01, // almost everything is stale
	})
	if err != nil {
		t.Fatalf("plan with stale threshold: %v", err)
	}

	staleRejections := 0
	for _, r := range response.Rejections {
		if r.Reason == "above stale_score_threshold" {
			staleRejections++
		}
	}
	if staleRejections == 0 {
		t.Fatal("expected at least one stale_score_threshold rejection with threshold 0.01")
	}
}

// TestPlanOptions_DefaultsMatchLegacyPlan verifies that PlanWithOptions
// with zero-value options produces the same result as legacy Plan().
func TestPlanOptions_DefaultsMatchLegacyPlan(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, DynamicTarget: DynamicTargetConfig{Enabled: false}})

	legacy, err := service.Plan(context.Background(), manifest.Repo, 10, formula.ModeCombination)
	if err != nil {
		t.Fatalf("legacy plan: %v", err)
	}

	opts, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target: 10,
		Mode:   formula.ModeCombination,
	})
	if err != nil {
		t.Fatalf("opts plan: %v", err)
	}

	// Core fields should match.
	if legacy.Target != opts.Target {
		t.Fatalf("target mismatch: legacy=%d opts=%d", legacy.Target, opts.Target)
	}
	if legacy.CandidatePoolSize != opts.CandidatePoolSize {
		t.Fatalf("pool size mismatch: legacy=%d opts=%d", legacy.CandidatePoolSize, opts.CandidatePoolSize)
	}
	if len(legacy.Selected) != len(opts.Selected) {
		t.Fatalf("selected count mismatch: legacy=%d opts=%d", len(legacy.Selected), len(opts.Selected))
	}
	if len(legacy.Ordering) != len(opts.Ordering) {
		t.Fatalf("ordering count mismatch: legacy=%d opts=%d", len(legacy.Ordering), len(opts.Ordering))
	}
	if legacy.Strategy != opts.Strategy {
		t.Fatalf("strategy mismatch: legacy=%q opts=%q", legacy.Strategy, opts.Strategy)
	}
}

// TestPlanOptions_ExcludeConflictsFiltersConflictingCandidates verifies that
// when ExcludeConflicts is true, PRs with conflict warnings are removed.
func TestPlanOptions_ExcludeConflictsFiltersConflictingCandidates(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow, DynamicTarget: DynamicTargetConfig{Enabled: false}})

	// Run with and without ExcludeConflicts.
	without, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target: 20,
		Mode:   formula.ModeCombination,
	})
	if err != nil {
		t.Fatalf("plan without exclude: %v", err)
	}

	with, err := service.PlanWithOptions(context.Background(), manifest.Repo, PlanOptions{
		Target:           20,
		Mode:             formula.ModeCombination,
		ExcludeConflicts: true,
	})
	if err != nil {
		t.Fatalf("plan with exclude: %v", err)
	}

	// With ExcludeConflicts, the pool should be <= the baseline.
	if with.CandidatePoolSize > without.CandidatePoolSize {
		t.Fatalf("exclude_conflicts pool (%d) should be <= baseline (%d)",
			with.CandidatePoolSize, without.CandidatePoolSize)
	}

	// Check for conflict-based rejections if any conflicts existed.
	for _, r := range with.Rejections {
		if r.Reason == "conflict warning (exclude_conflicts)" {
			// Found expected rejection — test passes.
			return
		}
	}

	// If no conflicts existed in the fixture, that's also valid.
	t.Log("no conflict warnings found in fixture — exclude_conflicts had no effect (valid)")
}
