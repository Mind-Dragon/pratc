package filter

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// PlannerPriority calculates a priority score for a PR based on CI status,
// review status, mergeability, age, and bot status.
func PlannerPriority(pr types.PR, now time.Time) float64 {
	score := 0.0

	// CI status scoring
	switch pr.CIStatus {
	case "success":
		score += 3
	case "pending", "unknown":
		score += 1
	case "failure":
		score -= 2
	}

	// Review status scoring
	switch pr.ReviewStatus {
	case "approved":
		score += 2
	case "changes_requested":
		score -= 2
	}

	// Mergeability scoring
	if pr.Mergeable == "mergeable" {
		score += 1
	}

	// Age scoring - older PRs get a small boost
	if updatedAt, err := time.Parse(time.RFC3339, pr.UpdatedAt); err == nil {
		ageDays := now.Sub(updatedAt).Hours() / 24
		score += math.Min(ageDays/15, 2)
	}

	// Bot PRs get a small boost
	if pr.IsBot {
		score += 0.5
	}

	return score
}

// PlannerRationale returns a human-readable explanation of why a PR was selected.
func PlannerRationale(pr types.PR) string {
	parts := []string{}

	if pr.CIStatus == "success" {
		parts = append(parts, "CI passing")
	}
	if pr.ReviewStatus == "approved" {
		parts = append(parts, "review approved")
	}
	if pr.Mergeable == "mergeable" {
		parts = append(parts, "mergeable")
	}
	if pr.IsBot {
		parts = append(parts, "bot update")
	}

	if len(parts) == 0 {
		parts = append(parts, "selected by heuristic scoring")
	}

	return strings.Join(parts, "; ")
}

// ScoreAndSortPool scores all PRs by priority and returns them sorted
// in descending order by score. For PRs with equal scores, sorts by PR number.
func ScoreAndSortPool(prs []types.PR, now time.Time) []types.PR {
	sorted := make([]types.PR, len(prs))
	copy(sorted, prs)

	// Sort by priority descending, then by PR number ascending for determinism
	sort.Slice(sorted, func(i, j int) bool {
		left := PlannerPriority(sorted[i], now)
		right := PlannerPriority(sorted[j], now)
		if left == right {
			return sorted[i].Number < sorted[j].Number
		}
		return left > right
	})

	return sorted
}
