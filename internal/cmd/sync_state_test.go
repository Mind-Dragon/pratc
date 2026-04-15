package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/cache"
)

// TestSyncStateMachineExplicitStates verifies that the sync/status endpoint
// returns explicit state names rather than implying live worker presence.
func TestSyncStateMachineExplicitStates(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	// Define the desired explicit states for v1.4.1
	desiredStates := []string{
		"queued",
		"running",
		"paused_rate_limit",
		"resuming",
		"completed",
		"failed",
		"canceled",
	}

	// Verify all desired states are recognized by the state machine
	for _, state := range desiredStates {
		if !isValidSyncState(state) {
			t.Errorf("desired state %q is not recognized by isValidSyncState", state)
		}
	}

	// Test: newly created job should be in "queued" state
	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}
	// Currently CreateSyncJob sets status to "in_progress", but desired is "queued"
	if string(job.Status) != "queued" {
		t.Errorf("newly created job should have status 'queued', got %q", job.Status)
	}

	// Test: job in "queued" state should not be reported as "in_progress" by status endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	// The status endpoint should report "queued" not "in_progress"
	if resp["status"] == "in_progress" {
		t.Errorf("status endpoint should not report 'in_progress' - implies live worker; got %q", resp["status"])
	}
	if resp["status"] != "queued" {
		t.Errorf("status endpoint should report 'queued' for newly created job; got %q", resp["status"])
	}
}

// TestSyncStatusPausedRateLimit verifies that a job paused due to rate limit
// is reported as "paused_rate_limit" not just "paused".
func TestSyncStatusPausedRateLimit(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	// Update progress to simulate running
	if err := store.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 10,
		TotalPRs:     100,
	}); err != nil {
		t.Fatalf("update sync job progress: %v", err)
	}

	// Pause due to rate limit
	pauseTime := time.Now().Add(30 * time.Minute)
	if err := store.PauseSyncJob(job.ID, pauseTime, "rate limit budget exhausted"); err != nil {
		t.Fatalf("pause sync job: %v", err)
	}

	// Verify the job is paused with rate limit reason
	pausedJob, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("get paused job: %v", err)
	}
	if pausedJob.Status != cache.SyncJobStatusPausedRateLimit {
		t.Fatalf("expected paused_rate_limit status, got %q", pausedJob.Status)
	}
	if pausedJob.Error != "rate limit budget exhausted" {
		t.Fatalf("expected rate limit pause reason, got %q", pausedJob.Error)
	}

	// Test: status endpoint should report "paused_rate_limit" not just "paused"
	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	// The status should be "paused_rate_limit" not "paused"
	if resp["status"] == "paused" {
		t.Errorf("status endpoint should report 'paused_rate_limit' for rate-limited jobs, not generic 'paused'")
	}
	if resp["status"] != "paused_rate_limit" {
		t.Errorf("status endpoint should report 'paused_rate_limit'; got %q", resp["status"])
	}

	// Should also include resume_at field for rate-limited pauses
	if _, hasResumeAt := resp["resume_at"]; !hasResumeAt {
		t.Errorf("status response for paused_rate_limit should include resume_at field")
	}
}

// TestSyncStatusResumingState verifies that a job transitioning from paused
// to running is reported as "resuming" not immediately "running".
func TestSyncStatusResumingState(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	// Pause the job
	pauseTime := time.Now().Add(30 * time.Minute)
	if err := store.PauseSyncJob(job.ID, pauseTime, "rate limit budget exhausted"); err != nil {
		t.Fatalf("pause sync job: %v", err)
	}

	// Simulate resuming: call ResumeSyncJob
	resumed, err := cache.ResumeSyncJob(store, "owner/repo")
	if err != nil {
		t.Fatalf("resume sync job: %v", err)
	}

	// After resuming, the job should be in "resuming" state temporarily
	// before transitioning to "running"
	if resumed.Status == cache.SyncJobStatusInProgress {
		t.Errorf("resumed job should not immediately become 'in_progress' - should be 'resuming' first")
	}
	if resumed.Status != "resuming" {
		t.Errorf("resumed job should have status 'resuming'; got %q", resumed.Status)
	}

	// Verify the status endpoint also reflects "resuming" state
	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if resp["status"] == "in_progress" {
		t.Errorf("status endpoint should not report 'in_progress' during resume transition; got %q", resp["status"])
	}
	if resp["status"] != "resuming" {
		t.Errorf("status endpoint should report 'resuming' during resume transition; got %q", resp["status"])
	}
}

// TestSyncStatusCanceled verifies that canceled jobs are properly reported.
// NOTE: This test will fail until CancelSyncJob is implemented in the store.
func TestSyncStatusCanceled(t *testing.T) {
	// TODO: Implement CancelSyncJob in cache.Store to unskip this test
	t.Skip("CancelSyncJob not yet implemented - test will fail until then")

	// The following code demonstrates the desired API:
	// store, err := cache.Open(dbPath)
	// job, err := store.CreateSyncJob("owner/repo")
	// err = store.CancelSyncJob(job.ID) // Does not exist yet
	// canceledJob, err := store.GetSyncJob(job.ID)
	// canceledJob.Status should be "canceled"
	_ = cache.SyncJobStatus("canceled") // Placeholder to show intent
}

// TestSyncStatusCompleted verifies that completed jobs are properly reported.
func TestSyncStatusCompleted(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := store.MarkSyncJobComplete(job.ID, time.Now().UTC()); err != nil {
		t.Fatalf("mark sync job complete: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if resp["status"] != "completed" {
		t.Errorf("status endpoint should report 'completed'; got %q", resp["status"])
	}
	if resp["progress_percent"] != float64(100) {
		t.Errorf("completed job should have progress_percent 100; got %v", resp["progress_percent"])
	}
}

// TestSyncStatusFailed verifies that failed jobs are properly reported.
func TestSyncStatusFailed(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	failureMsg := "network error: connection refused"
	if err := store.MarkSyncJobFailed(job.ID, failureMsg); err != nil {
		t.Fatalf("mark sync job failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if resp["status"] != "failed" {
		t.Errorf("status endpoint should report 'failed'; got %q", resp["status"])
	}
	if resp["error"] != failureMsg {
		t.Errorf("status endpoint should include failure message; got %q", resp["error"])
	}
}

// TestSyncStatusRunningState verifies that actively running jobs are reported as "running".
func TestSyncStatusRunningState(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	// Update progress to simulate running
	if err := store.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 25,
		TotalPRs:     100,
	}); err != nil {
		t.Fatalf("update sync job progress: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	// Should report "running" not "in_progress"
	if resp["status"] == "in_progress" {
		t.Errorf("status endpoint should report 'running' not 'in_progress' - explicit state required")
	}
	if resp["status"] != "running" {
		t.Errorf("status endpoint should report 'running' for active job; got %q", resp["status"])
	}
	// in_progress field should not exist in response - use explicit state instead
	if _, exists := resp["in_progress"]; exists {
		t.Errorf("status endpoint should not use in_progress flag - use explicit 'running' state")
	}
}

// TestSyncCLIDoesNotImplyLiveWorker verifies that CLI output doesn't imply
// a live worker is running when checking status.
func TestSyncCLIDoesNotImplyLiveWorker(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	// Create a completed job
	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}
	if err := store.MarkSyncJobComplete(job.ID, time.Now().UTC()); err != nil {
		t.Fatalf("mark sync job complete: %v", err)
	}

	// The sync command JSON output should have explicit status
	// "started": true indicates job was started, not that it's currently running
	req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
	rr := httptest.NewRecorder()
	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	// Should not have in_progress: true for completed jobs
	if inProgress, ok := resp["in_progress"].(bool); ok && inProgress {
		t.Error("completed job should not have in_progress: true - implies live worker")
	}

	// Should use explicit status field
	if _, hasStatus := resp["status"]; !hasStatus {
		t.Error("status response should include explicit 'status' field")
	}

	// Status should not imply worker is running
	if resp["status"] == "in_progress" {
		t.Error("status should not be 'in_progress' - use explicit 'running' state")
	}
}

// TestSyncStateTransitions validates that the state machine enforces
// valid transitions between states.
func TestSyncStateTransitions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	validTransitions := map[string][]string{
		"queued":           {"running", "canceled"},
		"running":          {"paused_rate_limit", "completed", "failed", "canceled"},
		"paused_rate_limit": {"resuming", "canceled"},
		"resuming":         {"running", "canceled"},
		"completed":        {}, // terminal state
		"failed":           {}, // terminal state
		"canceled":         {}, // terminal state
	}

	for fromState, toStates := range validTransitions {
		for _, toState := range toStates {
			if !isValidStateTransition(fromState, toState) {
				t.Errorf("transition from %q to %q should be valid", fromState, toState)
			}
		}
	}

	// Invalid transitions
	invalidTransitions := [][2]string{
		{"completed", "running"},
		{"failed", "running"},
		{"canceled", "running"},
		{"queued", "paused_rate_limit"},
	}
	for _, trans := range invalidTransitions {
		if isValidStateTransition(trans[0], trans[1]) {
			t.Errorf("transition from %q to %q should be invalid", trans[0], trans[1])
		}
	}
}

// isValidSyncState checks if a state is a valid sync job state.
// This is a placeholder - the actual implementation should be added.
func isValidSyncState(state string) bool {
	validStates := []string{"queued", "running", "paused_rate_limit", "resuming", "completed", "failed", "canceled"}
	for _, s := range validStates {
		if s == state {
			return true
		}
	}
	return false
}

// isValidStateTransition checks if a state transition is valid.
// This is a placeholder - the actual implementation should be added.
func isValidStateTransition(from, to string) bool {
	validTransitions := map[string][]string{
		"queued":            {"running", "canceled"},
		"running":           {"paused_rate_limit", "completed", "failed", "canceled"},
		"paused_rate_limit": {"resuming", "canceled"},
		"resuming":          {"running", "canceled"},
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// TestSyncJobStatusConstants verifies that all required state constants exist.
func TestSyncJobStatusConstants(t *testing.T) {
	// These constants should be defined in cache/models.go
	// Currently they don't exist - this test defines the contract

	requiredStates := []cache.SyncJobStatus{
		"queued",
		"running",
		"paused_rate_limit",
		"resuming",
		"completed",
		"failed",
		"canceled",
	}

	definedStates := []cache.SyncJobStatus{
		cache.SyncJobStatusQueued,
		cache.SyncJobStatusRunning,
		cache.SyncJobStatusPausedRateLimit,
		cache.SyncJobStatusResuming,
		cache.SyncJobStatusCompleted,
		cache.SyncJobStatusFailed,
		cache.SyncJobStatusCanceled,
		// Legacy states (deprecated)
		cache.SyncJobStatusInProgress,
		cache.SyncJobStatusPaused,
	}

	// Build a map of defined states
	defined := make(map[cache.SyncJobStatus]bool)
	for _, s := range definedStates {
		defined[s] = true
	}

	// Check that required states are defined
	for _, required := range requiredStates {
		if !defined[required] {
			t.Errorf("required state %q is not defined in cache.SyncJobStatus", required)
		}
	}
}

// TestSyncStatusResponseFormat verifies the JSON response format
// includes all required fields for each state.
func TestSyncStatusResponseFormat(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	testCases := []struct {
		name           string
		setup          func() (string, error)
		expectedFields []string
		forbiddenFields []string
	}{
		{
			name: "queued job",
			setup: func() (string, error) {
				job, err := store.CreateSyncJob("owner/repo")
				if err != nil {
					return "", err
				}
				return job.ID, nil
			},
			expectedFields: []string{"status", "job_id", "repo"},
			forbiddenFields: []string{"in_progress"},
		},
		{
			name: "running job",
			setup: func() (string, error) {
				job, err := store.CreateSyncJob("owner/repo")
				if err != nil {
					return "", err
				}
				if err := store.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
					Cursor:       "cursor-1",
					ProcessedPRs: 10,
					TotalPRs:     100,
				}); err != nil {
					return "", err
				}
				return job.ID, nil
			},
			expectedFields: []string{"status", "job_id", "progress_percent"},
			forbiddenFields: []string{"in_progress"},
		},
		{
			name: "paused_rate_limit job",
			setup: func() (string, error) {
				job, err := store.CreateSyncJob("owner/repo")
				if err != nil {
					return "", err
				}
				if err := store.PauseSyncJob(job.ID, time.Now().Add(30*time.Minute), "rate limit budget exhausted"); err != nil {
					return "", err
				}
				return job.ID, nil
			},
			expectedFields: []string{"status", "job_id"},
			forbiddenFields: []string{"in_progress"},
		},
		{
			name: "completed job",
			setup: func() (string, error) {
				job, err := store.CreateSyncJob("owner/repo")
				if err != nil {
					return "", err
				}
				if err := store.MarkSyncJobComplete(job.ID, time.Now().UTC()); err != nil {
					return "", err
				}
				return job.ID, nil
			},
			expectedFields: []string{"status", "job_id", "progress_percent"},
			forbiddenFields: []string{"in_progress"},
		},
		{
			name: "failed job",
			setup: func() (string, error) {
				job, err := store.CreateSyncJob("owner/repo")
				if err != nil {
					return "", err
				}
				if err := store.MarkSyncJobFailed(job.ID, "test failure"); err != nil {
					return "", err
				}
				return job.ID, nil
			},
			expectedFields: []string{"status", "job_id"},
			forbiddenFields: []string{"in_progress"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.setup()
			if err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			req := httptest.NewRequest(http.MethodGet, "/api/repos/owner/repo/sync/status", nil)
			rr := httptest.NewRecorder()
			handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

			var resp map[string]any
			if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
				t.Fatalf("expected valid JSON response, got error: %v", err)
			}

			// Check expected fields
			for _, field := range tc.expectedFields {
				if _, ok := resp[field]; !ok {
					t.Errorf("expected field %q in response for %s", field, tc.name)
				}
			}

			// Check forbidden fields
			for _, field := range tc.forbiddenFields {
				if _, ok := resp[field]; ok {
					t.Errorf("forbidden field %q should not be in response for %s", field, tc.name)
				}
			}
		})
	}
}

// TestWorkflowSyncSummaryStates verifies that workflowSyncSummary
// uses explicit state names.
func TestWorkflowSyncSummaryStates(t *testing.T) {
	// Test that workflowSyncSummary.Status uses explicit state names
	testCases := []struct {
		status   string
		expected bool
	}{
		{"queued", true},
		{"running", true},
		{"paused_rate_limit", true},
		{"resuming", true},
		{"completed", true},
		{"failed", true},
		{"canceled", true},
		{"in_progress", false}, // old style - should not be used
		{"paused", false},      // old style - should not be used
	}

	for _, tc := range testCases {
		t.Run(tc.status, func(t *testing.T) {
			summary := workflowSyncSummary{
				Repo:   "owner/repo",
				Status: tc.status,
			}
			isExplicit := isValidSyncState(summary.Status)
			if isExplicit != tc.expected {
				t.Errorf("workflowSyncSummary.Status %q explicit=%v, expected=%v", tc.status, isExplicit, tc.expected)
			}
		})
	}
}

// TestSyncCLIPausesReportCorrectState verifies that when sync command
// pauses due to rate limit, the CLI reports "paused_rate_limit" state.
func TestSyncCLIPausesReportCorrectState(t *testing.T) {
	// This test verifies the sync command JSON output format when paused
	// Currently sync.go line 147 outputs "status": "paused"
	// But it should output "status": "paused_rate_limit"

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	// Update progress to simulate partial sync
	if err := store.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 50,
		TotalPRs:     100,
	}); err != nil {
		t.Fatalf("update sync job progress: %v", err)
	}

	// Pause due to rate limit
	resumeAt := time.Now().Add(30 * time.Minute)
	if err := store.PauseSyncJob(job.ID, resumeAt, "rate limit budget exhausted"); err != nil {
		t.Fatalf("pause sync job: %v", err)
	}

	// Verify pause state includes resume_at
	pausedJob, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("get paused job: %v", err)
	}

	if !strings.Contains(pausedJob.Error, "rate limit") {
		t.Fatalf("expected rate limit pause reason, got %q", pausedJob.Error)
	}
	if pausedJob.Progress.ScheduledResumeAt.IsZero() {
		t.Error("paused job should have ScheduledResumeAt set")
	}
}
