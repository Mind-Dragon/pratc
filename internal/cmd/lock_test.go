package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestRepoLock_AcquireAndRelease(t *testing.T) {
	repo := "test/test-repo-lock-acquire"

	// Acquire the lock
	lock, err := AcquireRepoLock(repo)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}
	defer lock.Release()

	// Verify the lock file exists
	if _, err := os.Stat(lock.Path); os.IsNotExist(err) {
		t.Fatalf("lock file does not exist: %s", lock.Path)
	}

	// Verify the lock file contains correct data
	data, err := os.ReadFile(lock.Path)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	content, err := parseLockContent(string(data))
	if err != nil {
		t.Fatalf("failed to parse lock content: %v", err)
	}

	if content.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), content.PID)
	}
	if content.Repo != types.NormalizeRepoName(repo) {
		t.Errorf("expected repo %s, got %s", types.NormalizeRepoName(repo), content.Repo)
	}
	if content.Command != "pratc" {
		t.Errorf("expected command 'pratc', got %s", content.Command)
	}

	// Release the lock
	err = lock.Release()
	if err != nil {
		t.Fatalf("failed to release lock: %v", err)
	}

	// Verify the lock file is removed
	if _, err := os.Stat(lock.Path); !os.IsNotExist(err) {
		t.Fatalf("lock file should not exist after release: %s", lock.Path)
	}
}

func TestRepoLock_BlockedByActive(t *testing.T) {
	repo := "test/test-repo-lock-blocked"

	// Acquire the first lock
	lock1, err := AcquireRepoLock(repo)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	defer lock1.Release()

	// Try to acquire a second lock - should fail
	_, err = AcquireRepoLock(repo)
	if err == nil {
		t.Fatalf("expected error when acquiring second lock, got nil")
	}

	expectedMsg := "another prATC instance is running for repo"
	alreadyHoldingMsg := "already holding lock for repo"
	if !containsString(err.Error(), expectedMsg) && !containsString(err.Error(), alreadyHoldingMsg) {
		t.Errorf("expected error message to contain %q or %q, got %q", expectedMsg, alreadyHoldingMsg, err.Error())
	}
}

func TestRepoLock_StaleLockCleanup(t *testing.T) {
	repo := "test/test-repo-lock-stale"

	// Create a stale lock file with a fake dead PID
	lockPath, err := lockPathForRepo(repo)
	if err != nil {
		t.Fatalf("failed to get lock path: %v", err)
	}

	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatalf("failed to create lock directory: %v", err)
	}

	// Write a stale lock file with a PID that definitely doesn't exist
	stalePID := 999999999
	staleContent := fmt.Sprintf("PID: %d\nStartTime: %s\nCommand: pratc\nRepo: %s\n",
		stalePID, time.Now().UTC().Format(time.RFC3339), types.NormalizeRepoName(repo))
	if err := os.WriteFile(lockPath, []byte(staleContent), 0o644); err != nil {
		t.Fatalf("failed to write stale lock file: %v", err)
	}

	// Now acquire the lock - should succeed and clean up the stale one
	lock, err := AcquireRepoLock(repo)
	if err != nil {
		t.Fatalf("failed to acquire lock after stale cleanup: %v", err)
	}
	defer lock.Release()

	// Verify the lock file exists and has our PID
	if _, err := os.Stat(lock.Path); os.IsNotExist(err) {
		t.Fatalf("lock file does not exist: %s", lock.Path)
	}

	data, err := os.ReadFile(lock.Path)
	if err != nil {
		t.Fatalf("failed to read lock file: %v", err)
	}

	content, err := parseLockContent(string(data))
	if err != nil {
		t.Fatalf("failed to parse lock content: %v", err)
	}

	if content.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), content.PID)
	}
}

func TestRepoLock_DifferentReposIndependent(t *testing.T) {
	repo1 := "test/repo-one"
	repo2 := "test/repo-two"

	// Acquire locks for both repos
	lock1, err := AcquireRepoLock(repo1)
	if err != nil {
		t.Fatalf("failed to acquire lock for repo1: %v", err)
	}
	defer lock1.Release()

	lock2, err := AcquireRepoLock(repo2)
	if err != nil {
		t.Fatalf("failed to acquire lock for repo2: %v", err)
	}
	defer lock2.Release()

	// Both lock files should exist
	if _, err := os.Stat(lock1.Path); os.IsNotExist(err) {
		t.Fatalf("lock1 file does not exist: %s", lock1.Path)
	}
	if _, err := os.Stat(lock2.Path); os.IsNotExist(err) {
		t.Fatalf("lock2 file does not exist: %s", lock2.Path)
	}

	// The lock paths should be different
	if lock1.Path == lock2.Path {
		t.Errorf("expected different lock paths for different repos, got same: %s", lock1.Path)
	}
}

func TestRepoLock_NormalizesRepoName(t *testing.T) {
	repo1 := "OpenClaw/OpenClaw"
	repo2 := "openclaw/openclaw"

	// Both should resolve to the same lock file path
	lock1, err := AcquireRepoLock(repo1)
	if err != nil {
		t.Fatalf("failed to acquire lock for repo1: %v", err)
	}
	defer lock1.Release()

	// Try to acquire a lock for the normalized version - should fail because it uses the same lock file
	_, err = AcquireRepoLock(repo2)
	if err == nil {
		t.Fatalf("expected error when acquiring lock for normalized repo name, got nil")
	}

	expectedMsg := "another prATC instance is running for repo"
	alreadyHoldingMsg := "already holding lock for repo"
	if !containsString(err.Error(), expectedMsg) && !containsString(err.Error(), alreadyHoldingMsg) {
		t.Errorf("expected error message to contain %q or %q, got %q", expectedMsg, alreadyHoldingMsg, err.Error())
	}
}

func TestLockPathForRepo(t *testing.T) {
	tests := []struct {
		repo     string
		expected string
	}{
		{"test/repo", "test/repo"},
		{"Test/Repo", "test/repo"},
		{"TEST/REPO", "test/repo"},
		{"  test/repo  ", "test/repo"},
	}

	for _, tc := range tests {
		normalized := types.NormalizeRepoName(tc.repo)
		if normalized != tc.expected {
			t.Errorf("NormalizeRepoName(%q) = %q, want %q", tc.repo, normalized, tc.expected)
		}
	}
}

func TestParseLockContent(t *testing.T) {
	data := `PID: 12345
StartTime: 2026-04-18T12:00:00Z
Command: pratc
Repo: test/repo`

	content, err := parseLockContent(data)
	if err != nil {
		t.Fatalf("failed to parse lock content: %v", err)
	}

	if content.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", content.PID)
	}
	if content.StartTime != "2026-04-18T12:00:00Z" {
		t.Errorf("expected StartTime '2026-04-18T12:00:00Z', got %s", content.StartTime)
	}
	if content.Command != "pratc" {
		t.Errorf("expected Command 'pratc', got %s", content.Command)
	}
	if content.Repo != "test/repo" {
		t.Errorf("expected Repo 'test/repo', got %s", content.Repo)
	}
}

func TestLockStatus(t *testing.T) {
	repo := "test/test-repo-lock-status"

	// Initially, no lock should exist
	locked, holder, err := LockStatus(repo)
	if err != nil {
		t.Fatalf("failed to check lock status: %v", err)
	}
	if locked {
		t.Errorf("expected no lock to exist initially")
	}
	if holder != nil {
		t.Errorf("expected no holder, got %+v", holder)
	}

	// Acquire the lock
	lock, err := AcquireRepoLock(repo)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}
	defer lock.Release()

	// Now lock should exist
	locked, holder, err = LockStatus(repo)
	if err != nil {
		t.Fatalf("failed to check lock status: %v", err)
	}
	if !locked {
		t.Errorf("expected lock to exist")
	}
	if holder == nil {
		t.Errorf("expected holder to be non-nil")
	} else {
		if holder.PID != os.Getpid() {
			t.Errorf("expected PID %d, got %d", os.Getpid(), holder.PID)
		}
	}
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
