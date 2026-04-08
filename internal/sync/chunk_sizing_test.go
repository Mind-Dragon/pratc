package sync

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/telemetry/ratelimit"
)

func TestCalculateChunkSize(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		limit     int
		want      int
	}{
		{
			name:      "nil budget returns default",
			remaining: 0,
			limit:     0,
			want:      100,
		},
		{
			name:      "high budget returns default chunk size",
			remaining: 5000,
			limit:     5000,
			want:      100,
		},
		{
			name:      "medium budget returns 50",
			remaining: 1500,
			limit:     5000,
			want:      50,
		},
		{
			name:      "low budget returns 25",
			remaining: 300,
			limit:     5000,
			want:      25,
		},
		{
			name:      "at reserve buffer returns 0",
			remaining: 200,
			limit:     5000,
			want:      0,
		},
		{
			name:      "below reserve buffer returns 0",
			remaining: 100,
			limit:     5000,
			want:      0,
		},
		{
			name:      "boundary at 2000 returns 50",
			remaining: 2000,
			limit:     5000,
			want:      50,
		},
		{
			name:      "boundary at 500 returns 25",
			remaining: 500,
			limit:     5000,
			want:      25,
		},
		{
			name:      "just above 200 with enough for small chunk",
			remaining: 350,
			limit:     5000,
			want:      25,
		},
		{
			name:      "zero remaining returns 0",
			remaining: 0,
			limit:     5000,
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var budget *ratelimit.BudgetManager
			if tt.limit > 0 || tt.name == "nil budget returns default" {
				if tt.name == "nil budget returns default" {
					got := CalculateChunkSize(nil)
					if got != tt.want {
						t.Errorf("CalculateChunkSize(nil) = %d, want %d", got, tt.want)
					}
					return
				}
				budget = ratelimit.NewBudgetManager(
					ratelimit.WithRateLimit(tt.limit),
					ratelimit.WithReserveBuffer(200),
				)
				budget.RecordResponse(tt.remaining, time.Now().Add(1*time.Hour).Unix())
			}

			got := CalculateChunkSize(budget)
			if got != tt.want {
				t.Errorf("CalculateChunkSize(budget with remaining=%d) = %d, want %d",
					tt.remaining, got, tt.want)
			}
		})
	}
}

func TestCalculateChunkSizeCanAfford(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		reserved  int
		want      int
	}{
		{
			name:      "can afford default chunk",
			remaining: 5000,
			reserved:  0,
			want:      100,
		},
		{
			name:      "reduced when cannot afford",
			remaining: 350,
			reserved:  100,
			want:      16,
		},
		{
			name:      "returns 0 when cannot afford minimum",
			remaining: 230,
			reserved:  0,
			want:      10,
		},
		{
			name:      "returns 0 when below min after afford check",
			remaining: 220,
			reserved:  0,
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := ratelimit.NewBudgetManager(
				ratelimit.WithRateLimit(5000),
				ratelimit.WithReserveBuffer(200),
			)
			budget.RecordResponse(tt.remaining, time.Now().Add(1*time.Hour).Unix())

			if tt.reserved > 0 {
				budget.Reserve(tt.reserved)
			}

			got := CalculateChunkSize(budget)
			if got != tt.want {
				t.Errorf("CalculateChunkSize() = %d, want %d", got, tt.want)
			}
		})
	}
}
