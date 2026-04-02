package sync

import (
	"context"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func TestScheduler_CheckAndResume(t *testing.T) {
	t.Parallel()

	// Create in-memory store
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	// Create a paused job with next_scheduled_at in the past
	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	pastTime := time.Now().Add(-24 * time.Hour)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	// Verify the job is paused
	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}
	if pausedJobs[0].Status != "paused" {
		t.Errorf("expected paused status, got %s", pausedJobs[0].Status)
	}

	scheduler := NewScheduler(store)
	if err := scheduler.checkAndResume(context.Background()); err != nil {
		t.Fatalf("checkAndResume failed: %v", err)
	}

	// Verify the job is no longer paused
	pausedJobs, err = store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs after resume: %v", err)
	}
	if len(pausedJobs) != 0 {
		t.Errorf("expected 0 paused jobs after resume, got %d", len(pausedJobs))
	}

	// Verify the job status is now in_progress
	resumedJob, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get resumed job: %v", err)
	}
	if resumedJob.Status != "in_progress" {
		t.Errorf("expected in_progress status after resume, got %s", resumedJob.Status)
	}
}

func TestScheduler_CheckAndResume_FutureJobNotResumed(t *testing.T) {
	t.Parallel()

	// Create in-memory store
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	// Create a paused job with next_scheduled_at in the future
	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// Pause the job with a future scheduled time
	futureTime := time.Now().Add(10 * time.Minute)
	if err := store.PauseSyncJob(job.ID, futureTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	// Verify the job is paused
	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}

	// Create scheduler and run checkAndResume
	scheduler := NewScheduler(store)
	if err := scheduler.checkAndResume(context.Background()); err != nil {
		t.Fatalf("checkAndResume failed: %v", err)
	}

	// Verify the job is still paused (not resumed because it's not overdue)
	pausedJobs, err = store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs after check: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Errorf("expected 1 paused job (not overdue), got %d", len(pausedJobs))
	}
	if pausedJobs[0].Status != "paused" {
		t.Errorf("expected paused status, got %s", pausedJobs[0].Status)
	}
}

func TestScheduler_CheckAndResume_ZeroScheduledTimeSkipped(t *testing.T) {
	t.Parallel()

	// Create in-memory store
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	// Create a paused job with zero next_scheduled_at
	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	// Pause the job with zero time (not scheduled)
	zeroTime := time.Time{}
	if err := store.PauseSyncJob(job.ID, zeroTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	// Verify the job is paused
	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Fatalf("expected 1 paused job, got %d", len(pausedJobs))
	}

	// Create scheduler and run checkAndResume
	scheduler := NewScheduler(store)
	if err := scheduler.checkAndResume(context.Background()); err != nil {
		t.Fatalf("checkAndResume failed: %v", err)
	}

	// Verify the job is still paused (zero time should be skipped)
	pausedJobs, err = store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs after check: %v", err)
	}
	if len(pausedJobs) != 1 {
		t.Errorf("expected 1 paused job (zero time), got %d", len(pausedJobs))
	}
	if pausedJobs[0].Status != "paused" {
		t.Errorf("expected paused status, got %s", pausedJobs[0].Status)
	}
}

func TestScheduler_Run(t *testing.T) {
	t.Parallel()

	store, err := cache.Open("file::scheduler_run_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	repo := "test/repo"
	job, err := store.CreateSyncJob(repo)
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	pastTime := time.Now().Add(-10 * time.Minute)
	if err := store.PauseSyncJob(job.ID, pastTime, "rate limited"); err != nil {
		t.Fatalf("failed to pause sync job: %v", err)
	}

	scheduler := NewScheduler(store, WithCheckInterval(10*time.Millisecond))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- scheduler.Run(ctx)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("scheduler.Run failed: %v", err)
		}
	case <-ctx.Done():
	}

	pausedJobs, err := store.ListPausedSyncJobs()
	if err != nil {
		t.Fatalf("failed to list paused jobs after scheduler run: %v", err)
	}
	if len(pausedJobs) != 0 {
		t.Errorf("expected 0 paused jobs after scheduler run, got %d", len(pausedJobs))
	}

	resumedJob, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("failed to get resumed job: %v", err)
	}
	if resumedJob.Status != "in_progress" {
		t.Errorf("expected in_progress status after scheduler run, got %s", resumedJob.Status)
	}
}
