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
		if job.Status != StatusActive && job.Status != StatusPaused && job.Status != StatusQueued {
			t.Errorf("expected active, paused, or queued status, got %s for job %s", job.Status, job.ID)
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

func TestStoreGetActiveJobsWithError(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)
	cacheStore.Close()

	jobs := store.GetActiveJobs()
	if jobs != nil {
		t.Errorf("expected nil on error, got %v", jobs)
	}
}

func TestStoreGetJobHistoryWithError(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)
	cacheStore.Close()

	jobs := store.GetJobHistory(10)
	if jobs != nil {
		t.Errorf("expected nil on error, got %v", jobs)
	}
}

func TestStoreGetAllJobsWithError(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)
	cacheStore.Close()

	jobs := store.GetAllJobs()
	if jobs != nil {
		t.Errorf("expected nil on error, got %v", jobs)
	}
}

func TestStoreDB(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	db := store.DB()
	if db == nil {
		t.Fatal("expected non-nil DB")
	}

	var one int
	if err := db.QueryRow("SELECT 1").Scan(&one); err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	if one != 1 {
		t.Errorf("expected 1, got %d", one)
	}
}

func TestStoreJobToViewProgressCalculation(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 50,
		TotalPRs:     100,
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	jobs := store.GetAllJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].Progress != 50 {
		t.Errorf("expected progress 50, got %d", jobs[0].Progress)
	}
}

func TestStoreJobToViewZeroTotal(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 0,
		TotalPRs:     0,
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	jobs := store.GetAllJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].Progress != 0 {
		t.Errorf("expected progress 0 when total is 0, got %d", jobs[0].Progress)
	}
}

func TestStoreMapCacheStatus(t *testing.T) {
	tests := []struct {
		input    cache.SyncJobStatus
		expected string
	}{
		{cache.SyncJobStatusQueued, StatusQueued},
		{cache.SyncJobStatusRunning, StatusActive},
		{cache.SyncJobStatusResuming, StatusActive},
		{cache.SyncJobStatusPausedRateLimit, StatusPaused},
		{cache.SyncJobStatusCompleted, StatusCompleted},
		{cache.SyncJobStatusFailed, StatusFailed},
		{cache.SyncJobStatusCanceled, StatusFailed},
		// Legacy states (deprecated)
		{cache.SyncJobStatusInProgress, StatusActive},
		{cache.SyncJobStatusPaused, StatusPaused},
		{cache.SyncJobStatus("unknown"), StatusQueued},
		{"", StatusQueued},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := mapCacheStatus(tt.input)
			if result != tt.expected {
				t.Errorf("mapCacheStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStoreJobToViewCompletedDetail(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.MarkSyncJobComplete(job.ID, time.Now()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	jobs := store.GetAllJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].Detail != "Completed successfully" {
		t.Errorf("expected 'Completed successfully' detail, got %q", jobs[0].Detail)
	}
}

func TestStoreJobToViewErrorDetail(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	errorMsg := "network timeout"
	if err := cacheStore.MarkSyncJobFailed(job.ID, errorMsg); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	jobs := store.GetAllJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].Detail != errorMsg {
		t.Errorf("expected error detail %q, got %q", errorMsg, jobs[0].Detail)
	}
}

func TestStoreGetJobHistoryEmpty(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	jobs := store.GetJobHistory(10)
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs in history, got %d", len(jobs))
	}
}

func TestStoreGetActiveJobsEmpty(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	store := NewStore(cacheStore)

	for i := 0; i < 3; i++ {
		job, err := cacheStore.CreateSyncJob("owner/repo" + string(rune('a'+i)))
		if err != nil {
			t.Fatalf("create job: %v", err)
		}
		if err := cacheStore.MarkSyncJobComplete(job.ID, time.Now()); err != nil {
			t.Fatalf("complete job: %v", err)
		}
	}

	jobs := store.GetActiveJobs()
	if len(jobs) != 0 {
		t.Errorf("expected 0 active jobs, got %d", len(jobs))
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
