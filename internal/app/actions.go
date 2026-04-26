package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	actionpkg "github.com/jeffersonnunn/pratc/internal/actions"
	"github.com/jeffersonnunn/pratc/internal/types"
)

// ActionOptions controls ActionPlan generation. The service remains advisory-only:
// it computes work items/intents and never performs GitHub mutations.
type ActionOptions struct {
	PolicyProfile types.PolicyProfile
	LaneFilter    string
	DryRun        bool
}

type duplicateEvidence struct {
	groupID     string
	canonicalPR int
	confidence  float64
	conflict    bool
}

func (s Service) Actions(ctx context.Context, repo string, opts ActionOptions) (types.ActionPlan, error) {
	laneFilter, err := parseActionLaneFilter(opts.LaneFilter)
	if err != nil {
		return types.ActionPlan{}, err
	}

	analysis, err := s.Analyze(ctx, repo)
	if err != nil {
		return types.ActionPlan{}, err
	}

	profile := actionpkg.NormalizePolicyProfile(opts.PolicyProfile)
	nowFn := s.now
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	now := nowFn().UTC()
	generatedAt := now.Format(time.RFC3339)
	runID := fmt.Sprintf("action-plan-%s", now.Format("20060102T150405Z"))

	dupEvidence := buildDuplicateEvidence(analysis.DuplicateSynthesis)
	reviews := actionReviewResults(analysis)

	workItems := make([]types.ActionWorkItem, 0, len(reviews))
	intents := make([]types.ActionIntent, 0, len(reviews))
	for _, reviewResult := range reviews {
		evidence := classificationEvidenceFor(reviewResult.PRNumber, dupEvidence)
		decision := actionpkg.ClassifyLane(reviewResult, evidence)
		if laneFilter != "" && decision.Lane != laneFilter {
			continue
		}

		gate := actionpkg.ApplyPolicy(decision, profile)
		workItem := actionWorkItemFromDecision(analysis.Repo, decision, gate)
		workItems = append(workItems, workItem)
		intents = append(intents, actionIntentsFromDecision(analysis.Repo, runID, generatedAt, decision, gate, opts.DryRun)...)
	}

	plan := types.ActionPlan{
		SchemaVersion: "2.0",
		RunID:         runID,
		Repo:          analysis.Repo,
		PolicyProfile: profile,
		GeneratedAt:   generatedAt,
		CorpusSnapshot: types.ActionCorpusSnapshot{
			TotalPRs:          analysis.Counts.TotalPRs,
			HeadSHAIndexed:    false,
			AnalysisTruncated: analysis.AnalysisTruncated,
			MaxPRsApplied:     analysis.MaxPRsApplied,
		},
		Lanes:         summarizeActionLanes(workItems),
		WorkItems:     workItems,
		ActionIntents: intents,
	}
	plan.Audit = actionpkg.AuditActionPlan(plan)
	return plan, nil
}

func parseActionLaneFilter(raw string) (types.ActionLane, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	lane := types.ActionLane(trimmed)
	if isValidActionLane(lane) {
		return lane, nil
	}
	return "", fmt.Errorf("invalid action lane %q", raw)
}

func isValidActionLane(lane types.ActionLane) bool {
	switch lane {
	case types.ActionLaneFastMerge,
		types.ActionLaneFixAndMerge,
		types.ActionLaneDuplicateClose,
		types.ActionLaneRejectOrClose,
		types.ActionLaneFocusedReview,
		types.ActionLaneFutureOrReengage,
		types.ActionLaneHumanEscalate:
		return true
	default:
		return false
	}
}

func buildDuplicateEvidence(plans []types.DuplicateSynthesisPlan) map[int]duplicateEvidence {
	out := make(map[int]duplicateEvidence)
	for _, plan := range plans {
		canonical := plan.NominatedCanonicalPR
		if canonical == 0 {
			canonical = plan.OriginalCanonicalPR
		}
		for _, candidate := range plan.Candidates {
			if candidate.PRNumber == 0 || candidate.PRNumber == canonical {
				continue
			}
			out[candidate.PRNumber] = duplicateEvidence{
				groupID:     plan.GroupID,
				canonicalPR: canonical,
				confidence:  maxDuplicateConfidence(plan.Similarity, candidate.Confidence),
				conflict:    candidate.ConflictFootprint > 0,
			}
		}
	}
	return out
}

func classificationEvidenceFor(prNumber int, dup map[int]duplicateEvidence) actionpkg.ClassificationEvidence {
	if ev, ok := dup[prNumber]; ok {
		return actionpkg.ClassificationEvidence{
			DuplicateGroupID:    ev.groupID,
			CanonicalPRNumber:   ev.canonicalPR,
			DuplicateConfidence: ev.confidence,
			CanonicalConflict:   ev.conflict,
		}
	}
	return actionpkg.ClassificationEvidence{}
}

func actionReviewResults(analysis types.AnalysisResponse) []types.ReviewResult {
	if analysis.ReviewPayload != nil && len(analysis.ReviewPayload.Results) > 0 {
		results := append([]types.ReviewResult(nil), analysis.ReviewPayload.Results...)
		sort.Slice(results, func(i, j int) bool { return results[i].PRNumber < results[j].PRNumber })
		return results
	}

	results := make([]types.ReviewResult, 0, len(analysis.PRs))
	for _, pr := range analysis.PRs {
		results = append(results, types.ReviewResult{
			PRNumber:           pr.Number,
			Confidence:         0.0,
			Category:           types.ReviewCategoryUnknownEscalate,
			PriorityTier:       types.PriorityTierReviewRequired,
			Reasons:            []string{"review_payload_missing"},
			NextAction:         "human_review",
			TemporalBucket:     "future",
			BlastRadius:        "unknown",
			EvidenceReferences: []string{fmt.Sprintf("pr:%d", pr.Number)},
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].PRNumber < results[j].PRNumber })
	return results
}

func actionWorkItemFromDecision(repo string, decision actionpkg.LaneDecision, gate actionpkg.PolicyGateResult) types.ActionWorkItem {
	blocked := append([]string(nil), decision.BlockedReasons...)
	for _, denied := range gate.DeniedActions {
		blocked = append(blocked, fmt.Sprintf("policy_denied:%s:%s", denied.Action, denied.Reason))
	}
	return types.ActionWorkItem{
		ID:                      actionWorkItemID(decision.PRNumber),
		PRNumber:                decision.PRNumber,
		Lane:                    decision.Lane,
		State:                   types.ActionWorkItemStateProposed,
		PriorityScore:           decision.PriorityScore,
		Confidence:              decision.Confidence,
		RiskFlags:               append([]string(nil), decision.RiskFlags...),
		ReasonTrail:             append([]string(nil), decision.ReasonTrail...),
		EvidenceRefs:            append([]string(nil), decision.EvidenceRefs...),
		RequiredPreflightChecks: append([]types.ActionPreflight(nil), decision.RequiredPreflightChecks...),
		IdempotencyKey:          actionIdempotencyKey(repo, decision.PRNumber, decision.Lane, "work_item"),
		LeaseState:              types.ActionLease{},
		AllowedActions:          append([]types.ActionKind(nil), decision.AllowedActions...),
		BlockedReasons:          blocked,
		ProofBundleRefs:         []string{},
	}
}

func actionIntentsFromDecision(repo, runID, createdAt string, decision actionpkg.LaneDecision, gate actionpkg.PolicyGateResult, forceDryRun bool) []types.ActionIntent {
	intents := make([]types.ActionIntent, 0, len(gate.ProposedActions))
	for _, action := range gate.ProposedActions {
		dryRun := forceDryRun || gate.DryRun || !containsActionKind(gate.ExecutableActions, action)

		policyProfile := actionpkg.NormalizePolicyProfile(gate.Profile)
		reasons := append([]string(nil), decision.ReasonTrail...)
		if len(reasons) == 0 {
			reasons = []string{"no_reason_provided"}
		}
		evidenceRefs := append([]string(nil), decision.EvidenceRefs...)
		if len(evidenceRefs) == 0 {
			evidenceRefs = []string{fmt.Sprintf("classification:pr/%d", decision.PRNumber)}
		}
		preconditions := append([]types.ActionPreflight(nil), decision.RequiredPreflightChecks...)
		if preconditions == nil {
			preconditions = []types.ActionPreflight{}
		}

		intents = append(intents, types.ActionIntent{
			ID:             fmt.Sprintf("intent-%d-%s", decision.PRNumber, action),
			WorkItemID:     actionWorkItemID(decision.PRNumber),
			Action:         action,
			PRNumber:       decision.PRNumber,
			Lane:           decision.Lane,
			DryRun:         dryRun,
			PolicyProfile:  policyProfile,
			Confidence:     decision.Confidence,
			RiskFlags:      append([]string(nil), decision.RiskFlags...),
			Reasons:        reasons,
			EvidenceRefs:   evidenceRefs,
			Preconditions:  preconditions,
			IdempotencyKey: actionIdempotencyKey(repo, decision.PRNumber, decision.Lane, action),
			CreatedAt:      createdAt,
			Payload: map[string]any{
				"run_id":       runID,
				"work_item_id": actionWorkItemID(decision.PRNumber),
			},
		})
	}
	return intents
}

func summarizeActionLanes(workItems []types.ActionWorkItem) []types.ActionLaneSummary {
	byLane := make(map[types.ActionLane][]string)
	for _, item := range workItems {
		byLane[item.Lane] = append(byLane[item.Lane], item.ID)
	}
	order := []types.ActionLane{
		types.ActionLaneFastMerge,
		types.ActionLaneFixAndMerge,
		types.ActionLaneDuplicateClose,
		types.ActionLaneRejectOrClose,
		types.ActionLaneFocusedReview,
		types.ActionLaneFutureOrReengage,
		types.ActionLaneHumanEscalate,
	}
	summaries := make([]types.ActionLaneSummary, 0, len(byLane))
	for _, lane := range order {
		ids := byLane[lane]
		if len(ids) == 0 {
			continue
		}
		sort.Strings(ids)
		summaries = append(summaries, types.ActionLaneSummary{Lane: lane, Count: len(ids), WorkItemIDs: ids})
	}
	return summaries
}

func containsActionKind(actions []types.ActionKind, want types.ActionKind) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func actionWorkItemID(prNumber int) string {
	return fmt.Sprintf("wi-%d", prNumber)
}

func actionIdempotencyKey(repo string, prNumber int, lane types.ActionLane, action any) string {
	return fmt.Sprintf("%s#%d:%s:%v", repo, prNumber, lane, action)
}

func maxDuplicateConfidence(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
