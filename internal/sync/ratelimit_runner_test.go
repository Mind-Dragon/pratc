package sync

import (
	"context"
	"errors"
	"testing"

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
