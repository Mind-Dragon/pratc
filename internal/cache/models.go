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
	PRs        []types.PR
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
	SnapshotCeiling   int       `json:"snapshot_ceiling"`
	NextScheduledAt   time.Time `json:"next_scheduled_at"`
	EstimatedRequests int       `json:"estimated_requests"`
	ScheduledResumeAt time.Time `json:"scheduled_resume_at"`
	PauseReason       string    `json:"pause_reason"`
	LastBudgetCheck   time.Time `json:"last_budget_check"`
}

type SyncJobStatus string

const (
	// Explicit job states for v1.4.1+ state machine
	SyncJobStatusQueued          SyncJobStatus = "queued"            // Job created, not yet started
	SyncJobStatusRunning         SyncJobStatus = "running"           // Job actively processing
	SyncJobStatusPausedRateLimit SyncJobStatus = "paused_rate_limit" // Paused due to rate limit
	SyncJobStatusResuming        SyncJobStatus = "resuming"          // Transitioning from paused to running
	SyncJobStatusCompleted       SyncJobStatus = "completed"         // Terminal state
	SyncJobStatusFailed          SyncJobStatus = "failed"            // Terminal state
	SyncJobStatusCanceled        SyncJobStatus = "canceled"          // Terminal state

	// Legacy states (deprecated, kept for backward compatibility with existing code)
	SyncJobStatusInProgress SyncJobStatus = "in_progress" // Deprecated: use Running or Resuming
	SyncJobStatusPaused     SyncJobStatus = "paused"      // Deprecated: use PausedRateLimit
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

// IsPaused returns true if the job status indicates a paused state.
func (j SyncJob) IsPaused() bool {
	return j.Status == SyncJobStatusPaused || j.Status == SyncJobStatusPausedRateLimit
}

// IsTerminal returns true if the job is in a terminal state.
func (j SyncJob) IsTerminal() bool {
	return j.Status == SyncJobStatusCompleted || j.Status == SyncJobStatusFailed || j.Status == SyncJobStatusCanceled
}

// IsActive returns true if the job is in an active (non-terminal) state.
func (j SyncJob) IsActive() bool {
	return j.Status == SyncJobStatusQueued || j.Status == SyncJobStatusRunning || j.Status == SyncJobStatusResuming
}

type PRRecord = types.PR
