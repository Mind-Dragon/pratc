package executor

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// Test checkPROpen gate
func TestCheckPROpen_PassesWhenOpen(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", HeadSHA: "abc"})

	err := checkPROpen(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("checkPROpen should pass for open PR: %v", err)
	}
}

func TestCheckPROpen_FailsWhenClosed(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "closed", HeadSHA: "abc"})

	err := checkPROpen(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("checkPROpen should fail for closed PR")
	}
}

func TestCheckPROpen_FailsWhenPRNotFound(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()

	err := checkPROpen(ctx, fake, "owner/repo", 999)
	if err == nil {
		t.Fatal("checkPROpen should fail when PR not found")
	}
}

// Test checkHeadSHA gate
func TestCheckHeadSHA_PassesWhenMatches(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, HeadSHA: "abc123"})

	err := checkHeadSHA(ctx, fake, "owner/repo", 101, "abc123")
	if err != nil {
		t.Fatalf("checkHeadSHA should pass when SHA matches: %v", err)
	}
}

func TestCheckHeadSHA_PassesWhenExpectedEmpty(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, HeadSHA: "abc123"})

	err := checkHeadSHA(ctx, fake, "owner/repo", 101, "")
	if err != nil {
		t.Fatalf("checkHeadSHA should pass when expected SHA is empty: %v", err)
	}
}

func TestCheckHeadSHA_FailsWhenChanged(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, HeadSHA: "def456"})

	err := checkHeadSHA(ctx, fake, "owner/repo", 101, "abc123")
	if err == nil {
		t.Fatal("checkHeadSHA should fail when SHA changed")
	}
}

// Test checkBaseBranch gate
func TestCheckBaseBranch_PassesWhenInAllowedList(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	allowedBranches := []string{"main", "develop"}
	err := checkBaseBranch(ctx, fake, "owner/repo", 101, allowedBranches)
	if err != nil {
		t.Fatalf("checkBaseBranch should pass when branch in allowed list: %v", err)
	}
}

func TestCheckBaseBranch_PassesWhenNoRestrictions(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	err := checkBaseBranch(ctx, fake, "owner/repo", 101, []string{})
	if err != nil {
		t.Fatalf("checkBaseBranch should pass when no restrictions: %v", err)
	}
}

func TestCheckBaseBranch_FailsWhenNotInAllowedList(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	// The FakeGitHub returns "main" as the base branch by default
	allowedBranches := []string{"develop", "release"}
	err := checkBaseBranch(ctx, fake, "owner/repo", 101, allowedBranches)
	if err == nil {
		t.Fatal("checkBaseBranch should fail when branch not in allowed list")
	}
}

// Test checkCIGreen gate
func TestCheckCIGreen_PassesWhenGreen(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	err := checkCIGreen(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("checkCIGreen should pass when CI is green: %v", err)
	}
}

func TestCheckCIGreen_FailsWhenNotGreen(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})
	fake.SetTestConfig("failed", false, true, 5000) // Set CI status to failed

	err := checkCIGreen(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("checkCIGreen should fail when CI is not green")
	}
}

// Test checkMergeable gate
func TestCheckMergeable_PassesWhenMergeable(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true})

	err := checkMergeable(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("checkMergeable should pass when mergeable: %v", err)
	}
}

func TestCheckMergeable_FailsWhenNotMergeable(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: false})

	err := checkMergeable(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("checkMergeable should fail when not mergeable")
	}
}

// Test checkBranchProtection gate
func TestCheckBranchProtection_PassesWhenReviewsSatisfied(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})

	err := checkBranchProtection(ctx, fake, "owner/repo", 101)
	if err != nil {
		t.Fatalf("checkBranchProtection should pass when reviews satisfied: %v", err)
	}
}

func TestCheckBranchProtection_FailsWhenReviewsNotSatisfied(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open"})
	fake.SetTestConfig("success", false, false, 5000) // Set requiredReviews to false

	err := checkBranchProtection(ctx, fake, "owner/repo", 101)
	if err == nil {
		t.Fatal("checkBranchProtection should fail when reviews not satisfied")
	}
}

// Test checkTokenPermission gate
func TestCheckTokenPermission_Passes(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()

	err := checkTokenPermission(ctx, fake, "owner/repo", types.ActionKindMerge)
	if err != nil {
		t.Fatalf("checkTokenPermission should pass: %v", err)
	}
}

// Test checkRateLimit gate
func TestCheckRateLimit_PassesWhenAvailable(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()

	err := checkRateLimit(ctx, fake)
	if err != nil {
		t.Fatalf("checkRateLimit should pass when rate limit available: %v", err)
	}
}

func TestCheckRateLimit_FailsWhenExhausted(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.SetTestConfig("success", false, true, 0) // Set rateLimitRemaining to 0

	err := checkRateLimit(ctx, fake)
	if err == nil {
		t.Fatal("checkRateLimit should fail when rate limit is exhausted")
	}
}

// Test checkIdempotency gate
func TestCheckIdempotency_PassesWhenNotExecuted(t *testing.T) {
	ctx := context.Background()
	ledger := NewMemoryLedger()

	err := checkIdempotency(ctx, ledger, "test-key-123")
	if err != nil {
		t.Fatalf("checkIdempotency should pass when key not executed: %v", err)
	}
}

func TestCheckIdempotency_FailsWhenAlreadyExecuted(t *testing.T) {
	ctx := context.Background()
	ledger := NewMemoryLedger()

	// Record a result first
	result := types.ExecutionResult{
		IntentID:   "intent-123",
		Action:     types.ActionKindMerge,
		PRNumber:   101,
		DryRun:     false,
		Executed:   true,
		Result:     "merged",
		ExecutedAt: time.Now().Format(time.RFC3339Nano),
	}
	err := ledger.Record("test-key-456", result)
	if err != nil {
		t.Fatalf("Failed to record in ledger: %v", err)
	}

	err = checkIdempotency(ctx, ledger, "test-key-456")
	if err == nil {
		t.Fatal("checkIdempotency should fail when key already executed")
	}
}

func TestCheckIdempotency_FailsWhenKeyEmpty(t *testing.T) {
	ctx := context.Background()
	ledger := NewMemoryLedger()

	err := checkIdempotency(ctx, ledger, "")
	if err == nil {
		t.Fatal("checkIdempotency should fail when key is empty")
	}
}

// Test all gates together
func TestAllPreflightGates_PassWhenValid(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "abc123"})
	ledger := NewMemoryLedger()

	// Test each gate
	tests := []struct {
		name string
		fn   func() error
	}{
		{"checkPROpen", func() error { return checkPROpen(ctx, fake, "owner/repo", 101) }},
		{"checkHeadSHA", func() error { return checkHeadSHA(ctx, fake, "owner/repo", 101, "abc123") }},
		{"checkBaseBranch", func() error { return checkBaseBranch(ctx, fake, "owner/repo", 101, []string{"main"}) }},
		{"checkCIGreen", func() error { return checkCIGreen(ctx, fake, "owner/repo", 101) }},
		{"checkMergeable", func() error { return checkMergeable(ctx, fake, "owner/repo", 101) }},
		{"checkBranchProtection", func() error { return checkBranchProtection(ctx, fake, "owner/repo", 101) }},
		{"checkTokenPermission", func() error { return checkTokenPermission(ctx, fake, "owner/repo", types.ActionKindMerge) }},
		{"checkRateLimit", func() error { return checkRateLimit(ctx, fake) }},
		{"checkIdempotency", func() error { return checkIdempotency(ctx, ledger, "test-key") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); err != nil {
				t.Fatalf("%s should pass: %v", tt.name, err)
			}
		})
	}
}

func TestRunPreflightForMergePassesAllChecks(t *testing.T) {
	intent := mergeIntent(false)
	live := LivePRState{PRNumber: 101, State: "open", HeadSHA: "abc", CIStatus: "success", Mergeable: "clean", RequiredReviewsSatisfied: true, TokenScopeSufficient: true, RateLimitRemaining: 100}
	result := RunPreflight(intent, PreflightOptions{ExpectedHeadSHA: "abc", Live: live, Now: time.Date(2026, 4, 24, 16, 0, 0, 0, time.UTC)})
	if !result.AllPassed || result.FailedCount != 0 {
		t.Fatalf("preflight result = %+v", result)
	}
	if len(result.Checks) < 7 {
		t.Fatalf("expected all merge checks, got %d", len(result.Checks))
	}
}

func TestRunPreflightFailsChangedSHAAndCI(t *testing.T) {
	intent := mergeIntent(false)
	live := LivePRState{PRNumber: 101, State: "open", HeadSHA: "def", CIStatus: "failure", Mergeable: "clean", RequiredReviewsSatisfied: true, TokenScopeSufficient: true, RateLimitRemaining: 100}
	result := RunPreflight(intent, PreflightOptions{ExpectedHeadSHA: "abc", Live: live})
	if result.AllPassed || result.FailedCount != 2 {
		t.Fatalf("preflight result = %+v", result)
	}
	if checkStatus(result.Checks, CheckHeadSHAUnchanged) != PreflightStatusFailed {
		t.Fatalf("head SHA check not failed: %+v", result.Checks)
	}
	if checkStatus(result.Checks, CheckCIGreen) != PreflightStatusFailed {
		t.Fatalf("CI check not failed: %+v", result.Checks)
	}
}

func TestValidateProofBundle(t *testing.T) {
	item := sampleProofWorkItem()
	bundle := types.ProofBundle{ID: "proof-1", WorkItemID: item.ID, PRNumber: item.PRNumber, Summary: "fixed", EvidenceRefs: []string{"artifact:test.log"}, ArtifactRefs: []string{"/tmp/test.log"}, TestCommands: []string{"go test ./..."}, TestResults: []string{"pass"}, CreatedBy: "worker-a", CreatedAt: "2026-04-24T16:00:00Z"}
	if err := ValidateProofBundle(item, bundle); err != nil {
		t.Fatalf("valid proof bundle: %v", err)
	}
	bundle.PRNumber = 999
	if err := ValidateProofBundle(item, bundle); err == nil {
		t.Fatal("expected PR mismatch proof bundle error")
	}
}

func TestValidateProofBundleEdgeCases(t *testing.T) {
	item := sampleProofWorkItem()
	baseBundle := types.ProofBundle{
		ID:           "proof-1",
		WorkItemID:   item.ID,
		PRNumber:     item.PRNumber,
		Summary:      "fixed",
		EvidenceRefs: []string{"artifact:test.log"},
		ArtifactRefs: []string{"/tmp/test.log"},
		TestCommands: []string{"go test ./..."},
		TestResults:  []string{"pass"},
		CreatedBy:    "worker-a",
		CreatedAt:    "2026-04-24T16:00:00Z",
	}

	tests := []struct {
		name      string
		modify    func(*types.ProofBundle)
		expectErr bool
	}{
		{
			name: "missing ID",
			modify: func(b *types.ProofBundle) {
				b.ID = ""
			},
			expectErr: true,
		},
		{
			name: "missing summary",
			modify: func(b *types.ProofBundle) {
				b.Summary = ""
			},
			expectErr: true,
		},
		{
			name: "missing evidence refs",
			modify: func(b *types.ProofBundle) {
				b.EvidenceRefs = []string{}
			},
			expectErr: true,
		},
		{
			name: "missing artifact refs",
			modify: func(b *types.ProofBundle) {
				b.ArtifactRefs = []string{}
			},
			expectErr: true,
		},
		{
			name: "missing test commands",
			modify: func(b *types.ProofBundle) {
				b.TestCommands = []string{}
			},
			expectErr: true,
		},
		{
			name: "missing test results",
			modify: func(b *types.ProofBundle) {
				b.TestResults = []string{}
			},
			expectErr: true,
		},
		{
			name: "missing CreatedBy",
			modify: func(b *types.ProofBundle) {
				b.CreatedBy = ""
			},
			expectErr: true,
		},
		{
			name: "missing CreatedAt",
			modify: func(b *types.ProofBundle) {
				b.CreatedAt = ""
			},
			expectErr: true,
		},
		{
			name: "WorkItemID mismatch",
			modify: func(b *types.ProofBundle) {
				b.WorkItemID = "different-id"
			},
			expectErr: true,
		},
		{
			name: "PRNumber mismatch",
			modify: func(b *types.ProofBundle) {
				b.PRNumber = 999
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bundle := baseBundle
			tt.modify(&bundle)
			err := ValidateProofBundle(item, bundle)
			if tt.expectErr && err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.name, err)
			}
		})
	}
}

func sampleProofWorkItem() types.ActionWorkItem {
	return types.ActionWorkItem{ID: "wi-101", PRNumber: 101, Lane: types.ActionLaneFixAndMerge, State: types.ActionWorkItemStateTested}
}

func checkStatus(checks []types.ActionPreflight, name string) string {
	for _, check := range checks {
		if check.Check == name {
			return check.Status
		}
	}
	return ""
}

// Test ExecuteIntent with preflight checks
func TestExecuteIntent_PreflightChecksCalled(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, ledger)

	intent := mergeIntent(false)
	result, err := exec.ExecuteIntent(ctx, intent)
	if err != nil {
		t.Fatalf("execute intent should pass preflight: %v", err)
	}
	if !result.Executed {
		t.Fatalf("intent should be executed: %+v", result)
	}
}

func TestExecuteIntent_FailsWhenPRNotOpen(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "closed", Mergeable: true, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, ledger)

	intent := mergeIntent(false)
	_, err := exec.ExecuteIntent(ctx, intent)
	if err == nil {
		t.Fatal("execute intent should fail when PR is not open")
	}
}

func TestExecuteIntent_FailsWhenHeadSHAChanged(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: true, HeadSHA: "def456"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, ledger)

	intent := mergeIntent(false)
	intent.Payload = map[string]any{"expected_head_sha": "abc123"}
	_, err := exec.ExecuteIntent(ctx, intent)
	if err == nil {
		t.Fatal("execute intent should fail when head SHA changed")
	}
}

func TestExecuteIntent_FailsWhenNotMergeable(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: false, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, ledger)

	intent := mergeIntent(false)
	_, err := exec.ExecuteIntent(ctx, intent)
	if err == nil {
		t.Fatal("execute intent should fail when PR is not mergeable")
	}
}

func TestExecuteIntent_PreflightChecksSkippedForNonMergeActions(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeGitHub()
	fake.UpsertPR(FakePR{Number: 101, State: "open", Mergeable: false, HeadSHA: "abc"})
	ledger := NewMemoryLedger()
	exec := New(Config{Repo: "owner/repo", DryRun: false, PolicyProfile: types.PolicyProfileAutonomous}, fake, ledger)

	// Comment action should not require mergeability
	intent := types.ActionIntent{
		ID:             "intent-comment-101",
		Action:         types.ActionKindComment,
		PRNumber:       101,
		DryRun:         false,
		PolicyProfile:  types.PolicyProfileAutonomous,
		IdempotencyKey: "owner/repo#101:comment",
		Reasons:        []string{"test"},
	}
	result, err := exec.ExecuteIntent(ctx, intent)
	if err != nil {
		t.Fatalf("execute intent should pass for comment action: %v", err)
	}
	if !result.Executed {
		t.Fatalf("intent should be executed: %+v", result)
	}
}
