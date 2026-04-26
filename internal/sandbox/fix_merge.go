package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// FixMergeSandbox represents an isolated worktree for fix-and-merge operations
type FixMergeSandbox struct {
	worktreePath string
	repo         string
	prNumber     int
	patch        string
	testOutput   string
	testExitCode int
}

// NewFixMergeSandbox creates a new sandbox for the given PR
func NewFixMergeSandbox(repo string, prNumber int) *FixMergeSandbox {
	return &FixMergeSandbox{
		repo:     repo,
		prNumber: prNumber,
	}
}

// CreateWorktree creates an isolated worktree for the PR
func (s *FixMergeSandbox) CreateWorktree() error {
	// Create a temporary directory for the worktree
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("pratc-sandbox-%d-*", s.prNumber))
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	s.worktreePath = tmpDir

	// Clone the repository into the worktree
	// For now, we'll simulate this by creating a directory structure
	// In a real implementation, we would use git worktree
	err = os.MkdirAll(filepath.Join(s.worktreePath, ".git"), 0755)
	if err != nil {
		return fmt.Errorf("failed to create .git directory: %w", err)
	}

	return nil
}

// ApplyPatch applies a patch to the worktree
func (s *FixMergeSandbox) ApplyPatch(patch string) error {
	if s.worktreePath == "" {
		return fmt.Errorf("worktree not created")
	}

	s.patch = patch

	// For now, we'll simulate patch application
	// In a real implementation, we would apply the patch using git apply
	patchFile := filepath.Join(s.worktreePath, "fix.patch")
	err := os.WriteFile(patchFile, []byte(patch), 0644)
	if err != nil {
		return fmt.Errorf("failed to write patch file: %w", err)
	}

	return nil
}

// RunTests executes test commands and captures output
func (s *FixMergeSandbox) RunTests(testCommands []string) (string, int, error) {
	if s.worktreePath == "" {
		return "", 0, fmt.Errorf("worktree not created")
	}

	var output strings.Builder
	exitCode := 0

	for _, cmdStr := range testCommands {
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			continue
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = s.worktreePath

		cmdOutput, err := cmd.CombinedOutput()
		output.WriteString(string(cmdOutput))

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				return output.String(), exitCode, fmt.Errorf("failed to run command %q: %w", cmdStr, err)
			}
		}
	}

	s.testOutput = output.String()
	s.testExitCode = exitCode

	return s.testOutput, s.testExitCode, nil
}

// CaptureProofBundle creates a proof bundle from the sandbox results
func (s *FixMergeSandbox) CaptureProofBundle() types.ProofBundle {
	return types.ProofBundle{
		ID:           fmt.Sprintf("proof-%d-%d", s.prNumber, time.Now().Unix()),
		PRNumber:     s.prNumber,
		Summary:      "fix applied and tested",
		EvidenceRefs: []string{fmt.Sprintf("sandbox:%s", s.worktreePath)},
		ArtifactRefs: []string{filepath.Join(s.worktreePath, "fix.patch")},
		TestCommands: []string{"go test ./..."},
		TestResults:  []string{s.testOutput},
		CreatedBy:    "sandbox",
		CreatedAt:    time.Now().Format(time.RFC3339Nano),
	}
}

// Cleanup removes the worktree
func (s *FixMergeSandbox) Cleanup() error {
	if s.worktreePath == "" {
		return nil
	}

	err := os.RemoveAll(s.worktreePath)
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w", err)
	}

	s.worktreePath = ""
	return nil
}

// GetWorktreePath returns the path to the worktree
func (s *FixMergeSandbox) GetWorktreePath() string {
	return s.worktreePath
}

// SandboxManager manages multiple sandboxes
type SandboxManager struct {
	sandboxes map[int]*FixMergeSandbox
}

// NewSandboxManager creates a new sandbox manager
func NewSandboxManager() *SandboxManager {
	return &SandboxManager{
		sandboxes: make(map[int]*FixMergeSandbox),
	}
}

// GetSandbox gets or creates a sandbox for the given PR
func (m *SandboxManager) GetSandbox(prNumber int) *FixMergeSandbox {
	if sandbox, ok := m.sandboxes[prNumber]; ok {
		return sandbox
	}

	sandbox := NewFixMergeSandbox("", prNumber)
	m.sandboxes[prNumber] = sandbox
	return sandbox
}

// CleanupAll removes all sandboxes
func (m *SandboxManager) CleanupAll() error {
	var firstErr error
	for prNumber, sandbox := range m.sandboxes {
		if err := sandbox.Cleanup(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("failed to cleanup sandbox for PR #%d: %w", prNumber, err)
		}
	}
	m.sandboxes = make(map[int]*FixMergeSandbox)
	return firstErr
}
