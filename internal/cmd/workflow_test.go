package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIsWorkflowRateLimitPause(t *testing.T) {
	t.Parallel()

	if !isWorkflowRateLimitPause(errors.New("rate limit budget exhausted")) {
		t.Fatal("expected rate limit pause detector to match pause error")
	}
	if isWorkflowRateLimitPause(errors.New("something else")) {
		t.Fatal("expected rate limit pause detector to ignore unrelated errors")
	}
}

func TestDefaultWorkflowOutDirUsesRepoSlug(t *testing.T) {
	t.Parallel()

	out := defaultWorkflowOutDir("owner/repo")
	if !strings.Contains(out, "owner_repo") {
		t.Fatalf("expected output dir to contain repo slug, got %q", out)
	}
}

func TestSleepUntilReturnsForPastTime(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	if err := sleepUntil(ctx, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("sleepUntil returned error for past time: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("sleepUntil should return immediately for past time, took %s", elapsed)
	}
}
