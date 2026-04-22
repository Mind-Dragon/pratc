package formula

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// TestAverageReviewStatusScore_CompleteCoverage tests C10: Incomplete ReviewStatus enum.
//
// BUG: scoring.go lines 86-100 shows that "pending" and "in_progress" review statuses
// are not handled in the switch statement. They fall through to 0 silently, which means
// PRs in these states get no score contribution from review status. These statuses should
// have defined scores (e.g., 0.5 for pending, 0.3 for in_progress).
func TestAverageReviewStatusScore_CompleteCoverage(t *testing.T) {
	tests := []struct {
		name          string
		prs           []types.PR
		wantMinScore  float64 // minimum expected score for the specific status
		wantMaxScore  float64 // maximum expected score
		statusHandled bool    // whether the status is explicitly handled
	}{
		{
			name: "approved gives positive score",
			prs: []types.PR{
				{ReviewStatus: "approved", Number: 1},
			},
			wantMinScore:  0.9,
			statusHandled: true,
		},
		{
			name: "changes_requested gives negative score",
			prs: []types.PR{
				{ReviewStatus: "changes_requested", Number: 1},
			},
			wantMinScore:  -0.9,
			statusHandled: true,
		},
		{
			name: "review_required gives zero score",
			prs: []types.PR{
				{ReviewStatus: "review_required", Number: 1},
			},
			wantMinScore:  -0.1,
			wantMaxScore:  0.1,
			statusHandled: true,
		},
		{
			name: "pending should not fall through to zero",
			prs: []types.PR{
				{ReviewStatus: "pending", Number: 1},
			},
			wantMinScore:  0.3, // pending should have some positive score
			statusHandled: false, // This is the bug - it's NOT handled
		},
		{
			name: "in_progress should not fall through to zero",
			prs: []types.PR{
				{ReviewStatus: "in_progress", Number: 1},
			},
			wantMinScore:  0.2, // in_progress should have defined score
			statusHandled: false, // This is the bug - it's NOT handled
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := averageReviewStatusScore(tc.prs)

			if tc.wantMinScore > 0 && got < tc.wantMinScore {
				t.Errorf("averageReviewStatusScore() for %q = %f, want >= %f (pending/in_progress should not be 0)",
					tc.prs[0].ReviewStatus, got, tc.wantMinScore)
			}
			if tc.wantMaxScore > 0 && got > tc.wantMaxScore {
				t.Errorf("averageReviewStatusScore() for %q = %f, want <= %f",
					tc.prs[0].ReviewStatus, got, tc.wantMaxScore)
			}

			// The bug is that pending/in_progress return 0, so we check if they're handled
			if !tc.statusHandled && got == 0 {
				t.Errorf("averageReviewStatusScore() for %q = 0 (BUG: falls through to default case)",
					tc.prs[0].ReviewStatus)
			}
		})
	}
}
