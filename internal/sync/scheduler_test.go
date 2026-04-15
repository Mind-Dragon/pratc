package sync

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func TestSchedulerStartResumesEligibleJobs(t *testing.T) {
	t.Parallel()

	store, err := cache.Open("file::scheduler_start_resume?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("test/repo")
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	now := time.Now().UTC()
	if err := store.PauseSyncJob(job.ID, now.Add(-time.Hour), "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	if _, err := store.DB().Exec(`
		UPDATE sync_progress
		SET next_scheduled_at = ?, scheduled_resume_at = ?, updated_at = ?
		WHERE repo = ?
	`, now.Add(2*time.Hour).Format(time.RFC3339), now.Add(-time.Minute).Format(time.RFC3339), now.Format(time.RFC3339), job.Repo); err != nil {
		t.Fatalf("failed to override resume times: %v", err)
	}

	scheduler := NewScheduler(store, WithCheckInterval(10*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer scheduler.Stop()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		paused, err := store.ListPausedSyncJobs()
		if err != nil {
			t.Fatalf("failed to list paused jobs: %v", err)
		}
		if len(paused) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	paused, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(paused) != 0 {
		t.Fatalf("expected scheduler to resume eligible job, still paused: %d", len(paused))
	}

	resumed, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to read resumed job: %v", err)
	}
	if resumed.Status != cache.SyncJobStatusResuming {
		t.Fatalf("expected resumed job to be resuming, got %s", resumed.Status)
	}
}

func TestSchedulerStopPreventsFurtherChecks(t *testing.T) {
	t.Parallel()

	store, err := cache.Open("file::scheduler_stop?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("test/repo")
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	future := time.Now().UTC().Add(10 * time.Minute)
	if err := store.PauseSyncJob(job.ID, future, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	scheduler := NewScheduler(store, WithCheckInterval(10*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := scheduler.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	scheduler.Stop()

	time.Sleep(50 * time.Millisecond)

	paused, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(paused) != 1 {
		t.Fatalf("expected paused job to remain paused after Stop, got %d", len(paused))
	}
	if paused[0].Status != cache.SyncJobStatusPausedRateLimit {
		t.Fatalf("expected paused_rate_limit status, got %s", paused[0].Status)
	}
}
