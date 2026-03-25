package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type runnerFunc func(context.Context, string, func(string, map[string]any)) error

func (r runnerFunc) Run(ctx context.Context, repo string, emit func(string, map[string]any)) error {
	return r(ctx, repo, emit)
}

func TestManagerStreamReplaysBufferedEventsForLateSubscribers(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	manager := NewManager(runnerFunc(func(_ context.Context, repo string, emit func(string, map[string]any)) error {
		emit("progress", map[string]any{
			"repo":        repo,
			"processed":   3,
			"total":       10,
			"eta_seconds": 42,
		})
		close(done)
		return nil
	}))

	if err := manager.Start("octo/repo"); err != nil {
		t.Fatalf("start sync: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for buffered progress event")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/repos/octo/repo/sync/stream", nil)
	ctx, cancel := context.WithTimeout(req.Context(), 50*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	manager.Stream("octo/repo", rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "event: progress") {
		t.Fatalf("expected replayed progress event, got %s", body)
	}
	if !strings.Contains(body, "\"processed\":3") || !strings.Contains(body, "\"total\":10") {
		t.Fatalf("expected replayed progress payload, got %s", body)
	}
}

func TestManagerStartFailsWithoutRunner(t *testing.T) {
	t.Parallel()

	manager := NewManager(nil)
	if err := manager.Start("octo/repo"); err == nil {
		t.Fatal("expected start to fail without a runner")
	} else if !strings.Contains(err.Error(), "runner is required") {
		t.Fatalf("expected runner error, got %v", err)
	}
}
