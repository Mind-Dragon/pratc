package filter

import "github.com/jeffersonnunn/pratc/internal/types"

func FilterDraft(pr types.PR) bool {
	return pr.IsDraft
}

func FilterMergeConflict(pr types.PR) bool {
	return pr.Mergeable == "conflicting"
}

func FilterCIFailure(pr types.PR) bool {
	return pr.CIStatus == "failure"
}

func FilterBot(pr types.PR) bool {
	return pr.IsBot
}

func ApplyFilters(prs []types.PR, includeBots bool) (pool []types.PR, rejections []types.PlanRejection) {
	pool = make([]types.PR, 0, len(prs))
	rejections = make([]types.PlanRejection, 0)

	for _, pr := range prs {
		if FilterDraft(pr) {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "draft"})
			continue
		}
		if FilterMergeConflict(pr) {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "merge conflict"})
			continue
		}
		if FilterCIFailure(pr) {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "ci failure"})
			continue
		}
		if !includeBots && FilterBot(pr) {
			rejections = append(rejections, types.PlanRejection{PRNumber: pr.Number, Reason: "bot pr"})
			continue
		}
		pool = append(pool, pr)
	}

	return pool, rejections
}
