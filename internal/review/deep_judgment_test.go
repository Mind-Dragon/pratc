package review

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// =============================================================================
// computeBlastRadius tests (Layer 8)
// =============================================================================

func TestComputeBlastRadius_Low(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:   1,
		FilesChanged: []string{"src/main.go"},
	}
	got := computeBlastRadius(pr, nil)
	if got != "low" {
		t.Errorf("1 file, 0 conflicts: got %q, want low", got)
	}
}

func TestComputeBlastRadius_Medium(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		pr     types.PR
		conflicts []types.ConflictPair
	}{
		{
			name: "5 files",
			pr: types.PR{
				Number:       1,
				FilesChanged: []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
			},
			conflicts: nil,
		},
		{
			name: "3 conflicts",
			pr: types.PR{
				Number:       1,
				FilesChanged: []string{"a.go"},
			},
			conflicts: []types.ConflictPair{
				{SourcePR: 1, TargetPR: 2},
				{SourcePR: 1, TargetPR: 3},
				{SourcePR: 1, TargetPR: 4},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeBlastRadius(tt.pr, tt.conflicts)
			if got != "medium" {
				t.Errorf("got %q, want medium", got)
			}
		})
	}
}

func TestComputeBlastRadius_High(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		pr     types.PR
		conflicts []types.ConflictPair
	}{
		{
			name: "20 files",
			pr: types.PR{
				Number:       1,
				FilesChanged: make([]string, 20),
			},
			conflicts: nil,
		},
		{
			name: "10 conflicts",
			pr: types.PR{
				Number:       1,
				FilesChanged: []string{"a.go"},
			},
			conflicts: make([]types.ConflictPair, 10),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := computeBlastRadius(tt.pr, tt.conflicts)
			if got != "high" {
				t.Errorf("got %q, want high", got)
			}
		})
	}
}

// =============================================================================
// computeLeverage tests (Layer 9)
// =============================================================================

func TestComputeLeverage_NoConflicts(t *testing.T) {
	t.Parallel()
	prData := PRData{
		PR:            types.PR{Number: 1},
		ConflictPairs: []types.ConflictPair{},
	}
	got := computeLeverage(prData)
	if got != 0.2 {
		t.Errorf("0 conflicts: got %f, want 0.2", got)
	}
}

func TestComputeLeverage_FewConflicts(t *testing.T) {
	t.Parallel()
	prData := PRData{
		PR: types.PR{Number: 1},
		ConflictPairs: []types.ConflictPair{
			{SourcePR: 1, TargetPR: 2},
			{SourcePR: 1, TargetPR: 3},
		},
	}
	got := computeLeverage(prData)
	if got != 0.5 {
		t.Errorf("2 conflicts: got %f, want 0.5", got)
	}
}

func TestComputeLeverage_ManyConflicts(t *testing.T) {
	t.Parallel()
	prData := PRData{
		PR: types.PR{Number: 1},
		ConflictPairs: []types.ConflictPair{
			{SourcePR: 1, TargetPR: 2},
			{SourcePR: 1, TargetPR: 3},
			{SourcePR: 1, TargetPR: 4},
			{SourcePR: 1, TargetPR: 5},
			{SourcePR: 1, TargetPR: 6},
		},
	}
	got := computeLeverage(prData)
	if got != 0.8 {
		t.Errorf("5+ conflicts: got %f, want 0.8", got)
	}
}

// =============================================================================
// computeHasOwner tests (Layer 10)
// =============================================================================

func TestComputeHasOwner_ActiveAuthor(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:    1,
		Author:    "alice",
		UpdatedAt: time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339), // 30 days ago
	}
	got := computeHasOwner(pr)
	if !got {
		t.Errorf("active author should have owner")
	}
}

func TestComputeHasOwner_BotAuthor(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number: 1,
		Author: "dependabot[bot]",
		IsBot:  true,
	}
	got := computeHasOwner(pr)
	if !got {
		t.Errorf("bot author should have owner")
	}
}

func TestComputeHasOwner_EmptyAuthor(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number: 1,
		Author: "",
	}
	got := computeHasOwner(pr)
	if got {
		t.Errorf("empty author should not have owner")
	}
}

// =============================================================================
// computeReversible tests (Layer 15)
// =============================================================================

func TestComputeReversible_ConfigFiles(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:       1,
		FilesChanged: []string{"config.yaml", "settings.json"},
	}
	got := computeReversible(pr)
	if !got {
		t.Errorf("config files should be reversible")
	}
}

func TestComputeReversible_SourceCode(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:       1,
		FilesChanged: []string{"main.go", "app.go"},
	}
	got := computeReversible(pr)
	if got {
		t.Errorf(".go source files should not be reversible")
	}
}

func TestComputeReversible_AuthPath(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:       1,
		FilesChanged: []string{"auth/jwt.go", "auth/session.go"},
	}
	got := computeReversible(pr)
	if got {
		t.Errorf("auth/ path files should not be reversible")
	}
}

// =============================================================================
// computeSubstanceScore tests
// =============================================================================

func TestComputeSubstanceScore_HighSubstance(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:            1,
		FilesChanged:      []string{"src/a.go", "src/b.go", "src/c.go", "src/d.go", "src/e.go", "tests/a_test.go"},
		ChangedFilesCount: 6,
		UpdatedAt:         time.Now().Add(-3 * 24 * time.Hour).Format(time.RFC3339),
		Additions:         220,
		Deletions:         40,
	}
	got := computeSubstanceScore(pr, nil, nil)
	if got < 70 {
		t.Fatalf("expected high substance score >= 70, got %d", got)
	}
}

func TestComputeSubstanceScore_LowSubstance(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:            1,
		FilesChanged:      []string{"docs/readme.md"},
		ChangedFilesCount: 1,
		UpdatedAt:         time.Now().Add(-220 * 24 * time.Hour).Format(time.RFC3339),
		Additions:         3,
		Deletions:         1,
	}
	findings := []types.AnalyzerFinding{{Confidence: 0.9}, {Confidence: 0.8}}
	stale := &types.StalenessReport{Score: 80}
	got := computeSubstanceScore(pr, findings, stale)
	if got > 25 {
		t.Fatalf("expected low substance score <= 25, got %d", got)
	}
}

func TestComputeSubstanceScore_ChangedFilesFallback(t *testing.T) {
	t.Parallel()
	pr := types.PR{
		Number:            1,
		ChangedFilesCount: 12,
		UpdatedAt:         time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		Additions:         140,
		Deletions:         10,
	}
	got := computeSubstanceScore(pr, nil, nil)
	if got < 25 {
		t.Fatalf("expected changed_files_count fallback to contribute meaningfully, got %d", got)
	}
}
