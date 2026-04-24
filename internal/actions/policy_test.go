package actions

import (
	"slices"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func policyDecision(actions ...types.ActionKind) LaneDecision {
	return LaneDecision{
		PRNumber:       101,
		Lane:           types.ActionLaneFastMerge,
		Confidence:     0.91,
		AllowedActions: actions,
		ReasonTrail:    []string{"clean_fast_merge"},
		EvidenceRefs:   []string{"github:pr/101"},
		RequiredPreflightChecks: []types.ActionPreflight{
			{Check: "pr_still_open", Status: "passed", Required: true, Reason: "live preflight"},
			{Check: "head_sha_unchanged", Status: "passed", Required: true, Reason: "live preflight"},
			{Check: "ci_green", Status: "passed", Required: true, Reason: "live preflight"},
			{Check: "mergeable_clean", Status: "passed", Required: true, Reason: "live preflight"},
			{Check: "policy_profile_allows_merge", Status: "passed", Required: true, Reason: "live preflight"},
		},
	}
}

func TestNormalizePolicyProfile_DefaultsToAdvisory(t *testing.T) {
	if got := NormalizePolicyProfile(""); got != types.PolicyProfileAdvisory {
		t.Fatalf("empty policy = %q, want %q", got, types.PolicyProfileAdvisory)
	}
	if got := NormalizePolicyProfile(types.PolicyProfile("unknown")); got != types.PolicyProfileAdvisory {
		t.Fatalf("unknown policy = %q, want %q", got, types.PolicyProfileAdvisory)
	}
}

func TestApplyPolicy_AdvisoryZeroWrite(t *testing.T) {
	result := ApplyPolicy(policyDecision(types.ActionKindMerge, types.ActionKindComment), types.PolicyProfileAdvisory)
	if result.Profile != types.PolicyProfileAdvisory {
		t.Fatalf("profile = %q, want advisory", result.Profile)
	}
	if !result.DryRun {
		t.Fatalf("advisory result must be dry-run")
	}
	if len(result.ExecutableActions) != 0 {
		t.Fatalf("advisory must execute zero actions, got %v", result.ExecutableActions)
	}
	if len(result.DeniedActions) != 2 {
		t.Fatalf("advisory should deny all proposed actions, got %v", result.DeniedActions)
	}
	for _, denial := range result.DeniedActions {
		if denial.Reason != "advisory_zero_write" {
			t.Fatalf("denial reason = %q, want advisory_zero_write", denial.Reason)
		}
	}
}

func TestApplyPolicy_GuardedAllowsOnlyCommentAndLabel(t *testing.T) {
	decision := policyDecision(types.ActionKindMerge, types.ActionKindClose, types.ActionKindComment, types.ActionKindLabel, types.ActionKindApplyFix)
	result := ApplyPolicy(decision, types.PolicyProfileGuarded)
	if result.DryRun {
		t.Fatalf("guarded should not force dry-run for non-destructive actions")
	}
	if !slices.Equal(result.ExecutableActions, []types.ActionKind{types.ActionKindComment, types.ActionKindLabel}) {
		t.Fatalf("guarded executable = %v, want comment+label", result.ExecutableActions)
	}
	for _, action := range []types.ActionKind{types.ActionKindMerge, types.ActionKindClose, types.ActionKindApplyFix} {
		if !denied(result, action, "guarded_non_destructive_only") {
			t.Fatalf("guarded should deny %s, denials=%v", action, result.DeniedActions)
		}
	}
}

func TestApplyPolicy_AutonomousRequiresPassedPreflightForMerge(t *testing.T) {
	decision := policyDecision(types.ActionKindMerge, types.ActionKindComment)
	decision.RequiredPreflightChecks[2].Status = "pending"
	result := ApplyPolicy(decision, types.PolicyProfileAutonomous)
	if slices.Contains(result.ExecutableActions, types.ActionKindMerge) {
		t.Fatalf("autonomous must deny merge until required preflight passes, got %v", result.ExecutableActions)
	}
	if !slices.Contains(result.ExecutableActions, types.ActionKindComment) {
		t.Fatalf("autonomous should still allow comment, got %v", result.ExecutableActions)
	}
	if !denied(result, types.ActionKindMerge, "preflight_not_passed:ci_green") {
		t.Fatalf("expected ci_green preflight denial, got %v", result.DeniedActions)
	}
}

func TestApplyPolicy_AutonomousAllowsMergeAfterPreflight(t *testing.T) {
	result := ApplyPolicy(policyDecision(types.ActionKindMerge), types.PolicyProfileAutonomous)
	if result.DryRun {
		t.Fatalf("autonomous should not be dry-run after passing gates")
	}
	if !slices.Equal(result.ExecutableActions, []types.ActionKind{types.ActionKindMerge}) {
		t.Fatalf("autonomous executable = %v, want merge", result.ExecutableActions)
	}
	if len(result.DeniedActions) != 0 {
		t.Fatalf("unexpected denials: %v", result.DeniedActions)
	}
}

func TestApplyPolicy_CloseRequiresPreflightInAutonomous(t *testing.T) {
	decision := LaneDecision{
		PRNumber:       102,
		Lane:           types.ActionLaneDuplicateClose,
		Confidence:     0.88,
		AllowedActions: []types.ActionKind{types.ActionKindClose, types.ActionKindComment},
	}
	result := ApplyPolicy(decision, types.PolicyProfileAutonomous)
	if slices.Contains(result.ExecutableActions, types.ActionKindClose) {
		t.Fatalf("close without preflight must be denied, got executable %v", result.ExecutableActions)
	}
	if !slices.Contains(result.ExecutableActions, types.ActionKindComment) {
		t.Fatalf("comment should remain executable, got %v", result.ExecutableActions)
	}
	if !denied(result, types.ActionKindClose, "missing_required_preflight") {
		t.Fatalf("expected close missing preflight denial, got %v", result.DeniedActions)
	}
}

func TestApplyPolicy_HumanEscalateExecutesNothing(t *testing.T) {
	decision := LaneDecision{PRNumber: 103, Lane: types.ActionLaneHumanEscalate, AllowedActions: []types.ActionKind{types.ActionKindComment}}
	result := ApplyPolicy(decision, types.PolicyProfileAutonomous)
	if len(result.ExecutableActions) != 0 {
		t.Fatalf("human_escalate must execute nothing, got %v", result.ExecutableActions)
	}
	if !denied(result, types.ActionKindComment, "human_escalate_non_executable") {
		t.Fatalf("expected human escalation denial, got %v", result.DeniedActions)
	}
}

func TestApplyPolicy_PreservesProposedActionOrder(t *testing.T) {
	decision := policyDecision(types.ActionKindLabel, types.ActionKindComment, types.ActionKindMerge)
	result := ApplyPolicy(decision, types.PolicyProfileGuarded)
	if !slices.Equal(result.ProposedActions, decision.AllowedActions) {
		t.Fatalf("proposed actions = %v, want %v", result.ProposedActions, decision.AllowedActions)
	}
	if !slices.Equal(result.ExecutableActions, []types.ActionKind{types.ActionKindLabel, types.ActionKindComment}) {
		t.Fatalf("executable order = %v", result.ExecutableActions)
	}
}

func denied(result PolicyGateResult, action types.ActionKind, reason string) bool {
	for _, denial := range result.DeniedActions {
		if denial.Action == action && denial.Reason == reason {
			return true
		}
	}
	return false
}
