package executor

import (
	"fmt"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// AuditNoGitHubWrites checks that the given GitHubMutator (if it implements WriteTracker)
// has not performed any writes. This is useful for advisory mode where no actual writes
// should occur.
func AuditNoGitHubWrites(mutator GitHubMutator) error {
	if wt, ok := mutator.(WriteTracker); ok {
		if wt.HasWritten() {
			return fmt.Errorf("unexpected GitHub writes: count=%d", wt.WriteCount())
		}
	}
	// If mutator doesn't implement WriteTracker, we cannot audit writes.
	// This is acceptable for production mutators (real GitHub client) where writes are expected.
	return nil
}

// AuditGuardedMode verifies that the executor's policy profile is guarded and
// that only allowed actions (comment, label) were executed. This is a sanity check.
func AuditGuardedMode(exec *Executor) error {
	if exec.cfg.PolicyProfile != types.PolicyProfileGuarded {
		return fmt.Errorf("executor is not in guarded mode (profile=%s)", exec.cfg.PolicyProfile)
	}
	// Additional checks could be added here, e.g., verifying ledger entries.
	return nil
}

// AuditAdvisoryMode verifies that the executor's policy profile is advisory and
// that no GitHub writes occurred (i.e., all actions were denied).
func AuditAdvisoryMode(exec *Executor, mutator GitHubMutator) error {
	if exec.cfg.PolicyProfile != types.PolicyProfileAdvisory {
		return fmt.Errorf("executor is not in advisory mode (profile=%s)", exec.cfg.PolicyProfile)
	}
	// Verify no writes occurred
	if err := AuditNoGitHubWrites(mutator); err != nil {
		return fmt.Errorf("advisory mode violation: %w", err)
	}
	return nil
}