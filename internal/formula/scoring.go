package formula

import (
	"fmt"
	"math"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func ScoreCandidate(prs []types.PR, weights ScoreWeights, conflictCounts map[int]int, now time.Time) (float64, []string) {
	if len(prs) == 0 {
		return 0, []string{"empty_candidate"}
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	ageScore := averageAgeScore(prs, now)
	sizeScore := averageSizeScore(prs)
	ciScore := averageCIStatusScore(prs)
	reviewScore := averageReviewStatusScore(prs)
	conflictScore := averageConflictScore(prs, conflictCounts)
	clusterScore := clusterCoherenceScore(prs)

	total := (weights.Age * ageScore) +
		(weights.Size * sizeScore) +
		(weights.CIStatus * ciScore) +
		(weights.ReviewStatus * reviewScore) +
		(weights.ConflictCount * conflictScore) +
		(weights.ClusterCoherence * clusterScore)

	reasons := []string{
		fmt.Sprintf("age=%.4f", ageScore),
		fmt.Sprintf("size=%.4f", sizeScore),
		fmt.Sprintf("ci=%.4f", ciScore),
		fmt.Sprintf("review=%.4f", reviewScore),
		fmt.Sprintf("conflicts=%.4f", conflictScore),
		fmt.Sprintf("cluster=%.4f", clusterScore),
	}

	return total, reasons
}

func averageAgeScore(prs []types.PR, now time.Time) float64 {
	total := 0.0
	for _, pr := range prs {
		createdAt, err := time.Parse(time.RFC3339, pr.CreatedAt)
		if err != nil {
			createdAt = now
		}

		ageDays := now.Sub(createdAt).Hours() / 24
		total += math.Min(ageDays/30, 1)
	}

	return total / float64(len(prs))
}

func averageSizeScore(prs []types.PR) float64 {
	total := 0.0
	for _, pr := range prs {
		churn := pr.Additions + pr.Deletions + pr.ChangedFilesCount
		total += 1 / (1 + float64(churn))
	}

	return total / float64(len(prs))
}

func averageCIStatusScore(prs []types.PR) float64 {
	total := 0.0
	for _, pr := range prs {
		switch pr.CIStatus {
		case "success":
			total += 1
		case "pending":
			total += 0.5
		case "failure":
			total -= 1
		}
	}

	return total / float64(len(prs))
}

func averageReviewStatusScore(prs []types.PR) float64 {
	total := 0.0
	for _, pr := range prs {
		switch pr.ReviewStatus {
		case "approved":
			total += 1
		case "review_required":
			total += 0
		case "changes_requested":
			total -= 1
		case "pending", "in_progress":
			total += 0.3
		}
	}

	return total / float64(len(prs))
}

func averageConflictScore(prs []types.PR, conflictCounts map[int]int) float64 {
	total := 0.0
	for _, pr := range prs {
		count := conflictCounts[pr.Number]
		total += 1 / (1 + float64(count))
	}

	return total / float64(len(prs))
}

func clusterCoherenceScore(prs []types.PR) float64 {
	if len(prs) == 0 {
		return 0
	}

	counts := make(map[string]int, len(prs))
	maxCount := 0
	for _, pr := range prs {
		counts[pr.ClusterID]++
		if counts[pr.ClusterID] > maxCount {
			maxCount = counts[pr.ClusterID]
		}
	}

	return float64(maxCount) / float64(len(prs))
}
