package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/repo"
	"github.com/jeffersonnunn/pratc/internal/sandbox"
	"github.com/jeffersonnunn/pratc/internal/types"
	"github.com/jeffersonnunn/pratc/internal/workqueue"
)

type Config struct {
	Repo          string
	DryRun        bool
	PolicyProfile types.PolicyProfile
}

type MergeOptions struct {
	CommitTitle     string
	CommitMessage   string
	MergeMethod     string
	ExpectedHeadSHA string
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

	// Query methods for preflight checks
	GetPRState(ctx context.Context, repo string, prNumber int) (PRState, error)
	GetHeadSHA(ctx context.Context, repo string, prNumber int) (string, error)
	GetBaseBranch(ctx context.Context, repo string, prNumber int) (string, error)
	GetCIStatus(ctx context.Context, repo string, prNumber int) (string, error)
	GetMergeable(ctx context.Context, repo string, prNumber int) (bool, error)
	GetRequiredReviews(ctx context.Context, repo string, prNumber int) (bool, error)
	GetRateLimitRemaining(ctx context.Context) (int, error)

	// Query methods for verification
	GetComments(ctx context.Context, repo string, prNumber int) ([]Comment, error)
	GetLabels(ctx context.Context, repo string, prNumber int) ([]string, error)
}

// WriteTracker is an optional interface that GitHubMutator implementations can implement
// to track write operations for audit purposes.
type WriteTracker interface {
	HasWritten() bool
	WriteCount() int
}

type Comment struct {
	Body string
}

type PRState struct {
	Number     int
	State      string
	HeadSHA    string
	BaseBranch string
	Mergeable  bool
	CIStatus   string
}

type Executor struct {
	cfg            Config
	mutator        GitHubMutator
	ledger         Ledger
	now            func() time.Time
	sandboxManager *sandbox.SandboxManager
	mirrorPath     string
	mirrorPathErr  error
	queue          *workqueue.Queue
}

func New(cfg Config, mutator GitHubMutator, ledger Ledger) *Executor {
	if ledger == nil {
		ledger = NewMemoryLedger()
	}
	return &Executor{cfg: cfg, mutator: mutator, ledger: ledger, now: func() time.Time { return time.Now().UTC() }}
}

// getMirrorPath computes the local mirror path for the configured repo.
func (e *Executor) getMirrorPath() (string, error) {
	if e.mirrorPath != "" || e.mirrorPathErr != nil {
		return e.mirrorPath, e.mirrorPathErr
	}
	baseDir, err := repo.DefaultBaseDir()
	if err != nil {
		e.mirrorPathErr = err
		return "", err
	}
	path, err := repo.MirrorPath(baseDir, e.cfg.Repo)
	if err != nil {
		e.mirrorPathErr = err
		return "", err
	}
	e.mirrorPath = path
	return path, nil
}

// getSandboxManager returns the sandbox manager, creating it if needed.
func (e *Executor) getSandboxManager() (*sandbox.SandboxManager, error) {
	if e.sandboxManager != nil {
		return e.sandboxManager, nil
	}
	e.sandboxManager = sandbox.NewSandboxManager()
	return e.sandboxManager, nil
}

// ExecuteFixAndMerge runs the fix-and-merge sandbox for a work item.
// It creates an isolated worktree, applies the patch, runs test commands,
// captures a proof bundle, and returns it.
func (e *Executor) ExecuteFixAndMerge(ctx context.Context, workItemID string, prNumber int, patch string, testCommands []string) (types.ProofBundle, error) {
	// Get sandbox manager
	sbm, err := e.getSandboxManager()
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("sandbox manager: %w", err)
	}
	// Get sandbox for this PR
	sandbox := sbm.GetSandbox(prNumber)
	if sandbox == nil {
		return types.ProofBundle{}, fmt.Errorf("failed to get sandbox for PR %d", prNumber)
	}
	// Create isolated worktree
	if err := sandbox.CreateWorktree(); err != nil {
		return types.ProofBundle{}, fmt.Errorf("create worktree: %w", err)
	}
	// Ensure cleanup on exit (but keep worktree for debugging? we'll clean up later)
	defer sandbox.Cleanup()

	// Apply patch
	if err := sandbox.ApplyPatch(patch); err != nil {
		return types.ProofBundle{}, fmt.Errorf("apply patch: %w", err)
	}

	// Run test commands
	output, exitCode, err := sandbox.RunTests(testCommands)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("run tests: %w", err)
	}
	// Capture proof bundle
	bundle := sandbox.CaptureProofBundle()
	// Fill missing fields
	bundle.WorkItemID = workItemID
	bundle.PRNumber = prNumber
	bundle.Summary = fmt.Sprintf("Fix and merge sandbox proof for PR #%d (exit code %d)", prNumber, exitCode)
	// EvidenceRefs: store patch diff as artifact reference (maybe write to a file)
	patchFile := filepath.Join(sandbox.GetWorktreePath(), "patch.diff")
	if err := os.WriteFile(patchFile, []byte(patch), 0o644); err != nil {
		return types.ProofBundle{}, fmt.Errorf("write patch file: %w", err)
	}
	bundle.EvidenceRefs = []string{patchFile}
	// ArtifactRefs: store test output as artifact
	outputFile := filepath.Join(sandbox.GetWorktreePath(), "test_output.txt")
	if err := os.WriteFile(outputFile, []byte(output), 0o644); err != nil {
		return types.ProofBundle{}, fmt.Errorf("write test output file: %w", err)
	}
	bundle.ArtifactRefs = []string{outputFile}
	bundle.TestCommands = testCommands
	bundle.TestResults = []string{output}
	bundle.CreatedBy = "executor"
	bundle.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	// Validate proof bundle
	if err := ValidateProofBundle(types.ActionWorkItem{ID: workItemID, PRNumber: prNumber}, bundle); err != nil {
		return types.ProofBundle{}, fmt.Errorf("proof bundle validation failed: %w", err)
	}

	// Attach proof bundle to work item if queue is available
	if e.queue != nil {
		// TODO: need workerID; maybe get from lease? For now, use "executor"
		_, err := e.queue.AttachProof(ctx, workItemID, "executor", bundle)
		if err != nil {
			return types.ProofBundle{}, fmt.Errorf("attach proof bundle: %w", err)
		}
	}

	return bundle, nil
}

func (e *Executor) ExecuteIntent(ctx context.Context, intent types.ActionIntent) (types.ExecutionResult, error) {
	if e.mutator == nil {
		return types.ExecutionResult{}, fmt.Errorf("github mutator is required")
	}
	if intent.IdempotencyKey == "" {
		return types.ExecutionResult{}, fmt.Errorf("idempotency key is required")
	}

	// Record "proposed" transition
	preflightSnapshot := fmt.Sprintf(`{"intent_id":"%s","action":"%s","pr_number":%d,"dry_run":%t}`,
		intent.ID, intent.Action, intent.PRNumber, intent.DryRun)
	if err := e.ledger.RecordTransition(intent.IdempotencyKey, "proposed", preflightSnapshot, nil); err != nil {
		return types.ExecutionResult{}, err
	}

	if e.ledger.IsExecuted(intent.IdempotencyKey) {
		return types.ExecutionResult{
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
		result := types.ExecutionResult{
			IntentID:   intent.ID,
			Action:     intent.Action,
			PRNumber:   intent.PRNumber,
			DryRun:     effectiveDryRun(e.cfg, intent),
			ExecutedAt: e.now().Format(time.RFC3339Nano),
			Error:      err.Error(),
		}
		return e.failIntent(intent, result, err)
	}

	// Run all 9 preflight checks
	if err := e.runPreflightChecks(ctx, intent); err != nil {
		// Record "failed" transition with error
		errorSnapshot := fmt.Sprintf(`{"error":"%s"}`, err.Error())
		if recordErr := e.ledger.RecordTransition(intent.IdempotencyKey, "failed", errorSnapshot, nil); recordErr != nil {
			return types.ExecutionResult{}, fmt.Errorf("preflight check failed: %w (and failed to record: %v)", err, recordErr)
		}
		return types.ExecutionResult{}, fmt.Errorf("preflight check failed: %w", err)
	}

	// Record "preflighted" transition
	preflightResult := PreflightResult{
		PRNumber:  intent.PRNumber,
		AllPassed: true,
		Timestamp: e.now().Format(time.RFC3339Nano),
	}
	preflightJSON := fmt.Sprintf(`{"pr_number":%d,"all_passed":true,"timestamp":"%s"}`,
		preflightResult.PRNumber, preflightResult.Timestamp)
	if err := e.ledger.RecordTransition(intent.IdempotencyKey, "preflighted", preflightJSON, nil); err != nil {
		return types.ExecutionResult{}, err
	}

	dryRun := effectiveDryRun(e.cfg, intent)
	result := types.ExecutionResult{
		IntentID:   intent.ID,
		Action:     intent.Action,
		PRNumber:   intent.PRNumber,
		DryRun:     dryRun,
		ExecutedAt: e.now().Format(time.RFC3339Nano),
	}

	switch intent.Action {
	case types.ActionKindMerge:
		merge, err := e.mutator.Merge(ctx, e.cfg.Repo, intent.PRNumber, mergeOptionsFromPayload(intent.Payload), dryRun)
		if err != nil {
			result.Error = err.Error()
			return e.failIntent(intent, result, err)
		}
		result.Executed = !dryRun
		if merge.AlreadyMerged {
			result.Result = "already_merged"
		} else {
			result.Result = "merged"
		}
		// Verify merge succeeded
		if !dryRun {
			if err := VerifyMerge(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
				result.Error = err.Error()
				return e.failIntent(intent, result, err)
			}
		}
	case types.ActionKindClose:
		reason := firstString(intent.Reasons, "")
		commentBody := stringFromPayload(intent.Payload, "comment")
		if commentBody == "" {
			commentBody = stringFromPayload(intent.Payload, "comment_body")
		}
		if strings.TrimSpace(reason) == "" && strings.TrimSpace(commentBody) == "" {
			err := fmt.Errorf("close action requires reason or comment text")
			result.Error = err.Error()
			return e.failIntent(intent, result, err)
		}
		if commentBody != "" {
			if err := e.mutator.AddComment(ctx, e.cfg.Repo, intent.PRNumber, commentBody, dryRun); err != nil {
				result.Error = err.Error()
				return e.failIntent(intent, result, err)
			}
		}
		if err := e.mutator.Close(ctx, e.cfg.Repo, intent.PRNumber, firstString([]string{reason, commentBody}, "closed by prATC"), dryRun); err != nil {
			result.Error = err.Error()
			return e.failIntent(intent, result, err)
		}
		result.Executed = !dryRun
		result.Result = "closed"
		// Verify close succeeded
		if !dryRun {
			if err := VerifyClose(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
				result.Error = err.Error()
				return e.failIntent(intent, result, err)
			}
			if commentBody != "" {
				if err := VerifyComment(ctx, e.mutator, e.cfg.Repo, intent.PRNumber, commentBody); err != nil {
					result.Error = err.Error()
					return e.failIntent(intent, result, err)
				}
			}
		}
	case types.ActionKindComment, types.ActionKindRequestChanges:
		body := firstString(intent.Reasons, "prATC action intent")
		if err := e.mutator.AddComment(ctx, e.cfg.Repo, intent.PRNumber, body, dryRun); err != nil {
			result.Error = err.Error()
			return e.failIntent(intent, result, err)
		}
		result.Executed = !dryRun
		result.Result = "commented"
		// Verify comment was added
		if !dryRun {
			if err := VerifyComment(ctx, e.mutator, e.cfg.Repo, intent.PRNumber, body); err != nil {
				result.Error = err.Error()
				return e.failIntent(intent, result, err)
			}
		}
	case types.ActionKindLabel:
		labels := []string{"pratc-action"}
		if err := e.mutator.AddLabels(ctx, e.cfg.Repo, intent.PRNumber, labels, dryRun); err != nil {
			result.Error = err.Error()
			return e.failIntent(intent, result, err)
		}
		result.Executed = !dryRun
		result.Result = "labeled"
		// Verify labels were added
		if !dryRun {
			if err := VerifyLabels(ctx, e.mutator, e.cfg.Repo, intent.PRNumber, labels); err != nil {
				result.Error = err.Error()
				return e.failIntent(intent, result, err)
			}
		}
	case types.ActionKindApplyFix:
		if _, err := e.mutator.ApplyFix(ctx, e.cfg.Repo, intent.PRNumber, "", dryRun); err != nil {
			result.Error = err.Error()
			return e.failIntent(intent, result, err)
		}
		result.Executed = !dryRun
		result.Result = "fix_applied"
		// Verify fix was applied
		if !dryRun {
			if err := VerifyFixApplied(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
				result.Error = err.Error()
				return e.failIntent(intent, result, err)
			}
		}
	default:
		return types.ExecutionResult{}, fmt.Errorf("unknown action kind %q", intent.Action)
	}

	// Record "executed" transition
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return result, fmt.Errorf("marshal result: %w", err)
	}
	if err := e.ledger.RecordTransition(intent.IdempotencyKey, "executed", string(resultJSON), nil); err != nil {
		return e.failIntent(intent, result, err)
	}

	// For backward compatibility
	if err := e.ledger.Record(intent.IdempotencyKey, result); err != nil {
		return e.failIntent(intent, result, err)
	}

	return result, nil
}

// runPreflightChecks runs all 9 preflight checks for an intent
func (e *Executor) runPreflightChecks(ctx context.Context, intent types.ActionIntent) error {
	// Get expected head SHA from payload if available
	expectedHeadSHA := ""
	if intent.Payload != nil {
		if sha, ok := intent.Payload["expected_head_sha"].(string); ok {
			expectedHeadSHA = sha
		}
	}

	allowedBranches := stringSliceFromPayload(intent.Payload, "allowed_branches")

	// 1. checkPROpen - verify PR still open
	if err := checkPROpen(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
		return err
	}

	// 2. checkHeadSHA - verify head SHA unchanged
	if err := checkHeadSHA(ctx, e.mutator, e.cfg.Repo, intent.PRNumber, expectedHeadSHA); err != nil {
		return err
	}

	// 3. checkBaseBranch - verify base branch
	if err := checkBaseBranch(ctx, e.mutator, e.cfg.Repo, intent.PRNumber, allowedBranches); err != nil {
		return err
	}

	// 4. checkCIGreen - verify CI checks green (only for merge actions)
	if intent.Action == types.ActionKindMerge {
		if err := checkCIGreen(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
			return err
		}
	}

	// 5. checkMergeable - verify mergeability clean (only for merge actions)
	if intent.Action == types.ActionKindMerge {
		if err := checkMergeable(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
			return err
		}
	}

	// 6. checkBranchProtection - verify reviews satisfied (only for merge actions)
	if intent.Action == types.ActionKindMerge {
		if err := checkBranchProtection(ctx, e.mutator, e.cfg.Repo, intent.PRNumber); err != nil {
			return err
		}
	}

	// 7. checkTokenPermission - verify token has permission
	if err := checkTokenPermission(ctx, e.mutator, e.cfg.Repo, intent.Action); err != nil {
		return err
	}

	// 8. checkRateLimit - verify rate-limit budget
	if err := checkRateLimit(ctx, e.mutator); err != nil {
		return err
	}

	// 9. checkIdempotency - verify idempotency key not executed (already checked earlier)
	if err := checkIdempotency(ctx, e.ledger, intent.IdempotencyKey); err != nil {
		return err
	}

	return nil
}

func (e *Executor) failIntent(intent types.ActionIntent, result types.ExecutionResult, cause error) (types.ExecutionResult, error) {
	if cause == nil {
		cause = fmt.Errorf("intent failed")
	}
	if result.IntentID == "" {
		result.IntentID = intent.ID
	}
	if result.Action == "" {
		result.Action = intent.Action
	}
	if result.PRNumber == 0 {
		result.PRNumber = intent.PRNumber
	}
	if result.ExecutedAt == "" {
		result.ExecutedAt = e.now().Format(time.RFC3339Nano)
	}
	if result.Error == "" {
		result.Error = cause.Error()
	}
	snapshotBytes, marshalErr := json.Marshal(map[string]any{
		"intent_id":        intent.ID,
		"action":           intent.Action,
		"pr_number":        intent.PRNumber,
		"dry_run":          result.DryRun,
		"executed":         result.Executed,
		"already_executed": result.AlreadyExecuted,
		"result":           result.Result,
		"error":            result.Error,
		"timestamp":        result.ExecutedAt,
	})
	if marshalErr != nil {
		return result, fmt.Errorf("%w; failed to marshal failure snapshot: %v", cause, marshalErr)
	}
	if err := e.ledger.RecordTransition(intent.IdempotencyKey, "failed", string(snapshotBytes), nil); err != nil {
		return result, fmt.Errorf("%w; failed to record failure transition: %v", cause, err)
	}
	return result, cause
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

func mergeOptionsFromPayload(payload map[string]any) MergeOptions {
	if payload == nil {
		return MergeOptions{}
	}
	return MergeOptions{
		CommitTitle:     stringFromPayload(payload, "commit_title"),
		CommitMessage:   stringFromPayload(payload, "commit_message"),
		MergeMethod:     stringFromPayload(payload, "merge_method"),
		ExpectedHeadSHA: stringFromPayload(payload, "expected_head_sha"),
	}
}

func stringFromPayload(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	s, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func stringSliceFromPayload(payload map[string]any, key string) []string {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return compactStrings(typed)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				values = append(values, s)
			}
		}
		return compactStrings(values)
	default:
		return nil
	}
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
