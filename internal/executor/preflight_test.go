package executor

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

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
