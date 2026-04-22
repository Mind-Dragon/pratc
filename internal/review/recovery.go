package review

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// recoveryRule represents a single recovery rule with its predicate and action.
type recoveryRule struct {
	name   string
	reason string
	action func(pr types.PR, prData PRData) (reclassify bool, category types.ReviewCategory, temporalBucket string)
}

// RunSecondPass applies deterministic recovery rules to PRs that ended up in "blocked"
// or "low_value" territory. It takes a slice of ReviewResult and corresponding PRData,
// then returns updated ReviewResults with potential reclassifications.
//
// A PR is considered a candidate for second-pass recovery if:
//   - Category is NOT one of: merge_now, merge_after_focused_review, duplicate_superseded, problematic_quarantine
//   - This includes: unknown_escalate, confidence < 0.5, or TemporalBucket == "blocked"
//
// Recovery rules are applied in order (first match wins):
//   1. CI failing + last push <30d + not draft + <3 conflict pairs → needs_review
//   2. Draft + ≥2 reviews/activity + not stale → high_value
//   3. Mergeable=unknown + small size + no risk flags → needs_review
//   4. CI failing + last push >90d + no reviews → junk
//   5. >10 conflict pairs + stale → junk
//   6. Spam/junk markers (IsBot, empty title, empty body) → junk
//   7. All others remain blocked with "permanent_blocker: no recovery path found"
func RunSecondPass(results []types.ReviewResult, prDataMap map[int]PRData) []types.ReviewResult {
	// Define recovery rules in order (first match wins)
	rules := []recoveryRule{
		{
			name:   "recoverable_ci_failure",
			reason: "recoverable CI failure with recent activity",
			action: func(pr types.PR, prData PRData) (bool, types.ReviewCategory, string) {
				// Rule 1: CI failing + last push <30d + not draft + <3 conflict pairs → needs_review
				if !isCIFailing(pr.CIStatus) {
					return false, "", ""
				}
				if !isRecentPush(pr.UpdatedAt, 30) {
					return false, "", ""
				}
				if pr.IsDraft {
					return false, "", ""
				}
				if len(prData.ConflictPairs) >= 3 {
					return false, "", ""
				}
				return true, types.ReviewCategoryMergeAfterFocusedReview, "future"
			},
		},
		{
			name:   "active_draft",
			reason: "active draft with community engagement",
			action: func(pr types.PR, prData PRData) (bool, types.ReviewCategory, string) {
				// Rule 2: Draft + ≥2 reviews/activity (ReviewCount > 0 or CommentCount > 1) + not stale → high_value
				if !pr.IsDraft {
					return false, "", ""
				}
				hasActivity := pr.ReviewCount > 0 || pr.CommentCount > 1
				if !hasActivity {
					return false, "", ""
				}
				if isStale(prData.Staleness) {
					return false, "", ""
				}
				return true, types.ReviewCategoryMergeAfterFocusedReview, "future"
			},
		},
		{
			name:   "small_unknown_mergeable",
			reason: "small PR with unknown mergeability, likely safe",
			action: func(pr types.PR, prData PRData) (bool, types.ReviewCategory, string) {
				// Rule 3: Mergeable=unknown + small size (ChangedFilesCount <= 5) + no risk flags → needs_review
				if !isMergeableUnknown(pr.Mergeable) {
					return false, "", ""
				}
				if pr.ChangedFilesCount > 5 {
					return false, "", ""
				}
				if hasRiskFlags(pr, prData) {
					return false, "", ""
				}
				return true, types.ReviewCategoryMergeAfterFocusedReview, "future"
			},
		},
		{
			name:   "abandoned_failing_ci",
			reason: "abandoned failing PR",
			action: func(pr types.PR, prData PRData) (bool, types.ReviewCategory, string) {
				// Rule 4: CI failing + last push >90d + no reviews (ReviewCount == 0) → junk
				if !isCIFailing(pr.CIStatus) {
					return false, "", ""
				}
				if isRecentPush(pr.UpdatedAt, 90) {
					return false, "", ""
				}
				if pr.ReviewCount != 0 {
					return false, "", ""
				}
				return true, types.ReviewCategoryProblematicQuarantine, "junk"
			},
		},
		{
			name:   "stale_excessive_conflicts",
			reason: "stale PR with excessive conflicts",
			action: func(pr types.PR, prData PRData) (bool, types.ReviewCategory, string) {
				// Rule 5: >10 conflict pairs + stale (Staleness.Score > 50) → junk
				if len(prData.ConflictPairs) <= 10 {
					return false, "", ""
				}
				if !isStale(prData.Staleness) {
					return false, "", ""
				}
				return true, types.ReviewCategoryProblematicQuarantine, "junk"
			},
		},
		{
			name:   "garbage_markers",
			reason: "garbage markers detected",
			action: func(pr types.PR, prData PRData) (bool, types.ReviewCategory, string) {
				// Rule 6: Spam/junk markers (IsBot, empty title, empty body) → junk
				if pr.IsBot {
					return true, types.ReviewCategoryProblematicQuarantine, "junk"
				}
				if strings.TrimSpace(pr.Title) == "" {
					return true, types.ReviewCategoryProblematicQuarantine, "junk"
				}
				if strings.TrimSpace(pr.Body) == "" {
					return true, types.ReviewCategoryProblematicQuarantine, "junk"
				}
				return false, "", ""
			},
		},
	}

	// Categories that are NOT candidates for recovery
	nonRecoveryCategories := map[types.ReviewCategory]bool{
		types.ReviewCategoryMergeNow:                true,
		types.ReviewCategoryMergeAfterFocusedReview:  true,
		types.ReviewCategoryDuplicateSuperseded:      true,
		types.ReviewCategoryProblematicQuarantine:    true,
	}

	// Process each result
	for i := range results {
		result := &results[i]

		// Check if this PR is a candidate for recovery
		if nonRecoveryCategories[result.Category] {
			continue
		}

		// Get corresponding PRData
		prData, ok := prDataMap[result.PRNumber]
		if !ok {
			continue
		}

		// Apply recovery rules in order
		reclassified := false
		for _, rule := range rules {
			if reclassify, newCategory, temporalBucket := rule.action(prData.PR, prData); reclassify {
				// Record original category
				result.ReclassifiedFrom = string(result.Category)
				result.Category = newCategory
				result.TemporalBucket = temporalBucket
				result.ReclassificationReason = rule.reason
				result.Reasons = append(result.Reasons, fmt.Sprintf("recovery: %s", rule.reason))

				// Add Gate 17 "Recovery Assessment" decision layer
				result.DecisionLayers = append(result.DecisionLayers, types.DecisionLayer{
					Layer:     17,
					Name:      "Recovery Assessment",
					CostTier:  "cheap",
					Bucket:    temporalBucket,
					Status:    "observed",
					Reasons:   []string{rule.reason},
					Continued: false,
					Terminal:  true,
				})

				reclassified = true
				break // First match wins
			}
		}

		// If no rule matched, mark as permanent blocker
		if !reclassified {
			result.ReclassificationReason = "permanent_blocker: no recovery path found"
			result.Reasons = append(result.Reasons, "recovery: permanent_blocker: no recovery path found")
		}
	}

	// Sort results by PR number for deterministic output
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].PRNumber < results[j].PRNumber
	})

	return results
}

// isCIFailing returns true if the CI status indicates a failing state.
func isCIFailing(ciStatus string) bool {
	switch strings.ToLower(strings.TrimSpace(ciStatus)) {
	case "failure", "failed", "red", "error":
		return true
	default:
		return false
	}
}

// isRecentPush returns true if the PR was pushed within the specified number of days.
func isRecentPush(updatedAt string, withinDays int) bool {
	if updatedAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return false
	}
	return time.Since(t).Hours()/24 < float64(withinDays)
}

// isMergeableUnknown returns true if the mergeable status is unknown or empty.
func isMergeableUnknown(mergeable string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(mergeable))
	return trimmed == "" || trimmed == "unknown"
}

// isStale returns true if the PR is considered stale (staleness score > 50).
func isStale(staleness *types.StalenessReport) bool {
	if staleness == nil {
		return false
	}
	return staleness.Score > 50
}

// hasRiskFlags returns true if the PR has any risk flags.
func hasRiskFlags(pr types.PR, prData PRData) bool {
	// Check for high-risk indicators
	if pr.ChangedFilesCount > 10 {
		return true
	}
	if pr.Additions+pr.Deletions > 500 {
		return true
	}
	if len(prData.ConflictPairs) > 5 {
		return true
	}
	if isCIFailing(pr.CIStatus) {
		return true
	}
	return false
}
