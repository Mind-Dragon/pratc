package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
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
	expectedRemoteURL := types.GitHubURLPrefix + "owner/repo.git"
	expectedMirrorPath := filepath.Join(tempDir, "repos", "owner", "repo.git")

	t.Run("creates and initializes mirror for valid repo", func(t *testing.T) {
		worker := defaultWorker(nil, 0, "", nil)
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
		worker := defaultWorker(nil, 0, "", nil)
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

	worker := defaultWorker(nil, 0, "", nil)
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

func TestCursorPersistenceDuringSync(t *testing.T) {
	t.Parallel()

	store, err := cache.Open("file::cursor_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	cursorUpdates := []string{"cursor-1", "cursor-2", "cursor-3"}
	updateIndex := 0
	mockMetadata := &mockMetadataSource{
		onCursor: func(cursor string, processed int) {
			if updateIndex < len(cursorUpdates) {
				cursorUpdates[updateIndex] = cursor
				updateIndex++
			}
		},
	}

	worker := Worker{
		MirrorFactory: func(context.Context, string) (Mirror, error) {
			return &fakeMirror{}, nil
		},
		Metadata:   mockMetadata,
		CacheStore: store,
		Now:        func() time.Time { return time.Now().UTC() },
	}

	runner := NewRunner(worker, dbJobRecorder{}, job.ID)

	ctx := context.Background()
	cursorsReceived := []string{}
	_, err = runner.worker.SyncJob(ctx, repo, func(stage string, done, total int) {
	}, func(cursor string, processed int) {
		cursorsReceived = append(cursorsReceived, cursor)
	})

	if err != nil {
		t.Fatalf("sync job failed: %v", err)
	}

	if len(cursorsReceived) == 0 {
		t.Error("expected cursor updates to be received")
	}
}

type mockMetadataSource struct {
	onCursor func(cursor string, processed int)
}

func (m *mockMetadataSource) SyncRepo(ctx context.Context, repo string, progress func(done, total int), onCursor func(cursor string, processed int)) (MetadataSnapshot, error) {
	if onCursor != nil {
		onCursor("cursor-1", 10)
		onCursor("cursor-2", 20)
		onCursor("cursor-3", 30)
	}
	if m.onCursor != nil {
		onCursor = m.onCursor
	}
	return MetadataSnapshot{
		OpenPRs:   []int{1, 2, 3},
		SyncedPRs: 3,
	}, nil
}
