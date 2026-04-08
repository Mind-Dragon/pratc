package data

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func TestStoreGetAllJobs(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	jobs := store.GetAllJobs()
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs on fresh store, got %d", len(jobs))
	}

	job1, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job 1: %v", err)
	}
	job2, err := cacheStore.CreateSyncJob("owner/repo2")
	if err != nil {
		t.Fatalf("create sync job 2: %v", err)
	}

	jobs = store.GetAllJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	found := make(map[string]bool)
	for _, job := range jobs {
		found[job.ID] = true
	}
	if !found[job1.ID] || !found[job2.ID] {
		t.Fatalf("expected both job IDs, got %v", found)
	}
}

func TestStoreGetActiveJobs(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	active, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create active job: %v", err)
	}

	paused, err := cacheStore.CreateSyncJob("owner/paused-repo")
	if err != nil {
		t.Fatalf("create paused job: %v", err)
	}
	if err := cacheStore.PauseSyncJob(paused.ID, time.Now().Add(time.Hour), "rate limit"); err != nil {
		t.Fatalf("pause job: %v", err)
	}

	completed, err := cacheStore.CreateSyncJob("owner/completed-repo")
	if err != nil {
		t.Fatalf("create completed job: %v", err)
	}
	if err := cacheStore.MarkSyncJobComplete(completed.ID, time.Now()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	failed, err := cacheStore.CreateSyncJob("owner/failed-repo")
	if err != nil {
		t.Fatalf("create failed job: %v", err)
	}
	if err := cacheStore.MarkSyncJobFailed(failed.ID, "network error"); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	jobs := store.GetActiveJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 active jobs, got %d", len(jobs))
	}

	found := make(map[string]bool)
	for _, job := range jobs {
		found[job.ID] = true
		if job.Status != StatusActive && job.Status != StatusPaused {
			t.Errorf("expected active or paused status, got %s for job %s", job.Status, job.ID)
		}
	}
	if !found[active.ID] {
		t.Errorf("expected active job %s in results", active.ID)
	}
	if !found[paused.ID] {
		t.Errorf("expected paused job %s in results", paused.ID)
	}
}

func TestStoreGetJobHistory(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	job1, err := cacheStore.CreateSyncJob("owner/repo1")
	if err != nil {
		t.Fatalf("create job 1: %v", err)
	}
	cacheStore.MarkSyncJobComplete(job1.ID, time.Now())

	job2, err := cacheStore.CreateSyncJob("owner/repo2")
	if err != nil {
		t.Fatalf("create job 2: %v", err)
	}
	cacheStore.MarkSyncJobFailed(job2.ID, "test error")

	_, err = cacheStore.CreateSyncJob("owner/repo3")
	if err != nil {
		t.Fatalf("create job 3: %v", err)
	}

	history := store.GetJobHistory(10)
	if len(history) != 2 {
		t.Fatalf("expected 2 jobs in history, got %d", len(history))
	}

	for _, job := range history {
		if job.Status != StatusCompleted && job.Status != StatusFailed {
			t.Errorf("expected completed or failed status, got %s for job %s", job.Status, job.ID)
		}
	}
}

func TestStoreGetJobHistoryRespectsLimit(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	for i := 0; i < 5; i++ {
		repo := "owner/repo" + string(rune('a'+i))
		job, err := cacheStore.CreateSyncJob(repo)
		if err != nil {
			t.Fatalf("create job %d: %v", i, err)
		}
		cacheStore.MarkSyncJobComplete(job.ID, time.Now())
	}

	history := store.GetJobHistory(3)
	if len(history) != 3 {
		t.Fatalf("expected 3 jobs with limit, got %d", len(history))
	}
}

func TestStoreGetAllJobsExcludesEmptyRepo(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	job1, err := cacheStore.CreateSyncJob("owner/repo1")
	if err != nil {
		t.Fatalf("create job 1: %v", err)
	}

	jobs := store.GetAllJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != job1.ID {
		t.Errorf("expected job ID %s, got %s", job1.ID, jobs[0].ID)
	}
}

func newTestCacheStore(t *testing.T) *cache.Store {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test-cache.db")
	cacheStore, err := cache.Open(path)
	if err != nil {
		t.Fatalf("open cache store: %v", err)
	}
	t.Cleanup(func() {
		cacheStore.Close()
	})

	return cacheStore
}
