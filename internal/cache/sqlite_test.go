package cache

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

func TestCacheUpsertAndQuery(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	for i := 1; i <= 10; i++ {
		pr := samplePR(i)
		if i%2 == 0 {
			pr.BaseBranch = "release"
			pr.CIStatus = "failing"
		}
		if err := store.UpsertPR(pr); err != nil {
			t.Fatalf("upsert pr %d: %v", i, err)
		}
	}

	mainPRs, err := store.ListPRs(PRFilter{Repo: "owner/repo", BaseBranch: "main"})
	if err != nil {
		t.Fatalf("list main prs: %v", err)
	}
	if len(mainPRs) != 5 {
		t.Fatalf("expected 5 main prs, got %d", len(mainPRs))
	}

	releasePRs, err := store.ListPRs(PRFilter{Repo: "owner/repo", BaseBranch: "release", CIStatus: "failing"})
	if err != nil {
		t.Fatalf("list release prs: %v", err)
	}
	if len(releasePRs) != 5 {
		t.Fatalf("expected 5 release prs, got %d", len(releasePRs))
	}
}

func TestCacheUpdatedSinceFilter(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	oldPR := samplePR(1)
	oldPR.UpdatedAt = "2026-03-12T10:00:00Z"
	if err := store.UpsertPR(oldPR); err != nil {
		t.Fatalf("upsert old pr: %v", err)
	}

	newPR := samplePR(2)
	newPR.UpdatedAt = "2026-03-12T12:00:00Z"
	if err := store.UpsertPR(newPR); err != nil {
		t.Fatalf("upsert new pr: %v", err)
	}

	prs, err := store.ListPRs(PRFilter{
		Repo:         "owner/repo",
		UpdatedSince: mustParseTime(t, "2026-03-12T11:00:00Z"),
	})
	if err != nil {
		t.Fatalf("list prs by updated since: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 2 {
		t.Fatalf("expected only PR 2, got %+v", prs)
	}
}

func TestCacheLastSyncRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	want := mustParseTime(t, "2026-03-12T13:00:00Z")

	if err := store.SetLastSync("owner/repo", want); err != nil {
		t.Fatalf("set last sync: %v", err)
	}

	got, err := store.LastSync("owner/repo")
	if err != nil {
		t.Fatalf("get last sync: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestCacheMergedPRRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	merged := MergedPR{
		Repo:         "owner/repo",
		Number:       99,
		MergedAt:     mustParseTime(t, "2026-03-11T18:00:00Z"),
		FilesTouched: []string{"internal/planner/plan.go", "internal/types/models.go"},
	}

	if err := store.UpsertMergedPR(merged); err != nil {
		t.Fatalf("upsert merged pr: %v", err)
	}

	got, err := store.ListMergedPRs("owner/repo")
	if err != nil {
		t.Fatalf("list merged prs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 merged pr, got %d", len(got))
	}
	if got[0].Number != merged.Number || len(got[0].FilesTouched) != 2 {
		t.Fatalf("unexpected merged pr: %+v", got[0])
	}
}

func TestCacheSyncJobLifecycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	store.now = func() time.Time {
		return mustParseTime(t, "2026-03-12T14:00:00Z")
	}

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := store.UpdateSyncJobProgress(job.ID, SyncProgress{
		Cursor:       "cursor-2",
		ProcessedPRs: 25,
		TotalPRs:     100,
	}); err != nil {
		t.Fatalf("update sync job progress: %v", err)
	}

	if err := store.MarkSyncJobComplete(job.ID, mustParseTime(t, "2026-03-12T14:05:00Z")); err != nil {
		t.Fatalf("mark job complete: %v", err)
	}

	got, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("get sync job: %v", err)
	}
	if got.Status != SyncJobStatusCompleted {
		t.Fatalf("expected completed status, got %s", got.Status)
	}
	if got.Progress.Cursor != "cursor-2" || got.Progress.ProcessedPRs != 25 || got.Progress.TotalPRs != 100 {
		t.Fatalf("unexpected job progress: %+v", got.Progress)
	}
}

func TestCacheResumeSyncJobReturnsLatestInProgress(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	first, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create first job: %v", err)
	}
	if err := store.MarkSyncJobFailed(first.ID, "network error"); err != nil {
		t.Fatalf("fail first job: %v", err)
	}

	second, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create second job: %v", err)
	}
	if err := store.UpdateSyncJobProgress(second.ID, SyncProgress{Cursor: "cursor-live", ProcessedPRs: 12, TotalPRs: 50}); err != nil {
		t.Fatalf("update second job: %v", err)
	}

	got, ok, err := store.ResumeSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("resume sync job: %v", err)
	}
	if !ok {
		t.Fatal("expected resumable sync job")
	}
	if got.ID != second.ID || got.Progress.Cursor != "cursor-live" {
		t.Fatalf("unexpected resumed job: %+v", got)
	}
}

func TestCacheWALConcurrentReadWrite(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	if mode, err := store.JournalMode(); err != nil {
		t.Fatalf("journal mode: %v", err)
	} else if mode != "wal" {
		t.Fatalf("expected wal journal mode, got %q", mode)
	}

	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 1; i <= 25; i++ {
			if err := store.UpsertPR(samplePR(i)); err != nil {
				t.Errorf("upsert pr %d: %v", i, err)
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := store.ListPRs(PRFilter{Repo: "owner/repo"}); err != nil {
				t.Errorf("list prs during write: %v", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	writer.Wait()
	<-done
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "cache.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})

	return store
}

func samplePR(number int) types.PR {
	return types.PR{
		ID:                fmt.Sprintf("PR_%d", number),
		Repo:              "owner/repo",
		Number:            number,
		Title:             fmt.Sprintf("PR %d", number),
		Body:              "Body",
		URL:               fmt.Sprintf("https://github.com/owner/repo/pull/%d", number),
		Author:            "octocat",
		Labels:            []string{"triage"},
		FilesChanged:      []string{fmt.Sprintf("internal/service/file_%d.go", number)},
		ReviewStatus:      "approved",
		CIStatus:          "passing",
		Mergeable:         "mergeable",
		BaseBranch:        "main",
		HeadBranch:        fmt.Sprintf("feature/%d", number),
		CreatedAt:         "2026-03-12T09:00:00Z",
		UpdatedAt:         fmt.Sprintf("2026-03-12T%02d:00:00Z", 10+number%10),
		Additions:         10 + number,
		Deletions:         number,
		ChangedFilesCount: 1,
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return parsed
}
