package ratelimit

import (
	"testing"
	"time"
)

func TestNewBudgetManager(t *testing.T) {
	bm := NewBudgetManager()

	if bm.Limit != 5000 {
		t.Errorf("expected Limit=5000, got %d", bm.Limit)
	}
	if bm.Remaining != 5000 {
		t.Errorf("expected Remaining=5000, got %d", bm.Remaining)
	}
	if bm.ResetTime.Before(time.Now().Add(59 * time.Minute)) {
		t.Errorf("expected ResetTime ~1hr from now, got %v", bm.ResetTime)
	}
}

func TestCanAfford(t *testing.T) {
	tests := []struct {
		name       string
		remaining  int
		requests   int
		wantAfford bool
	}{
		{
			name:       "fresh budget can afford requests",
			remaining:  5000,
			requests:   100,
			wantAfford: true,
		},
		{
			name:       "exact budget at reserve boundary",
			remaining:  200,
			requests:   0,
			wantAfford: true,
		},
		{
			name:       "below reserve boundary",
			remaining:  199,
			requests:   0,
			wantAfford: false,
		},
		{
			name:       "partial usage still affords",
			remaining:  4500,
			requests:   100,
			wantAfford: true,
		},
		{
			name:       "exhausted budget cannot afford",
			remaining:  0,
			requests:   1,
			wantAfford: false,
		},
		{
			name:       "large request with buffer",
			remaining:  300,
			requests:   50,
			wantAfford: true,
		},
		{
			name:       "request exceeds remaining minus buffer",
			remaining:  250,
			requests:   51,
			wantAfford: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := BudgetManager{
				Limit:     5000,
				Remaining: tt.remaining,
				ResetTime: time.Now().Add(1 * time.Hour),
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

	if bm.Remaining != 4500 {
		t.Errorf("expected Remaining=4500, got %d", bm.Remaining)
	}
	if bm.ResetTime.Unix() != resetEpoch {
		t.Errorf("expected ResetTime=%d, got %d", resetEpoch, bm.ResetTime.Unix())
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
			name:      "at reserve returns zero",
			remaining: 200,
			resetIn:   30 * time.Minute,
			wantZero:  true,
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
			name:      "reset in past returns zero",
			remaining: 0,
			resetIn:   -5 * time.Minute,
			wantZero:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bm := BudgetManager{
				Limit:     5000,
				Remaining: tt.remaining,
				ResetTime: time.Now().Add(tt.resetIn),
			}
			got := bm.WaitDuration()
			if tt.wantZero {
				if got != 0 {
					t.Errorf("WaitDuration() = %v, want 0", got)
				}
			} else {
				if got == 0 {
					t.Errorf("WaitDuration() = 0, want non-zero")
				}
				if !approxDuration(got, tt.resetIn, 2*time.Second) {
					t.Errorf("WaitDuration() = %v, want approximately %v", got, tt.resetIn)
				}
			}
		})
	}
}

func TestString(t *testing.T) {
	bm := BudgetManager{
		Limit:       5000,
		Remaining:   4500,
		ResetTime:   time.Now().Add(42*time.Minute + 12*time.Second),
		LastUpdated: time.Now(),
	}

	s := bm.String()

	if s == "" {
		t.Error("String() returned empty string")
	}
}
