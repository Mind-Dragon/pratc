package review

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestBuildDecisionLayersProducesSixteenLayers(t *testing.T) {
	prData := PRData{
		PR: types.PR{
			Number:   42,
			Title:    "Bump dependency",
			Body:     "",
			Author:   "dependabot[bot]",
			IsDraft:  true,
			IsBot:    true,
			Mergeable: "conflicting",
			Additions: 120,
			Deletions: 40,
			FilesChanged: []string{"go.mod", "go.sum"},
		},
		Repo:         "owner/repo",
		ClusterID:     "cluster-1",
		ClusterLabel:  "cluster-1",
		RelatedPRs:    []types.PR{{Number: 7, Title: "Related PR"}},
		DuplicateGroups: []types.DuplicateGroup{{
			CanonicalPRNumber: 7,
			DuplicatePRNums:   []int{42},
			Reason:            "same feature area",
		}},
		ConflictPairs: []types.ConflictPair{{
			SourcePR:     42,
			TargetPR:     7,
			ConflictType: "dependency",
			Reason:       "base branch depends on head branch",
		}},
		Staleness: &types.StalenessReport{
			PRNumber: 42,
			Score:    82,
			Reasons:  []string{"inactive for months"},
		},
		AnalyzedAt: time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
	}

	safety := MergeSafetyResult{
		IsSafe:     false,
		Confidence: 0.41,
		Reasons:    []string{"merge conflicts"},
		Blockers:   []string{"CI failing"},
	}
	problem := ProblematicPRResult{
		IsProblematic: true,
		ProblemType:   "spam",
		Confidence:    0.91,
		Reasons:       []string{"bot author", "empty body"},
	}

	layers := buildDecisionLayers(prData, safety, problem, types.ReviewCategoryProblematicQuarantine, 0.41)
	if len(layers) != 16 {
		t.Fatalf("layers = %d, want 16", len(layers))
	}
	if layers[0].Layer != 1 || layers[0].Name != "Garbage" || layers[0].Bucket != "junk" {
		t.Fatalf("layer 1 = %#v, want garbage/junk", layers[0])
	}
	if !contains(layers[0].Reasons, "draft PR") || !contains(layers[0].Reasons, "bot-authored PR") {
		t.Fatalf("layer 1 reasons = %#v, want draft/bot reasons", layers[0].Reasons)
	}
	if layers[1].Layer != 2 || layers[1].Bucket != "duplicate" {
		t.Fatalf("layer 2 = %#v, want duplicate", layers[1])
	}
	if layers[5].Layer != 6 || layers[5].Name != "Confidence" {
		t.Fatalf("layer 6 = %#v, want confidence", layers[5])
	}
	if layers[15].Layer != 16 || layers[15].Bucket != "junk" {
		t.Fatalf("layer 16 = %#v, want junk signal quality", layers[15])
	}
	if !contains(layers[15].Reasons, "confidence 0.41") {
		t.Fatalf("layer 16 reasons = %#v, want confidence trail", layers[15].Reasons)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
