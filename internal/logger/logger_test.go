package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	logger := New("test-component")
	if logger == nil {
		t.Fatal("New() returned nil")
	}
	if logger.component != "test-component" {
		t.Errorf("expected component 'test-component', got '%s'", logger.component)
	}
}

func TestLoggerInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := New("cli")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Logger = logger.Logger.With("component", "cli")

	logger.Info("test message", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["msg"] != "test message" {
		t.Errorf("expected msg 'test message', got '%v'", entry["msg"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("expected level 'INFO', got '%v'", entry["level"])
	}
	if entry["key"] != "value" {
		t.Errorf("expected key 'value', got '%v'", entry["key"])
	}
}

func TestLoggerError(t *testing.T) {
	var buf bytes.Buffer
	logger := New("app")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Error("error occurred", "err", "something failed")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["msg"] != "error occurred" {
		t.Errorf("expected msg 'error occurred', got '%v'", entry["msg"])
	}
	if entry["level"] != "ERROR" {
		t.Errorf("expected level 'ERROR', got '%v'", entry["level"])
	}
	if entry["err"] != "something failed" {
		t.Errorf("expected err 'something failed', got '%v'", entry["err"])
	}
}

func TestFromContextNil(t *testing.T) {
	logger := FromContext(nil)
	if logger == nil {
		t.Fatal("FromContext(nil) returned nil")
	}
}

func TestFromContextWithValues(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-123")
	ctx = ContextWithRepo(ctx, "owner/repo")
	ctx = ContextWithJobID(ctx, "job-456")

	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("FromContext() returned nil")
	}
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := New("github")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.WithRequestID("req-789").Info("test")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["request_id"] != "req-789" {
		t.Errorf("expected request_id 'req-789', got '%v'", entry["request_id"])
	}
}

func TestWithRepo(t *testing.T) {
	var buf bytes.Buffer
	logger := New("cache")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.WithRepo("acme/webapp").Info("test")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["repo"] != "acme/webapp" {
		t.Errorf("expected repo 'acme/webapp', got '%v'", entry["repo"])
	}
}

func TestWithJobID(t *testing.T) {
	var buf bytes.Buffer
	logger := New("sync")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.WithJobID("sync-001").Info("test")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["job_id"] != "sync-001" {
		t.Errorf("expected job_id 'sync-001', got '%v'", entry["job_id"])
	}
}

func TestContextWithRequestID(t *testing.T) {
	ctx := ContextWithRequestID(context.Background(), "test-req-id")
	got := RequestIDFromContext(ctx)
	if got != "test-req-id" {
		t.Errorf("expected 'test-req-id', got '%s'", got)
	}
}

func TestContextWithRequestIDNil(t *testing.T) {
	got := RequestIDFromContext(nil)
	if got != "" {
		t.Errorf("expected empty string for nil context, got '%s'", got)
	}
}

func TestRequestIDFromContextWithNoValue(t *testing.T) {
	ctx := context.Background()
	got := RequestIDFromContext(ctx)
	if got != "" {
		t.Errorf("expected empty string for context without request ID, got '%s'", got)
	}
}

func TestContextWithRepo(t *testing.T) {
	ctx := ContextWithRepo(context.Background(), "test/repo")
	got := RepoFromContext(ctx)
	if got != "test/repo" {
		t.Errorf("expected 'test/repo', got '%s'", got)
	}
}

func TestContextWithRepoNil(t *testing.T) {
	got := RepoFromContext(nil)
	if got != "" {
		t.Errorf("expected empty string for nil context, got '%s'", got)
	}
}

func TestRepoFromContextWithNoValue(t *testing.T) {
	ctx := context.Background()
	got := RepoFromContext(ctx)
	if got != "" {
		t.Errorf("expected empty string for context without repo, got '%s'", got)
	}
}

func TestContextWithJobID(t *testing.T) {
	ctx := ContextWithJobID(context.Background(), "test-job-id")
	got := JobIDFromContext(ctx)
	if got != "test-job-id" {
		t.Errorf("expected 'test-job-id', got '%s'", got)
	}
}

func TestContextWithJobIDNil(t *testing.T) {
	got := JobIDFromContext(nil)
	if got != "" {
		t.Errorf("expected empty string for nil context, got '%s'", got)
	}
}

func TestJobIDFromContextWithNoValue(t *testing.T) {
	ctx := context.Background()
	got := JobIDFromContext(ctx)
	if got != "" {
		t.Errorf("expected empty string for context without job ID, got '%s'", got)
	}
}

func TestContextKeyUniqueness(t *testing.T) {
	// Verify that different context keys don't collide
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-val")
	ctx = ContextWithRepo(ctx, "repo-val")
	ctx = ContextWithJobID(ctx, "job-val")

	reqID := RequestIDFromContext(ctx)
	repo := RepoFromContext(ctx)
	jobID := JobIDFromContext(ctx)

	if reqID != "req-val" {
		t.Errorf("request_id mismatch: expected 'req-val', got '%s'", reqID)
	}
	if repo != "repo-val" {
		t.Errorf("repo mismatch: expected 'repo-val', got '%s'", repo)
	}
	if jobID != "job-val" {
		t.Errorf("job_id mismatch: expected 'job-val', got '%s'", jobID)
	}
}

func TestContextHandlerHandle(t *testing.T) {
	var buf bytes.Buffer
	ctx := ContextWithJobID(ContextWithRepo(ContextWithRequestID(context.Background(), "req-123"), "owner/repo"), "job-456")

	handler := newContextHandler("test", &buf)
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test msg", 0)
	record.Add(string(requestIDKey), "req-abc")

	if err := handler.Handle(ctx, record); err != nil {
		t.Fatalf("Handle() returned error: %v", err)
	}
}

func TestContextHandlerEnabled(t *testing.T) {
	handler := newContextHandler("test", &bytes.Buffer{})
	ctx := context.Background()

	if !handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("Expected Enabled to return true for LevelInfo")
	}
	if handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("Expected Enabled to return false for LevelDebug")
	}
}

func TestContextHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := newContextHandler("test", &buf)
	newHandler := handler.WithAttrs([]slog.Attr{slog.String("extra", "value")})

	if newHandler == nil {
		t.Fatal("WithAttrs returned nil")
	}

	ch, ok := newHandler.(*contextHandler)
	if !ok {
		t.Fatal("WithAttrs did not return *contextHandler")
	}

	if ch.component != "test" {
		t.Errorf("expected component 'test', got '%s'", ch.component)
	}
}

func TestContextHandlerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := newContextHandler("test", &buf)
	newHandler := handler.WithGroup("mygroup")

	if newHandler == nil {
		t.Fatal("WithGroup returned nil")
	}

	ch, ok := newHandler.(*contextHandler)
	if !ok {
		t.Fatal("WithGroup did not return *contextHandler")
	}

	if ch.component != "test" {
		t.Errorf("expected component 'test', got '%s'", ch.component)
	}
}

func TestReplaceAttr(t *testing.T) {
	replacer := replaceAttr("mycomponent")

	// Test replacing empty component
	attr := replacer(nil, slog.String("component", ""))
	if attr.Value.String() != "mycomponent" {
		t.Errorf("expected 'mycomponent', got '%s'", attr.Value.String())
	}

	// Test preserving non-empty component in non-root group
	attr = replacer([]string{"group"}, slog.String("component", "other"))
	if attr.Value.String() != "other" {
		t.Errorf("expected 'other', got '%s'", attr.Value.String())
	}

	// Test passing through other attributes
	attr = replacer(nil, slog.String("other", "value"))
	if attr.Key != "other" {
		t.Errorf("expected 'other', got '%s'", attr.Key)
	}
}

func TestLoggerInfoNoKeyValues(t *testing.T) {
	var buf bytes.Buffer
	logger := New("cli")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Info("simple message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["msg"] != "simple message" {
		t.Errorf("expected msg 'simple message', got '%v'", entry["msg"])
	}
}

func TestLoggerErrorNoKeyValues(t *testing.T) {
	var buf bytes.Buffer
	logger := New("app")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Error("error message")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["msg"] != "error message" {
		t.Errorf("expected msg 'error message', got '%v'", entry["msg"])
	}
	if entry["level"] != "ERROR" {
		t.Errorf("expected level 'ERROR', got '%v'", entry["level"])
	}
}

func TestLoggerMultipleKeyValues(t *testing.T) {
	var buf bytes.Buffer
	logger := New("ml")
	logger.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	logger.Info("multiple values", "count", 42, "name", "test", "flag", true)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if entry["count"].(float64) != 42 {
		t.Errorf("expected count 42, got '%v'", entry["count"])
	}
	if entry["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", entry["name"])
	}
	if entry["flag"] != true {
		t.Errorf("expected flag true, got '%v'", entry["flag"])
	}
}

func TestFromContextWithNoValues(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("FromContext() returned nil for empty context")
	}
}

func TestFromContextWithPartialValues(t *testing.T) {
	// Only request_id
	ctx := ContextWithRequestID(context.Background(), "only-req")
	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("FromContext() returned nil")
	}

	// Only repo
	ctx2 := ContextWithRepo(context.Background(), "only/repo")
	logger2 := FromContext(ctx2)
	if logger2 == nil {
		t.Fatal("FromContext() returned nil")
	}

	// Only job_id
	ctx3 := ContextWithJobID(context.Background(), "only-job")
	logger3 := FromContext(ctx3)
	if logger3 == nil {
		t.Fatal("FromContext() returned nil")
	}
}
