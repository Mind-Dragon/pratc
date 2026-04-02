package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
)

// Scheduler runs in the background and checks for paused sync jobs whose next_scheduled_at has passed,
// resuming them by updating their status back to in_progress.
type Scheduler struct {
	store         *cache.Store
	checkInterval time.Duration
	logger        *logger.Logger
}

// SchedulerOption represents a functional option for configuring a Scheduler.
type SchedulerOption func(*Scheduler)

// WithCheckInterval sets the interval at which the scheduler checks for overdue paused jobs.
// Default is 30 seconds.
func WithCheckInterval(d time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		s.checkInterval = d
	}
}

// NewScheduler creates a new Scheduler instance with the given cache store and optional configuration.
func NewScheduler(store *cache.Store, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{
		store:         store,
		checkInterval: 30 * time.Second,
		logger:        logger.New("scheduler"),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Run starts the scheduler loop that checks for paused jobs to resume.
// It runs until the context is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.checkAndResume(ctx); err != nil {
				s.logger.Error("failed to check and resume paused jobs", "error", err.Error())
			}
		}
	}
}

// checkAndResume lists all paused sync jobs and resumes those whose next_scheduled_at has passed.
func (s *Scheduler) checkAndResume(ctx context.Context) error {
	pausedJobs, err := s.store.ListPausedSyncJobs()
	if err != nil {
		return fmt.Errorf("failed to list paused sync jobs: %w", err)
	}

	now := time.Now().UTC()
	for _, job := range pausedJobs {
		// Skip jobs without a scheduled time (zero value means not scheduled)
		if job.Progress.NextScheduledAt.IsZero() {
			continue
		}

		// Check if the job is overdue
		if job.Progress.NextScheduledAt.UTC().Before(now) {
			// Resume the job by updating its status back to in_progress
			if err := s.store.ResumeSyncJobByID(job.ID); err != nil {
				s.logger.Error("failed to resume paused job", "job_id", job.ID, "repo", job.Repo, "error", err.Error())
				continue
			}
			s.logger.Info("resumed paused job", "job_id", job.ID, "repo", job.Repo, "next_scheduled_at", job.Progress.NextScheduledAt.Format(time.RFC3339))
		}
	}

	return nil
}
