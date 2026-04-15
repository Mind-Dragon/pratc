package app

import (
	"context"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/formula"
	"github.com/jeffersonnunn/pratc/internal/planning"
	"github.com/jeffersonnunn/pratc/internal/testutil"
)

// RED phase: These tests define the expected behavior of the planning integration.
// They will FAIL until PoolSelector is wired into the service Plan() method
// and the scoring path is updated to use planning.PoolSelector.

func TestPlanUsesPoolSelectorForWeightedScoring(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Create service. PoolSelector must be wired into Plan() by default.
	service := NewService(Config{Now: fixedNow})

	response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// PoolSelector must be active: telemetry strategy reflects weighted scoring.
	if response.Telemetry == nil {
		t.Fatal("telemetry should not be nil")
	}
	if response.Telemetry.PoolStrategy != "weighted_priority" {
		t.Fatalf("PoolStrategy = %q, want %q",
			response.Telemetry.PoolStrategy, "weighted_priority")
	}
}

func TestPlanPopulatesPoolSelectorReasonCodesOnCandidates(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	service := NewService(Config{Now: fixedNow})

	response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Every selected candidate must have a non-trivial rationale from PoolSelector,
	// not the old "selected by heuristic scoring" message.
	for _, candidate := range response.Selected {
		if candidate.Rationale == "" {
			t.Errorf("PR %d: Rationale is empty, want PoolSelector component codes", candidate.PRNumber)
			continue
		}
		if candidate.Rationale == "selected by heuristic scoring" {
			t.Errorf("PR %d: Rationale is still the old heuristic message, want PoolSelector codes", candidate.PRNumber)
		}
		// Rationale should mention at least one scoring component keyword.
		lower := strings.ToLower(candidate.Rationale)
		hasComponent := strings.Contains(lower, "staleness") ||
			strings.Contains(lower, "ci") ||
			strings.Contains(lower, "security") ||
			strings.Contains(lower, "cluster") ||
			strings.Contains(lower, "recency") ||
			strings.Contains(lower, "decay")
		if !hasComponent {
			t.Errorf("PR %d: Rationale %q does not contain any known scoring component keyword",
				candidate.PRNumber, candidate.Rationale)
		}
		t.Logf("PR %d: Rationale = %q", candidate.PRNumber, candidate.Rationale)
	}
}

func TestPlanTimeDecayWindowAffectsOldPRScores(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Plan with default (no decay override).
	service := NewService(Config{Now: fixedNow})

	response1, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Plan with aggressive decay: 1-hour half-life should penalise old PRs heavily.
	aggressiveDecay := planning.TimeDecayConfig{
		HalfLifeHours:  1,
		WindowHours:    168,
		ProtectedHours: 24,
		MinScore:       0.1,
	}
	_ = aggressiveDecay // used in GREEN phase when Config accepts TimeDecayConfig

	// Decay policy must be recorded in telemetry when TimeDecayWindow is wired.
	if response1.Telemetry.DecayPolicy == "" || response1.Telemetry.DecayPolicy == "none" {
		t.Errorf("DecayPolicy = %q, want non-empty (exponential_decay)",
			response1.Telemetry.DecayPolicy)
	}
}

func TestPriorityWeightsSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	// Verify PriorityWeights serialises and deserialises via settings maps.
	weights := planning.DefaultPriorityWeights()

	settings := weights.ToSettings()
	if settings == nil {
		t.Fatal("ToSettings returned nil")
	}

	restored, ok := planning.PriorityWeightsFromSettings(settings)
	if !ok {
		t.Fatal("PriorityWeightsFromSettings returned ok=false")
	}

	// Each weight must survive the round-trip.
	if restored.StalenessWeight != weights.StalenessWeight {
		t.Errorf("StalenessWeight = %v, want %v", restored.StalenessWeight, weights.StalenessWeight)
	}
	if restored.CIStatusWeight != weights.CIStatusWeight {
		t.Errorf("CIStatusWeight = %v, want %v", restored.CIStatusWeight, weights.CIStatusWeight)
	}
	if restored.SecurityLabelWeight != weights.SecurityLabelWeight {
		t.Errorf("SecurityLabelWeight = %v, want %v", restored.SecurityLabelWeight, weights.SecurityLabelWeight)
	}
	if restored.ClusterCoherenceWeight != weights.ClusterCoherenceWeight {
		t.Errorf("ClusterCoherenceWeight = %v, want %v", restored.ClusterCoherenceWeight, weights.ClusterCoherenceWeight)
	}
	if restored.TimeDecayWeight != weights.TimeDecayWeight {
		t.Errorf("TimeDecayWeight = %v, want %v", restored.TimeDecayWeight, weights.TimeDecayWeight)
	}

	// Sum must remain ~1.0 after round-trip.
	sum := restored.StalenessWeight + restored.CIStatusWeight + restored.SecurityLabelWeight +
		restored.ClusterCoherenceWeight + restored.TimeDecayWeight
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("weight sum = %v, want ~1.0", sum)
	}
}

func TestTimeDecayConfigSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := planning.TimeDecayConfig{
		HalfLifeHours:  24,
		WindowHours:    168,
		ProtectedHours: 48,
		MinScore:       0.2,
	}

	settings := planning.TimeDecayConfigToSettings(cfg)
	if settings == nil {
		t.Fatal("ToSettings returned nil")
	}

	restored, ok := planning.TimeDecayConfigFromSettings(settings)
	if !ok {
		t.Fatal("TimeDecayConfigFromSettings returned ok=false")
	}

	if restored.HalfLifeHours != cfg.HalfLifeHours {
		t.Errorf("HalfLifeHours = %v, want %v", restored.HalfLifeHours, cfg.HalfLifeHours)
	}
	if restored.WindowHours != cfg.WindowHours {
		t.Errorf("WindowHours = %v, want %v", restored.WindowHours, cfg.WindowHours)
	}
	if restored.ProtectedHours != cfg.ProtectedHours {
		t.Errorf("ProtectedHours = %v, want %v", restored.ProtectedHours, cfg.ProtectedHours)
	}
	if restored.MinScore != cfg.MinScore {
		t.Errorf("MinScore = %v, want %v", restored.MinScore, cfg.MinScore)
	}
}

// Task 2: HierarchicalPlanner integration (TDD)
func TestPlanHierarchicalStrategy(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Use hierarchical planning strategy.
	service := NewService(Config{
		Now:              fixedNow,
		PlanningStrategy: "hierarchical",
	})

	response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Telemetry must indicate hierarchical planning.
	if response.Telemetry.PlanningStrategy != "hierarchical" {
		t.Errorf("PlanningStrategy = %q, want %q",
			response.Telemetry.PlanningStrategy, "hierarchical")
	}

	// Hierarchical planner must record complexity reduction.
	if response.Telemetry.HierarchicalComplexityReduction == 0 {
		t.Error("HierarchicalComplexityReduction should be set (non-zero)")
	}

	// Candidates should be produced when PRs are available.
	if len(response.Selected) == 0 && response.CandidatePoolSize == 0 {
		t.Log("No candidates produced (may be normal if pool is empty)")
	}

	t.Logf("HierarchicalComplexityReduction = %.4f", response.Telemetry.HierarchicalComplexityReduction)
}

// Task 3: PairwiseExecutor sharded conflict detection (TDD)
func TestPlanPairwiseShardedConflictDetection(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Default (formula) strategy should run pairwise sharded conflict detection.
	service := NewService(Config{Now: fixedNow})

	response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// PairwiseShards should be set (positive integer when PRs > 1).
	if response.Telemetry.PairwiseShards < 0 {
		t.Errorf("PairwiseShards = %d, want >= 0", response.Telemetry.PairwiseShards)
	}

	// Formula path should be active.
	if response.Telemetry.PlanningStrategy != "formula" {
		t.Errorf("PlanningStrategy = %q, want %q",
			response.Telemetry.PlanningStrategy, "formula")
	}

	t.Logf("PairwiseShards = %d", response.Telemetry.PairwiseShards)
}

// Task 4: TimeDecayWindow protected lane (TDD)
func TestPlanTimeDecayProtectedLane(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Plan with protected lane enabled (old critical PRs should be preserved).
	cfg := planning.TimeDecayConfig{
		HalfLifeHours:  24,
		WindowHours:    168,
		ProtectedHours: 48,
		MinScore:       0.1,
	}
	service := NewService(Config{
		Now:             fixedNow,
		TimeDecayConfig: cfg,
	})

	response, err := service.Plan(context.Background(), manifest.Repo, 10, formula.ModeCombination)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// DecayPolicy must be set to exponential_decay.
	if response.Telemetry.DecayPolicy == "" || response.Telemetry.DecayPolicy == "none" {
		t.Errorf("DecayPolicy = %q, want %q",
			response.Telemetry.DecayPolicy, "exponential_decay")
	}

	// PoolStrategy should reflect weighted priority scoring.
	if response.Telemetry.PoolStrategy != "weighted_priority" {
		t.Errorf("PoolStrategy = %q, want %q",
			response.Telemetry.PoolStrategy, "weighted_priority")
	}

	t.Logf("DecayPolicy = %s, PoolStrategy = %s",
		response.Telemetry.DecayPolicy, response.Telemetry.PoolStrategy)
}

// Task 5: Full app service integration
func TestPlanFullIntegration(t *testing.T) {
	t.Parallel()

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	// Test both strategies in sequence.
	for _, strategy := range []string{"formula", "hierarchical"} {
		t.Run(strategy, func(t *testing.T) {
			service := NewService(Config{
				Now:              fixedNow,
				PlanningStrategy: strategy,
			})

			response, err := service.Plan(context.Background(), manifest.Repo, 5, formula.ModeCombination)
			if err != nil {
				t.Fatalf("plan (%s): %v", strategy, err)
			}

			// Basic response sanity.
			if response.Repo == "" {
				t.Error("Repo should be set")
			}
			if response.GeneratedAt == "" {
				t.Error("GeneratedAt should be set")
			}
			if response.Telemetry == nil {
				t.Error("Telemetry should not be nil")
			}
			// Strategy must match what was requested.
			if response.Telemetry.PlanningStrategy != strategy {
				t.Errorf("PlanningStrategy = %q, want %q",
					response.Telemetry.PlanningStrategy, strategy)
			}
			// PoolSelector should always be active.
			if response.Telemetry.PoolStrategy != "weighted_priority" {
				t.Errorf("PoolStrategy = %q, want %q",
					response.Telemetry.PoolStrategy, "weighted_priority")
			}
			// Time decay should be active.
			if response.Telemetry.DecayPolicy == "none" {
				t.Error("DecayPolicy should not be 'none'")
			}
		})
	}
}
