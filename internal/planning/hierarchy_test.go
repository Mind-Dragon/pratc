package planning

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestHierarchicalConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  HierarchicalConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
			config: HierarchicalConfig{
				MaxClusters:           5,
				MaxPerCluster:         10,
				TargetTotal:           20,
				UseDependencyOrdering: true,
			},
			wantErr: false,
		},
		{
			name: "invalid_max_clusters_zero",
			config: HierarchicalConfig{
				MaxClusters:   0,
				MaxPerCluster: 10,
				TargetTotal:   20,
			},
			wantErr: true,
			errMsg:  "max_clusters must be > 0",
		},
		{
			name: "invalid_max_per_cluster_zero",
			config: HierarchicalConfig{
				MaxClusters:   5,
				MaxPerCluster: 0,
				TargetTotal:   20,
			},
			wantErr: true,
			errMsg:  "max_per_cluster must be > 0",
		},
		{
			name: "invalid_target_total_zero",
			config: HierarchicalConfig{
				MaxClusters:   5,
				MaxPerCluster: 10,
				TargetTotal:   0,
			},
			wantErr: true,
			errMsg:  "target_total must be > 0",
		},
		{
			name: "insufficient_candidate_pool",
			config: HierarchicalConfig{
				MaxClusters:   2,
				MaxPerCluster: 5,
				TargetTotal:   20,
			},
			wantErr: true,
			errMsg:  "candidate pool (max_clusters × max_per_cluster) must be >= target_total",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Validate() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestDefaultHierarchicalConfig(t *testing.T) {
	cfg := DefaultHierarchicalConfig()

	if cfg.MaxClusters != 5 {
		t.Errorf("MaxClusters = %d, want 5", cfg.MaxClusters)
	}
	if cfg.MaxPerCluster != 10 {
		t.Errorf("MaxPerCluster = %d, want 10", cfg.MaxPerCluster)
	}
	if cfg.TargetTotal != 20 {
		t.Errorf("TargetTotal = %d, want 20", cfg.TargetTotal)
	}
	if !cfg.UseDependencyOrdering {
		t.Errorf("UseDependencyOrdering = false, want true")
	}
}

func TestNewHierarchicalPlanner(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()

	tests := []struct {
		name    string
		config  HierarchicalConfig
		wantErr bool
	}{
		{
			name: "valid_config",
			config: HierarchicalConfig{
				MaxClusters:   5,
				MaxPerCluster: 10,
				TargetTotal:   20,
			},
			wantErr: false,
		},
		{
			name: "invalid_config",
			config: HierarchicalConfig{
				MaxClusters:   0,
				MaxPerCluster: 10,
				TargetTotal:   20,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner, err := NewHierarchicalPlanner(ps, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewHierarchicalPlanner() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && planner == nil {
				t.Error("NewHierarchicalPlanner() returned nil planner")
			}
		})
	}
}

func TestHierarchicalPlanner_Plan_EmptyPRs(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()
	cfg := DefaultHierarchicalConfig()
	planner, err := NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		t.Fatalf("NewHierarchicalPlanner() error = %v", err)
	}

	ctx := context.Background()
	result := planner.Plan(ctx, "test/repo", []types.PR{}, DefaultTimeDecayConfig())

	if result == nil {
		t.Fatal("Plan() returned nil result")
	}
	if result.Repo != "test/repo" {
		t.Errorf("Repo = %v, want test/repo", result.Repo)
	}
	if len(result.FinalCandidates) != 0 {
		t.Errorf("FinalCandidates = %d, want 0", len(result.FinalCandidates))
	}
	if len(result.Ordering) != 0 {
		t.Errorf("Ordering = %d, want 0", len(result.Ordering))
	}
	if result.ComplexityReduction != 1.0 {
		t.Errorf("ComplexityReduction = %v, want 1.0", result.ComplexityReduction)
	}
}

func TestHierarchicalPlanner_Plan_UnclusteredPRs(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()
	cfg := HierarchicalConfig{
		MaxClusters:   3,
		MaxPerCluster: 5,
		TargetTotal:   5,
	}
	planner, err := NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		t.Fatalf("NewHierarchicalPlanner() error = %v", err)
	}

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	planner.poolSelector.Now = func() time.Time { return now }

	prs := []types.PR{
		{Number: 1, Title: "PR 1", ClusterID: "", CreatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 2, Title: "PR 2", ClusterID: "", CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 3, Title: "PR 3", ClusterID: "", CreatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), CIStatus: "pending"},
	}

	ctx := context.Background()
	result := planner.Plan(ctx, "test/repo", prs, DefaultTimeDecayConfig())

	if result == nil {
		t.Fatal("Plan() returned nil result")
	}
	// Unclustered PRs won't be selected since cluster selection requires ClusterID
	// This is expected behavior - hierarchical planning works with clusters
	if len(result.SelectedClusters) != 0 {
		t.Errorf("SelectedClusters = %d, want 0 (no clusters)", len(result.SelectedClusters))
	}
}

func TestHierarchicalPlanner_Plan_WithClusters(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()
	cfg := HierarchicalConfig{
		MaxClusters:   3,
		MaxPerCluster: 5,
		TargetTotal:   5,
	}
	planner, err := NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		t.Fatalf("NewHierarchicalPlanner() error = %v", err)
	}

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	planner.poolSelector.Now = func() time.Time { return now }

	// Create PRs in different clusters
	prs := []types.PR{
		// Cluster A - high priority (recent, CI passing)
		{Number: 1, Title: "PR 1A", ClusterID: "cluster-a", CreatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 2, Title: "PR 2A", ClusterID: "cluster-a", CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 3, Title: "PR 3A", ClusterID: "cluster-a", CreatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), CIStatus: "success"},

		// Cluster B - medium priority
		{Number: 4, Title: "PR 4B", ClusterID: "cluster-b", CreatedAt: now.Add(-96 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-96 * time.Hour).Format(time.RFC3339), CIStatus: "pending"},
		{Number: 5, Title: "PR 5B", ClusterID: "cluster-b", CreatedAt: now.Add(-120 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-120 * time.Hour).Format(time.RFC3339), CIStatus: "pending"},

		// Cluster C - low priority (old, CI failing)
		{Number: 6, Title: "PR 6C", ClusterID: "cluster-c", CreatedAt: now.Add(-168 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-168 * time.Hour).Format(time.RFC3339), CIStatus: "failure"},
		{Number: 7, Title: "PR 7C", ClusterID: "cluster-c", CreatedAt: now.Add(-192 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-192 * time.Hour).Format(time.RFC3339), CIStatus: "failure"},
	}

	ctx := context.Background()
	result := planner.Plan(ctx, "test/repo", prs, DefaultTimeDecayConfig())

	if result == nil {
		t.Fatal("Plan() returned nil result")
	}
	if len(result.SelectedClusters) == 0 {
		t.Error("SelectedClusters = 0, want > 0")
	}
	if len(result.FinalCandidates) == 0 {
		t.Error("FinalCandidates = 0, want > 0")
	}
	if len(result.FinalCandidates) > cfg.TargetTotal {
		t.Errorf("FinalCandidates = %d, want <= %d", len(result.FinalCandidates), cfg.TargetTotal)
	}

	// Verify complexity reduction is calculated
	if result.ComplexityReduction < 1.0 {
		t.Errorf("ComplexityReduction = %v, want >= 1.0", result.ComplexityReduction)
	}

	// Verify telemetry is populated
	if result.Telemetry.PoolStrategy != "hierarchical_three_level" {
		t.Errorf("PoolStrategy = %v, want hierarchical_three_level", result.Telemetry.PoolStrategy)
	}
	if len(result.Telemetry.StageLatenciesMS) == 0 {
		t.Error("StageLatenciesMS is empty")
	}
}

func TestHierarchicalPlanner_Plan_DeterministicOrdering(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()
	cfg := DefaultHierarchicalConfig()
	planner, err := NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		t.Fatalf("NewHierarchicalPlanner() error = %v", err)
	}

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	planner.poolSelector.Now = func() time.Time { return now }

	// Create PRs in clusters
	prs := []types.PR{
		{Number: 10, Title: "PR 10", ClusterID: "cluster-1", CreatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 5, Title: "PR 5", ClusterID: "cluster-1", CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 15, Title: "PR 15", ClusterID: "cluster-1", CreatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-72 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
	}

	ctx := context.Background()

	// Run multiple times to verify determinism
	var firstOrdering []int
	for i := 0; i < 3; i++ {
		result := planner.Plan(ctx, "test/repo", prs, DefaultTimeDecayConfig())

		currentOrdering := make([]int, len(result.Ordering))
		for j, c := range result.Ordering {
			currentOrdering[j] = c.PR.Number
		}

		if i == 0 {
			firstOrdering = currentOrdering
		} else {
			for j := range currentOrdering {
				if currentOrdering[j] != firstOrdering[j] {
					t.Errorf("Run %d: ordering[%d] = %d, want %d (non-deterministic)", i, j, currentOrdering[j], firstOrdering[j])
				}
			}
		}
	}
}

func TestHierarchicalPlanner_Plan_SecurityRelevance(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()
	cfg := HierarchicalConfig{
		MaxClusters:   2,
		MaxPerCluster: 5,
		TargetTotal:   5,
	}
	planner, err := NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		t.Fatalf("NewHierarchicalPlanner() error = %v", err)
	}

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	planner.poolSelector.Now = func() time.Time { return now }

	// Create PRs with security labels
	prs := []types.PR{
		{Number: 1, Title: "Security fix", ClusterID: "security", Labels: []string{"security", "critical"}, CreatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 2, Title: "CVE patch", ClusterID: "security", Labels: []string{"cve", "urgent"}, CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
		{Number: 3, Title: "Normal feature", ClusterID: "features", Labels: []string{"feature"}, CreatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), UpdatedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), CIStatus: "success"},
	}

	ctx := context.Background()
	result := planner.Plan(ctx, "test/repo", prs, DefaultTimeDecayConfig())

	if result == nil {
		t.Fatal("Plan() returned nil result")
	}

	// Security PRs should be prioritized
	if len(result.FinalCandidates) == 0 {
		t.Fatal("FinalCandidates is empty")
	}

	// First candidate should be from security cluster
	if len(result.FinalCandidates) > 0 && result.FinalCandidates[0].ClusterID != "security" {
		t.Errorf("First candidate cluster = %v, want security", result.FinalCandidates[0].ClusterID)
	}
}

func TestHierarchicalPlanner_ConvertToMergePlan(t *testing.T) {
	hr := &HierarchyResult{
		Repo:        "test/repo",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Config: HierarchicalConfig{
			MaxClusters:   5,
			MaxPerCluster: 10,
			TargetTotal:   20,
		},
		FinalCandidates: []HierarchicalCandidate{
			{
				PR:            types.PR{Number: 1, Title: "PR 1", FilesChanged: []string{"file1.go"}},
				ClusterID:     "cluster-1",
				PriorityScore: 0.85,
				Level1Rank:    0,
				Level2Rank:    1,
				Level3Rank:    1,
			},
			{
				PR:            types.PR{Number: 2, Title: "PR 2", FilesChanged: []string{"file2.go"}},
				ClusterID:     "cluster-1",
				PriorityScore: 0.75,
				Level1Rank:    0,
				Level2Rank:    2,
				Level3Rank:    2,
			},
		},
		Ordering: []HierarchicalCandidate{
			{
				PR:            types.PR{Number: 1, Title: "PR 1"},
				ClusterID:     "cluster-1",
				PriorityScore: 0.85,
				Level3Rank:    1,
			},
			{
				PR:            types.PR{Number: 2, Title: "PR 2"},
				ClusterID:     "cluster-1",
				PriorityScore: 0.75,
				Level3Rank:    2,
			},
		},
		ComplexityReduction: 100.0,
	}

	plan := hr.ConvertToMergePlan()

	if plan.PlanID != "hierarchical_plan" {
		t.Errorf("PlanID = %v, want hierarchical_plan", plan.PlanID)
	}
	if plan.Mode != "hierarchical_three_level" {
		t.Errorf("Mode = %v, want hierarchical_three_level", plan.Mode)
	}
	if len(plan.Selected) != 2 {
		t.Errorf("Selected = %d, want 2", len(plan.Selected))
	}
	if len(plan.Ordering) != 2 {
		t.Errorf("Ordering = %d, want 2", len(plan.Ordering))
	}
	if plan.TotalScore != 1.6 {
		t.Errorf("TotalScore = %v, want 1.6", plan.TotalScore)
	}
}

func TestHierarchicalPlanner_calculateComplexityReduction(t *testing.T) {
	ps := NewPoolSelectorWithDefaults()
	cfg := DefaultHierarchicalConfig()
	planner, err := NewHierarchicalPlanner(ps, cfg)
	if err != nil {
		t.Fatalf("NewHierarchicalPlanner() error = %v", err)
	}

	tests := []struct {
		name             string
		totalPRs         int
		selectedClusters int
		candidates       int
		wantMinReduction float64
	}{
		{
			name:             "small_scale",
			totalPRs:         100,
			selectedClusters: 5,
			candidates:       50,
			wantMinReduction: 1.0,
		},
		{
			name:             "medium_scale",
			totalPRs:         1000,
			selectedClusters: 10,
			candidates:       100,
			wantMinReduction: 10.0,
		},
		{
			name:             "large_scale",
			totalPRs:         6000,
			selectedClusters: 20,
			candidates:       200,
			wantMinReduction: 100.0,
		},
		{
			name:             "zero_prs",
			totalPRs:         0,
			selectedClusters: 0,
			candidates:       0,
			wantMinReduction: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reduction := planner.calculateComplexityReduction(tt.totalPRs, tt.selectedClusters, tt.candidates)
			if reduction < tt.wantMinReduction {
				t.Errorf("calculateComplexityReduction() = %v, want >= %v", reduction, tt.wantMinReduction)
			}
		})
	}
}

func TestHierarchyResult_GenerateWarnings(t *testing.T) {
	tests := []struct {
		name              string
		result            HierarchyResult
		wantWarningCount  int
		wantWarningSubstr string
	}{
		{
			name: "with_rejections",
			result: HierarchyResult{
				Rejections: []HierarchyRejection{
					{PRNumber: 1, Reason: "test"},
				},
				ComplexityReduction: 100.0,
			},
			wantWarningCount:  1,
			wantWarningSubstr: "rejected",
		},
		{
			name: "low_complexity_reduction",
			result: HierarchyResult{
				Rejections:          []HierarchyRejection{},
				ComplexityReduction: 5.0,
			},
			wantWarningCount:  1,
			wantWarningSubstr: "low complexity reduction",
		},
		{
			name: "high_complexity_reduction",
			result: HierarchyResult{
				Rejections:          []HierarchyRejection{},
				ComplexityReduction: 2e6,
			},
			wantWarningCount:  1,
			wantWarningSubstr: "very high complexity reduction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.result.generateWarnings()
			if len(warnings) != tt.wantWarningCount {
				t.Errorf("generateWarnings() = %d warnings, want %d", len(warnings), tt.wantWarningCount)
			}
			if tt.wantWarningSubstr != "" && len(warnings) > 0 {
				found := false
				for _, w := range warnings {
					if strings.Contains(w, tt.wantWarningSubstr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("generateWarnings() warnings = %v, want substring %q", warnings, tt.wantWarningSubstr)
				}
			}
		})
	}
}

func TestHierarchyError_Error(t *testing.T) {
	err := &HierarchyError{msg: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() = %v, want test error", err.Error())
	}
}
