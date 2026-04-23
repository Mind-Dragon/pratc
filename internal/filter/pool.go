package filter

import (
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// CapPool limits the candidate pool to the specified size and returns the
// capped pool and rejections for PRs that exceeded the cap.
//
// This helper is available for explicit opt-in callers only. The default
// BuildCandidatePool runtime path does not call it and therefore does not apply
// any implicit cap.
func CapPool(prs []types.PR, maxSize int) (pool []types.PR, rejections []types.PlanRejection) {
	if len(prs) <= maxSize {
		return prs, nil
	}

	pool = prs[:maxSize]
	rejections = make([]types.PlanRejection, 0, len(prs)-maxSize)

	for _, pr := range prs[maxSize:] {
		rejections = append(rejections, types.PlanRejection{
			PRNumber: pr.Number,
			Reason:   "candidate pool cap",
		})
	}

	return pool, rejections
}

// AssignClusterIDs assigns cluster IDs to PRs based on the provided mapping.
// PRs that already have a cluster ID are not modified.
func AssignClusterIDs(prs []types.PR, clusterByPR map[int]string) {
	for i := range prs {
		if prs[i].ClusterID == "" {
			if clusterID, ok := clusterByPR[prs[i].Number]; ok {
				prs[i].ClusterID = clusterID
			}
		}
	}
}

// SortPoolByPriority scores and sorts the pool by priority in descending order.
// PRs with equal scores are sorted by PR number ascending for determinism.
func SortPoolByPriority(prs []types.PR, now time.Time) []types.PR {
	return ScoreAndSortPool(prs, now)
}
