package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/logger"
)

func TestOpenSyncStoreWithFallback_UsesEphemeralStoreOnPrimaryFailure(t *testing.T) {
	prev := openCacheStoreFn
	t.Cleanup(func() {
		openCacheStoreFn = prev
	})

	openCacheStoreFn = func(string) (*cache.Store, error) {
		return nil, errors.New("primary cache open failed")
	}

	store, fellBack, err := openSyncStoreWithFallback("/definitely/not/writable/pratc.db", logger.New("test"))
	if err != nil {
		t.Fatalf("openSyncStoreWithFallback() error = %v", err)
	}
	if !fellBack {
		t.Fatal("expected fallback store to be used")
	}
	if store == nil {
		t.Fatal("expected fallback store, got nil")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close fallback store: %v", err)
	}
}

func TestWaitAndRetrySync_ReentersAfterBudgetPause(t *testing.T) {
	ctx := context.Background()
	var pauseCalls int
	var sleepCalls int
	attemptCalls := 0

	err := waitAndRetrySync(
		ctx,
		true,
		func() time.Time { return time.Unix(123, 0) },
		func(time.Time, string) error {
			pauseCalls++
			return nil
		},
		func(context.Context, time.Time) error {
			sleepCalls++
			return nil
		},
		func() error {
			attemptCalls++
			if attemptCalls == 1 {
				return errors.New("rate limit budget exhausted")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("waitAndRetrySync() error = %v", err)
	}
	if attemptCalls != 2 {
		t.Fatalf("waitAndRetrySync() attemptCalls = %d, want 2", attemptCalls)
	}
	if pauseCalls != 1 {
		t.Fatalf("waitAndRetrySync() pauseCalls = %d, want 1", pauseCalls)
	}
	if sleepCalls != 1 {
		t.Fatalf("waitAndRetrySync() sleepCalls = %d, want 1", sleepCalls)
	}
}

func TestWaitAndRetrySync_DoesNotRetryHardFailure(t *testing.T) {
	ctx := context.Background()
	attemptCalls := 0
	want := errors.New("network is down")

	err := waitAndRetrySync(
		ctx,
		true,
		func() time.Time { return time.Unix(123, 0) },
		func(time.Time, string) error {
			t.Fatal("pause callback should not be called for hard failure")
			return nil
		},
		func(context.Context, time.Time) error {
			t.Fatal("sleep callback should not be called for hard failure")
			return nil
		},
		func() error {
			attemptCalls++
			return want
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("waitAndRetrySync() error = %v, want %v", err, want)
	}
	if attemptCalls != 1 {
		t.Fatalf("waitAndRetrySync() attemptCalls = %d, want 1", attemptCalls)
	}
}
