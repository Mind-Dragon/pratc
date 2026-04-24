package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type Config struct {
	Repo          string
	DryRun        bool
	PolicyProfile types.PolicyProfile
}

type ExecutionResult struct {
	IntentID        string           `json:"intent_id"`
	Action          types.ActionKind `json:"action"`
	PRNumber        int              `json:"pr_number"`
	DryRun          bool             `json:"dry_run"`
	Executed        bool             `json:"executed"`
	AlreadyExecuted bool             `json:"already_executed"`
	Result          string           `json:"result"`
	Error           string           `json:"error,omitempty"`
	ExecutedAt      string           `json:"executed_at"`
}

type MergeOptions struct {
	CommitTitle   string
	CommitMessage string
	MergeMethod   string
}

type MergeResult struct {
	Merged        bool
	SHA           string
	AlreadyMerged bool
}

type ApplyFixResult struct {
	Applied    bool
	NewBranch  string
	TestOutput string
}

type GitHubMutator interface {
	Merge(ctx context.Context, repo string, prNumber int, opts MergeOptions, dryRun bool) (MergeResult, error)
	Close(ctx context.Context, repo string, prNumber int, reason string, dryRun bool) error
	AddComment(ctx context.Context, repo string, prNumber int, body string, dryRun bool) error
	AddLabels(ctx context.Context, repo string, prNumber int, labels []string, dryRun bool) error
	ApplyFix(ctx context.Context, repo string, prNumber int, patch string, dryRun bool) (ApplyFixResult, error)
}

type Ledger interface {
	IsExecuted(key string) bool
	Record(key string, result ExecutionResult) error
}

type Executor struct {
	cfg     Config
	mutator GitHubMutator
	ledger  Ledger
	now     func() time.Time
}

func New(cfg Config, mutator GitHubMutator, ledger Ledger) *Executor {
	if ledger == nil {
		ledger = NewMemoryLedger()
	}
	return &Executor{cfg: cfg, mutator: mutator, ledger: ledger, now: func() time.Time { return time.Now().UTC() }}
}

func (e *Executor) ExecuteIntent(ctx context.Context, intent types.ActionIntent) (ExecutionResult, error) {
	if e.mutator == nil {
		return ExecutionResult{}, fmt.Errorf("github mutator is required")
	}
	if intent.IdempotencyKey == "" {
		return ExecutionResult{}, fmt.Errorf("idempotency key is required")
	}
	if e.ledger.IsExecuted(intent.IdempotencyKey) {
		return ExecutionResult{
			IntentID:        intent.ID,
			Action:          intent.Action,
			PRNumber:        intent.PRNumber,
			DryRun:          effectiveDryRun(e.cfg, intent),
			Executed:        false,
			AlreadyExecuted: true,
			Result:          "already_executed",
			ExecutedAt:      e.now().Format(time.RFC3339Nano),
		}, nil
	}
	if err := e.policyAllows(intent); err != nil {
		return ExecutionResult{}, err
	}
	dryRun := effectiveDryRun(e.cfg, intent)
	result := ExecutionResult{
		IntentID:   intent.ID,
		Action:     intent.Action,
		PRNumber:   intent.PRNumber,
		DryRun:     dryRun,
		ExecutedAt: e.now().Format(time.RFC3339Nano),
	}

	switch intent.Action {
	case types.ActionKindMerge:
		merge, err := e.mutator.Merge(ctx, e.cfg.Repo, intent.PRNumber, MergeOptions{}, dryRun)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Executed = !dryRun
		if merge.AlreadyMerged {
			result.Result = "already_merged"
		} else {
			result.Result = "merged"
		}
	case types.ActionKindClose:
		if err := e.mutator.Close(ctx, e.cfg.Repo, intent.PRNumber, firstString(intent.Reasons, "closed by prATC"), dryRun); err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Executed = !dryRun
		result.Result = "closed"
	case types.ActionKindComment, types.ActionKindRequestChanges:
		if err := e.mutator.AddComment(ctx, e.cfg.Repo, intent.PRNumber, firstString(intent.Reasons, "prATC action intent"), dryRun); err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Executed = !dryRun
		result.Result = "commented"
	case types.ActionKindLabel:
		if err := e.mutator.AddLabels(ctx, e.cfg.Repo, intent.PRNumber, []string{"pratc-action"}, dryRun); err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Executed = !dryRun
		result.Result = "labeled"
	case types.ActionKindApplyFix:
		if _, err := e.mutator.ApplyFix(ctx, e.cfg.Repo, intent.PRNumber, "", dryRun); err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Executed = !dryRun
		result.Result = "fix_applied"
	default:
		return ExecutionResult{}, fmt.Errorf("unknown action kind %q", intent.Action)
	}
	if err := e.ledger.Record(intent.IdempotencyKey, result); err != nil {
		return ExecutionResult{}, err
	}
	return result, nil
}

func (e *Executor) policyAllows(intent types.ActionIntent) error {
	if intent.Action == "" {
		return fmt.Errorf("action is required")
	}
	if effectiveDryRun(e.cfg, intent) {
		return nil
	}
	switch e.cfg.PolicyProfile {
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
		return fmt.Errorf("unknown policy profile %q", e.cfg.PolicyProfile)
	}
}

func effectiveDryRun(cfg Config, intent types.ActionIntent) bool {
	return cfg.DryRun || intent.DryRun
}

func firstString(values []string, fallback string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return fallback
}

type MemoryLedger struct {
	mu      sync.Mutex
	results map[string]ExecutionResult
}

func NewMemoryLedger() *MemoryLedger {
	return &MemoryLedger{results: map[string]ExecutionResult{}}
}

func (l *MemoryLedger) IsExecuted(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.results[key]
	return ok
}

func (l *MemoryLedger) Record(key string, result ExecutionResult) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.results[key]; ok {
		return fmt.Errorf("idempotency key %q already recorded", key)
	}
	l.results[key] = result
	return nil
}
