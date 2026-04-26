package executor

import (
	"testing"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestAuditNoGitHubWrites_NoWrites(t *testing.T) {
	fake := NewFakeGitHub()
	// No writes performed
	err := AuditNoGitHubWrites(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditNoGitHubWrites_WithWrites(t *testing.T) {
	fake := NewFakeGitHub()
	// Simulate a write
	fake.writes = 1
	err := AuditNoGitHubWrites(fake)
	if err == nil {
		t.Fatal("expected error for writes")
	}
	// Reset and verify passes
	fake.writes = 0
	err = AuditNoGitHubWrites(fake)
	if err != nil {
		t.Fatalf("unexpected error after reset: %v", err)
	}
}

func TestAuditGuardedMode_Passes(t *testing.T) {
	fake := NewFakeGitHub()
	ledger := NewMemoryLedger()
	cfg := Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileGuarded,
	}
	exec := New(cfg, fake, ledger)
	err := AuditGuardedMode(exec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditGuardedMode_FailsWhenNotGuarded(t *testing.T) {
	fake := NewFakeGitHub()
	ledger := NewMemoryLedger()
	cfg := Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAdvisory,
	}
	exec := New(cfg, fake, ledger)
	err := AuditGuardedMode(exec)
	if err == nil {
		t.Fatal("expected error for non-guarded profile")
	}
}

func TestAuditAdvisoryMode_Passes(t *testing.T) {
	fake := NewFakeGitHub()
	ledger := NewMemoryLedger()
	cfg := Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAdvisory,
	}
	exec := New(cfg, fake, ledger)
	err := AuditAdvisoryMode(exec, fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuditAdvisoryMode_FailsWhenWrites(t *testing.T) {
	fake := NewFakeGitHub()
	ledger := NewMemoryLedger()
	cfg := Config{
		Repo:          "owner/repo",
		DryRun:        false,
		PolicyProfile: types.PolicyProfileAdvisory,
	}
	exec := New(cfg, fake, ledger)
	// Simulate a write
	fake.writes = 1
	err := AuditAdvisoryMode(exec, fake)
	if err == nil {
		t.Fatal("expected error for writes in advisory mode")
	}
}