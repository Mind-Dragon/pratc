package cmd

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestIsWorkflowRateLimitPause(t *testing.T) {
	t.Parallel()

	if !isWorkflowRateLimitPause(errors.New("rate limit budget exhausted")) {
		t.Fatal("expected rate limit pause detector to match pause error")
	}
	if isWorkflowRateLimitPause(errors.New("something else")) {
		t.Fatal("expected rate limit pause detector to ignore unrelated errors")
	}
}

func TestWorkflowRateLimitPauseNotice(t *testing.T) {
	t.Parallel()

	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	repo := "owner/repo"
	for i := 1; i <= 3; i++ {
		if err := store.UpsertPR(types.PR{Repo: repo, Number: i}); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}

	msg, err := buildWorkflowRateLimitPauseNotice(store, repo, cache.SyncJob{
		Progress: cache.SyncProgress{ProcessedPRs: 3, TotalPRs: 7},
	})
	if err != nil {
		t.Fatalf("build pause notice: %v", err)
	}
	if !strings.Contains(msg, "4 remain") {
		t.Fatalf("expected remaining count in message, got %q", msg)
	}
	if !strings.Contains(msg, "cached 3-PR snapshot") {
		t.Fatalf("expected cached snapshot count in message, got %q", msg)
	}
}

func TestDefaultWorkflowOutDirUsesRepoSlug(t *testing.T) {
	t.Parallel()

	out := defaultWorkflowOutDir("owner/repo")
	if !strings.Contains(out, "owner_repo") {
		t.Fatalf("expected output dir to contain repo slug, got %q", out)
	}
}

func TestReuseCachedWorkflowSyncSummary(t *testing.T) {
	t.Parallel()

	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	repo := "owner/repo"
	if err := store.SetLastSync(repo, time.Date(2026, 3, 12, 15, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("set last sync: %v", err)
	}
	if err := store.UpsertPR(types.PR{Repo: repo, Number: 1}); err != nil {
		t.Fatalf("upsert pr: %v", err)
	}

	summary, reused, err := reuseCachedWorkflowSyncSummary(store, repo)
	if err != nil {
		t.Fatalf("reuse cached sync summary: %v", err)
	}
	if !reused {
		t.Fatal("expected cached sync summary to be reused")
	}
	if summary.Status != string(cache.SyncJobStatusCompleted) {
		t.Fatalf("expected completed status, got %q", summary.Status)
	}
	if summary.Budget != "local-first cache reuse" {
		t.Fatalf("expected local-first cache reuse budget marker, got %q", summary.Budget)
	}
}
