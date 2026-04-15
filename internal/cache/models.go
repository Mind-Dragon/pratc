package cache

import (
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type PRFilter struct {
	Repo         string
	BaseBranch   string
	CIStatus     string
	UpdatedSince time.Time
}

type PRPage struct {
	PRs       []types.PR
	NextCursor string
	HasMore    bool
}

type MergedPR struct {
	Repo         string
	Number       int
	MergedAt     time.Time
	FilesTouched []string
}

type SyncProgress struct {
	Cursor            string    `json:"cursor"`
	ProcessedPRs      int       `json:"processed_prs"`
	TotalPRs          int       `json:"total_prs"`
	NextScheduledAt   time.Time `json:"next_scheduled_at"`
	EstimatedRequests int       `json:"estimated_requests"`
	ScheduledResumeAt time.Time `json:"scheduled_resume_at"`
	PauseReason       string    `json:"pause_reason"`
	LastBudgetCheck   time.Time `json:"last_budget_check"`
}

type SyncJobStatus string

const (
	SyncJobStatusInProgress SyncJobStatus = "in_progress"
	SyncJobStatusCompleted  SyncJobStatus = "completed"
	SyncJobStatusFailed     SyncJobStatus = "failed"
	SyncJobStatusPaused     SyncJobStatus = "paused"
)

type SyncJob struct {
	ID         string
	Repo       string
	Status     SyncJobStatus
	Progress   SyncProgress
	LastSyncAt time.Time
	Error      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// IsPaused returns true if the job status is paused.
func (j SyncJob) IsPaused() bool {
	return j.Status == SyncJobStatusPaused
}

type PRRecord = types.PR
