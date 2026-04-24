// Package data provides data models and storage abstractions for the monitor package.
// It defines structures for metrics, events, and monitor state that are
// shared between TUI and web implementations.
package data

import (
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
)

const (
	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusFailed    = "failed"
	StatusQueued    = "queued"
	StatusCompleted = "completed"
)

// SyncJobView represents a sync job for display in the monitor.
// It provides a flattened view of job state suitable for real-time updates.
type SyncJobView struct {
	ID       string
	Repo     string
	Progress int
	Status   string
	Detail   string
	ETA      time.Duration
	Batch    int
}

// RateLimitView represents the current GitHub API rate limit state.
// This is used by both TUI and web monitor to display quota information.
type RateLimitView struct {
	Remaining    int
	Total        int
	ResetTime    time.Time
	UsageHistory []RateLimitPoint
}

// RateLimitPoint represents a single point in the rate limit usage history.
type RateLimitPoint struct {
	Timestamp time.Time
	Remaining int
	Used      int
}

// LogEntry represents a single log line for display in the monitor.
// These entries are typically sourced from application logs and filtered
// for relevance to the current repo or operation.
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Repo      string
	Message   string
	Metadata  map[string]string
}

// ActivityBucket represents aggregated activity metrics over a time window.
// Used for sparklines and activity graphs in the monitor.
type ActivityBucket struct {
	TimeWindow   time.Time
	RequestCount int
	JobCount     int
	AvgDuration  time.Duration
}

// PRDetailView represents a pull request detail view for the monitor.
// It combines PR metadata with action work item information for display.
type PRDetailView struct {
	Title          string
	Author         string
	Age            time.Duration
	Status         string
	Lane           types.ActionLane
	Bucket         string
	Confidence     float64
	Reasons        []string
	DecisionLayers []types.DecisionLayer
	EvidenceRefs   []string
	DuplicateRefs  []int
	SynthesisRefs  []string
	RiskFlags      []string
	AllowedActions []types.ActionKind
	WorkItemID     string
	State          types.ActionWorkItemState
}

// New v2.0 types

type CorpusStats struct {
	TotalPRs       int
	LastSync       time.Time
	SyncJobsActive int
	AuditEntries   int
}

type ExecutorState struct {
	PendingIntents   int
	ClaimedItems     int
	InProgressItems  int
	CompletedItems   int
	FailedItems      int
	ProofBundleCount int
}

type ProofBundleRef struct {
	ID         string
	WorkItemID string
	PRNumber   int
	Summary    string
	CreatedAt  time.Time
}

type AuditLedgerEntry struct {
	Timestamp  time.Time
	Action     string // state transition or audit action
	WorkItemID string
	PRNumber   int
	Reason     string
	Actor      string
}

type AuditLedger struct {
	Entries []AuditLedgerEntry
}

// DataUpdate is a container for all monitor view data.
// It is sent by the broadcaster to all connected clients when state changes.
type DataUpdate struct {
	Timestamp       time.Time
	SyncJobs        []SyncJobView
	RateLimit       RateLimitView
	RecentLogs      []LogEntry
	ActivityBuckets []ActivityBucket
	ActionPlan      *types.ActionPlan `json:"action_plan,omitempty"`

	// New v2.0 fields
	CorpusStats   CorpusStats     `json:"corpus_stats,omitempty"`
	ExecutorState ExecutorState   `json:"executor_state,omitempty"`
	ProofBundles  []ProofBundleRef `json:"proof_bundles,omitempty"`
	AuditLedger   AuditLedger     `json:"audit_ledger,omitempty"`
}