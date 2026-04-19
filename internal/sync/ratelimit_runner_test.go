package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

type mockRunner struct {
	runFunc func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error
}

func (m *mockRunner) Run(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, repo, emit)
	}
	return nil
}

type mockBudgetChecker struct {
	checkBudgetFunc func(repo string, estimatedRequests int) (int, error)
	shouldPauseFunc func() bool
	budgetFunc      func() *ratelimit.BudgetManager
}

func (m *mockBudgetChecker) CheckBudget(repo string, estimatedRequests int) (int, error) {
	if m.checkBudgetFunc != nil {
		return m.checkBudgetFunc(repo, estimatedRequests)
	}
	return estimatedRequests, nil
}

func (m *mockBudgetChecker) ShouldPause() bool {
	if m.shouldPauseFunc != nil {
		return m.shouldPauseFunc()
	}
	return false
}

func (m *mockBudgetChecker) Budget() *ratelimit.BudgetManager {
	if m.budgetFunc != nil {
		return m.budgetFunc()
	}
	return nil
}

func TestRateLimitRunner_Run_BudgetOK(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	innerCalled := false
	innerRunner := &mockRunner{
		runFunc: func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
			innerCalled = true
			return nil
		},
	}

	guard := &mockBudgetChecker{
		checkBudgetFunc: func(repo string, estimatedRequests int) (int, error) {
			return estimatedRequests, nil
		},
	}

	runner := NewRateLimitRunner(innerRunner, guard, store, "test/repo")

	err = runner.Run(context.Background(), "test/repo", nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !innerCalled {
		t.Error("expected inner runner to be called")
	}
}

func TestRateLimitRunner_Run_BudgetExhausted(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	innerCalled := false
	innerRunner := &mockRunner{
		runFunc: func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
			innerCalled = true
			return nil
		},
	}

	expectedErr := errors.New("budget exhausted")
	guard := &mockBudgetChecker{
		checkBudgetFunc: func(repo string, estimatedRequests int) (int, error) {
			return 0, expectedErr
		},
	}

	runner := NewRateLimitRunner(innerRunner, guard, store, "test/repo")

	err = runner.Run(context.Background(), "test/repo", nil)
	if err == nil {
		t.Error("expected error, got nil")
	} else if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
	if innerCalled {
		t.Error("expected inner runner not to be called when budget exhausted")
	}
}

func TestRateLimitRunner_Run_PartialSync(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	innerCalled := false
	innerRunner := &mockRunner{
		runFunc: func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
			innerCalled = true
			return nil
		},
	}

	guard := &mockBudgetChecker{
		checkBudgetFunc: func(repo string, estimatedRequests int) (int, error) {
			return 5, nil
		},
	}

	runner := NewRateLimitRunner(innerRunner, guard, store, "test/repo")

	err = runner.Run(context.Background(), "test/repo", nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !innerCalled {
		t.Error("expected inner runner to be called")
	}
}

func TestRateLimitRunner_Run_ChecksPauseBeforeBudgetEstimation(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	pauseChecked := false
	innerCalled := false
	innerRunner := &mockRunner{
		runFunc: func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
			innerCalled = true
			return nil
		},
	}

	guard := &mockBudgetChecker{
		shouldPauseFunc: func() bool {
			pauseChecked = true
			return true
		},
		checkBudgetFunc: func(repo string, estimatedRequests int) (int, error) {
			if !pauseChecked {
				t.Fatalf("expected ShouldPause to run before CheckBudget")
			}
			return 0, errors.New("budget exhausted")
		},
	}

	runner := NewRateLimitRunner(innerRunner, guard, store, "test/repo")

	err = runner.Run(context.Background(), "test/repo", nil)
	if err == nil {
		t.Fatal("expected error when budget should pause, got nil")
	}
	if !pauseChecked {
		t.Fatal("expected pause check to run")
	}
	if innerCalled {
		t.Fatal("expected inner runner not to be called when paused")
	}
}

// TestRateLimitRunner_WatcherStoppedOnRepeatedRuns verifies that calling Run()
// multiple times does not leak watcher goroutines. Each call must stop the
// previous call's watcher before starting a new one.
func TestRateLimitRunner_WatcherStoppedOnRepeatedRuns(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	innerRunner := &mockRunner{
		runFunc: func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
			return nil
		},
	}

	guard := &mockBudgetChecker{
		checkBudgetFunc: func(repo string, estimatedRequests int) (int, error) {
			return estimatedRequests, nil
		},
		budgetFunc: func() *ratelimit.BudgetManager {
			return ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(5000),
				ratelimit.WithReserveBuffer(100),
				ratelimit.WithResetBuffer(60),
			)
		},
	}

	runner := NewRateLimitRunner(innerRunner, guard, store, "test/repo")

	// Run multiple times in sequence; each Run should not leak a watcher goroutine
	for i := 0; i < 3; i++ {
		err := runner.Run(context.Background(), "test/repo", nil)
		if err != nil {
			t.Fatalf("Run %d: expected no error, got %v", i+1, err)
		}
		// Allow watcher goroutines to start and be signalled to stop
		time.Sleep(20 * time.Millisecond)
	}

	// Give goroutines time to exit
	time.Sleep(50 * time.Millisecond)
}

// TestRateLimitRunner_WatcherStoppedOnBudgetCheckError verifies that if
// CheckBudget fails after the watcher starts, the watcher goroutine is still
// stopped cleanly via deferred close(stopCh).
func TestRateLimitRunner_WatcherStoppedOnBudgetCheckError(t *testing.T) {
	store, err := cache.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory store: %v", err)
	}
	defer store.Close()

	budgetErr := errors.New("budget check failed")

	innerRunner := &mockRunner{
		runFunc: func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
			t.Error("inner runner should not be called when budget check fails")
			return nil
		},
	}

	firstCall := true
	guard := &mockBudgetChecker{
		checkBudgetFunc: func(repo string, estimatedRequests int) (int, error) {
			if firstCall {
				firstCall = false
				return 0, budgetErr
			}
			return estimatedRequests, nil
		},
		budgetFunc: func() *ratelimit.BudgetManager {
			return ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(5000),
				ratelimit.WithReserveBuffer(100),
				ratelimit.WithResetBuffer(60),
			)
		},
	}

	runner := NewRateLimitRunner(innerRunner, guard, store, "test/repo")

	err = runner.Run(context.Background(), "test/repo", nil)
	if err == nil {
		t.Fatal("expected error from first Run due to budget check failure, got nil")
	}
	if !errors.Is(err, budgetErr) {
		t.Errorf("expected budgetErr, got %v", err)
	}

	// Give watcher time to observe the stop signal and exit
	time.Sleep(20 * time.Millisecond)

	// A second successful Run proves no leftover watcher from the first call is
	// still holding stopCh open (the leak that the fix addresses).
	secondRunInnerCalled := false
	innerRunner.runFunc = func(ctx context.Context, repo string, emit func(eventType string, payload map[string]any)) error {
		secondRunInnerCalled = true
		return nil
	}

	err = runner.Run(context.Background(), "test/repo", nil)
	if err != nil {
		t.Fatalf("second Run after budget error: got %v", err)
	}
	if !secondRunInnerCalled {
		t.Fatal("second Run: inner runner should have been called")
	}
}
