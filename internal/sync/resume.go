package sync

import (
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

// ResumeJob finds a paused sync job for a repo, checks if it's time to resume,
// and transitions it back to in_progress status.
func ResumeJob(store *cache.Store, repo string) (cache.SyncJob, error) {
	job, err := store.GetPausedSyncJobByRepo(repo)
	if err != nil {
		return cache.SyncJob{}, fmt.Errorf("resume job: %w", err)
	}

	// If NextScheduledAt is zero or in the future, return error
	now := time.Now().UTC()
	nextScheduledAt := job.Progress.NextScheduledAt
	if nextScheduledAt.IsZero() || nextScheduledAt.After(now) {
		if nextScheduledAt.IsZero() {
			return cache.SyncJob{}, fmt.Errorf("sync job not yet due for resume, no scheduled time")
		}
		return cache.SyncJob{}, fmt.Errorf("sync job not yet due for resume, scheduled at %s", nextScheduledAt.Format(time.RFC3339))
	}

	// Update sync_jobs SET status='resuming', error_message='', updated_at=now WHERE id=jobID
	if err := store.ResumeSyncJobByID(job.ID); err != nil {
		return cache.SyncJob{}, fmt.Errorf("resume job: %w", err)
	}

	// Update the job with new status and timestamp
	job.Status = cache.SyncJobStatusResuming
	job.Error = ""
	job.UpdatedAt = now

	return job, nil
}
