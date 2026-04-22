package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestLoadAnalystDataset_BuildsRowsAndRecommendations(t *testing.T) {
	tmp := t.TempDir()

	analyze := types.AnalysisResponse{
		Repo: "owner/repo",
		PRs: []types.PR{
			{Number: 11, Title: "Useful feature", Author: "alice", ClusterID: "c1", UpdatedAt: "2026-04-10T12:00:00Z"},
			{Number: 12, Title: "Duplicate feature", Author: "bob", ClusterID: "c1", UpdatedAt: "2026-04-01T12:00:00Z", IsBot: true},
			{Number: 13, Title: "Bump deps", Author: "dependabot[bot]", UpdatedAt: "2026-03-01T12:00:00Z", IsBot: true},
		},
		Duplicates: []types.DuplicateGroup{{
			CanonicalPRNumber: 11,
			DuplicatePRNums:   []int{12},
			Similarity:        0.97,
			Reason:            "same feature area",
		}},
		StalenessSignals: []types.StalenessReport{{
			PRNumber: 13,
			Score:    88,
			Reasons:  []string{"inactive for months"},
		}},
		ReviewPayload: &types.ReviewResponse{
			TotalPRs:    3,
			ReviewedPRs: 3,
			Results: []types.ReviewResult{
				{PRNumber: 11, Title: "Useful feature", Author: "alice", Category: types.ReviewCategoryMergeNow, PriorityTier: types.PriorityTierFastMerge, Confidence: 0.93, Reasons: []string{"approved", "CI passing"}, DecisionLayers: []types.DecisionLayer{{Layer: 1, Name: "Garbage", Bucket: "low_value", Status: "clear", Reasons: []string{"no garbage"}}, {Layer: 16, Name: "Signal quality", Bucket: "high_value", Status: "observed", Reasons: []string{"confidence 0.93"}}}, NextAction: "merge", ReclassifiedFrom: "low_value", ReclassificationReason: "quick win: small focused PR with passing CI", BatchTag: "typo-batch", AnalyzerFindings: []types.AnalyzerFinding{{AnalyzerName: "quality", AnalyzerVersion: "0.1.0", Finding: "production code changed without test evidence", Confidence: 0.75, Subsystem: "api", SignalType: "subsystem_tag"}}},
				{PRNumber: 12, Title: "Duplicate feature", Author: "bob", Category: types.ReviewCategoryDuplicateSuperseded, PriorityTier: types.PriorityTierBlocked, Confidence: 0.87, Reasons: []string{"duplicate"}, NextAction: "duplicate"},
				{PRNumber: 13, Title: "Bump deps", Author: "dependabot[bot]", Category: types.ReviewCategoryProblematicQuarantine, PriorityTier: types.PriorityTierBlocked, Confidence: 0.91, Reasons: []string{"bot author", "empty body"}, ProblemType: "junk", NextAction: "close", ReclassifiedFrom: "blocked", ReclassificationReason: "abandoned failing PR"},
			},
		},
	}
	plan := types.PlanResponse{
		Repo:              "owner/repo",
		Target:            10,
		CandidatePoolSize: 3,
		Selected:          []types.MergePlanCandidate{{PRNumber: 11, Title: "Useful feature", Score: 0.93, Reasons: []string{"approved"}}},
		Rejections:        []types.PlanRejection{{PRNumber: 12, Reason: "duplicate"}, {PRNumber: 13, Reason: "junk"}},
	}

	writeJSON(t, filepath.Join(tmp, "analyze.json"), analyze)
	writeJSON(t, filepath.Join(tmp, "step-5-plan.json"), plan)

	data, err := LoadAnalystDataset(tmp, "owner/repo")
	if err != nil {
		t.Fatalf("loadAnalystDataset: %v", err)
	}
	if len(data.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(data.Rows))
	}
	if len(data.Duplicates) != 1 {
		t.Fatalf("duplicates = %d, want 1", len(data.Duplicates))
	}
	if len(data.JunkRows) != 1 {
		t.Fatalf("junk rows = %d, want 1", len(data.JunkRows))
	}
	if len(data.TopUsefulRows) == 0 || data.TopUsefulRows[0].PRNumber != 11 {
		t.Fatalf("expected PR #11 as top useful row, got %#v", data.TopUsefulRows)
	}
	if data.CategoryCounts["junk"] != 1 {
		t.Fatalf("junk category count = %d, want 1", data.CategoryCounts["junk"])
	}
	if !containsSubstring(data.Rows[0].Reasons, "L1 Garbage:") {
		t.Fatalf("expected decision layer summary in reasons, got %#v", data.Rows[0].Reasons)
	}
	if !containsSubstring(data.Rows[0].Reasons, "quality/api") {
		t.Fatalf("expected analyzer finding summary in reasons, got %#v", data.Rows[0].Reasons)
	}
	if len(data.Rows[0].AnalyzerFindings) != 1 {
		t.Fatalf("expected analyzer findings to be preserved on analyst row, got %#v", data.Rows[0].AnalyzerFindings)
	}
	if len(data.ReclassifiedLowValue) != 1 || data.ReclassifiedLowValue[0].PRNumber != 11 {
		t.Fatalf("expected PR #11 in low-value reclassified rows, got %#v", data.ReclassifiedLowValue)
	}
	if len(data.ReclassifiedBlocked) != 1 || data.ReclassifiedBlocked[0].PRNumber != 13 {
		t.Fatalf("expected PR #13 in blocked reclassified rows, got %#v", data.ReclassifiedBlocked)
	}
	if len(data.BatchTagged) != 1 || data.BatchTagged[0].BatchTag != "typo-batch" {
		t.Fatalf("expected typo-batch row, got %#v", data.BatchTagged)
	}
}

func containsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

func writeJSON(t *testing.T, path string, v any) {
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
