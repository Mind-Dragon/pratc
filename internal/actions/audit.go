package actions

import (
	"fmt"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const (
	AuditCheckLaneCoverage       = "lane_coverage"
	AuditCheckAdvisoryZeroWrite  = "advisory_zero_write"
	AuditCheckIntentCompleteness = "intent_completeness"
)

// AuditActionPlan evaluates v2 action-plan invariants that are local to the
// action engine contract. It does not perform live GitHub checks.
func AuditActionPlan(plan types.ActionPlan) types.ActionPlanAudit {
	return types.ActionPlanAudit{
		Checks: []types.ActionPlanAuditCheck{
			auditLaneCoverage(plan),
			auditAdvisoryZeroWrite(plan),
			auditIntentCompleteness(plan),
		},
	}
}

func auditLaneCoverage(plan types.ActionPlan) types.ActionPlanAuditCheck {
	check := types.ActionPlanAuditCheck{Name: AuditCheckLaneCoverage, Status: "pass", Reason: "every work item has exactly one primary action lane"}

	if plan.CorpusSnapshot.TotalPRs > 0 && len(plan.WorkItems) != plan.CorpusSnapshot.TotalPRs {
		check.Status = "fail"
		check.Reason = fmt.Sprintf("work item count %d does not cover corpus total %d", len(plan.WorkItems), plan.CorpusSnapshot.TotalPRs)
		return check
	}

	validLanes := map[types.ActionLane]bool{
		types.ActionLaneFastMerge:        true,
		types.ActionLaneFixAndMerge:      true,
		types.ActionLaneDuplicateClose:   true,
		types.ActionLaneRejectOrClose:    true,
		types.ActionLaneFocusedReview:    true,
		types.ActionLaneFutureOrReengage: true,
		types.ActionLaneHumanEscalate:    true,
	}
	laneCounts := make(map[types.ActionLane]int)
	itemLaneByID := make(map[string]types.ActionLane)
	seenPRs := make(map[int]bool)

	for _, item := range plan.WorkItems {
		if item.Lane == "" {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("work item %s for PR #%d has empty lane", item.ID, item.PRNumber)
			return check
		}
		if !validLanes[item.Lane] {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("work item %s for PR #%d has unknown lane %q", item.ID, item.PRNumber, item.Lane)
			return check
		}
		if seenPRs[item.PRNumber] {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("PR #%d appears in multiple work items", item.PRNumber)
			return check
		}
		seenPRs[item.PRNumber] = true
		laneCounts[item.Lane]++
		if item.ID != "" {
			itemLaneByID[item.ID] = item.Lane
		}
	}

	for _, summary := range plan.Lanes {
		if !validLanes[summary.Lane] {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("lane summary has unknown lane %q", summary.Lane)
			return check
		}
		if got := laneCounts[summary.Lane]; got != summary.Count {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("lane %s summary count %d does not match work item count %d", summary.Lane, summary.Count, got)
			return check
		}
		if len(summary.WorkItemIDs) != summary.Count {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("lane %s summary has %d work item IDs but count %d", summary.Lane, len(summary.WorkItemIDs), summary.Count)
			return check
		}
		for _, id := range summary.WorkItemIDs {
			lane, ok := itemLaneByID[id]
			if !ok {
				check.Status = "fail"
				check.Reason = fmt.Sprintf("lane %s summary references unknown work item %s", summary.Lane, id)
				return check
			}
			if lane != summary.Lane {
				check.Status = "fail"
				check.Reason = fmt.Sprintf("lane %s summary references work item %s in lane %s", summary.Lane, id, lane)
				return check
			}
		}
	}

	return check
}

func auditAdvisoryZeroWrite(plan types.ActionPlan) types.ActionPlanAuditCheck {
	check := types.ActionPlanAuditCheck{Name: AuditCheckAdvisoryZeroWrite, Status: "pass", Reason: "advisory profile has only dry-run action intents"}
	if NormalizePolicyProfile(plan.PolicyProfile) != types.PolicyProfileAdvisory {
		check.Status = "manual"
		check.Reason = fmt.Sprintf("plan policy %q is not advisory; zero-write check is advisory-only", plan.PolicyProfile)
		return check
	}
	for _, intent := range plan.ActionIntents {
		if !intent.DryRun {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d is not dry-run under advisory policy", intent.ID, intent.PRNumber)
			return check
		}
		if NormalizePolicyProfile(intent.PolicyProfile) != types.PolicyProfileAdvisory {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has policy %q under advisory plan", intent.ID, intent.PRNumber, intent.PolicyProfile)
			return check
		}
	}
	return check
}

func auditIntentCompleteness(plan types.ActionPlan) types.ActionPlanAuditCheck {
	check := types.ActionPlanAuditCheck{Name: AuditCheckIntentCompleteness, Status: "pass", Reason: "every action intent has required fields populated"}
	for _, intent := range plan.ActionIntents {
		if len(intent.Reasons) == 0 {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has empty reasons", intent.ID, intent.PRNumber)
			return check
		}
		if len(intent.EvidenceRefs) == 0 {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has empty evidence_refs", intent.ID, intent.PRNumber)
			return check
		}
		if intent.IdempotencyKey == "" {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has empty idempotency_key", intent.ID, intent.PRNumber)
			return check
		}
		if intent.Confidence <= 0.0 {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has invalid confidence %.2f", intent.ID, intent.PRNumber, intent.Confidence)
			return check
		}
		if intent.Preconditions == nil {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has nil preconditions", intent.ID, intent.PRNumber)
			return check
		}
		if intent.PolicyProfile == "" {
			check.Status = "fail"
			check.Reason = fmt.Sprintf("intent %s for PR #%d has empty policy_profile", intent.ID, intent.PRNumber)
			return check
		}
	}
	return check
}
