package executor

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const (
	PreflightStatusPassed  = "passed"
	PreflightStatusFailed  = "failed"
	PreflightStatusSkipped = "skipped"

	CheckRateLimitBudget      = "rate_limit_sufficient"
	CheckTokenScope           = "token_scope_sufficient"
	CheckPRStillOpen          = "pr_still_open"
	CheckHeadSHAUnchanged     = "head_sha_unchanged"
	CheckCIGreen              = "ci_green"
	CheckMergeableClean       = "mergeable_clean"
	CheckBranchProtection     = "branch_protection_satisfied"
	CheckPolicyAllowsMutation = "policy_profile_allows_mutation"
)

type LivePRState struct {
	PRNumber                 int
	State                    string
	HeadSHA                  string
	CIStatus                 string
	Mergeable                string
	RequiredReviewsSatisfied bool
	TokenScopeSufficient     bool
	RateLimitRemaining       int
}

type PreflightOptions struct {
	ExpectedHeadSHA string
	Live            LivePRState
	Now             time.Time
}

type PreflightResult struct {
	PRNumber    int
	Checks      []types.ActionPreflight
	AllPassed   bool
	FailedCount int
	Timestamp   string
}

func RunPreflight(intent types.ActionIntent, opts PreflightOptions) PreflightResult {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	result := PreflightResult{PRNumber: intent.PRNumber, AllPassed: true, Timestamp: now.Format(time.RFC3339Nano)}
	add := func(name, status, reason string, required bool) {
		if status == PreflightStatusFailed {
			result.AllPassed = false
			result.FailedCount++
		}
		result.Checks = append(result.Checks, types.ActionPreflight{
			Check:        name,
			Status:       status,
			Reason:       reason,
			EvidenceRefs: []string{fmt.Sprintf("live:pr:%d:%s", intent.PRNumber, name)},
			Required:     required,
			CheckedAt:    now.Format(time.RFC3339Nano),
		})
	}

	if opts.Live.RateLimitRemaining > 0 {
		add(CheckRateLimitBudget, PreflightStatusPassed, "rate limit budget available", true)
	} else {
		add(CheckRateLimitBudget, PreflightStatusFailed, "rate limit budget exhausted", true)
	}
	if opts.Live.TokenScopeSufficient {
		add(CheckTokenScope, PreflightStatusPassed, "token scope sufficient", true)
	} else {
		add(CheckTokenScope, PreflightStatusFailed, "token scope insufficient", true)
	}
	if strings.EqualFold(opts.Live.State, "open") {
		add(CheckPRStillOpen, PreflightStatusPassed, "PR is open", true)
	} else {
		add(CheckPRStillOpen, PreflightStatusFailed, "PR is not open", true)
	}
	if opts.ExpectedHeadSHA == "" || opts.ExpectedHeadSHA == opts.Live.HeadSHA {
		add(CheckHeadSHAUnchanged, PreflightStatusPassed, "head SHA unchanged", true)
	} else {
		add(CheckHeadSHAUnchanged, PreflightStatusFailed, "head SHA changed", true)
	}
	if intent.Action == types.ActionKindMerge {
		if isGreenCI(opts.Live.CIStatus) {
			add(CheckCIGreen, PreflightStatusPassed, "CI green", true)
		} else {
			add(CheckCIGreen, PreflightStatusFailed, "CI not green", true)
		}
		if isMergeableClean(opts.Live.Mergeable) {
			add(CheckMergeableClean, PreflightStatusPassed, "mergeability clean", true)
		} else {
			add(CheckMergeableClean, PreflightStatusFailed, "mergeability not clean", true)
		}
		if opts.Live.RequiredReviewsSatisfied {
			add(CheckBranchProtection, PreflightStatusPassed, "branch protection satisfied", true)
		} else {
			add(CheckBranchProtection, PreflightStatusFailed, "branch protection not satisfied", true)
		}
	} else {
		add(CheckCIGreen, PreflightStatusSkipped, "not required for non-merge action", false)
		add(CheckMergeableClean, PreflightStatusSkipped, "not required for non-merge action", false)
		add(CheckBranchProtection, PreflightStatusSkipped, "not required for non-merge action", false)
	}
	if intent.PolicyProfile == types.PolicyProfileAdvisory && !intent.DryRun {
		add(CheckPolicyAllowsMutation, PreflightStatusFailed, "advisory policy denies mutation", true)
	} else {
		add(CheckPolicyAllowsMutation, PreflightStatusPassed, "policy allows dry-run or mutation", true)
	}
	return result
}

func ValidateProofBundle(item types.ActionWorkItem, bundle types.ProofBundle) error {
	if bundle.ID == "" {
		return fmt.Errorf("proof bundle id is required")
	}
	if bundle.WorkItemID != item.ID {
		return fmt.Errorf("proof bundle work item mismatch: %s != %s", bundle.WorkItemID, item.ID)
	}
	if bundle.PRNumber != item.PRNumber {
		return fmt.Errorf("proof bundle PR mismatch: %d != %d", bundle.PRNumber, item.PRNumber)
	}
	if strings.TrimSpace(bundle.Summary) == "" {
		return fmt.Errorf("proof bundle summary is required")
	}
	if len(bundle.EvidenceRefs) == 0 || len(bundle.ArtifactRefs) == 0 {
		return fmt.Errorf("proof bundle evidence and artifacts are required")
	}
	if len(bundle.TestCommands) == 0 || len(bundle.TestResults) == 0 {
		return fmt.Errorf("proof bundle test commands and results are required")
	}
	if bundle.CreatedBy == "" || bundle.CreatedAt == "" {
		return fmt.Errorf("proof bundle creator and timestamp are required")
	}
	return nil
}

func isGreenCI(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "passed", "green", "clean":
		return true
	default:
		return false
	}
}

func isMergeableClean(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "clean", "mergeable", "true", "yes":
		return true
	default:
		return false
	}
}
