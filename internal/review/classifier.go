package review

import (
	"fmt"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type MergeSafetyResult struct {
	IsSafe     bool
	Confidence float64
	Reasons    []string
	Blockers   []string
}

func ClassifyMergeSafety(pr types.PR, conflictPairs []types.ConflictPair) MergeSafetyResult {
	var reasons, blockers []string
	confidence := 1.0

	switch pr.CIStatus {
	case "success", "passed", "green":
		reasons = append(reasons, "CI passing")
	case "failure", "failed", "red", "error":
		blockers = append(blockers, "CI failing")
		confidence -= 0.2
	case "pending", "running", "in_progress":
		confidence -= 0.1
		reasons = append(reasons, "CI pending")
	case "":
		confidence -= 0.1
	default:
		confidence -= 0.1
	}

	switch pr.ReviewStatus {
	case "approved":
		reasons = append(reasons, "approved")
	case "changes_requested":
		blockers = append(blockers, "changes requested")
		confidence -= 0.2
	case "pending", "review_required", "required":
		if !pr.IsDraft {
			blockers = append(blockers, "not reviewed")
			confidence -= 0.15
		} else {
			confidence -= 0.05
		}
	case "":
		confidence -= 0.1
	default:
		confidence -= 0.1
	}

	switch pr.Mergeable {
	case "true", "clean", "mergeable":
		reasons = append(reasons, "mergeable")
	case "false", "unclean", "conflicted", "dirty":
		blockers = append(blockers, "merge conflicts")
		confidence -= 0.25
	case "":
		confidence -= 0.15
	default:
		confidence -= 0.15
	}

	if pr.IsDraft {
		blockers = append(blockers, "draft PR")
		confidence -= 0.15
	} else {
		reasons = append(reasons, "not draft")
	}

	if len(conflictPairs) > 0 {
		conflictFiles := gatherConflictFiles(conflictPairs, pr.Number)
		if len(conflictFiles) > 0 {
			blockers = append(blockers, formatConflictBlocker(conflictFiles))
			confidence -= 0.2
		}
	} else {
		confidence -= 0.05
	}

	if confidence < 0.0 {
		confidence = 0.0
	}

	return MergeSafetyResult{
		IsSafe:     len(blockers) == 0,
		Confidence: confidence,
		Reasons:    reasons,
		Blockers:   blockers,
	}
}

func gatherConflictFiles(pairs []types.ConflictPair, prNumber int) []string {
	files := make(map[string]struct{})
	for _, pair := range pairs {
		if pair.SourcePR == prNumber || pair.TargetPR == prNumber {
			for _, f := range pair.FilesTouched {
				files[f] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(files))
	for f := range files {
		result = append(result, f)
	}
	return result
}

func formatConflictBlocker(files []string) string {
	if len(files) == 0 {
		return "semantic conflicts"
	}
	if len(files) == 1 {
		return "conflicts on file: " + files[0]
	}
	if len(files) == 2 {
		return "conflicts on files: " + files[0] + ", " + files[1]
	}
	return fmt.Sprintf("conflicts on %d files: %s, %s, %s and %d more",
		len(files), files[0], files[1], files[2], len(files)-3)
}
