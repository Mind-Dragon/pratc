package actions

import (
	"slices"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// --- Test helpers ---

func makeReviewResult(prNumber int, confidence float64, category types.ReviewCategory, priorityTier types.PriorityTier, blockers []string, reasons []string, nextAction string, temporalBucket string, blastRadius string, evidenceRefs []string) types.ReviewResult {
	return types.ReviewResult{
		PRNumber:           prNumber,
		Confidence:         confidence,
		Category:           category,
		PriorityTier:       priorityTier,
		Blockers:           blockers,
		Reasons:            reasons,
		NextAction:         nextAction,
		TemporalBucket:     temporalBucket,
		BlastRadius:        blastRadius,
		EvidenceReferences: evidenceRefs,
	}
}

func makeEvidence(duplicateGroupID string, canonicalPRNumber int, duplicateConfidence float64, canonicalConflict bool, securitySensitive, legalOrLicenseRisk, unclearOwnership bool) ClassificationEvidence {
	return ClassificationEvidence{
		DuplicateGroupID:    duplicateGroupID,
		CanonicalPRNumber:   canonicalPRNumber,
		DuplicateConfidence: duplicateConfidence,
		CanonicalConflict:   canonicalConflict,
		SecuritySensitive:   securitySensitive,
		LegalOrLicenseRisk:  legalOrLicenseRisk,
		UnclearOwnership:    unclearOwnership,
	}
}

// --- Test: low confidence routes to human_escalate ---

func TestClassifyLane_LowConfidence_HumanEscalate(t *testing.T) {
	result := makeReviewResult(1, 0.40, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"minimal_diff"}, "", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("low confidence 0.40: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
	if len(decision.AllowedActions) != 0 {
		t.Errorf("low confidence should have no allowed actions, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.BlockedReasons, "low_confidence") {
		t.Errorf("missing low_confidence blocked reason, got %v", decision.BlockedReasons)
	}
}

// --- Test: high-risk / security-sensitive routes to human_escalate ---

func TestClassifyLane_SecuritySensitive_HumanEscalate(t *testing.T) {
	result := makeReviewResult(2, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"ci_passing"}, "", "now", "high", nil)
	evidence := makeEvidence("", 0, 0, false, true, false, false)
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("security-sensitive high-confidence: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}

// --- Test: legal/license risk routes to human_escalate ---

func TestClassifyLane_LegalRisk_HumanEscalate(t *testing.T) {
	result := makeReviewResult(3, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"clean_diff"}, "", "now", "low", nil)
	evidence := makeEvidence("", 0, 0, false, false, true, false)
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("legal/license risk: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}

// --- Test: unclear ownership routes to human_escalate ---

func TestClassifyLane_UnclearOwnership_HumanEscalate(t *testing.T) {
	result := makeReviewResult(4, 0.80, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired,
		nil, []string{"needs_owner"}, "", "now", "medium", nil)
	evidence := makeEvidence("", 0, 0, false, false, false, true)
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("unclear ownership: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}

// --- Test: duplicate with full metadata routes to duplicate_close ---

func TestClassifyLane_DuplicateWithCanonical_DuplicateClose(t *testing.T) {
	result := makeReviewResult(5, 0.88, types.ReviewCategoryDuplicateSuperseded, types.PriorityTierBlocked,
		nil, []string{"duplicate_of_pr_42"}, "", "now", "low", []string{"dup_group_1"})
	evidence := makeEvidence("dup_group_1", 42, 0.85, false, false, false, false)
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneDuplicateClose {
		t.Errorf("duplicate with canonical: got lane %q, want %q", decision.Lane, types.ActionLaneDuplicateClose)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindComment) {
		t.Errorf("duplicate_close should allow comment, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindClose) {
		t.Errorf("duplicate_close should allow close, got %v", decision.AllowedActions)
	}
	if slices.Contains(decision.AllowedActions, types.ActionKindMerge) {
		t.Errorf("duplicate_close must not allow merge, got %v", decision.AllowedActions)
	}
}

// --- Test: duplicate missing canonical routes to human_escalate ---

func TestClassifyLane_DuplicateMissingCanonical_HumanEscalate(t *testing.T) {
	result := makeReviewResult(6, 0.88, types.ReviewCategoryDuplicateSuperseded, types.PriorityTierBlocked,
		nil, []string{"duplicate_of_pr_42"}, "", "now", "low", nil)
	evidence := makeEvidence("", 0, 0.85, false, false, false, false) // no group ID
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("duplicate missing canonical: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}

// --- Test: duplicate with canonical conflict routes to human_escalate ---

func TestClassifyLane_DuplicateCanonicalConflict_HumanEscalate(t *testing.T) {
	result := makeReviewResult(7, 0.88, types.ReviewCategoryDuplicateSuperseded, types.PriorityTierBlocked,
		nil, []string{"duplicate_of_pr_42"}, "", "now", "low", nil)
	evidence := makeEvidence("dup_group_1", 42, 0.85, true, false, false, false) // canonical conflict
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("duplicate with canonical conflict: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}

// --- Test: problematic high confidence routes to reject_or_close ---

func TestClassifyLane_ProblematicHighConfidence_RejectOrClose(t *testing.T) {
	result := makeReviewResult(8, 0.90, types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked,
		[]string{"ci_failure"}, []string{"ci_failing"}, "", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneRejectOrClose {
		t.Errorf("problematic high confidence: got lane %q, want %q", decision.Lane, types.ActionLaneRejectOrClose)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindComment) {
		t.Errorf("reject_or_close should allow comment, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindClose) {
		t.Errorf("reject_or_close should allow close, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindLabel) {
		t.Errorf("reject_or_close should allow label, got %v", decision.AllowedActions)
	}
	if slices.Contains(decision.AllowedActions, types.ActionKindMerge) {
		t.Errorf("reject_or_close must not allow merge, got %v", decision.AllowedActions)
	}
}

// --- Test: problematic low confidence routes to human_escalate ---

func TestClassifyLane_ProblematicLowConfidence_HumanEscalate(t *testing.T) {
	result := makeReviewResult(9, 0.60, types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked,
		[]string{"ci_failure"}, []string{"ci_failing"}, "", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("problematic low confidence: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}

// --- Test: clean fast merge routes to fast_merge with preflight checks ---

func TestClassifyLane_CleanFastMerge_FastMerge(t *testing.T) {
	result := makeReviewResult(10, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"ci_passing", "approved", "clean"}, "", "now", "low", nil)
	result.Mergeable = "clean"
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFastMerge {
		t.Errorf("clean fast merge: got lane %q, want %q", decision.Lane, types.ActionLaneFastMerge)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindMerge) {
		t.Errorf("fast_merge should allow merge, got %v", decision.AllowedActions)
	}
	// Required preflight checks must be present
	if len(decision.RequiredPreflightChecks) == 0 {
		t.Errorf("fast_merge must have required preflight checks, got none")
	}
	for _, check := range decision.RequiredPreflightChecks {
		if !check.Required {
			t.Errorf("fast_merge preflight check %q must be required", check.Check)
		}
		if check.Status != "pending" {
			t.Errorf("fast_merge preflight check %q status must be pending, got %q", check.Check, check.Status)
		}
	}
}

// --- Test: repairable blocked PR routes to fix_and_merge ---

func TestClassifyLane_RepairableBlocked_FixAndMerge(t *testing.T) {
	result := makeReviewResult(11, 0.75, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		[]string{"ci_failure"}, []string{"test_gap", "lint_error"}, "resolve_ci", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFixAndMerge {
		t.Errorf("repairable blocked: got lane %q, want %q", decision.Lane, types.ActionLaneFixAndMerge)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindApplyFix) {
		t.Errorf("fix_and_merge should allow apply_fix, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindComment) {
		t.Errorf("fix_and_merge should allow comment, got %v", decision.AllowedActions)
	}
	if slices.Contains(decision.AllowedActions, types.ActionKindMerge) {
		t.Errorf("fix_and_merge must not allow merge in Wave 1C, got %v", decision.AllowedActions)
	}
}

// --- Test: future bucket routes to future_or_reengage ---

func TestClassifyLane_FutureBucket_FutureOrReengage(t *testing.T) {
	result := makeReviewResult(12, 0.80, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired,
		nil, []string{"needs_author_response"}, "", "future", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFutureOrReengage {
		t.Errorf("future bucket: got lane %q, want %q", decision.Lane, types.ActionLaneFutureOrReengage)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindComment) {
		t.Errorf("future_or_reengage should allow comment, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindLabel) {
		t.Errorf("future_or_reengage should allow label, got %v", decision.AllowedActions)
	}
}

// --- Test: default meaningful PR routes to focused_review ---

func TestClassifyLane_Default_FocusedReview(t *testing.T) {
	result := makeReviewResult(13, 0.75, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired,
		nil, []string{"needs_focused_review"}, "", "now", "medium", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFocusedReview {
		t.Errorf("default meaningful PR: got lane %q, want %q", decision.Lane, types.ActionLaneFocusedReview)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindComment) {
		t.Errorf("focused_review should allow comment, got %v", decision.AllowedActions)
	}
	if !slices.Contains(decision.AllowedActions, types.ActionKindLabel) {
		t.Errorf("focused_review should allow label, got %v", decision.AllowedActions)
	}
}

// --- Test: blocked PR with merge-ish next action never allows merge ---

func TestClassifyLane_BlockedWithMergeNextAction_NoMerge(t *testing.T) {
	result := makeReviewResult(14, 0.70, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		[]string{"merge_conflict"}, []string{"needs_rebase"}, "rebase_and_merge", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if slices.Contains(decision.AllowedActions, types.ActionKindMerge) {
		t.Errorf("blocked PR must not allow merge, got %v", decision.AllowedActions)
	}
}

// --- Test: high blast radius never fast_merge without override ---

func TestClassifyLane_HighBlastRadius_HumanEscalate(t *testing.T) {
	result := makeReviewResult(15, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"ci_passing", "approved"}, "", "now", "high", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("high blast radius should escalate: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
	if slices.Contains(decision.AllowedActions, types.ActionKindMerge) {
		t.Errorf("high blast radius must not allow merge, got %v", decision.AllowedActions)
	}
}

// --- Test: every case returns exactly one non-empty lane ---

func TestClassifyLane_ExactlyOneNonEmptyLane(t *testing.T) {
	cases := []struct {
		name     string
		result   types.ReviewResult
		evidence ClassificationEvidence
	}{
		{
			name:     "low_confidence",
			result:   makeReviewResult(100, 0.30, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, nil, nil, "", "now", "low", nil),
			evidence: ClassificationEvidence{},
		},
		{
			name:     "security_sensitive",
			result:   makeReviewResult(101, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, nil, nil, "", "now", "low", nil),
			evidence: makeEvidence("", 0, 0, false, true, false, false),
		},
		{
			name:     "duplicate_full",
			result:   makeReviewResult(102, 0.88, types.ReviewCategoryDuplicateSuperseded, types.PriorityTierBlocked, nil, nil, "", "now", "low", nil),
			evidence: makeEvidence("g1", 42, 0.85, false, false, false, false),
		},
		{
			name:     "problematic_high_conf",
			result:   makeReviewResult(103, 0.85, types.ReviewCategoryProblematicQuarantine, types.PriorityTierBlocked, nil, nil, "", "now", "low", nil),
			evidence: ClassificationEvidence{},
		},
		{
			name: "clean_fast_merge",
			result: func() types.ReviewResult {
				result := makeReviewResult(104, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, nil, nil, "", "now", "low", nil)
				result.Mergeable = "clean"
				return result
			}(),
			evidence: ClassificationEvidence{},
		},
		{
			name:     "repairable_blocked",
			result:   makeReviewResult(105, 0.75, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked, []string{"ci_failure"}, nil, "resolve_ci", "now", "low", nil),
			evidence: ClassificationEvidence{},
		},
		{
			name:     "future_reengage",
			result:   makeReviewResult(106, 0.80, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, nil, nil, "", "future", "low", nil),
			evidence: ClassificationEvidence{},
		},
		{
			name:     "default_focused_review",
			result:   makeReviewResult(107, 0.75, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired, nil, nil, "", "now", "medium", nil),
			evidence: ClassificationEvidence{},
		},
		{
			name:     "high_blast_radius",
			result:   makeReviewResult(108, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge, nil, nil, "", "now", "high", nil),
			evidence: ClassificationEvidence{},
		},
		{
			name:     "blocked_conflict_merge_next",
			result:   makeReviewResult(109, 0.70, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked, []string{"conflict"}, nil, "rebase_and_merge", "now", "low", nil),
			evidence: ClassificationEvidence{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := ClassifyLane(tc.result, tc.evidence)
			if decision.Lane == "" {
				t.Errorf("case %q: Lane must never be empty", tc.name)
			}
		})
	}
}

// --- Test: reason trail preserves existing evidence refs ---

func TestClassifyLane_DuplicateSignalsFromNextAction_DuplicateClose(t *testing.T) {
	result := makeReviewResult(16, 0.86, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		nil, []string{"superseded_by_pr_42"}, "duplicate_close", "now", "low", []string{"dup_group_1"})
	evidence := makeEvidence("dup_group_1", 42, 0.84, false, false, false, false)
	decision := ClassifyLane(result, evidence)
	if decision.Lane != types.ActionLaneDuplicateClose {
		t.Errorf("duplicate next_action should route duplicate_close: got %q", decision.Lane)
	}
}

func TestClassifyLane_JunkSignalsFromTemporalBucket_RejectOrClose(t *testing.T) {
	result := makeReviewResult(17, 0.86, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		nil, []string{"spam_marker"}, "", "junk", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneRejectOrClose {
		t.Errorf("junk temporal bucket should route reject_or_close: got %q", decision.Lane)
	}
}

func TestClassifyLane_FastMergeRequiresMergeableSignal(t *testing.T) {
	result := makeReviewResult(18, 0.91, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"ci_passing", "approved"}, "", "now", "low", nil)
	result.Mergeable = ""
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane == types.ActionLaneFastMerge {
		t.Fatalf("fast_merge requires explicit mergeable clean signal, got %q", decision.Lane)
	}
}

func TestClassifyLane_RepairableSignalFromReason_FixAndMerge(t *testing.T) {
	result := makeReviewResult(19, 0.72, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		nil, []string{"missing_generated_artifact"}, "", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFixAndMerge {
		t.Errorf("repairable reason should route fix_and_merge: got %q", decision.Lane)
	}
}

// --- Test: reason trail preserves existing evidence refs ---

func TestClassifyLane_ReasonTrail_PreservesEvidenceRefs(t *testing.T) {
	result := makeReviewResult(200, 0.90, types.ReviewCategoryMergeNow, types.PriorityTierFastMerge,
		nil, []string{"ci_passing"}, "", "now", "low", []string{"evidence_ref_1", "evidence_ref_2"})
	result.Mergeable = "clean"
	decision := ClassifyLane(result, ClassificationEvidence{})
	for _, ref := range result.EvidenceReferences {
		if !slices.Contains(decision.EvidenceRefs, ref) {
			t.Errorf("existing evidence ref %q not preserved in decision EvidenceRefs %v", ref, decision.EvidenceRefs)
		}
	}
}

// --- Test: duplicate_close reason trail includes duplicate evidence ---

func TestClassifyLane_DuplicateClose_ReasonTrailHasDuplicateEvidence(t *testing.T) {
	result := makeReviewResult(201, 0.88, types.ReviewCategoryDuplicateSuperseded, types.PriorityTierBlocked,
		nil, []string{"duplicate_of_pr_42"}, "", "now", "low", []string{"dup_evidence"})
	evidence := makeEvidence("dup_group_1", 42, 0.85, false, false, false, false)
	decision := ClassifyLane(result, evidence)
	found := false
	for _, r := range decision.ReasonTrail {
		if r == "duplicate_of_pr_42" || r == "duplicate_confidence_0.85" || r == "canonical_pr_42" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("duplicate_close ReasonTrail should include duplicate evidence names, got %v", decision.ReasonTrail)
	}
}

// --- Test: repairable blockers identified by keyword ---

func TestClassifyLane_Repairable_LintFailure(t *testing.T) {
	result := makeReviewResult(202, 0.72, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		[]string{"lint_failure"}, []string{"lint_error"}, "fix_lint", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFixAndMerge {
		t.Errorf("lint failure should route to fix_and_merge, got %q", decision.Lane)
	}
}

// --- Test: repairable blockers - missing generated artifact ---

func TestClassifyLane_Repairable_MissingArtifact(t *testing.T) {
	result := makeReviewResult(203, 0.68, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		[]string{"missing_generated_file"}, []string{"missing_openapi_spec"}, "generate_artifact", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneFixAndMerge {
		t.Errorf("missing artifact should route to fix_and_merge, got %q", decision.Lane)
	}
}

// --- Test: fix_and_merge below confidence threshold routes to human_escalate ---

func TestClassifyLane_RepairableBelowThreshold_HumanEscalate(t *testing.T) {
	result := makeReviewResult(204, 0.55, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierBlocked,
		[]string{"ci_failure"}, []string{"test_gap"}, "resolve_ci", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("repairable below 0.65 threshold should route to human_escalate, got %q", decision.Lane)
	}
}

// --- Test: conflicting evidence routes to human_escalate ---

func TestClassifyLane_ConflictingEvidence_HumanEscalate(t *testing.T) {
	result := makeReviewResult(205, 0.80, types.ReviewCategoryMergeAfterFocusedReview, types.PriorityTierReviewRequired,
		nil, []string{"ci_passing", "ci_failing"}, "", "now", "low", nil)
	decision := ClassifyLane(result, ClassificationEvidence{})
	if decision.Lane != types.ActionLaneHumanEscalate {
		t.Errorf("conflicting evidence: got lane %q, want %q", decision.Lane, types.ActionLaneHumanEscalate)
	}
}
