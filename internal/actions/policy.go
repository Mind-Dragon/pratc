package actions

import (
	"slices"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// ActionDenial records why a proposed action is not executable under a policy profile.
type ActionDenial struct {
	Action types.ActionKind
	Reason string
}

// PolicyGateResult is the policy-filtered view of a lane decision.
// ProposedActions preserves classifier output; ExecutableActions are the actions
// this policy profile may actually run after gate checks.
type PolicyGateResult struct {
	Profile           types.PolicyProfile
	DryRun            bool
	ProposedActions   []types.ActionKind
	ExecutableActions []types.ActionKind
	DeniedActions     []ActionDenial
	Reasons           []string
}

// NormalizePolicyProfile returns advisory for empty or unrecognized profiles.
func NormalizePolicyProfile(profile types.PolicyProfile) types.PolicyProfile {
	switch profile {
	case types.PolicyProfileAdvisory, types.PolicyProfileGuarded, types.PolicyProfileAutonomous:
		return profile
	default:
		return types.PolicyProfileAdvisory
	}
}

// ApplyPolicy gates a classifier lane decision through a policy profile.
// It never mutates external state; it only describes which proposed actions may execute.
func ApplyPolicy(decision LaneDecision, profile types.PolicyProfile) PolicyGateResult {
	normalized := NormalizePolicyProfile(profile)
	result := PolicyGateResult{
		Profile:         normalized,
		DryRun:          normalized == types.PolicyProfileAdvisory,
		ProposedActions: slices.Clone(decision.AllowedActions),
		Reasons:         slices.Clone(decision.ReasonTrail),
	}

	if decision.Lane == types.ActionLaneHumanEscalate {
		denyAll(&result, decision.AllowedActions, "human_escalate_non_executable")
		return result
	}

	switch normalized {
	case types.PolicyProfileAdvisory:
		denyAll(&result, decision.AllowedActions, "advisory_zero_write")
		return result
	case types.PolicyProfileGuarded:
		for _, action := range decision.AllowedActions {
			if action == types.ActionKindComment || action == types.ActionKindLabel {
				result.ExecutableActions = append(result.ExecutableActions, action)
				continue
			}
			result.DeniedActions = append(result.DeniedActions, ActionDenial{Action: action, Reason: "guarded_non_destructive_only"})
		}
		return result
	case types.PolicyProfileAutonomous:
		for _, action := range decision.AllowedActions {
			if requiresLivePreflight(action) {
				if reason := preflightDenialReason(decision.RequiredPreflightChecks); reason != "" {
					result.DeniedActions = append(result.DeniedActions, ActionDenial{Action: action, Reason: reason})
					continue
				}
			}
			result.ExecutableActions = append(result.ExecutableActions, action)
		}
		return result
	default:
		denyAll(&result, decision.AllowedActions, "advisory_zero_write")
		return result
	}
}

func denyAll(result *PolicyGateResult, actions []types.ActionKind, reason string) {
	for _, action := range actions {
		result.DeniedActions = append(result.DeniedActions, ActionDenial{Action: action, Reason: reason})
	}
}

func requiresLivePreflight(action types.ActionKind) bool {
	switch action {
	case types.ActionKindMerge, types.ActionKindClose:
		return true
	default:
		return false
	}
}

func preflightDenialReason(checks []types.ActionPreflight) string {
	requiredSeen := false
	for _, check := range checks {
		if !check.Required {
			continue
		}
		requiredSeen = true
		if check.Status != "passed" {
			if check.Check == "" {
				return "preflight_not_passed:unknown"
			}
			return "preflight_not_passed:" + check.Check
		}
	}
	if !requiredSeen {
		return "missing_required_preflight"
	}
	return ""
}
