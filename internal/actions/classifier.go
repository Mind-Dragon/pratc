// Package actions provides the action lane classifier, policy gates, and ActionPlan builder
// for the prATC v2.0 autonomous engine.
//
// This wave (Wave 1C): deterministic lane classifier only.
//   - ClassifyLane(result types.ReviewResult, evidence ClassificationEvidence) LaneDecision
//   - Every PR lands in exactly one primary action lane.
//   - Blocked PRs never emit a merge AllowedAction.
package actions

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// ClassificationEvidence captures non-review facts that influence lane routing
// but are not present in types.ReviewResult.
type ClassificationEvidence struct {
	// DuplicateGroupID is the stable identifier for the duplicate group this PR belongs to.
	DuplicateGroupID string
	// CanonicalPRNumber is the PR number that is the canonical representative of the duplicate group.
	CanonicalPRNumber int
	// DuplicateConfidence is the pairwise similarity confidence for the duplicate relationship.
	DuplicateConfidence float64
	// CanonicalConflict is true when the canonical PR of the duplicate group has an unresolved conflict.
	CanonicalConflict bool
	// SecuritySensitive is true when this PR touches security-sensitive code or dependencies.
	SecuritySensitive bool
	// LegalOrLicenseRisk is true when this PR introduces new dependencies or licensing concerns.
	LegalOrLicenseRisk bool
	// UnclearOwnership is true when the PR author is unknown or the PR is abandoned.
	UnclearOwnership bool
}

// LaneDecision carries the deterministic lane assignment for a single PR along with
// all supporting metadata for the action engine.
type LaneDecision struct {
	PRNumber                int
	Lane                    types.ActionLane
	Confidence              float64
	RiskFlags               []string
	ReasonTrail             []string
	EvidenceRefs            []string
	AllowedActions          []types.ActionKind
	BlockedReasons          []string
	RequiredPreflightChecks []types.ActionPreflight
	PriorityScore           float64
}

// ClassifyLane deterministically assigns a PR to exactly one action lane based on
// the review result and supplementary evidence.
//
// Rules are evaluated in priority order:
//
//	a. confidence < 0.50 → human_escalate (no actions, low_confidence blocked)
//	b. explicit human triggers (security-sensitive, legal/license, unclear ownership,
//	   conflicting evidence) → human_escalate (no actions)
//	c. duplicate/superseded with group ID + canonical PR + confidence >= 0.80
//	   + no canonical conflict → duplicate_close; else human_escalate
//	d. problematic/quarantine/junk with confidence >= 0.80 → reject_or_close;
//	   else human_escalate
//	e. clean fast merge (merge_now/priority=fast_merge, confidence >= 0.85,
//	   mergeable clean/mergeable, no blockers, temporal now/empty,
//	   blast radius low/empty, no risk flags) → fast_merge; else → focused_review
//	f. repairable blocked (confidence >= 0.65, small-repair blockers) → fix_and_merge
//	g. future/reengage temporal bucket → future_or_reengage
//	h. default → focused_review
//
// AllowedActions per lane:
//   - fast_merge:       merge
//   - fix_and_merge:    apply_fix, comment
//   - duplicate_close:   comment, close
//   - reject_or_close:  comment, close, label
//   - focused_review:   comment, label
//   - future_or_reengage: comment, label
//   - human_escalate:   (none)
func ClassifyLane(result types.ReviewResult, evidence ClassificationEvidence) LaneDecision {
	decision := LaneDecision{
		PRNumber:     result.PRNumber,
		Confidence:   result.Confidence,
		EvidenceRefs: slices.Clone(result.EvidenceReferences),
	}

	// Always seed the reason trail with the existing review reasons.
	decision.ReasonTrail = slices.Clone(result.Reasons)

	// a. Low confidence → human_escalate
	if result.Confidence < 0.50 {
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "low_confidence")
		decision.ReasonTrail = append(decision.ReasonTrail, "confidence_below_0.50")
		return decision
	}

	// b. Explicit human triggers
	if evidence.SecuritySensitive || result.BlastRadius == "high" {
		decision.Lane = types.ActionLaneHumanEscalate
		if evidence.SecuritySensitive {
			decision.BlockedReasons = append(decision.BlockedReasons, "security_sensitive")
			decision.RiskFlags = append(decision.RiskFlags, "security_sensitive")
			decision.ReasonTrail = append(decision.ReasonTrail, "security_sensitive")
		}
		if result.BlastRadius == "high" {
			decision.BlockedReasons = append(decision.BlockedReasons, "high_blast_radius")
			decision.RiskFlags = append(decision.RiskFlags, "high_blast_radius")
			decision.ReasonTrail = append(decision.ReasonTrail, "high_blast_radius")
		}
		return decision
	}
	if evidence.LegalOrLicenseRisk {
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "legal_or_license_risk")
		decision.RiskFlags = append(decision.RiskFlags, "legal_or_license_risk")
		decision.ReasonTrail = append(decision.ReasonTrail, "legal_or_license_risk")
		return decision
	}
	if evidence.UnclearOwnership {
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "unclear_ownership")
		decision.ReasonTrail = append(decision.ReasonTrail, "unclear_ownership")
		return decision
	}
	if hasConflictingEvidence(result.Reasons) {
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "conflicting_evidence")
		decision.ReasonTrail = append(decision.ReasonTrail, "conflicting_evidence")
		return decision
	}

	// c. Duplicate / superseded
	if isDuplicateSignal(result) {
		if evidence.DuplicateGroupID != "" && evidence.CanonicalPRNumber > 0 &&
			evidence.DuplicateConfidence >= 0.80 && !evidence.CanonicalConflict {
			decision.Lane = types.ActionLaneDuplicateClose
			decision.AllowedActions = []types.ActionKind{types.ActionKindComment, types.ActionKindClose}
			decision.ReasonTrail = append(decision.ReasonTrail,
				"duplicate_group:"+evidence.DuplicateGroupID,
				fmt.Sprintf("canonical_pr:%d", evidence.CanonicalPRNumber),
				"duplicate_confidence:"+formatFloat(evidence.DuplicateConfidence),
			)
			return decision
		}
		// Missing required duplicate metadata → escalate
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "duplicate_missing_metadata")
		decision.ReasonTrail = append(decision.ReasonTrail, "duplicate_incomplete_metadata")
		return decision
	}

	// d. Problematic / quarantine / junk
	if isProblematicSignal(result) {
		if result.Confidence >= 0.80 {
			decision.Lane = types.ActionLaneRejectOrClose
			decision.AllowedActions = []types.ActionKind{
				types.ActionKindComment,
				types.ActionKindClose,
				types.ActionKindLabel,
			}
			decision.ReasonTrail = append(decision.ReasonTrail, "problematic_confidence:"+formatFloat(result.Confidence))
			return decision
		}
		// Problematic but low confidence → escalate
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "problematic_low_confidence")
		decision.ReasonTrail = append(decision.ReasonTrail, "problematic_confidence_below_0.80")
		return decision
	}

	// e. Clean fast merge
	if isCleanFastMerge(result) {
		decision.Lane = types.ActionLaneFastMerge
		decision.AllowedActions = []types.ActionKind{types.ActionKindMerge}
		decision.RequiredPreflightChecks = buildFastMergePreflightChecks()
		decision.ReasonTrail = append(decision.ReasonTrail, "clean_fast_merge")
		decision.PriorityScore = computePriorityScore(result)
		return decision
	}

	// f. Repairable blocked PR
	if isRepairableBlocked(result) && result.Confidence >= 0.65 {
		decision.Lane = types.ActionLaneFixAndMerge
		decision.AllowedActions = []types.ActionKind{types.ActionKindApplyFix, types.ActionKindComment}
		decision.ReasonTrail = append(decision.ReasonTrail, "repairable_blocked")
		decision.PriorityScore = computePriorityScore(result)
		return decision
	}
	// Repairable but below confidence threshold → escalate (needs human judgment before repair attempt)
	if isRepairableBlocked(result) {
		decision.Lane = types.ActionLaneHumanEscalate
		decision.BlockedReasons = append(decision.BlockedReasons, "repairable_below_confidence_threshold")
		decision.ReasonTrail = append(decision.ReasonTrail, "repairable_blocked_low_confidence")
		return decision
	}

	// g. Future / reengage
	if result.TemporalBucket == "future" || result.NextAction == "author_response" || result.NextAction == "product_wait" {
		decision.Lane = types.ActionLaneFutureOrReengage
		decision.AllowedActions = []types.ActionKind{types.ActionKindComment, types.ActionKindLabel}
		decision.ReasonTrail = append(decision.ReasonTrail, "future_reengage_temporal_bucket")
		decision.PriorityScore = computePriorityScore(result)
		return decision
	}

	// h. Default → focused_review
	decision.Lane = types.ActionLaneFocusedReview
	decision.AllowedActions = []types.ActionKind{types.ActionKindComment, types.ActionKindLabel}
	decision.ReasonTrail = append(decision.ReasonTrail, "default_focused_review")
	decision.PriorityScore = computePriorityScore(result)
	return decision
}

// --- Helper predicates ---

// isDuplicateSignal returns true for duplicate or superseded review categories,
// temporal buckets, next actions, or reason markers.
func isDuplicateSignal(result types.ReviewResult) bool {
	if result.Category == types.ReviewCategoryDuplicateSuperseded {
		return true
	}
	return textHasAny(result.TemporalBucket, "duplicate", "superseded") ||
		textHasAny(result.NextAction, "duplicate", "superseded") ||
		anyTextHasAny(result.Reasons, "duplicate", "superseded")
}

// isProblematicSignal returns true for problematic, quarantine, junk, close,
// or reject signals from category, temporal bucket, next action, or reasons.
func isProblematicSignal(result types.ReviewResult) bool {
	if result.Category == types.ReviewCategoryProblematicQuarantine {
		return true
	}
	return textHasAny(result.TemporalBucket, "junk", "spam", "reject", "close", "quarantine") ||
		textHasAny(result.NextAction, "junk", "spam", "reject", "close", "quarantine") ||
		anyTextHasAny(result.Reasons, "junk", "spam", "malformed", "unsafe", "malicious", "reject", "close", "quarantine")
}

// isCleanFastMerge returns true when the PR meets all fast_merge criteria:
// - Category is merge_now or PriorityTier is fast_merge
// - Confidence >= 0.85
// - Mergeable is "clean" or "mergeable" (or empty/unknown with no blockers)
// - No blockers
// - Temporal bucket is "now" or empty
// - Blast radius is "low" or empty
// - No risk flags from evidence
func isCleanFastMerge(result types.ReviewResult) bool {
	if result.Confidence < 0.85 {
		return false
	}
	if len(result.Blockers) > 0 {
		return false
	}
	if result.Category != types.ReviewCategoryMergeNow && result.PriorityTier != types.PriorityTierFastMerge {
		return false
	}
	// Mergeable must be an explicit clean/mergeable signal.
	if result.Mergeable != "clean" && result.Mergeable != "mergeable" && result.Mergeable != "true" {
		return false
	}
	// Temporal bucket must be "now" or empty
	if result.TemporalBucket != "" && result.TemporalBucket != "now" {
		return false
	}
	// Blast radius must be "low" or empty
	if result.BlastRadius != "" && result.BlastRadius != "low" {
		return false
	}
	return true
}

// isRepairableBlocked returns true when the PR has blockers that represent small,
// automatable repairs: test gap, lint error, CI failure, minor conflict/rebase,
// missing generated artifact, review comment repair.
func isRepairableBlocked(result types.ReviewResult) bool {
	signals := make([]string, 0, len(result.Blockers)+len(result.Reasons)+1)
	signals = append(signals, result.Blockers...)
	signals = append(signals, result.Reasons...)
	signals = append(signals, result.NextAction)
	if len(signals) == 0 {
		return false
	}
	smallRepairKeywords := []string{
		"test_gap", "test gap", "missing_test",
		"lint", "lint_error", "lint_failure", "format",
		"ci_failure", "ci failure", "ci_failing", "resolve_ci",
		"conflict", "rebase", "needs_rebase", "merge_conflict",
		"missing_generated", "generated_file", "missing_artifact",
		"review_comment", "review_feedback", "address_comments",
	}
	return anyTextHasAny(signals, smallRepairKeywords...)
}

// hasConflictingEvidence returns true when the reason trail contains signals
// that directly contradict each other (e.g., both "ci_passing" and "ci_failing").
func hasConflictingEvidence(reasons []string) bool {
	hasPassingCI := false
	hasFailingCI := false
	for _, r := range reasons {
		lower := strings.ToLower(r)
		if strings.Contains(lower, "ci_passing") || strings.Contains(lower, "ci green") || strings.Contains(lower, "ci pass") {
			hasPassingCI = true
		}
		if strings.Contains(lower, "ci_failing") || strings.Contains(lower, "ci failure") || strings.Contains(lower, "ci fail") {
			hasFailingCI = true
		}
	}
	return hasPassingCI && hasFailingCI
}

func anyTextHasAny(items []string, needles ...string) bool {
	for _, item := range items {
		if textHasAny(item, needles...) {
			return true
		}
	}
	return false
}

func textHasAny(value string, needles ...string) bool {
	lower := strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

// buildFastMergePreflightChecks returns the placeholder preflight checks for a fast_merge lane.
// These are always "pending" with required=true; actual status is populated by the executor.
func buildFastMergePreflightChecks() []types.ActionPreflight {
	return []types.ActionPreflight{
		{Check: "pr_still_open", Status: "pending", Required: true, Reason: "live preflight"},
		{Check: "head_sha_unchanged", Status: "pending", Required: true, Reason: "live preflight"},
		{Check: "ci_green", Status: "pending", Required: true, Reason: "live preflight"},
		{Check: "mergeable_clean", Status: "pending", Required: true, Reason: "live preflight"},
		{Check: "policy_profile_allows_merge", Status: "pending", Required: true, Reason: "live preflight"},
	}
}

// computePriorityScore returns a simple composite priority score.
func computePriorityScore(result types.ReviewResult) float64 {
	score := result.Confidence
	switch result.BlastRadius {
	case "low":
		score += 0.10
	case "medium":
		score += 0.05
	case "high":
		score -= 0.05
	}
	if result.PriorityTier == types.PriorityTierFastMerge {
		score += 0.10
	}
	if score > 1.0 {
		return 1.0
	}
	if score < 0.0 {
		return 0.0
	}
	return score
}

// formatFloat formats a float64 to a short decimal string for reason trail.
func formatFloat(f float64) string {
	s := fmt.Sprintf("%.2f", f)
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}
