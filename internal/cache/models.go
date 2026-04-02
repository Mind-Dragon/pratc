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

type MergedPR struct {
	Repo         string
	Number       int
	MergedAt     time.Time
	FilesTouched []string
}

type SyncProgress struct {
	Cursor            string
	ProcessedPRs      int
	TotalPRs          int
	NextScheduledAt   time.Time
	EstimatedRequests int
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

type PRRecord = types.PR
