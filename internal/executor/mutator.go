package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// Mutator handles GitHub mutation operations with policy enforcement
type Mutator struct {
	mutator GitHubMutator
	ledger  Ledger
	cfg     Config
	now     func() time.Time
}

// NewMutator creates a new mutator with the given configuration
func NewMutator(cfg Config, mutator GitHubMutator, ledger Ledger) *Mutator {
	if ledger == nil {
		ledger = NewMemoryLedger()
	}
	return &Mutator{
		mutator: mutator,
		ledger:  ledger,
		cfg:     cfg,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// ExecuteComment executes a comment action intent
func (m *Mutator) ExecuteComment(ctx context.Context, intent types.ActionIntent) (types.ExecutionResult, error) {
	if intent.Action != types.ActionKindComment {
		return types.ExecutionResult{}, fmt.Errorf("expected comment action, got %s", intent.Action)
	}

	return m.executeAction(ctx, intent, func(dryRun bool) error {
		body := firstString(intent.Reasons, "prATC action intent")
		return m.mutator.AddComment(ctx, m.cfg.Repo, intent.PRNumber, body, dryRun)
	})
}

// ExecuteLabel executes a label action intent
func (m *Mutator) ExecuteLabel(ctx context.Context, intent types.ActionIntent) (types.ExecutionResult, error) {
	if intent.Action != types.ActionKindLabel {
		return types.ExecutionResult{}, fmt.Errorf("expected label action, got %s", intent.Action)
	}

	return m.executeAction(ctx, intent, func(dryRun bool) error {
		labels := []string{"pratc-action"}
		if intent.Payload != nil {
			if customLabels, ok := intent.Payload["labels"].([]string); ok && len(customLabels) > 0 {
				labels = customLabels
			}
		}
		return m.mutator.AddLabels(ctx, m.cfg.Repo, intent.PRNumber, labels, dryRun)
	})
}

// executeAction is a helper that handles common execution logic
func (m *Mutator) executeAction(ctx context.Context, intent types.ActionIntent, mutateFunc func(dryRun bool) error) (types.ExecutionResult, error) {
	if intent.IdempotencyKey == "" {
		return types.ExecutionResult{}, fmt.Errorf("idempotency key is required")
	}

	if m.ledger.IsExecuted(intent.IdempotencyKey) {
		return types.ExecutionResult{
			IntentID:        intent.ID,
			Action:          intent.Action,
			PRNumber:        intent.PRNumber,
			DryRun:          effectiveDryRun(m.cfg, intent),
			Executed:        false,
			AlreadyExecuted: true,
			Result:          "already_executed",
			ExecutedAt:      m.now().Format(time.RFC3339Nano),
		}, nil
	}

	if err := m.policyAllows(intent); err != nil {
		return types.ExecutionResult{}, err
	}

	// Run preflight checks
	if err := m.runPreflightChecks(ctx, intent); err != nil {
		return types.ExecutionResult{}, fmt.Errorf("preflight check failed: %w", err)
	}

	dryRun := effectiveDryRun(m.cfg, intent)
	result := types.ExecutionResult{
		IntentID:   intent.ID,
		Action:     intent.Action,
		PRNumber:   intent.PRNumber,
		DryRun:     dryRun,
		ExecutedAt: m.now().Format(time.RFC3339Nano),
	}

	if err := mutateFunc(dryRun); err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Executed = !dryRun
	result.Result = "success"

	if err := m.ledger.Record(intent.IdempotencyKey, result); err != nil {
		return types.ExecutionResult{}, err
	}

	return result, nil
}

// policyAllows checks if the intent is allowed under the current policy
func (m *Mutator) policyAllows(intent types.ActionIntent) error {
	if intent.Action == "" {
		return fmt.Errorf("action is required")
	}

	if effectiveDryRun(m.cfg, intent) {
		return nil
	}

	switch m.cfg.PolicyProfile {
	case types.PolicyProfileAdvisory, "":
		return fmt.Errorf("advisory policy denies non-dry-run action %s", intent.Action)
	case types.PolicyProfileGuarded:
		if intent.Action == types.ActionKindComment || intent.Action == types.ActionKindLabel {
			return nil
		}
		return fmt.Errorf("guarded policy denies action %s", intent.Action)
	case types.PolicyProfileAutonomous:
		return nil
	default:
		return fmt.Errorf("unknown policy profile %q", m.cfg.PolicyProfile)
	}
}

// runPreflightChecks runs all preflight checks for an intent
func (m *Mutator) runPreflightChecks(ctx context.Context, intent types.ActionIntent) error {
	// Get expected head SHA from payload if available
	expectedHeadSHA := ""
	if intent.Payload != nil {
		if sha, ok := intent.Payload["expected_head_sha"].(string); ok {
			expectedHeadSHA = sha
		}
	}

	// Get allowed branches from payload if available
	allowedBranches := []string{}
	if intent.Payload != nil {
		if branches, ok := intent.Payload["allowed_branches"].([]string); ok {
			allowedBranches = branches
		}
	}

	// 1. checkPROpen - verify PR still open
	if err := checkPROpen(ctx, m.mutator, m.cfg.Repo, intent.PRNumber); err != nil {
		return err
	}

	// 2. checkHeadSHA - verify head SHA unchanged
	if err := checkHeadSHA(ctx, m.mutator, m.cfg.Repo, intent.PRNumber, expectedHeadSHA); err != nil {
		return err
	}

	// 3. checkBaseBranch - verify base branch
	if err := checkBaseBranch(ctx, m.mutator, m.cfg.Repo, intent.PRNumber, allowedBranches); err != nil {
		return err
	}

	// 4. checkTokenPermission - verify token has permission
	if err := checkTokenPermission(ctx, m.mutator, m.cfg.Repo, intent.Action); err != nil {
		return err
	}

	// 5. checkRateLimit - verify rate-limit budget
	if err := checkRateLimit(ctx, m.mutator); err != nil {
		return err
	}

	// 6. checkIdempotency - verify idempotency key not executed (already checked earlier)
	if err := checkIdempotency(ctx, m.ledger, intent.IdempotencyKey); err != nil {
		return err
	}

	return nil
}
