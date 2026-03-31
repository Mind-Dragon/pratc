package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultWorkerMirrorFactoryInitializesMirror(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "pratc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set PRATC_CACHE_DIR to temp dir
	originalCacheDir := os.Getenv("PRATC_CACHE_DIR")
	os.Setenv("PRATC_CACHE_DIR", tempDir)
	defer os.Setenv("PRATC_CACHE_DIR", originalCacheDir)

	ctx := context.Background()
	repoID := "owner/repo"
	expectedRemoteURL := "https://github.com/owner/repo.git"
	expectedMirrorPath := filepath.Join(tempDir, "repos", "owner", "repo.git")

	t.Run("creates and initializes mirror for valid repo", func(t *testing.T) {
		worker := defaultWorker(nil)
		mirror, err := worker.MirrorFactory(ctx, repoID)
		if err != nil {
			t.Fatalf("MirrorFactory returned error: %v", err)
		}
		if mirror == nil {
			t.Fatalf("MirrorFactory returned nil mirror")
		}

		// Verify mirror directory exists
		if _, err := os.Stat(expectedMirrorPath); os.IsNotExist(err) {
			t.Fatalf("expected mirror directory to exist at %s", expectedMirrorPath)
		}

		// Verify it's a bare git repo by checking config
		configPath := filepath.Join(expectedMirrorPath, "config")
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read git config: %v", err)
		}

		configContent := string(configData)
		if !strings.Contains(configContent, "bare = true") {
			t.Errorf("expected git config to contain 'bare = true', got: %s", configContent)
		}

		// Verify origin remote is set correctly
		cmd := exec.Command("git", "--git-dir", expectedMirrorPath, "remote", "get-url", "origin")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to get remote URL: %v, output: %s", err, string(output))
		}

		actualURL := strings.TrimSpace(string(output))
		if actualURL != expectedRemoteURL {
			t.Errorf("expected remote URL %q, got %q", expectedRemoteURL, actualURL)
		}
	})

	t.Run("returns error for invalid repo ID format", func(t *testing.T) {
		worker := defaultWorker(nil)
		invalidRepoIDs := []string{
			"invalid",        // missing slash
			"/repo",          // missing owner
			"owner/",         // missing repo name
			"",               // empty string
			"ow..ner/repo",   // path traversal
			"owner/re..po",   // path traversal
			"owner@repo/bad", // invalid characters
		}

		for _, invalidRepoID := range invalidRepoIDs {
			t.Run(invalidRepoID, func(t *testing.T) {
				_, err := worker.MirrorFactory(ctx, invalidRepoID)
				if err == nil {
					t.Errorf("expected error for invalid repo ID %q, got nil", invalidRepoID)
				}
			})
		}
	})
}

func TestDefaultWorkerNowFunction(t *testing.T) {
	t.Parallel()

	worker := defaultWorker(nil)
	if worker.Now == nil {
		t.Fatal("expected Now function to be set")
	}

	// Verify Now returns a time
	now := worker.Now()
	if now.IsZero() {
		t.Error("expected Now to return non-zero time")
	}

	// Verify it's close to current time (within 1 second)
	expectedNow := time.Now().UTC()
	timeDiff := now.Sub(expectedNow)
	if timeDiff < -time.Second || timeDiff > time.Second {
		t.Errorf("expected Now to be close to current time, got %v, expected ~%v", now, expectedNow)
	}
}
