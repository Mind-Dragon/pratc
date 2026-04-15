package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestLoadReviewSection_TracksRiskBuckets(t *testing.T) {
	tmp := t.TempDir()
	analyze := types.AnalysisResponse{
		Repo: "owner/repo",
		ReviewPayload: &types.ReviewResponse{
			TotalPRs:    2,
			ReviewedPRs: 2,
			Buckets: []types.BucketCount{
				{Bucket: "now", Count: 1},
				{Bucket: "future", Count: 1},
			},
			PriorityTiers: []types.PriorityTierCount{
				{Tier: "fast_merge", Count: 1},
				{Tier: "review_required", Count: 1},
			},
			RiskBuckets: []types.BucketCount{
				{Bucket: "security_risk", Count: 1},
				{Bucket: "reliability_risk", Count: 2},
				{Bucket: "performance_risk", Count: 3},
			},
		},
	}
	writeReviewJSON(t, filepath.Join(tmp, "analyze.json"), analyze)

	section, err := LoadReviewSection(tmp, "owner/repo")
	if err != nil {
		t.Fatalf("LoadReviewSection: %v", err)
	}
	if section.Dashboard.SecurityRisk != 1 {
		t.Fatalf("security risk = %d, want 1", section.Dashboard.SecurityRisk)
	}
	if section.Dashboard.ReliabilityRisk != 2 {
		t.Fatalf("reliability risk = %d, want 2", section.Dashboard.ReliabilityRisk)
	}
	if section.Dashboard.PerformanceRisk != 3 {
		t.Fatalf("performance risk = %d, want 3", section.Dashboard.PerformanceRisk)
	}
}

func writeReviewJSON(t *testing.T, path string, v any) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}
