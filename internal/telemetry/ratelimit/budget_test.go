package ratelimit

import (
	"strings"
	"testing"
	"time"
)

func TestNewBudgetManager(t *testing.T) {
	bm := NewBudgetManager()

	if bm.Limit != 5000 {
		t.Errorf("expected Limit=5000, got %d", bm.Limit)
	}
	if bm.Remaining() != 5000 {
		t.Errorf("expected Remaining=5000, got %d", bm.Remaining())
	}
	if bm.ResetAt().Before(time.Now().Add(59 * time.Minute)) {
		t.Errorf("expected ResetAt ~1hr from now, got %v", bm.ResetAt())
	}
}

func TestNewBudgetManagerWithOptions(t *testing.T) {
	bm := NewBudgetManager(
		WithRateLimit(10000),
		WithReserveBuffer(500),
		WithResetBuffer(30),
	)

	if bm.Limit != 10000 {
		t.Errorf("expected Limit=10000, got %d", bm.Limit)
	}
	if bm.reserveBuffer != 500 {
		t.Errorf("expected reserveBuffer=500, got %d", bm.reserveBuffer)
	}
	if bm.resetBuffer != 30*time.Second {
		t.Errorf("expected resetBuffer=30s, got %v", bm.resetBuffer)
	}
}

func TestRemaining(t *testing.T) {
	bm := NewBudgetManager()

	if bm.Remaining() != 5000 {
		t.Errorf("expected Remaining=5000, got %d", bm.Remaining())
	}

	bm.RecordResponse(3000, time.Now().Add(30*time.Minute).Unix())
	if bm.Remaining() != 3000 {
		t.Errorf("expected Remaining=3000 after RecordResponse, got %d", bm.Remaining())
	}
}

func TestResetAt(t *testing.T) {
	bm := NewBudgetManager()
	resetTime := time.Now().Add(45 * time.Minute)

	bm.RecordResponse(4000, resetTime.Unix())

	// Compare at second precision since Unix() truncates to seconds
	if bm.ResetAt().Unix() != resetTime.Unix() {
		t.Errorf("expected ResetAt=%d, got %d", resetTime.Unix(), bm.ResetAt().Unix())
	}
}

func TestShouldPause(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		reserved  int
		wantPause bool
	}{
		{
			name:      "plenty of budget",
			remaining: 5000,
			reserved:  0,
			wantPause: false,
		},
		{
			name:      "exactly at reserve boundary",
			remaining: 200,
			reserved:  0,
			wantPause: true,
		},
		{
			name:      "below reserve boundary",
			remaining: 199,
			reserved:  0,
			wantPause: true,
		},
		{
			name:      "with reservations pushing below reserve",
			remaining: 500,
			reserved:  350,
			wantPause: false,
		},
		{
			name:      "with reservations at boundary",
			remaining: 500,
			reserved:  300,
			wantPause: true,
		},
		{
			name:      "exhausted",
			remaining: 0,
			reserved:  0,
			wantPause: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := NewBudgetManager()
			bm.RecordResponse(tt.remaining, time.Now().Add(30*time.Minute).Unix())
			if tt.reserved > 0 {
				bm.Reserve(tt.reserved)
			}

			got := bm.ShouldPause()
			if got != tt.wantPause {
				t.Errorf("ShouldPause() = %v, want %v", got, tt.wantPause)
			}
		})
	}
}

func TestReserve(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		reserve   int
		wantErr   bool
	}{
		{
			name:      "reserve within budget",
			remaining: 5000,
			reserve:   100,
			wantErr:   false,
		},
		{
			name:      "reserve exact available",
			remaining: 4800,
			reserve:   4800,
			wantErr:   false,
		},
		{
			name:      "reserve exceeds available",
			remaining: 5000,
			reserve:   4900,
			wantErr:   true,
		},
		{
			name:      "reserve negative",
			remaining: 5000,
			reserve:   -1,
			wantErr:   true,
		},
		{
			name:      "reserve zero",
			remaining: 5000,
			reserve:   0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := NewBudgetManager()
			bm.RecordResponse(tt.remaining, time.Now().Add(30*time.Minute).Unix())

			err := bm.Reserve(tt.reserve)
			if (err != nil) != tt.wantErr {
				t.Errorf("Reserve(%d) error = %v, wantErr %v", tt.reserve, err, tt.wantErr)
			}

			if err == nil {
				expectedRemaining := tt.remaining - tt.reserve
				if bm.Remaining() != expectedRemaining {
					t.Errorf("after Reserve(%d), Remaining() = %d, want %d", tt.reserve, bm.Remaining(), expectedRemaining)
				}
			}
		})
	}
}

func TestRelease(t *testing.T) {
	bm := NewBudgetManager()
	bm.RecordResponse(1000, time.Now().Add(30*time.Minute).Unix())

	// Reserve some requests
	if err := bm.Reserve(500); err != nil {
		t.Fatalf("Reserve(500) failed: %v", err)
	}

	if bm.Remaining() != 500 {
		t.Errorf("after Reserve(500), Remaining() = %d, want 500", bm.Remaining())
	}

	// Release some back
	bm.Release(200)
	if bm.Remaining() != 700 {
		t.Errorf("after Release(200), Remaining() = %d, want 700", bm.Remaining())
	}

	// Release more than reserved (should cap at reserved amount)
	bm.Release(500)
	if bm.Remaining() != 1000 {
		t.Errorf("after Release(500), Remaining() = %d, want 1000", bm.Remaining())
	}

	// Release negative (should be no-op)
	bm.Release(-10)
	if bm.Remaining() != 1000 {
		t.Errorf("after Release(-10), Remaining() = %d, want 1000", bm.Remaining())
	}
}

func TestEstimatedCompletionTime(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		resetIn   time.Duration
		work      int
		wantMin   time.Duration
		wantMax   time.Duration
	}{
		{
			name:      "no work",
			remaining: 5000,
			resetIn:   30 * time.Minute,
			work:      0,
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "sufficient budget",
			remaining: 5000,
			resetIn:   30 * time.Minute,
			work:      100,
			wantMin:   100 * time.Second,
			wantMax:   100 * time.Second,
		},
		{
			name:      "insufficient budget - need to wait for reset",
			remaining: 100,
			resetIn:   20 * time.Minute,
			work:      500,
			wantMin:   20*time.Minute + 15*time.Second + 500*time.Second - 5*time.Second, // allow 5s tolerance
			wantMax:   21*time.Minute + 15*time.Second + 500*time.Second,
		},
		{
			name:      "reset in past",
			remaining: 0,
			resetIn:   -5 * time.Minute,
			work:      100,
			wantMin:   15*time.Second + 100*time.Second,
			wantMax:   16*time.Second + 100*time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := NewBudgetManager()
			resetTime := time.Now().Add(tt.resetIn)
			bm.RecordResponse(tt.remaining, resetTime.Unix())

			got := bm.EstimatedCompletionTime(tt.work)

			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("EstimatedCompletionTime(%d) = %v, want between %v and %v",
					tt.work, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestCanAfford(t *testing.T) {
	tests := []struct {
		name       string
		remaining  int
		reserved   int
		requests   int
		wantAfford bool
	}{
		{
			name:       "fresh budget can afford requests",
			remaining:  5000,
			reserved:   0,
			requests:   100,
			wantAfford: true,
		},
		{
			name:       "exact budget at reserve boundary",
			remaining:  200,
			reserved:   0,
			requests:   0,
			wantAfford: true,
		},
		{
			name:       "below reserve boundary",
			remaining:  199,
			reserved:   0,
			requests:   0,
			wantAfford: false,
		},
		{
			name:       "partial usage still affords",
			remaining:  4500,
			reserved:   0,
			requests:   100,
			wantAfford: true,
		},
		{
			name:       "exhausted budget cannot afford",
			remaining:  0,
			reserved:   0,
			requests:   1,
			wantAfford: false,
		},
		{
			name:       "large request with buffer",
			remaining:  300,
			reserved:   0,
			requests:   50,
			wantAfford: true,
		},
		{
			name:       "request exceeds remaining minus buffer",
			remaining:  250,
			reserved:   0,
			requests:   51,
			wantAfford: false,
		},
		{
			name:       "with reservations reducing available",
			remaining:  500,
			reserved:   250,
			requests:   50,
			wantAfford: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := NewBudgetManager()
			bm.RecordResponse(tt.remaining, time.Now().Add(1*time.Hour).Unix())
			if tt.reserved > 0 {
				bm.Reserve(tt.reserved)
			}
			got := bm.CanAfford(tt.requests)
			if got != tt.wantAfford {
				t.Errorf("CanAfford(%d) = %v, want %v", tt.requests, got, tt.wantAfford)
			}
		})
	}
}

func TestRecordResponse(t *testing.T) {
	bm := NewBudgetManager()
	now := time.Now()
	resetEpoch := now.Add(30 * time.Minute).Unix()

	bm.RecordResponse(4500, resetEpoch)

	if bm.Remaining() != 4500 {
		t.Errorf("expected Remaining=4500, got %d", bm.Remaining())
	}
	if bm.ResetAt().Unix() != resetEpoch {
		t.Errorf("expected ResetAt=%d, got %d", resetEpoch, bm.ResetAt().Unix())
	}
}

func TestWaitDuration(t *testing.T) {
	approxDuration := func(a, b, tolerance time.Duration) bool {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff <= tolerance
	}

	tests := []struct {
		name      string
		remaining int
		resetIn   time.Duration
		wantZero  bool
	}{
		{
			name:      "above reserve returns zero",
			remaining: 300,
			resetIn:   30 * time.Minute,
			wantZero:  true,
		},
		{
			name:      "at reserve returns positive (includes buffer)",
			remaining: 200,
			resetIn:   30 * time.Minute,
			wantZero:  false,
		},
		{
			name:      "below reserve returns positive wait",
			remaining: 199,
			resetIn:   30 * time.Minute,
			wantZero:  false,
		},
		{
			name:      "exhausted returns wait",
			remaining: 0,
			resetIn:   15 * time.Minute,
			wantZero:  false,
		},
		{
			name:      "reset in past returns buffer only",
			remaining: 0,
			resetIn:   -5 * time.Minute,
			wantZero:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := NewBudgetManager()
			bm.RecordResponse(tt.remaining, time.Now().Add(tt.resetIn).Unix())
			got := bm.WaitDuration()
			if tt.wantZero {
				if got != 0 {
					t.Errorf("WaitDuration() = %v, want 0", got)
				}
			} else {
				if got == 0 {
					t.Errorf("WaitDuration() = 0, want non-zero")
				}
				// Should be approximately resetIn + 15s buffer
				expected := tt.resetIn + 15*time.Second
				if expected < 0 {
					expected = 15 * time.Second
				}
				if !approxDuration(got, expected, 2*time.Second) {
					t.Errorf("WaitDuration() = %v, want approximately %v", got, expected)
				}
			}
		})
	}
}

func TestString(t *testing.T) {
	bm := NewBudgetManager()
	bm.RecordResponse(4500, time.Now().Add(42*time.Minute+12*time.Second).Unix())

	s := bm.String()

	if s == "" {
		t.Error("String() returned empty string")
	}

	if !strings.Contains(s, "4500") {
		t.Errorf("String() should contain remaining count, got: %s", s)
	}

	if !strings.Contains(s, "5000") {
		t.Errorf("String() should contain limit, got: %s", s)
	}
}

func TestConcurrency(t *testing.T) {
	bm := NewBudgetManager()
	bm.RecordResponse(1000, time.Now().Add(30*time.Minute).Unix())

	// Concurrent reserves
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			bm.Reserve(50)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have reserved 500 total
	if bm.Remaining() != 500 {
		t.Errorf("after concurrent reserves, Remaining() = %d, want 500", bm.Remaining())
	}

	// Concurrent releases
	for i := 0; i < 10; i++ {
		go func() {
			bm.Release(50)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have released all
	if bm.Remaining() != 1000 {
		t.Errorf("after concurrent releases, Remaining() = %d, want 1000", bm.Remaining())
	}
}

func TestBudgetManager_WithMetrics(t *testing.T) {
	metrics := NewMetrics()
	bm := NewBudgetManager(WithMetrics(metrics))

	// Initially should not pause (5000 remaining > 200 reserve)
	if bm.ShouldPause() {
		t.Error("ShouldPause() = true, want false with fresh budget")
	}

	// Budget pauses should be 0 (ShouldPause returned false)
	snap := metrics.Snapshot()
	if snap.BudgetPauses != 0 {
		t.Errorf("BudgetPauses = %d, want 0 (ShouldPause was false)", snap.BudgetPauses)
	}

	// Set remaining to exactly at reserve buffer boundary
	bm.RecordResponse(200, time.Now().Add(30*time.Minute).Unix())

	// At boundary, ShouldPause returns true
	if !bm.ShouldPause() {
		t.Error("ShouldPause() = false, want true at reserve boundary")
	}

	// Budget pauses should be 1 (ShouldPause returned true)
	snap = metrics.Snapshot()
	if snap.BudgetPauses != 1 {
		t.Errorf("BudgetPauses = %d, want 1 (ShouldPause was true)", snap.BudgetPauses)
	}

	// Call ShouldPause again - should increment again
	if !bm.ShouldPause() {
		t.Error("ShouldPause() = false, want true at reserve boundary (second call)")
	}

	snap = metrics.Snapshot()
	if snap.BudgetPauses != 2 {
		t.Errorf("BudgetPauses = %d, want 2 (ShouldPause was true twice)", snap.BudgetPauses)
	}
}

func TestBudgetManager_WithMetrics_NoPause(t *testing.T) {
	metrics := NewMetrics()
	bm := NewBudgetManager(WithMetrics(metrics))

	// Set remaining well above reserve
	bm.RecordResponse(1000, time.Now().Add(30*time.Minute).Unix())

	// Should not pause
	if bm.ShouldPause() {
		t.Error("ShouldPause() = true, want false with sufficient budget")
	}

	// Budget pauses should remain 0
	snap := metrics.Snapshot()
	if snap.BudgetPauses != 0 {
		t.Errorf("BudgetPauses = %d, want 0 (ShouldPause never returned true)", snap.BudgetPauses)
	}
}

func TestBudgetManager_NoMetrics(t *testing.T) {
	// BudgetManager without metrics should not panic
	bm := NewBudgetManager()

	// Set remaining below reserve
	bm.RecordResponse(100, time.Now().Add(30*time.Minute).Unix())

	// Should pause without metrics
	if !bm.ShouldPause() {
		t.Error("ShouldPause() = false, want true below reserve")
	}

	// No panic should occur (metrics is nil)
}

// B1: CanAfford boundary inverted at budget.go line 206
// Bug: CanAfford(0) returns false when available == reserveBuffer, but should return true.
// The boundary check at line 206 uses > instead of >=, causing an off-by-one error
// when determining if we can afford zero requests at the exact reserve boundary.
func TestCanAfford_ZeroRequests(t *testing.T) {
	bm := NewBudgetManager()

	// Set remaining to exactly the reserve buffer (200)
	bm.RecordResponse(200, time.Now().Add(30*time.Minute).Unix())

	// CanAfford(0) should return true when available == reserveBuffer
	if !bm.CanAfford(0) {
		t.Errorf("CanAfford(0) = false, want true when available == reserveBuffer (200)")
	}
}

// B4: RecordResponse accepts negative remaining at budget.go line 212-218
// Bug: RecordResponse(-1) stores -1 in RemainingVal instead of clamping to 0.
// Negative remaining values are invalid and should be treated as 0.
func TestRecordResponse_NegativeRemaining(t *testing.T) {
	bm := NewBudgetManager()

	// Record a negative remaining value (simulating malformed API response)
	bm.RecordResponse(-1, time.Now().Add(30*time.Minute).Unix())

	// Remaining should be clamped to 0, not stored as -1
	if bm.Remaining() < 0 {
		t.Errorf("Remaining() = %d, want >= 0 (negative should be clamped)", bm.Remaining())
	}
	if bm.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0 after RecordResponse(-1, ...)", bm.Remaining())
	}
}
