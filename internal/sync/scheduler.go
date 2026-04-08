package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
)

type Scheduler struct {
	store         *cache.Store
	checkInterval time.Duration
	logger        *logger.Logger

	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
}

type SchedulerOption func(*Scheduler)

func WithCheckInterval(d time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		s.checkInterval = d
	}
}

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

func (s *Scheduler) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	childCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.running = true
	done := s.done
	s.mu.Unlock()

	s.logger.Info("scheduler started", "check_interval", s.checkInterval.String())

	go s.run(childCtx, done)
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	running := s.running
	s.cancel = nil
	s.done = nil
	s.running = false
	s.mu.Unlock()

	if !running {
		return
	}
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if err := s.Start(ctx); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	<-ctx.Done()
	s.Stop()
	return nil
}

func (s *Scheduler) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer func() {
		s.mu.Lock()
		s.cancel = nil
		s.done = nil
		s.running = false
		s.mu.Unlock()
		s.logger.Info("scheduler stopped")
	}()

	if err := s.CheckAndResume(ctx); err != nil {
		s.logger.Error("scheduler check failed", "error", err.Error())
	}

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.CheckAndResume(ctx); err != nil {
				s.logger.Error("scheduler check failed", "error", err.Error())
			}
		}
	}
}

func (s *Scheduler) CheckAndResume(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	pausedJobs, err := s.store.ListPausedSyncJobs()
	if err != nil {
		return fmt.Errorf("list paused sync jobs: %w", err)
	}

	now := time.Now().UTC()
	for _, job := range pausedJobs {
		if ctx != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
		}

		resumeAt := job.Progress.ScheduledResumeAt
		if resumeAt.IsZero() {
			resumeAt = job.Progress.NextScheduledAt
		}
		if resumeAt.IsZero() || resumeAt.After(now) {
			continue
		}

		if err := s.store.ResumeSyncJobByID(job.ID); err != nil {
			s.logger.Error("failed to resume paused job", "job_id", job.ID, "repo", job.Repo, "error", err.Error())
			continue
		}

		s.logger.Info("resumed paused job", "job_id", job.ID, "repo", job.Repo, "scheduled_resume_at", resumeAt.Format(time.RFC3339))
	}

	return nil
}
