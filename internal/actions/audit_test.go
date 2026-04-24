package actions

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func sampleAuditedActionPlan() types.ActionPlan {
	return types.ActionPlan{
		SchemaVersion: "2.0",
		RunID:         "run-audit",
		Repo:          "owner/repo",
		PolicyProfile: types.PolicyProfileAdvisory,
		GeneratedAt:   "2026-04-24T09:00:00Z",
		CorpusSnapshot: types.ActionCorpusSnapshot{
			TotalPRs:          2,
			HeadSHAIndexed:    true,
			AnalysisTruncated: false,
			MaxPRsApplied:     0,
		},
		Lanes: []types.ActionLaneSummary{
			{Lane: types.ActionLaneFastMerge, Count: 1, WorkItemIDs: []string{"wi-1"}},
			{Lane: types.ActionLaneFocusedReview, Count: 1, WorkItemIDs: []string{"wi-2"}},
		},
		WorkItems: []types.ActionWorkItem{
			{ID: "wi-1", PRNumber: 1, Lane: types.ActionLaneFastMerge, State: types.ActionWorkItemStateProposed, Confidence: 0.91, AllowedActions: []types.ActionKind{types.ActionKindMerge}},
			{ID: "wi-2", PRNumber: 2, Lane: types.ActionLaneFocusedReview, State: types.ActionWorkItemStateProposed, Confidence: 0.75, AllowedActions: []types.ActionKind{types.ActionKindComment}},
		},
		ActionIntents: []types.ActionIntent{
			{ID: "intent-1", Action: types.ActionKindMerge, PRNumber: 1, Lane: types.ActionLaneFastMerge, DryRun: true, PolicyProfile: types.PolicyProfileAdvisory, Confidence: 0.91, Reasons: []string{"clean_fast_merge"}, EvidenceRefs: []string{"github:pr/1"}, Preconditions: []types.ActionPreflight{{Check: "ci_green", Status: "pending", Required: true}}, IdempotencyKey: "owner/repo#1:merge", CreatedAt: "2026-04-24T09:01:00Z"},
			{ID: "intent-2", Action: types.ActionKindComment, PRNumber: 2, Lane: types.ActionLaneFocusedReview, DryRun: true, PolicyProfile: types.PolicyProfileAdvisory, Confidence: 0.75, Reasons: []string{"needs_review"}, EvidenceRefs: []string{"github:pr/2"}, Preconditions: []types.ActionPreflight{}, IdempotencyKey: "owner/repo#2:comment", CreatedAt: "2026-04-24T09:02:00Z"},
		},
	}
}

func TestAuditActionPlan_PassesLaneCoverageAndAdvisoryZeroWrite(t *testing.T) {
	audit := AuditActionPlan(sampleAuditedActionPlan())
	assertAuditStatus(t, audit, AuditCheckLaneCoverage, "pass")
	assertAuditStatus(t, audit, AuditCheckAdvisoryZeroWrite, "pass")
}

func TestAuditActionPlan_FailsWhenWorkItemMissingLane(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.WorkItems[1].Lane = ""
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckLaneCoverage, "fail")
}

func TestAuditActionPlan_FailsWhenLaneSummaryCountDoesNotMatchWorkItems(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.Lanes[0].Count = 2
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckLaneCoverage, "fail")
}

func TestAuditActionPlan_FailsWhenCorpusCountNotCovered(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.CorpusSnapshot.TotalPRs = 3
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckLaneCoverage, "fail")
}

func TestAuditActionPlan_FailsAdvisoryZeroWriteWhenIntentNotDryRun(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].DryRun = false
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckAdvisoryZeroWrite, "fail")
}

func TestAuditActionPlan_FailsAdvisoryZeroWriteWhenIntentPolicyProfileDiffers(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].PolicyProfile = types.PolicyProfileAutonomous
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckAdvisoryZeroWrite, "fail")
}

func TestAuditActionPlan_SkipsAdvisoryZeroWriteForNonAdvisoryPlan(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.PolicyProfile = types.PolicyProfileGuarded
	plan.ActionIntents[0].DryRun = false
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckAdvisoryZeroWrite, "manual")
}

func TestAuditActionPlan_FailsIntentMissingReasons(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].Reasons = nil
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_FailsIntentMissingEvidenceRefs(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].EvidenceRefs = nil
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_FailsIntentMissingIdempotencyKey(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].IdempotencyKey = ""
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_FailsIntentMissingConfidence(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].Confidence = 0.0
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_FailsIntentNegativeConfidence(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].Confidence = -0.1
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_FailsIntentMissingPreconditions(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].Preconditions = nil
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_FailsIntentMissingPolicyProfile(t *testing.T) {
	plan := sampleAuditedActionPlan()
	plan.ActionIntents[0].PolicyProfile = ""
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "fail")
}

func TestAuditActionPlan_PassesIntentWithAllFieldsPopulated(t *testing.T) {
	plan := sampleAuditedActionPlan()
	audit := AuditActionPlan(plan)
	assertAuditStatus(t, audit, AuditCheckIntentCompleteness, "pass")
}

func assertAuditStatus(t *testing.T, audit types.ActionPlanAudit, checkName, want string) {
	t.Helper()
	for _, check := range audit.Checks {
		if check.Name == checkName {
			if check.Status != want {
				t.Fatalf("check %s status = %s, want %s; reason=%s", checkName, check.Status, want, check.Reason)
			}
			return
		}
	}
	t.Fatalf("audit missing check %s; checks=%v", checkName, audit.Checks)
}
