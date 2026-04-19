package sync

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

func TestRateLimitGuardLogsBudgetGatePassed(t *testing.T) {
	t.Parallel()

	tempDir, err := os.MkdirTemp("", "pratc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := cache.Open(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("failed to open cache store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	jobID := "test/repo-" + strconv.FormatInt(now.UnixNano(), 10)
	_, err = store.DB().Exec(`
		INSERT INTO sync_jobs (id, repo, status, error_message, last_sync_at, created_at, updated_at)
		VALUES (?, ?, ?, '', '', ?, ?)
	`, jobID, "test/repo", "in_progress", now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	metrics := ratelimit.NewMetrics()
	budget := ratelimit.NewBudgetManager()
	budget.RecordResponse(500, time.Now().Add(time.Hour).Unix())

	guard := NewRateLimitGuard(budget, metrics, store, jobID)
	var buf bytes.Buffer
	guard.log = logger.NewForTest(&buf, "ratelimit_guard")

	chunkSize, err := guard.CheckBudget("test/repo", 100)
	if err != nil {
		t.Fatalf("CheckBudget() error = %v", err)
	}
	if chunkSize <= 0 {
		t.Fatalf("expected positive chunk size, got %d", chunkSize)
	}
	out := buf.String()
	if !strings.Contains(out, "budget gate evaluated") || !strings.Contains(out, "budget gate passed") {
		t.Fatalf("expected budget pass logs, got %s", out)
	}
}

func TestRateLimitGuardLogsBudgetGatePause(t *testing.T) {
	t.Parallel()

	tempDir, err := os.MkdirTemp("", "pratc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := cache.Open(tempDir + "/test.db")
	if err != nil {
		t.Fatalf("failed to open cache store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	jobID := "test/repo-" + strconv.FormatInt(now.UnixNano(), 10)
	_, err = store.DB().Exec(`
		INSERT INTO sync_jobs (id, repo, status, error_message, last_sync_at, created_at, updated_at)
		VALUES (?, ?, ?, '', '', ?, ?)
	`, jobID, "test/repo", "in_progress", now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("failed to create sync job: %v", err)
	}

	metrics := ratelimit.NewMetrics()
	budget := ratelimit.NewBudgetManager()
	budget.RecordResponse(0, time.Now().Add(time.Hour).Unix())

	guard := NewRateLimitGuard(budget, metrics, store, jobID)
	var buf bytes.Buffer
	guard.log = logger.NewForTest(&buf, "ratelimit_guard")

	chunkSize, err := guard.CheckBudget("test/repo", 100)
	if err == nil {
		t.Fatal("expected CheckBudget to fail for exhausted budget")
	}
	if chunkSize != 0 {
		t.Fatalf("expected zero chunk size, got %d", chunkSize)
	}
	out := buf.String()
	if !strings.Contains(out, "budget gate evaluated") || !strings.Contains(out, "budget gate paused sync") {
		t.Fatalf("expected budget pause logs, got %s", out)
	}
}
