package filter

import (
	"math"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestPlannerPriority(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		pr        types.PR
		wantScore float64
		tolerance float64
	}{
		{
			name: "CI success approved mergeable",
			pr: types.PR{
				Number:       1,
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-03-21T10:00:00Z",
			},
			wantScore: 6.0,
			tolerance: 0.01,
		},
		{
			name: "CI failure changes requested",
			pr: types.PR{
				Number:       2,
				CIStatus:     "failure",
				ReviewStatus: "changes_requested",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-03-21T10:00:00Z",
			},
			wantScore: -3.0,
			tolerance: 0.01,
		},
		{
			name: "CI pending no review",
			pr: types.PR{
				Number:       3,
				CIStatus:     "pending",
				ReviewStatus: "",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-03-21T10:00:00Z",
			},
			wantScore: 2.0,
			tolerance: 0.01,
		},
		{
			name: "bot PR with CI success",
			pr: types.PR{
				Number:       4,
				CIStatus:     "success",
				ReviewStatus: "",
				Mergeable:    "mergeable",
				IsBot:        true,
				UpdatedAt:    "2026-03-21T10:00:00Z",
			},
			wantScore: 4.5,
			tolerance: 0.01,
		},
		{
			name: "old PR age bonus",
			pr: types.PR{
				Number:       5,
				CIStatus:     "success",
				ReviewStatus: "",
				Mergeable:    "mergeable",
				UpdatedAt:    "2026-03-06T12:00:00Z",
			},
			wantScore: 5.0,
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlannerPriority(tt.pr, now)
			diff := math.Abs(got - tt.wantScore)
			if diff > tt.tolerance {
				t.Errorf("PlannerPriority() = %v, want %v (diff %v > tolerance %v)", got, tt.wantScore, diff, tt.tolerance)
			}
		})
	}
}

func TestPlannerRationale(t *testing.T) {
	tests := []struct {
		name string
		pr   types.PR
		want string
	}{
		{
			name: "CI success only",
			pr: types.PR{
				CIStatus: "success",
			},
			want: "CI passing",
		},
		{
			name: "CI success and approved",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "approved",
			},
			want: "CI passing; review approved",
		},
		{
			name: "CI success approved mergeable",
			pr: types.PR{
				CIStatus:     "success",
				ReviewStatus: "approved",
				Mergeable:    "mergeable",
			},
			want: "CI passing; review approved; mergeable",
		},
		{
			name: "bot PR",
			pr: types.PR{
				CIStatus: "success",
				IsBot:    true,
			},
			want: "CI passing; bot update",
		},
		{
			name: "no special conditions",
			pr: types.PR{
				CIStatus:     "",
				ReviewStatus: "",
				Mergeable:    "",
				IsBot:        false,
			},
			want: "selected by heuristic scoring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlannerRationale(tt.pr)
			if got != tt.want {
				t.Errorf("PlannerRationale() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScoreAndSortPool(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{
			Number:       3,
			CIStatus:     "success",
			ReviewStatus: "approved",
			Mergeable:    "mergeable",
			IsBot:        true,
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
		{
			Number:       1,
			CIStatus:     "success",
			ReviewStatus: "",
			Mergeable:    "",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
		{
			Number:       2,
			CIStatus:     "failure",
			ReviewStatus: "changes_requested",
			Mergeable:    "conflicting",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
	}

	pool := ScoreAndSortPool(prs, now)

	if len(pool) != 3 {
		t.Fatalf("ScoreAndSortPool() pool size = %d, want 3", len(pool))
	}

	if pool[0].Number != 3 {
		t.Errorf("First PR = %d, want 3 (highest score)", pool[0].Number)
	}

	if pool[1].Number != 1 {
		t.Errorf("Second PR = %d, want 1", pool[1].Number)
	}

	if pool[2].Number != 2 {
		t.Errorf("Third PR = %d, want 2 (lowest score)", pool[2].Number)
	}
}

func TestScoreAndSortPool_Deterministic(t *testing.T) {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)

	prs := []types.PR{
		{
			Number:       2,
			CIStatus:     "success",
			ReviewStatus: "",
			Mergeable:    "",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
		{
			Number:       1,
			CIStatus:     "success",
			ReviewStatus: "",
			Mergeable:    "",
			UpdatedAt:    "2026-03-21T10:00:00Z",
		},
	}

	pool1 := ScoreAndSortPool(prs, now)
	pool2 := ScoreAndSortPool(prs, now)

	if len(pool1) != len(pool2) {
		t.Fatal("ScoreAndSortPool() results differ in length")
	}

	for i := range pool1 {
		if pool1[i].Number != pool2[i].Number {
			t.Errorf("Position %d: PR %d vs PR %d (not deterministic)", i, pool1[i].Number, pool2[i].Number)
		}
	}
}
