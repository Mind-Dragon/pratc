package planning

import (
	"context"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPriorityWeights_Validate(t *testing.T) {
	tests := []struct {
		name    string
		weights PriorityWeights
		wantErr bool
	}{
		{
			name:    "valid default weights",
			weights: DefaultPriorityWeights(),
			wantErr: false,
		},
		{
			name: "valid custom weights",
			weights: PriorityWeights{
				StalenessWeight:        0.25,
				CIStatusWeight:         0.30,
				SecurityLabelWeight:    0.15,
				ClusterCoherenceWeight: 0.20,
				TimeDecayWeight:        0.10,
			},
			wantErr: false,
		},
		{
			name: "invalid weights sum",
			weights: PriorityWeights{
				StalenessWeight:        0.50,
				CIStatusWeight:         0.40,
				SecurityLabelWeight:    0.10,
				ClusterCoherenceWeight: 0.10,
				TimeDecayWeight:        0.10,
			},
			wantErr: true,
		},
		{
			name: "negative staleness weight",
			weights: PriorityWeights{
				StalenessWeight:        -0.1,
				CIStatusWeight:         0.40,
				SecurityLabelWeight:    0.30,
				ClusterCoherenceWeight: 0.20,
				TimeDecayWeight:        0.10,
			},
			wantErr: true,
		},
		{
			name: "weight greater than 1",
			weights: PriorityWeights{
				StalenessWeight:        1.5,
				CIStatusWeight:         0.25,
				SecurityLabelWeight:    0.15,
				ClusterCoherenceWeight: 0.10,
				TimeDecayWeight:        -0.0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.weights.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPoolSelector_SelectCandidates_Deterministic(t *testing.T) {
	weights := DefaultPriorityWeights()
	selector, err := NewPoolSelector(weights)
	if err != nil {
		t.Fatalf("NewPoolSelector() error = %v", err)
	}

	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := generateTestPRs(50)
	decayConfig := DefaultTimeDecayConfig()

	ctx := context.Background()

	result1 := selector.SelectCandidates(ctx, "owner/repo", prs, 20, decayConfig)
	result2 := selector.SelectCandidates(ctx, "owner/repo", prs, 20, decayConfig)

	if result1.GeneratedAt != result2.GeneratedAt {
		t.Error("Determinism violated: GeneratedAt differs between runs")
	}

	if result1.SelectedCount != result2.SelectedCount {
		t.Error("Determinism violated: SelectedCount differs between runs")
	}

	for i := range result1.Selected {
		if result1.Selected[i].PR.Number != result2.Selected[i].PR.Number {
			t.Errorf("Determinism violated at index %d: got PR %d, want PR %d",
				i, result1.Selected[i].PR.Number, result2.Selected[i].PR.Number)
		}
		if math.Abs(result1.Selected[i].PriorityScore-result2.Selected[i].PriorityScore) > 0.0001 {
			t.Errorf("Determinism violated at index %d: score differ", i)
		}
	}
}

func TestPoolSelector_SelectCandidates_RespectsTargetSize(t *testing.T) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := generateTestPRs(100)
	decayConfig := DefaultTimeDecayConfig()

	testCases := []struct {
		targetSize int
	}{
		{targetSize: 10},
		{targetSize: 25},
		{targetSize: 50},
		{targetSize: 100},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			result := selector.SelectCandidates(context.Background(), "owner/repo", prs, tc.targetSize, decayConfig)

			if result.SelectedCount != tc.targetSize {
				t.Errorf("SelectedCount = %d, want %d", result.SelectedCount, tc.targetSize)
			}

			if result.ExcludedCount != len(prs)-tc.targetSize {
				t.Errorf("ExcludedCount = %d, want %d", result.ExcludedCount, len(prs)-tc.targetSize)
			}
		})
	}
}

func TestPoolSelector_SelectCandidates_ReasonCodes(t *testing.T) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := []types.PR{
		{
			Number:    1,
			CIStatus:  "success",
			UpdatedAt: "2026-03-21T10:00:00Z",
			CreatedAt: "2026-03-21T08:00:00Z",
			Labels:    []string{"security", "bug"},
			ClusterID: "cluster-a",
		},
		{
			Number:    2,
			CIStatus:  "failure",
			UpdatedAt: "2026-03-20T10:00:00Z",
			CreatedAt: "2026-03-15T08:00:00Z",
			Labels:    []string{},
			ClusterID: "",
		},
		{
			Number:    3,
			CIStatus:  "pending",
			UpdatedAt: "2026-03-21T11:00:00Z",
			CreatedAt: "2026-03-21T09:00:00Z",
			Labels:    []string{"enhancement"},
			ClusterID: "cluster-b",
		},
	}

	decayConfig := DefaultTimeDecayConfig()
	result := selector.SelectCandidates(context.Background(), "owner/repo", prs, 3, decayConfig)

	if len(result.Selected) != 3 {
		t.Fatalf("Expected 3 selected, got %d", len(result.Selected))
	}

	for _, candidate := range result.Selected {
		if len(candidate.ReasonCodes) == 0 {
			t.Errorf("PR %d has no reason codes", candidate.PR.Number)
		}
	}

	hasSecurityRelevant := false
	for _, candidate := range result.Selected {
		for _, code := range candidate.ReasonCodes {
			if code == ReasonCodeSecurityRelevant {
				hasSecurityRelevant = true
			}
		}
	}
	if !hasSecurityRelevant {
		t.Error("Expected at least one PR to have security-relevant reason code")
	}
}

func TestPoolSelector_SelectCandidates_WeightsAffectOrdering(t *testing.T) {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	ciTestPRs := []types.PR{
		{
			Number:    1,
			CIStatus:  "success",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
		},
		{
			Number:    2,
			CIStatus:  "failure",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
		},
		{
			Number:    3,
			CIStatus:  "success",
			CreatedAt: "2026-03-10T08:00:00Z",
			UpdatedAt: "2026-03-10T10:00:00Z",
		},
	}

	stalePRs := []types.PR{
		{
			Number:    1,
			CIStatus:  "success",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
		},
		{
			Number:    2,
			CIStatus:  "success",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
		},
		{
			Number:    3,
			CIStatus:  "success",
			CreatedAt: "2026-03-10T08:00:00Z",
			UpdatedAt: "2026-03-10T10:00:00Z",
		},
	}

	securityPRs := []types.PR{
		{
			Number:    1,
			CIStatus:  "success",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
		},
		{
			Number:    2,
			CIStatus:  "success",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
		},
		{
			Number:    3,
			CIStatus:  "success",
			CreatedAt: "2026-03-21T08:00:00Z",
			UpdatedAt: "2026-03-21T10:00:00Z",
			Labels:    []string{"security"},
		},
	}

	decayConfig := DefaultTimeDecayConfig()

	t.Run("ci_weight_high", func(t *testing.T) {
		weights := PriorityWeights{
			StalenessWeight:        0.10,
			CIStatusWeight:         0.60,
			SecurityLabelWeight:    0.10,
			ClusterCoherenceWeight: 0.10,
			TimeDecayWeight:        0.10,
		}
		selector, _ := NewPoolSelector(weights)
		selector.Now = func() time.Time { return fixedTime }

		result := selector.SelectCandidates(context.Background(), "owner/repo", ciTestPRs, 3, decayConfig)

		if result.Selected[0].PR.Number != 1 {
			t.Errorf("Expected PR 1 first with high CI weight, got PR %d", result.Selected[0].PR.Number)
		}
	})

	t.Run("staleness_weight_high", func(t *testing.T) {
		weights := PriorityWeights{
			StalenessWeight:        0.60,
			CIStatusWeight:         0.10,
			SecurityLabelWeight:    0.10,
			ClusterCoherenceWeight: 0.10,
			TimeDecayWeight:        0.10,
		}
		selector, _ := NewPoolSelector(weights)
		selector.Now = func() time.Time { return fixedTime }

		result := selector.SelectCandidates(context.Background(), "owner/repo", stalePRs, 3, decayConfig)

		if result.Selected[0].PR.Number != 3 {
			t.Errorf("Expected PR 3 first with high staleness weight, got PR %d", result.Selected[0].PR.Number)
		}
	})

	t.Run("security_label_weight_high", func(t *testing.T) {
		weights := PriorityWeights{
			StalenessWeight:        0.10,
			CIStatusWeight:         0.10,
			SecurityLabelWeight:    0.60,
			ClusterCoherenceWeight: 0.10,
			TimeDecayWeight:        0.10,
		}
		selector, _ := NewPoolSelector(weights)
		selector.Now = func() time.Time { return fixedTime }

		result := selector.SelectCandidates(context.Background(), "owner/repo", securityPRs, 3, decayConfig)

		if result.Selected[0].PR.Number != 3 {
			t.Errorf("Expected PR 3 first with high security label weight, got PR %d", result.Selected[0].PR.Number)
		}
	})
}

func TestPoolSelector_ClusterCoherenceScoring(t *testing.T) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := []types.PR{
		{Number: 1, ClusterID: "cluster-a", CIStatus: "success", CreatedAt: "2026-03-21T08:00:00Z"},
		{Number: 2, ClusterID: "cluster-a", CIStatus: "success", CreatedAt: "2026-03-21T08:00:00Z"},
		{Number: 3, ClusterID: "cluster-a", CIStatus: "success", CreatedAt: "2026-03-21T08:00:00Z"},
		{Number: 4, ClusterID: "cluster-b", CIStatus: "success", CreatedAt: "2026-03-21T08:00:00Z"},
		{Number: 5, ClusterID: "", CIStatus: "success", CreatedAt: "2026-03-21T08:00:00Z"},
	}

	decayConfig := DefaultTimeDecayConfig()
	result := selector.SelectCandidatesWithClusterCoherence(context.Background(), "owner/repo", prs, 5, decayConfig)

	clusterACount := 0
	for _, c := range result.Selected {
		if c.PR.ClusterID == "cluster-a" {
			clusterACount++
		}
	}

	if clusterACount < 2 {
		t.Errorf("Expected cluster-a to have multiple PRs selected due to coherence boost, got %d", clusterACount)
	}
}

func TestPoolSelector_TimeDecayConfig(t *testing.T) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := []types.PR{
		{Number: 1, CreatedAt: "2026-03-21T08:00:00Z", CIStatus: "success"},
		{Number: 2, CreatedAt: "2026-03-18T08:00:00Z", CIStatus: "success"},
		{Number: 3, CreatedAt: "2026-03-01T08:00:00Z", CIStatus: "success"},
	}

	t.Run("recent_pr_high_score", func(t *testing.T) {
		config := TimeDecayConfig{
			HalfLifeHours:  72.0,
			WindowHours:    168.0,
			ProtectedHours: 336.0,
		}
		result := selector.SelectCandidates(context.Background(), "owner/repo", prs, 3, config)

		if result.Selected[0].PR.Number != 1 {
			t.Errorf("Expected PR 1 (most recent) first, got PR %d", result.Selected[0].PR.Number)
		}
	})

	t.Run("protected_lane_boost", func(t *testing.T) {
		config := TimeDecayConfig{
			HalfLifeHours:  24.0,
			WindowHours:    48.0,
			ProtectedHours: 72.0,
		}
		result := selector.SelectCandidates(context.Background(), "owner/repo", prs, 3, config)

		if result.Selected[0].PR.Number != 3 {
			t.Errorf("Expected PR 3 (protected lane) first, got PR %d", result.Selected[0].PR.Number)
		}
	})
}

func TestPoolSelector_EmptyPRList(t *testing.T) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	result := selector.SelectCandidates(context.Background(), "owner/repo", []types.PR{}, 10, DefaultTimeDecayConfig())

	if result.InputCount != 0 {
		t.Errorf("InputCount = %d, want 0", result.InputCount)
	}
	if result.SelectedCount != 0 {
		t.Errorf("SelectedCount = %d, want 0", result.SelectedCount)
	}
	if result.ExcludedCount != 0 {
		t.Errorf("ExcludedCount = %d, want 0", result.ExcludedCount)
	}
}

func TestPoolSelector_TargetLargerThanInput(t *testing.T) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := generateTestPRs(5)
	result := selector.SelectCandidates(context.Background(), "owner/repo", prs, 100, DefaultTimeDecayConfig())

	if result.SelectedCount != 5 {
		t.Errorf("SelectedCount = %d, want 5", result.SelectedCount)
	}
	if result.ExcludedCount != 0 {
		t.Errorf("ExcludedCount = %d, want 0", result.ExcludedCount)
	}
}

func TestValidatePoolResult(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		result := &PoolResult{
			InputCount:    10,
			SelectedCount: 5,
			ExcludedCount: 5,
			Selected: []PoolCandidate{
				{PriorityScore: 1.0}, {PriorityScore: 0.8},
			},
		}
		if err := ValidatePoolResult(result); err != nil {
			t.Errorf("ValidatePoolResult() error = %v, want nil", err)
		}
	})

	t.Run("nil result", func(t *testing.T) {
		if err := ValidatePoolResult(nil); err != ErrNilPoolResult {
			t.Errorf("ValidatePoolResult(nil) = %v, want %v", err, ErrNilPoolResult)
		}
	})

	t.Run("count mismatch", func(t *testing.T) {
		result := &PoolResult{
			InputCount:    10,
			SelectedCount: 5,
			ExcludedCount: 4,
		}
		if err := ValidatePoolResult(result); err != ErrPoolCountMismatch {
			t.Errorf("ValidatePoolResult() = %v, want %v", err, ErrPoolCountMismatch)
		}
	})

	t.Run("not sorted deterministically", func(t *testing.T) {
		result := &PoolResult{
			InputCount:    2,
			SelectedCount: 2,
			ExcludedCount: 0,
			Selected: []PoolCandidate{
				{PriorityScore: 0.5}, {PriorityScore: 0.8},
			},
		}
		if err := ValidatePoolResult(result); err != ErrPoolNotDeterministic {
			t.Errorf("ValidatePoolResult() = %v, want %v", err, ErrPoolNotDeterministic)
		}
	})
}

func TestMergePoolResults(t *testing.T) {
	now := time.Now().UTC()

	result1 := &PoolResult{
		Repo:          "owner/repo1",
		GeneratedAt:   now.Format(time.RFC3339),
		InputCount:    10,
		SelectedCount: 3,
		ExcludedCount: 7,
		Selected: []PoolCandidate{
			{PR: types.PR{Number: 1}, PriorityScore: 0.9},
			{PR: types.PR{Number: 2}, PriorityScore: 0.8},
			{PR: types.PR{Number: 3}, PriorityScore: 0.7},
		},
	}

	result2 := &PoolResult{
		Repo:          "owner/repo2",
		GeneratedAt:   now.Format(time.RFC3339),
		InputCount:    20,
		SelectedCount: 5,
		ExcludedCount: 15,
		Selected: []PoolCandidate{
			{PR: types.PR{Number: 101}, PriorityScore: 0.95},
			{PR: types.PR{Number: 102}, PriorityScore: 0.85},
			{PR: types.PR{Number: 103}, PriorityScore: 0.75},
			{PR: types.PR{Number: 104}, PriorityScore: 0.65},
			{PR: types.PR{Number: 105}, PriorityScore: 0.55},
		},
	}

	merged := MergePoolResults(result1, result2)

	if merged.InputCount != 30 {
		t.Errorf("InputCount = %d, want 30", merged.InputCount)
	}
	if merged.SelectedCount != 8 {
		t.Errorf("SelectedCount = %d, want 8", merged.SelectedCount)
	}

	sortedScores := make([]float64, len(merged.Selected))
	for i, c := range merged.Selected {
		sortedScores[i] = c.PriorityScore
	}
	if !sort.SliceIsSorted(sortedScores, func(i, j int) bool {
		return sortedScores[i] > sortedScores[j]
	}) {
		t.Error("Merged results are not sorted deterministically")
	}
}

func TestPriorityWeightsFromSettings(t *testing.T) {
	settings := map[string]any{
		"staleness_weight":         0.35,
		"ci_status_weight":         0.25,
		"security_label_weight":    0.15,
		"cluster_coherence_weight": 0.15,
		"time_decay_weight":        0.10,
	}

	weights, ok := PriorityWeightsFromSettings(settings)
	if !ok {
		t.Fatal("PriorityWeightsFromSettings returned false")
	}

	if math.Abs(weights.StalenessWeight-0.35) > 0.001 {
		t.Errorf("StalenessWeight = %v, want 0.35", weights.StalenessWeight)
	}
	if math.Abs(weights.CIStatusWeight-0.25) > 0.001 {
		t.Errorf("CIStatusWeight = %v, want 0.25", weights.CIStatusWeight)
	}
}

func TestNormalizeWeights(t *testing.T) {
	weights := PriorityWeights{
		StalenessWeight:        0.30,
		CIStatusWeight:         0.30,
		SecurityLabelWeight:    0.30,
		ClusterCoherenceWeight: 0.30,
		TimeDecayWeight:        0.30,
	}

	normalized := NormalizeWeights(weights)
	sum := normalized.StalenessWeight + normalized.CIStatusWeight + normalized.SecurityLabelWeight +
		normalized.ClusterCoherenceWeight + normalized.TimeDecayWeight

	if math.Abs(sum-1.0) > 0.001 {
		t.Errorf("Normalized weights sum to %v, want 1.0", sum)
	}
}

func TestPoolResult_Clone(t *testing.T) {
	result := &PoolResult{
		Repo:          "owner/repo",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		InputCount:    10,
		SelectedCount: 5,
		ExcludedCount: 5,
		Selected: []PoolCandidate{
			{PR: types.PR{Number: 1}, PriorityScore: 0.9},
		},
		Excluded: []ExclusionReason{
			{PR: types.PR{Number: 6}, Reason: "exceeded"},
		},
	}

	cloned := result.Clone()

	if cloned == result {
		t.Error("Clone returned same pointer")
	}

	if cloned.InputCount != result.InputCount {
		t.Errorf("Clone InputCount = %d, want %d", cloned.InputCount, result.InputCount)
	}

	if len(cloned.Selected) != len(result.Selected) {
		t.Errorf("Clone Selected length = %d, want %d", len(cloned.Selected), len(result.Selected))
	}

	if len(cloned.Excluded) != len(result.Excluded) {
		t.Errorf("Clone Excluded length = %d, want %d", len(cloned.Excluded), len(result.Excluded))
	}
}

func TestEqualWeights(t *testing.T) {
	w1 := DefaultPriorityWeights()
	w2 := DefaultPriorityWeights()

	if !EqualWeights(w1, w2, 0.001) {
		t.Error("Equal weights returned false for identical weights")
	}

	w2.StalenessWeight += 0.01
	if EqualWeights(w1, w2, 0.001) {
		t.Error("Equal weights returned true for different weights")
	}
}

func TestSettingsKeys(t *testing.T) {
	keys := SettingsKeys()

	expectedKeys := []string{
		"staleness_weight",
		"ci_status_weight",
		"security_label_weight",
		"cluster_coherence_weight",
		"time_decay_weight",
		"half_life_hours",
		"window_hours",
		"protected_hours",
		"min_score",
	}

	if len(keys) != len(expectedKeys) {
		t.Errorf("SettingsKeys() returned %d keys, want %d", len(keys), len(expectedKeys))
	}
}

func TestAddPoolKeysToSettings(t *testing.T) {
	allowed := map[string]struct{}{
		"existing_key": {},
	}

	result := AddPoolKeysToSettings(allowed)

	if _, ok := result["existing_key"]; !ok {
		t.Error("Existing key was lost")
	}

	for _, k := range SettingsKeys() {
		if _, ok := result[k]; !ok {
			t.Errorf("Key %s not added to settings", k)
		}
	}
}

func BenchmarkPoolSelector_SelectCandidates(b *testing.B) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := generateTestPRs(1000)
	decayConfig := DefaultTimeDecayConfig()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selector.SelectCandidates(ctx, "owner/repo", prs, 100, decayConfig)
	}
}

func BenchmarkPoolSelector_SelectCandidatesWithClusterCoherence(b *testing.B) {
	selector := NewPoolSelectorWithDefaults()
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	selector.Now = func() time.Time { return fixedTime }

	prs := generateTestPRs(500)
	decayConfig := DefaultTimeDecayConfig()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		selector.SelectCandidatesWithClusterCoherence(ctx, "owner/repo", prs, 100, decayConfig)
	}
}

func generateTestPRs(count int) []types.PR {
	fixedTime := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	prs := make([]types.PR, count)

	ciStatuses := []string{"success", "failure", "pending"}
	clusters := []string{"cluster-a", "cluster-b", "cluster-c", ""}
	labels := [][]string{
		{"security"},
		{"bug"},
		{"enhancement"},
		{"security", "urgent"},
		{},
	}

	for i := 0; i < count; i++ {
		prNum := i + 1
		hoursOld := float64(i * 2)

		prs[i] = types.PR{
			ID:        string(rune('A' + i%26)),
			Repo:      "owner/repo",
			Number:    prNum,
			Title:     "Test PR",
			CIStatus:  ciStatuses[i%len(ciStatuses)],
			ClusterID: clusters[i%len(clusters)],
			Labels:    labels[i%len(labels)],
			CreatedAt: fixedTime.Add(-time.Duration(hoursOld) * time.Hour).Format(time.RFC3339),
			UpdatedAt: fixedTime.Add(-time.Duration(hoursOld/2) * time.Hour).Format(time.RFC3339),
		}
	}

	return prs
}
