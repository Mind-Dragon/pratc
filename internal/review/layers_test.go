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

// TestGateJourneyOrdered verifies that gates are always ordered 1-16.
func TestGateJourneyOrdered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prData  PRData
		safety  MergeSafetyResult
		problem ProblematicPRResult
		cat     types.ReviewCategory
	}{
		{
			name: "draft PR exits at gate 1",
			prData: PRData{
				PR: types.PR{Number: 1, IsDraft: true, Title: "WIP"},
			},
			problem: ProblematicPRResult{IsProblematic: false},
			cat:    types.ReviewCategoryProblematicQuarantine,
		},
		{
			name: "spam PR exits at gate 3",
			prData: PRData{
				PR: types.PR{Number: 2, Title: "Buy now!", Body: "CLICK HERE"},
			},
			problem: ProblematicPRResult{IsProblematic: true, ProblemType: "spam"},
			cat:    types.ReviewCategoryProblematicQuarantine,
		},
		{
			name: "merge_now PR passes all gates",
			prData: PRData{
				PR: types.PR{
					Number:   3,
					Title:    "Add feature",
					Body:     "Implements X",
					Author:   "alice",
					IsDraft:  false,
					IsBot:    false,
					Mergeable: "true",
					Additions: 100,
					Deletions: 10,
					FilesChanged: []string{"feature.go"},
				},
			},
			safety:  MergeSafetyResult{IsSafe: true, Confidence: 0.9, Reasons: []string{"clean"}},
			problem: ProblematicPRResult{IsProblematic: false},
			cat:     types.ReviewCategoryMergeNow,
		},
		{
			name: "duplicate superseded continues to gate 16",
			prData: PRData{
				PR: types.PR{
					Number:   4,
					Title:    "Fix bug",
					Body:     "Fixes the thing",
					Author:   "bob",
					IsDraft:  false,
					IsBot:    false,
				},
				DuplicateGroups: []types.DuplicateGroup{{CanonicalPRNumber: 1}},
			},
			problem: ProblematicPRResult{IsProblematic: false},
			cat:     types.ReviewCategoryDuplicateSuperseded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layers := buildDecisionLayers(tt.prData, tt.safety, tt.problem, tt.cat, 0.8)
			if len(layers) != 16 {
				t.Fatalf("expected 16 layers, got %d", len(layers))
			}
			for i, layer := range layers {
				if layer.Layer != i+1 {
					t.Errorf("layer[%d].Layer = %d, want %d", i, layer.Layer, i+1)
				}
			}
		})
	}
}

// TestGateJourneyCostTiers verifies cost tier assignment matches gate position.
func TestGateJourneyCostTiers(t *testing.T) {
	t.Parallel()

	prData := PRData{
		PR: types.PR{
			Number:   1,
			Title:    "Real PR",
			Body:     "Description",
			Author:   "alice",
			IsDraft:  false,
			IsBot:    false,
			Mergeable: "true",
		},
	}
	safety := MergeSafetyResult{IsSafe: true, Confidence: 0.85}
	problem := ProblematicPRResult{IsProblematic: false}

	layers := buildDecisionLayers(prData, safety, problem, types.ReviewCategoryMergeNow, 0.85)

	// Gates 1-3 are cheap
	for i := 0; i < 3; i++ {
		if layers[i].CostTier != "cheap" {
			t.Errorf("layer %d: CostTier = %q, want cheap", i+1, layers[i].CostTier)
		}
	}
	// Gates 4-5 are medium
	for i := 3; i < 5; i++ {
		if layers[i].CostTier != "medium" {
			t.Errorf("layer %d: CostTier = %q, want medium", i+1, layers[i].CostTier)
		}
	}
	// Gates 6-16 are expensive
	for i := 5; i < 16; i++ {
		if layers[i].CostTier != "expensive" {
			t.Errorf("layer %d: CostTier = %q, want expensive", i+1, layers[i].CostTier)
		}
	}
}

// TestGateJourneyEarlyExitSemantics verifies Terminal and Continued flags for early exits.
func TestGateJourneyEarlyExitSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		prData        PRData
		problem       ProblematicPRResult
		wantTerminal  int    // gate number where Terminal=true
		wantContinued []int  // gate numbers where Continued=true
	}{
		{
			name: "draft PR exits at gate 1",
			prData: PRData{
				PR: types.PR{Number: 1, IsDraft: true, Title: "WIP", Body: "work in progress"},
			},
			problem:      ProblematicPRResult{IsProblematic: false},
			wantTerminal: 1,
			wantContinued: []int{}, // nothing continues
		},
		{
			name: "bot PR exits at gate 1",
			prData: PRData{
				PR: types.PR{Number: 2, IsBot: true, Title: "auto", Body: "automated"},
			},
			problem:      ProblematicPRResult{IsProblematic: false},
			wantTerminal: 1,
			wantContinued: []int{},
		},
		{
			name: "spam PR exits at gate 3",
			prData: PRData{
				PR: types.PR{Number: 3, Title: "Buy now!!!", Body: "CLICK HERE NOW"},
			},
			problem:      ProblematicPRResult{IsProblematic: true, ProblemType: "spam"},
			wantTerminal: 3,
			wantContinued: []int{1, 2},
		},
		{
			name: "broken PR exits at gate 3",
			prData: PRData{
				PR: types.PR{Number: 4, Title: "Fix", Body: "broken fix"},
			},
			problem:      ProblematicPRResult{IsProblematic: true, ProblemType: "broken"},
			wantTerminal: 3,
			wantContinued: []int{1, 2},
		},
		{
			name: "suspicious PR exits at gate 3",
			prData: PRData{
				PR: types.PR{Number: 5, Title: "Update", Body: "suspicious update"},
			},
			problem:      ProblematicPRResult{IsProblematic: true, ProblemType: "suspicious"},
			wantTerminal: 3,
			wantContinued: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safety := MergeSafetyResult{IsSafe: true, Confidence: 0.5}
			cat := types.ReviewCategoryProblematicQuarantine
			if !tt.problem.IsProblematic {
				cat = types.ReviewCategoryMergeNow
			}
			layers := buildDecisionLayers(tt.prData, safety, tt.problem, cat, 0.5)

			// Verify terminal gate
			terminalCount := 0
			for _, l := range layers {
				if l.Terminal {
					terminalCount++
					if l.Layer != tt.wantTerminal {
						t.Errorf("Terminal=true at layer %d, want %d", l.Layer, tt.wantTerminal)
					}
				}
			}
			if terminalCount != 1 {
				t.Errorf("exactly one terminal gate expected, got %d", terminalCount)
			}

			// Verify continued gates
			for _, g := range tt.wantContinued {
				if !layers[g-1].Continued {
					t.Errorf("gate %d: Continued=false, want true", g)
				}
			}
			// Gates after terminal should not continue
			for i := tt.wantTerminal; i < 16; i++ {
				if layers[i].Continued {
					t.Errorf("gate %d: Continued=true after terminal gate %d", i+1, tt.wantTerminal)
				}
			}
		})
	}
}

// TestGateJourneyFullTraversal verifies Terminal=true only at gate 16 for non-disposal PRs.
func TestGateJourneyFullTraversal(t *testing.T) {
	t.Parallel()

	prData := PRData{
		PR: types.PR{
			Number:    1,
			Title:     "Good PR",
			Body:      "Description",
			Author:    "alice",
			IsDraft:   false,
			IsBot:     false,
			Mergeable: "true",
			Additions: 100,
			Deletions: 10,
			FilesChanged: []string{"feature.go"},
		},
	}
	safety := MergeSafetyResult{IsSafe: true, Confidence: 0.85, Reasons: []string{"clean"}}
	problem := ProblematicPRResult{IsProblematic: false}

	layers := buildDecisionLayers(prData, safety, problem, types.ReviewCategoryMergeNow, 0.85)

	// Only gate 16 should be terminal
	terminalGates := 0
	for i, l := range layers {
		if l.Terminal {
			terminalGates++
			if l.Layer != 16 {
				t.Errorf("unexpected terminal at gate %d, want gate 16", l.Layer)
			}
		}
		if l.Continued && i < 15 {
			// gates 1-15 should continue
			continue
		}
		if !l.Continued && i < 15 {
			t.Errorf("gate %d: Continued=false before terminal gate 16", i+1)
		}
	}
	if terminalGates != 1 {
		t.Errorf("exactly one terminal gate expected, got %d", terminalGates)
	}

	// Gate 16 should be terminal and not continue
	if !layers[15].Terminal {
		t.Errorf("gate 16 should be terminal")
	}
	if layers[15].Continued {
		t.Errorf("gate 16 should not continue")
	}
}

// TestGateJourneyDuplicateSuperseded verifies duplicate category continues to gate 16.
func TestGateJourneyDuplicateSuperseded(t *testing.T) {
	t.Parallel()

	prData := PRData{
		PR: types.PR{
			Number:   1,
			Title:    "Dup PR",
			Body:     "Duplicate",
			Author:   "bob",
			IsDraft:  false,
			IsBot:    false,
			Mergeable: "true",
		},
		DuplicateGroups: []types.DuplicateGroup{{
			CanonicalPRNumber: 42,
			Reason:            "same feature area",
		}},
	}
	safety := MergeSafetyResult{IsSafe: true, Confidence: 0.7}
	problem := ProblematicPRResult{IsProblematic: false}

	layers := buildDecisionLayers(prData, safety, problem, types.ReviewCategoryDuplicateSuperseded, 0.7)

	// Duplicates currently pass all gates to gate 16
	if !layers[15].Terminal {
		t.Errorf("duplicate PR should be terminal at gate 16")
	}
	if layers[15].Continued {
		t.Errorf("gate 16 should not continue")
	}
	// Gates 1-15 should continue
	for i := 0; i < 15; i++ {
		if !layers[i].Continued {
			t.Errorf("gate %d: should continue for duplicate PR", i+1)
		}
		if layers[i].Terminal {
			t.Errorf("gate %d: should not be terminal for duplicate PR", i+1)
		}
	}
}

// TestGateJourneyAllGatesHaveRequiredFields verifies every gate has all required fields.
func TestGateJourneyAllGatesHaveRequiredFields(t *testing.T) {
	t.Parallel()

	prData := PRData{
		PR: types.PR{
			Number:    1,
			Title:     "Test PR",
			Body:      "Body",
			Author:    "alice",
			IsDraft:   false,
			IsBot:     false,
			Mergeable: "true",
			Additions: 50,
			Deletions: 10,
			FilesChanged: []string{"main.go"},
		},
	}
	safety := MergeSafetyResult{IsSafe: true, Confidence: 0.8}
	problem := ProblematicPRResult{IsProblematic: false}

	layers := buildDecisionLayers(prData, safety, problem, types.ReviewCategoryMergeNow, 0.8)

	for i, l := range layers {
		if l.Layer == 0 {
			t.Errorf("gate %d: Layer not set", i+1)
		}
		if l.Name == "" {
			t.Errorf("gate %d: Name not set", i+1)
		}
		if l.CostTier == "" {
			t.Errorf("gate %d: CostTier not set", i+1)
		}
		if l.Status == "" {
			t.Errorf("gate %d: Status not set", i+1)
		}
		// Reasons can be empty but should not be nil
		if l.Reasons == nil {
			t.Errorf("gate %d: Reasons is nil, want empty slice", i+1)
		}
	}
}
