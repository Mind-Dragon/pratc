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

func TestHandleRepoActionStartsSyncJob(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	if resp["in_progress"] != false {
		t.Fatalf("expected in_progress false, got %v", resp["in_progress"])
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

	if resp["status"] != "in_progress" {
		t.Fatalf("expected status in_progress, got %v", resp["status"])
	}
	if resp["in_progress"] != true {
		t.Fatalf("expected in_progress true, got %v", resp["in_progress"])
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
	if resp["in_progress"] != false {
		t.Fatalf("expected in_progress false, got %v", resp["in_progress"])
	}
	if resp["progress_percent"] != float64(100) {
		t.Fatalf("expected progress_percent 100, got %v", resp["progress_percent"])
	}
}
