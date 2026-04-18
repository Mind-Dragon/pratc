package cmd

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

// RepoLock provides process-level mutual exclusion for repo operations.
type RepoLock struct {
	Path string
	File *os.File
	Repo string
}

// lockContent holds the data written to a lock file
type lockContent struct {
	PID       int    `json:"pid"`
	StartTime string `json:"start_time"`
	Command   string `json:"command"`
	Repo      string `json:"repo"`
}

// AcquireRepoLock attempts to acquire an exclusive lock for the given repository.
// Returns a RepoLock if successful, or an error if another instance is already running.
func AcquireRepoLock(repo string) (*RepoLock, error) {
	repo = types.NormalizeRepoName(repo)

	lockPath, err := lockPathForRepo(repo)
	if err != nil {
		return nil, fmt.Errorf("compute lock path: %w", err)
	}

	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	// Try to create the lock file exclusively (atomic create)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		// Lock file already exists — check if it's stale
		if os.IsExist(err) {
			if staleErr := checkExistingLock(lockPath, repo); staleErr != nil {
				return nil, staleErr
			}
			// Stale lock was cleaned up, retry acquisition
			file, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				// Another process might have acquired in the meantime
				if os.IsExist(err) {
					return nil, checkExistingLock(lockPath, repo)
				}
				return nil, fmt.Errorf("create lock file after stale cleanup: %w", err)
			}
		} else {
			return nil, fmt.Errorf("create lock file: %w", err)
		}
	}

	// Write lock content
	content := lockContent{
		PID:       os.Getpid(),
		StartTime: time.Now().UTC().Format(time.RFC3339),
		Command:   "pratc",
		Repo:      repo,
	}
	if err := writeLockContent(file, content); err != nil {
		file.Close()
		os.Remove(lockPath)
		return nil, fmt.Errorf("write lock content: %w", err)
	}

	// File is intentionally kept open to maintain the lock
	return &RepoLock{
		Path: lockPath,
		File: file,
		Repo: repo,
	}, nil
}

// Release releases the lock by closing and removing the lock file.
func (l *RepoLock) Release() error {
	if l == nil || l.File == nil {
		return nil
	}
	l.File.Close()
	return os.Remove(l.Path)
}

// checkExistingLock checks if an existing lock file represents an active process.
// Returns an error if the lock is still active, nil (and removes stale lock) otherwise.
func checkExistingLock(lockPath, repo string) error {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		// Can't read it, assume it's stale and remove
		os.Remove(lockPath)
		return nil
	}

	// Parse the lock content
	content, err := parseLockContent(string(data))
	if err != nil {
		// Invalid content, remove stale lock
		os.Remove(lockPath)
		return nil
	}

	// If the PID in the lock is our own process, we already hold the lock
	// (shouldn't happen with O_EXCL but can occur if we crashed and restarted)
	if content.PID == os.Getpid() {
		// We already hold this lock - this is an error since O_EXCL should have succeeded
		return fmt.Errorf("already holding lock for repo %s (PID %d, started %s)",
			repo, content.PID, content.StartTime)
	}

	// Check if the PID is still alive
	if !isProcessAlive(content.PID) {
		// Process is dead — clean up
		os.Remove(lockPath)
		return nil
	}

	// Process is alive — check if it's still running pratc against this repo
	if isPratcProcessRunning(content.PID, repo) {
		return fmt.Errorf("another prATC instance is running for repo %s (PID %d, started %s)",
			repo, content.PID, content.StartTime)
	}

	// Process is alive but not running pratc against this repo — stale lock, clean up
	os.Remove(lockPath)
	return nil
}

// isProcessAlive checks if a process with the given PID is still running.
// Uses syscall.Kill to check process existence - signal 0 doesn't send anything
// but returns error if process doesn't exist.
func isProcessAlive(pid int) bool {
	// Signal 0 checks if process exists without sending a signal
	err := syscall.Kill(pid, 0)
	return err == nil
}

// isPratcProcessRunning checks if any pratc process with the given PID is running
// against the specified repo. Uses simple PID matching.
func isPratcProcessRunning(pid int, repo string) bool {
	// Check if there's a process with this PID running
	if !isProcessAlive(pid) {
		return false
	}

	// The process is alive - now we need to verify it's a pratc process
	// running against the same repo. We use ps to check.
	return checkPratcProcessMatches(pid, repo)
}

// checkPratcProcessMatches checks if a specific PID is running pratc against the given repo.
func checkPratcProcessMatches(pid int, repo string) bool {
	// Use ps to get the command line for the specific PID
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "args=")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	cmdLine := strings.TrimSpace(string(output))
	// Check if the command line contains pratc and the repo
	return strings.Contains(cmdLine, "pratc") && strings.Contains(cmdLine, repo)
}

// lockPathForRepo computes the lock file path for a given repository.
func lockPathForRepo(repo string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get user home directory: %w", err)
	}

	// Normalize the repo name to ensure consistent lock paths
	normalized := types.NormalizeRepoName(repo)
	hash := md5Hash(normalized)

	lockDir := filepath.Join(home, ".pratc", "locks")
	return filepath.Join(lockDir, hash+".lock"), nil
}

// md5Hash returns the MD5 hash of the input string as a hex string.
func md5Hash(input string) string {
	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

// writeLockContent writes lock metadata to the file.
func writeLockContent(file *os.File, content lockContent) error {
	data := fmt.Sprintf("PID: %d\nStartTime: %s\nCommand: %s\nRepo: %s\n",
		content.PID, content.StartTime, content.Command, content.Repo)
	_, err := file.WriteString(data)
	return err
}

// parseLockContent parses lock file content.
func parseLockContent(data string) (lockContent, error) {
	var content lockContent
	lines := strings.Split(strings.TrimSpace(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "PID":
			pid, err := strconv.Atoi(value)
			if err != nil {
				return content, fmt.Errorf("invalid PID: %w", err)
			}
			content.PID = pid
		case "StartTime":
			content.StartTime = value
		case "Command":
			content.Command = value
		case "Repo":
			content.Repo = value
		}
	}
	return content, nil
}

// LockStatus checks if a lock exists for the given repo and returns status info.
func LockStatus(repo string) (locked bool, holder *lockContent, err error) {
	repo = types.NormalizeRepoName(repo)

	lockPath, err := lockPathForRepo(repo)
	if err != nil {
		return false, nil, err
	}

	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		return false, nil, nil
	} else if err != nil {
		return false, nil, err
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false, nil, err
	}

	content, err := parseLockContent(string(data))
	if err != nil {
		return false, nil, err
	}

	// If our own PID holds the lock, we're locked
	if content.PID == os.Getpid() {
		return true, &content, nil
	}

	// Check if another process holds the lock and is still running
	if isProcessAlive(content.PID) && isPratcProcessRunning(content.PID, repo) {
		return true, &content, nil
	}

	// Lock is stale
	return false, nil, nil
}

// ForceAcquireRepoLock acquires a lock even if another process holds it.
// This should only be used with the --force flag.
func ForceAcquireRepoLock(repo string) (*RepoLock, error) {
	repo = types.NormalizeRepoName(repo)

	lockPath, err := lockPathForRepo(repo)
	if err != nil {
		return nil, fmt.Errorf("compute lock path: %w", err)
	}

	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	// Remove any existing lock file
	os.Remove(lockPath)

	// Create a new lock file
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}

	content := lockContent{
		PID:       os.Getpid(),
		StartTime: time.Now().UTC().Format(time.RFC3339),
		Command:   "pratc",
		Repo:      repo,
	}
	if err := writeLockContent(file, content); err != nil {
		file.Close()
		os.Remove(lockPath)
		return nil, fmt.Errorf("write lock content: %w", err)
	}

	return &RepoLock{
		Path: lockPath,
		File: file,
		Repo: repo,
	}, nil
}
