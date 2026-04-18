package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestSyncIntegration_UpsertPR_Works(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping")
	}

	tempDir, err := os.MkdirTemp("", "pratc-upsert-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	cacheStore, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open cache store: %v", err)
	}
	defer cacheStore.Close()

	testPR := types.PR{
		ID:                "PR_123",
		Repo:              "test/repo",
		Number:            123,
		Title:             "Test PR",
		Body:              "Test body",
		URL:               types.GitHubURLPrefix + "test/repo/pull/123",
		Author:            "testuser",
		Labels:            []string{"bug", "enhancement"},
		FilesChanged:      []string{"src/main.go", "src/lib.go"},
		ReviewStatus:      "APPROVED",
		CIStatus:          "SUCCESS",
		Mergeable:         "true",
		BaseBranch:        "main",
		HeadBranch:        "feature/test",
		ClusterID:         "",
		CreatedAt:         "2024-01-01T00:00:00Z",
		UpdatedAt:         "2024-01-02T00:00:00Z",
		IsDraft:           false,
		IsBot:             false,
		Additions:         100,
		Deletions:         50,
		ChangedFilesCount: 2,
	}

	err = cacheStore.UpsertPR(testPR)
	if err != nil {
		t.Fatalf("UpsertPR failed: %v", err)
	}

	prs, err := cacheStore.ListPRs(cache.PRFilter{Repo: "test/repo"})
	if err != nil {
		t.Fatalf("ListPRs failed: %v", err)
	}

	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}

	if prs[0].Title != "Test PR" {
		t.Errorf("expected title 'Test PR', got '%s'", prs[0].Title)
	}

	if prs[0].Number != 123 {
		t.Errorf("expected number 123, got %d", prs[0].Number)
	}
}

func TestSyncIntegration_CurrentSyncBehavior(t *testing.T) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		t.Skip("GITHUB_TOKEN not set, skipping")
	}

	tempDir, err := os.MkdirTemp("", "pratc-sync-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	cacheStore, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open cache store: %v", err)
	}
	defer cacheStore.Close()

	prsBefore, err := cacheStore.ListPRs(cache.PRFilter{Repo: "jeffersonnunn/test-repo"})
	if err != nil {
		t.Fatalf("failed to query PRs: %v", err)
	}
	t.Logf("PR count before sync: %d", len(prsBefore))

	worker := defaultWorker(nil, 0)
	t.Logf("Worker.CacheStore is nil (expected with nil passed)")
	t.Logf("Worker.Metadata type: %T", worker.Metadata)

	_ = cacheStore
}
