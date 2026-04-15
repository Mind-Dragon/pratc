package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/app"
	"github.com/jeffersonnunn/pratc/internal/cache"
	prsync "github.com/jeffersonnunn/pratc/internal/sync"
	"github.com/jeffersonnunn/pratc/internal/testutil"
	_ "modernc.org/sqlite"
)

type fakeRepoSyncAPI struct {
	startedRepo string
	streamRepo  string
	startErr    error
	streamCode  int
	streamBody  string
}

func (f *fakeRepoSyncAPI) Start(repo string) error {
	f.startedRepo = repo
	return f.startErr
}

func (f *fakeRepoSyncAPI) Stream(repo string, w http.ResponseWriter, _ *http.Request) {
	f.streamRepo = repo
	if f.streamCode == 0 {
		f.streamCode = http.StatusOK
	}
	w.WriteHeader(f.streamCode)
	_, _ = w.Write([]byte(f.streamBody))
}

type testMetadataSource struct{ err error }

func (m testMetadataSource) SyncRepo(_ context.Context, _ string, progress func(done, total int), onCursor func(cursor string, processed int)) (prsync.MetadataSnapshot, error) {
	if progress != nil {
		progress(1, 1)
	}
	if m.err != nil {
		return prsync.MetadataSnapshot{}, m.err
	}
	return prsync.MetadataSnapshot{OpenPRs: []int{1}, SyncedPRs: 1}, nil
}

type testMirror struct{}

func (testMirror) FetchAll(_ context.Context, _ []int, _ func(done, total int)) error { return nil }
func (testMirror) FetchAllWithSkipped(_ context.Context, _ []int, _ func(done, total int)) ([]int, error) {
	return nil, nil
}
func (testMirror) PruneClosedPRs(_ context.Context, _ []int) error { return nil }
func (testMirror) Drift(_ context.Context, _ map[int]string) (map[int]string, error) {
	return map[int]string{}, nil
}

func withRepoSyncManager(t *testing.T, factory func(jobDBPath, jobID string) *prsync.Manager) {
	t.Helper()
	original := newRepoSyncManager
	newRepoSyncManager = factory
	t.Cleanup(func() { newRepoSyncManager = original })
}

func TestHandleRepoActionStartsSyncJob(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/repos/octo/repo/sync", nil)
	rr := httptest.NewRecorder()
	syncAPI := &fakeRepoSyncAPI{}

	handleRepoAction(rr, req, app.Service{}, syncAPI)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d body=%s", rr.Code, rr.Body.String())
	}
	if syncAPI.startedRepo != "octo/repo" {
		t.Fatalf("expected sync start for octo/repo, got %q", syncAPI.startedRepo)
	}
	if !strings.Contains(rr.Body.String(), "started") {
		t.Fatalf("expected started response body, got %s", rr.Body.String())
	}
}

func TestHandleRepoActionStreamsSyncEvents(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/repos/octo/repo/sync/stream", nil)
	rr := httptest.NewRecorder()
	syncAPI := &fakeRepoSyncAPI{streamBody: "event: progress\n\ndata: {}\n\n"}

	handleRepoAction(rr, req, app.Service{}, syncAPI)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if syncAPI.streamRepo != "octo/repo" {
		t.Fatalf("expected stream repo octo/repo, got %q", syncAPI.streamRepo)
	}
	if !strings.Contains(rr.Body.String(), "event: progress") {
		t.Fatalf("expected progress event payload, got %s", rr.Body.String())
	}
}

func TestParseRepoActionPathSupportsNestedActions(t *testing.T) {
	repo, action, ok := parseRepoActionPath("/api/repos/octo/repo/sync/stream")
	if !ok {
		t.Fatalf("expected path to parse")
	}
	if repo != "octo/repo" {
		t.Fatalf("expected repo octo/repo, got %q", repo)
	}
	if action != "sync/stream" {
		t.Fatalf("expected nested action sync/stream, got %q", action)
	}
}

func TestHandleRepoActionSyncStatusNever(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	req := httptest.NewRequest(http.MethodGet, "/api/repos/octo/repo/sync/status", nil)
	rr := httptest.NewRecorder()

	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if resp["repo"] != "octo/repo" {
		t.Fatalf("expected repo octo/repo, got %v", resp["repo"])
	}
	if resp["status"] != "never" {
		t.Fatalf("expected status never, got %v", resp["status"])
	}
	// New contract: no in_progress field - use explicit state instead
	if _, hasInProgress := resp["in_progress"]; hasInProgress {
		t.Fatalf("expected no in_progress field in response (use explicit state), got %v", resp["in_progress"])
	}
	if resp["progress_percent"] != float64(0) {
		t.Fatalf("expected progress_percent 0, got %v", resp["progress_percent"])
	}
}

func TestHandleRepoActionSyncStatusInProgress(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("octo/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}
	if err := store.UpdateSyncJobProgress(job.ID, cache.SyncProgress{Cursor: "cursor-1", ProcessedPRs: 3, TotalPRs: 10}); err != nil {
		t.Fatalf("update sync progress: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/octo/repo/sync/status", nil)
	rr := httptest.NewRecorder()

	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	// New contract: explicit "running" state instead of "in_progress"
	if resp["status"] != "running" {
		t.Fatalf("expected status running, got %v", resp["status"])
	}
	// No in_progress field - use explicit state
	if _, hasInProgress := resp["in_progress"]; hasInProgress {
		t.Fatalf("expected no in_progress field in response (use explicit state), got %v", resp["in_progress"])
	}
	if resp["progress_percent"] != float64(30) {
		t.Fatalf("expected progress_percent 30, got %v", resp["progress_percent"])
	}
}

func TestHandleRepoActionSyncStatusCompleted(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	if err := store.SetLastSync("octo/repo", time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)); err != nil {
		t.Fatalf("set last sync: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/octo/repo/sync/status", nil)
	rr := httptest.NewRecorder()

	handleRepoAction(rr, req, app.Service{}, &fakeRepoSyncAPI{})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}

	if resp["status"] != "completed" {
		t.Fatalf("expected status completed, got %v", resp["status"])
	}
	// New contract: no in_progress field - use explicit state instead
	if _, hasInProgress := resp["in_progress"]; hasInProgress {
		t.Fatalf("expected no in_progress field in response (use explicit state), got %v", resp["in_progress"])
	}
	if resp["progress_percent"] != float64(100) {
		t.Fatalf("expected progress_percent 100, got %v", resp["progress_percent"])
	}
}

func TestHandleAnalyzeReturnsImmediateSyncInProgressResponseWhenBackgroundSyncStarts(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)
	t.Setenv("PRATC_CACHE_TTL", "1h")
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	if _, err := store.CreateSyncJob("octo/repo"); err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/octo/repo/analyze", nil)
	rr := httptest.NewRecorder()
	// Use a properly initialized service with cache store so active sync is visible.
	svc := app.NewService(app.Config{CacheStore: store})
	handleAnalyze(rr, req, svc, "octo/repo")

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	// New contract: sync_status is "running" not "in_progress"
	if resp["sync_status"] != "running" {
		t.Fatalf("expected sync_status running, got %v", resp["sync_status"])
	}
	jobID, _ := resp["job_id"].(string)
	if jobID == "" {
		t.Fatalf("expected job_id in response, got %v", resp)
	}
}

func waitForSyncJobStatus(t *testing.T, dbPath, jobID, wantStatus string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		store, err := cache.Open(dbPath)
		if err == nil {
			jobs, listErr := store.ListSyncJobs()
			_ = store.Close()
			if listErr == nil {
				for _, job := range jobs {
					if job.ID == jobID && string(job.Status) == wantStatus {
						return
					}
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("sync job %s did not reach status %s within %s", jobID, wantStatus, timeout)
}

func TestDefaultRunnerMarksFailedJobWhenWorkerErrors(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	job, err := store.CreateSyncJob("octo/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	runner := prsync.NewRunner(prsync.Worker{
		MirrorFactory: func(context.Context, string) (prsync.Mirror, error) { return testMirror{}, nil },
		Metadata:      testMetadataSource{err: fmt.Errorf("boom")},
		Now:           func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}, prsync.NewDBJobRecorder(dbPath), job.ID)

	if err := runner.Run(context.Background(), "octo/repo", nil); err == nil {
		t.Fatal("expected runner error")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	var status, errMsg string
	if err := db.QueryRow(`SELECT status, COALESCE(error_message, '') FROM sync_jobs WHERE repo = ? ORDER BY created_at DESC LIMIT 1`, "octo/repo").Scan(&status, &errMsg); err != nil {
		t.Fatalf("query sync job: %v", err)
	}
	if status != string(cache.SyncJobStatusFailed) {
		t.Fatalf("expected failed job status, got %q", status)
	}
	if !strings.Contains(errMsg, "boom") {
		t.Fatalf("expected stored failure message, got %q", errMsg)
	}
}

func TestHandleAnalyzeWaitsForActiveSyncAndReturnsAnalysisWhenItCompletes(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)
	t.Setenv("PRATC_CACHE_TTL", "1h")

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	service := app.NewService(app.Config{CacheStore: store})

	job, err := store.CreateSyncJob(manifest.Repo)
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}
	if job.Status != cache.SyncJobStatusQueued {
		t.Fatalf("expected sync job to start queued, got %s", job.Status)
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/api/repos/"+manifest.Repo+"/analyze", nil)
	firstRR := httptest.NewRecorder()
	handleAnalyze(firstRR, firstReq, service, manifest.Repo)
	if firstRR.Code != http.StatusAccepted {
		t.Fatalf("expected first request status 202, got %d body=%s", firstRR.Code, firstRR.Body.String())
	}

	resumed, ok, err := store.ResumeSyncJob(manifest.Repo)
	if err != nil {
		t.Fatalf("resume sync job: %v", err)
	}
	if !ok {
		t.Fatal("expected active sync job before completion")
	}
	// Under new explicit-state contract, ResumeSyncJob returns job in queued/running/resuming state
	// A newly created job that hasn't started processing should be in queued state
	if resumed.ID != job.ID {
		t.Fatalf("expected resumed job to match original job ID")
	}
	if resumed.Status != cache.SyncJobStatusQueued && resumed.Status != cache.SyncJobStatusRunning {
		t.Fatalf("expected resumable job in queued or running state, got %s", resumed.Status)
	}

	time.Sleep(50 * time.Millisecond)

	got, err := store.GetSyncJob(job.ID)
	if err != nil {
		t.Fatalf("get sync job: %v", err)
	}
	// Job should remain in queued or running state until completion is explicit
	if got.Status != cache.SyncJobStatusQueued && got.Status != cache.SyncJobStatusRunning {
		t.Fatalf("expected sync job to stay queued or running until completion is explicit, got %s", got.Status)
	}

	if err := store.MarkSyncJobComplete(job.ID, time.Now().UTC()); err != nil {
		t.Fatalf("mark sync job complete: %v", err)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/api/repos/"+manifest.Repo+"/analyze", nil)
	secondRR := httptest.NewRecorder()
	handleAnalyze(secondRR, secondReq, service, manifest.Repo)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("expected second request status 200, got %d body=%s", secondRR.Code, secondRR.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(secondRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if resp["repo"] != manifest.Repo {
		t.Fatalf("expected repo %q, got %v", manifest.Repo, resp["repo"])
	}
	if resp["counts"] == nil {
		t.Fatalf("expected analysis payload, got %v", resp)
	}
}

func TestHandleAnalyzeTimesOutWhileSyncStaysActive(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pratc.db")
	t.Setenv("PRATC_DB_PATH", dbPath)
	t.Setenv("PRATC_CACHE_TTL", "1h")

	manifest, err := testutil.LoadManifest()
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}

	store, err := cache.Open(dbPath)
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer store.Close()

	if _, err := store.CreateSyncJob(manifest.Repo); err != nil {
		t.Fatalf("create active sync job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/"+manifest.Repo+"/analyze", nil)
	rr := httptest.NewRecorder()
	handleAnalyze(rr, req, app.NewService(app.Config{CacheStore: store}), manifest.Repo)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected timeout status 202, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected valid JSON response, got error: %v", err)
	}
	if jobID, _ := resp["job_id"].(string); jobID == "" {
		t.Fatalf("expected job_id in timeout response, got %v", resp)
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "retry after completion") {
		t.Fatalf("expected timeout message, got %v", resp["message"])
	}
}
