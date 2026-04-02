package ratelimit

import (
	"testing"
)

func TestMetrics_InitialState(t *testing.T) {
	m := NewMetrics()
	snap := m.Snapshot()

	if snap.RequestsTotal != 0 {
		t.Errorf("RequestsTotal initial = %d, want 0", snap.RequestsTotal)
	}
	if snap.RateLimitHits != 0 {
		t.Errorf("RateLimitHits initial = %d, want 0", snap.RateLimitHits)
	}
	if snap.SecondaryLimitHits != 0 {
		t.Errorf("SecondaryLimitHits initial = %d, want 0", snap.SecondaryLimitHits)
	}
	if snap.Retries != 0 {
		t.Errorf("Retries initial = %d, want 0", snap.Retries)
	}
	if snap.BudgetPauses != 0 {
		t.Errorf("BudgetPauses initial = %d, want 0", snap.BudgetPauses)
	}
}

func TestMetrics_Increment(t *testing.T) {
	tests := []struct {
		name     string
		method   func(*Metrics)
		field    string
		expected int64
	}{
		{"IncRequests", func(m *Metrics) { m.IncRequests() }, "RequestsTotal", 1},
		{"IncRateLimitHit", func(m *Metrics) { m.IncRateLimitHit() }, "RateLimitHits", 1},
		{"IncSecondaryLimitHit", func(m *Metrics) { m.IncSecondaryLimitHit() }, "SecondaryLimitHits", 1},
		{"IncRetry", func(m *Metrics) { m.IncRetry() }, "Retries", 1},
		{"IncBudgetPause", func(m *Metrics) { m.IncBudgetPause() }, "BudgetPauses", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMetrics()
			tt.method(m)
			snap := m.Snapshot()

			switch tt.field {
			case "RequestsTotal":
				if snap.RequestsTotal != tt.expected {
					t.Errorf("RequestsTotal = %d, want %d", snap.RequestsTotal, tt.expected)
				}
			case "RateLimitHits":
				if snap.RateLimitHits != tt.expected {
					t.Errorf("RateLimitHits = %d, want %d", snap.RateLimitHits, tt.expected)
				}
			case "SecondaryLimitHits":
				if snap.SecondaryLimitHits != tt.expected {
					t.Errorf("SecondaryLimitHits = %d, want %d", snap.SecondaryLimitHits, tt.expected)
				}
			case "Retries":
				if snap.Retries != tt.expected {
					t.Errorf("Retries = %d, want %d", snap.Retries, tt.expected)
				}
			case "BudgetPauses":
				if snap.BudgetPauses != tt.expected {
					t.Errorf("BudgetPauses = %d, want %d", snap.BudgetPauses, tt.expected)
				}
			}
		})
	}
}

func TestMetrics_MultipleIncrements(t *testing.T) {
	m := NewMetrics()

	m.IncRequests()
	m.IncRequests()
	m.IncRequests()
	m.IncRateLimitHit()
	m.IncSecondaryLimitHit()
	m.IncRetry()
	m.IncRetry()
	m.IncBudgetPause()

	snap := m.Snapshot()

	if snap.RequestsTotal != 3 {
		t.Errorf("RequestsTotal = %d, want 3", snap.RequestsTotal)
	}
	if snap.RateLimitHits != 1 {
		t.Errorf("RateLimitHits = %d, want 1", snap.RateLimitHits)
	}
	if snap.SecondaryLimitHits != 1 {
		t.Errorf("SecondaryLimitHits = %d, want 1", snap.SecondaryLimitHits)
	}
	if snap.Retries != 2 {
		t.Errorf("Retries = %d, want 2", snap.Retries)
	}
	if snap.BudgetPauses != 1 {
		t.Errorf("BudgetPauses = %d, want 1", snap.BudgetPauses)
	}
}

func TestMetrics_SnapshotIsolation(t *testing.T) {
	m := NewMetrics()
	m.IncRequests()
	m.IncRateLimitHit()

	snap1 := m.Snapshot()

	m.IncSecondaryLimitHit()
	m.IncRetry()
	m.IncBudgetPause()

	snap2 := m.Snapshot()

	if snap1.RequestsTotal != snap2.RequestsTotal {
		t.Errorf("Snapshot isolation failed: snap1.RequestsTotal=%d, snap2.RequestsTotal=%d",
			snap1.RequestsTotal, snap2.RequestsTotal)
	}
	if snap1.RateLimitHits != snap2.RateLimitHits {
		t.Errorf("Snapshot isolation failed: snap1.RateLimitHits=%d, snap2.RateLimitHits=%d",
			snap1.RateLimitHits, snap2.RateLimitHits)
	}
	if snap2.SecondaryLimitHits != 1 {
		t.Errorf("SecondaryLimitHits = %d, want 1", snap2.SecondaryLimitHits)
	}
	if snap2.Retries != 1 {
		t.Errorf("Retries = %d, want 1", snap2.Retries)
	}
	if snap2.BudgetPauses != 1 {
		t.Errorf("BudgetPauses = %d, want 1", snap2.BudgetPauses)
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()

	m.IncRequests()
	m.IncRateLimitHit()
	m.IncSecondaryLimitHit()
	m.IncRetry()
	m.IncBudgetPause()

	m.Reset()

	snap := m.Snapshot()

	if snap.RequestsTotal != 0 {
		t.Errorf("After reset, RequestsTotal = %d, want 0", snap.RequestsTotal)
	}
	if snap.RateLimitHits != 0 {
		t.Errorf("After reset, RateLimitHits = %d, want 0", snap.RateLimitHits)
	}
	if snap.SecondaryLimitHits != 0 {
		t.Errorf("After reset, SecondaryLimitHits = %d, want 0", snap.SecondaryLimitHits)
	}
	if snap.Retries != 0 {
		t.Errorf("After reset, Retries = %d, want 0", snap.Retries)
	}
	if snap.BudgetPauses != 0 {
		t.Errorf("After reset, BudgetPauses = %d, want 0", snap.BudgetPauses)
	}
}

func TestMetrics_ResetThenIncrement(t *testing.T) {
	m := NewMetrics()

	m.IncRequests()
	m.IncRateLimitHit()
	m.Reset()

	m.IncSecondaryLimitHit()
	m.IncRetry()
	m.IncBudgetPause()

	snap := m.Snapshot()

	if snap.RequestsTotal != 0 {
		t.Errorf("After reset+increment, RequestsTotal = %d, want 0", snap.RequestsTotal)
	}
	if snap.RateLimitHits != 0 {
		t.Errorf("After reset+increment, RateLimitHits = %d, want 0", snap.RateLimitHits)
	}
	if snap.SecondaryLimitHits != 1 {
		t.Errorf("After reset+increment, SecondaryLimitHits = %d, want 1", snap.SecondaryLimitHits)
	}
	if snap.Retries != 1 {
		t.Errorf("After reset+increment, Retries = %d, want 1", snap.Retries)
	}
	if snap.BudgetPauses != 1 {
		t.Errorf("After reset+increment, BudgetPauses = %d, want 1", snap.BudgetPauses)
	}
}

func TestMetrics_String(t *testing.T) {
	m := NewMetrics()
	m.IncRequests()
	m.IncRateLimitHit()
	m.IncSecondaryLimitHit()
	m.IncRetry()
	m.IncBudgetPause()

	str := m.String()

	if str == "" {
		t.Error("String() returned empty string")
	}

	expected := "Metrics: requests=1 rate_limit_hits=1 secondary_limit_hits=1 retries=1 budget_pauses=1"
	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}

func TestMetrics_StringEmpty(t *testing.T) {
	m := NewMetrics()

	str := m.String()

	expected := "Metrics: requests=0 rate_limit_hits=0 secondary_limit_hits=0 retries=0 budget_pauses=0"
	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}
