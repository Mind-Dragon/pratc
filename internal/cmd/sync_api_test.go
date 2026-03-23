package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/app"
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
