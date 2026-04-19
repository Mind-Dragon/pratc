package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
)

func TestOpenSyncStoreWithFallbackLogsPrimaryAndFallback(t *testing.T) {
	prevOpen := openCacheStoreFn
	t.Cleanup(func() { openCacheStoreFn = prevOpen })

	var buf bytes.Buffer
	log := logger.NewForTest(&buf, "sync")

	openCacheStoreFn = func(string) (*cache.Store, error) {
		return nil, errors.New("cache open failed")
	}

	store, fellBack, err := openSyncStoreWithFallback("/tmp/missing.db", log)
	if err != nil {
		t.Fatalf("openSyncStoreWithFallback() error = %v", err)
	}
	if store == nil || !fellBack {
		t.Fatalf("expected fallback store, got store=%v fellBack=%v", store, fellBack)
	}
	_ = store.Close()

	out := buf.String()
	if !strings.Contains(out, "opening sync cache store") {
		t.Fatalf("expected opening log, got %s", out)
	}
	if !strings.Contains(out, "cache store unavailable, using ephemeral fallback") {
		t.Fatalf("expected fallback log, got %s", out)
	}
}

func TestAttemptTokenFallbackWithTraceLogsSelectionAndRotation(t *testing.T) {
	prevDiscover := discoverTokensFn
	t.Cleanup(func() { discoverTokensFn = prevDiscover })

	discoverTokensFn = func(context.Context) ([]string, error) {
		return []string{"token-1", "token-2"}, nil
	}

	var buf bytes.Buffer
	log := logger.NewForTest(&buf, "sync")

	var calls int
	err := attemptTokenFallbackWithTrace(context.Background(), log, func(token string) error {
		calls++
		if token == "token-1" {
			return errors.New("403 Forbidden: rate limit")
		}
		if token == "token-2" {
			return nil
		}
		return errors.New("unexpected token")
	})
	if err != nil {
		t.Fatalf("attemptTokenFallbackWithTrace() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 token attempts, got %d", calls)
	}

	out := buf.String()
	for _, want := range []string{
		"token pool discovered",
		"selected GitHub token",
		"GitHub token exhausted, rotating",
		"GitHub token succeeded",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected log %q, got %s", want, out)
		}
	}
}
