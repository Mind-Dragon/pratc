package executor

import (
	"context"
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

// 9 Preflight Gate Functions

// checkPROpen verifies that the PR is still open
func checkPROpen(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	prState, err := mutator.GetPRState(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get PR state: %w", err)
	}
	if strings.ToLower(prState.State) != "open" {
		return fmt.Errorf("PR #%d is not open (state: %s)", prNumber, prState.State)
	}
	return nil
}

// checkHeadSHA verifies that the head SHA is unchanged
func checkHeadSHA(ctx context.Context, mutator GitHubMutator, repo string, prNumber int, expectedSHA string) error {
	if expectedSHA == "" {
		// No expected SHA to check against
		return nil
	}
	currentSHA, err := mutator.GetHeadSHA(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get head SHA: %w", err)
	}
	if currentSHA != expectedSHA {
		return fmt.Errorf("head SHA changed: expected %s, got %s", expectedSHA, currentSHA)
	}
	return nil
}

// checkBaseBranch verifies that the base branch is in the allowed list
func checkBaseBranch(ctx context.Context, mutator GitHubMutator, repo string, prNumber int, allowedBranches []string) error {
	if len(allowedBranches) == 0 {
		// No restrictions on base branch
		return nil
	}
	baseBranch, err := mutator.GetBaseBranch(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get base branch: %w", err)
	}
	for _, allowed := range allowedBranches {
		if baseBranch == allowed {
			return nil
		}
	}
	return fmt.Errorf("base branch %s is not in allowed list: %v", baseBranch, allowedBranches)
}

// checkCIGreen verifies that CI checks are green
func checkCIGreen(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	ciStatus, err := mutator.GetCIStatus(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get CI status: %w", err)
	}
	if !isGreenCI(ciStatus) {
		return fmt.Errorf("CI is not green (status: %s)", ciStatus)
	}
	return nil
}

// checkMergeable verifies that the PR is mergeable
func checkMergeable(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	mergeable, err := mutator.GetMergeable(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get mergeable status: %w", err)
	}
	if !mergeable {
		return fmt.Errorf("PR #%d is not mergeable", prNumber)
	}
	return nil
}

// checkBranchProtection verifies that required reviews are satisfied
func checkBranchProtection(ctx context.Context, mutator GitHubMutator, repo string, prNumber int) error {
	reviewsSatisfied, err := mutator.GetRequiredReviews(ctx, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get required reviews: %w", err)
	}
	if !reviewsSatisfied {
		return fmt.Errorf("required reviews not satisfied for PR #%d", prNumber)
	}
	return nil
}

// checkTokenPermission verifies that the token has the required permission
func checkTokenPermission(ctx context.Context, mutator GitHubMutator, repo string, action types.ActionKind) error {
	// For now, we'll assume the token has permission if we can query the rate limit
	// In a real implementation, this would check token scopes
	_, err := mutator.GetRateLimitRemaining(ctx)
	if err != nil {
		return fmt.Errorf("failed to check token permission: %w", err)
	}
	// Additional token permission checks would go here
	return nil
}

// checkRateLimit verifies that there's rate limit budget remaining
func checkRateLimit(ctx context.Context, mutator GitHubMutator) error {
	remaining, err := mutator.GetRateLimitRemaining(ctx)
	if err != nil {
		return fmt.Errorf("failed to get rate limit: %w", err)
	}
	if remaining <= 0 {
		return fmt.Errorf("rate limit budget exhausted (remaining: %d)", remaining)
	}
	return nil
}

// checkIdempotency verifies that the idempotency key has not been executed
func checkIdempotency(ctx context.Context, ledger Ledger, key string) error {
	if key == "" {
		return fmt.Errorf("idempotency key is required")
	}
	if ledger.IsExecuted(key) {
		return fmt.Errorf("idempotency key %q has already been executed", key)
	}
	return nil
}

// RunPreflight runs all preflight checks for an action intent
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
