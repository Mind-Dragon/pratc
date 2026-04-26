package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateWorktree_CreatesDirectory(t *testing.T) {
	sandbox := NewFixMergeSandbox("owner/repo", 101)
	
	err := sandbox.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	
	// Verify the worktree directory was created
	if sandbox.worktreePath == "" {
		t.Fatal("worktreePath should not be empty")
	}
	
	// Verify the directory exists
	_, err = os.Stat(sandbox.worktreePath)
	if err != nil {
		t.Fatalf("worktree directory does not exist: %v", err)
	}
	
	// Cleanup
	sandbox.Cleanup()
}

func TestApplyPatch_WritesPatchFile(t *testing.T) {
	sandbox := NewFixMergeSandbox("owner/repo", 101)
	
	err := sandbox.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer sandbox.Cleanup()
	
	patch := "diff --git a/file.txt b/file.txt\n+test content"
	err = sandbox.ApplyPatch(patch)
	if err != nil {
		t.Fatalf("ApplyPatch failed: %v", err)
	}
	
	// Verify the patch file was created
	patchFile := filepath.Join(sandbox.worktreePath, "fix.patch")
	_, err = os.Stat(patchFile)
	if err != nil {
		t.Fatalf("patch file does not exist: %v", err)
	}
}

func TestRunTests_ExecutesCommands(t *testing.T) {
	sandbox := NewFixMergeSandbox("owner/repo", 101)
	
	err := sandbox.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer sandbox.Cleanup()
	
	// Create a simple test file
	testFile := filepath.Join(sandbox.worktreePath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	// Run a simple command
	output, exitCode, err := sandbox.RunTests([]string{"echo 'test output'"})
	if err != nil {
		t.Fatalf("RunTests failed: %v", err)
	}
	
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	
	if output == "" {
		t.Fatal("expected output, got empty string")
	}
}

func TestCaptureProofBundle_CreatesValidBundle(t *testing.T) {
	sandbox := NewFixMergeSandbox("owner/repo", 101)
	
	err := sandbox.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer sandbox.Cleanup()
	
	bundle := sandbox.CaptureProofBundle()
	
	if bundle.PRNumber != 101 {
		t.Fatalf("expected PR number 101, got %d", bundle.PRNumber)
	}
	
	if bundle.Summary == "" {
		t.Fatal("bundle summary should not be empty")
	}
	
	if len(bundle.EvidenceRefs) == 0 {
		t.Fatal("bundle should have evidence refs")
	}
}

func TestCleanup_RemovesWorktree(t *testing.T) {
	sandbox := NewFixMergeSandbox("owner/repo", 101)
	
	err := sandbox.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	
	worktreePath := sandbox.worktreePath
	
	// Verify the directory exists
	_, err = os.Stat(worktreePath)
	if err != nil {
		t.Fatalf("worktree directory does not exist: %v", err)
	}
	
	// Cleanup
	err = sandbox.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	
	// Verify the directory was removed
	_, err = os.Stat(worktreePath)
	if !os.IsNotExist(err) {
		t.Fatal("worktree directory should have been removed")
	}
}

func TestSandboxManager_GetSandbox_CreatesNew(t *testing.T) {
	manager := NewSandboxManager()
	
	sandbox := manager.GetSandbox(101)
	if sandbox == nil {
		t.Fatal("GetSandbox should return a sandbox")
	}
	
	if sandbox.prNumber != 101 {
		t.Fatalf("expected PR number 101, got %d", sandbox.prNumber)
	}
	
	// Get the same sandbox again
	sandbox2 := manager.GetSandbox(101)
	if sandbox != sandbox2 {
		t.Fatal("GetSandbox should return the same sandbox for the same PR")
	}
}

func TestSandboxManager_CleanupAll(t *testing.T) {
	manager := NewSandboxManager()
	
	sandbox1 := manager.GetSandbox(101)
	sandbox2 := manager.GetSandbox(102)
	
	err := sandbox1.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer sandbox1.Cleanup()
	
	err = sandbox2.CreateWorktree()
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}
	defer sandbox2.Cleanup()
	
	// Cleanup all
	err = manager.CleanupAll()
	if err != nil {
		t.Fatalf("CleanupAll failed: %v", err)
	}
	
	// Verify sandboxes were cleared
	if len(manager.sandboxes) != 0 {
		t.Fatalf("expected 0 sandboxes, got %d", len(manager.sandboxes))
	}
}
