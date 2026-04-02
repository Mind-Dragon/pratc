package ratelimit

import "testing"

func TestCalculateChunkSize(t *testing.T) {
	tests := []struct {
		name            string
		totalPRs        int
		remainingBudget int
		reserveBuffer   int
		opts            []ChunkOption
		want            int
	}{
		{
			name:            "full budget large repo",
			totalPRs:        6646,
			remainingBudget: 5000,
			reserveBuffer:   200,
			want:            1600, // min(6646, 4800/3)
		},
		{
			name:            "low budget still has chunk",
			totalPRs:        6646,
			remainingBudget: 300,
			reserveBuffer:   200,
			want:            33, // min(6646, 100/3) = 33
		},
		{
			name:            "below reserve returns zero",
			totalPRs:        6646,
			remainingBudget: 100,
			reserveBuffer:   200,
			want:            0,
		},
		{
			name:            "zero PRs returns zero",
			totalPRs:        0,
			remainingBudget: 5000,
			reserveBuffer:   200,
			want:            0,
		},
		{
			name:            "small repo within budget",
			totalPRs:        50,
			remainingBudget: 5000,
			reserveBuffer:   200,
			want:            50, // min(50, 1600) = 50
		},
		{
			name:            "zero budget returns zero",
			totalPRs:        100,
			remainingBudget: 0,
			reserveBuffer:   200,
			want:            0,
		},
		{
			name:            "remaining equals reserve buffer",
			totalPRs:        100,
			remainingBudget: 200,
			reserveBuffer:   200,
			want:            0,
		},
		{
			name:            "exactly at boundary",
			totalPRs:        1000,
			remainingBudget: 500,
			reserveBuffer:   200,
			want:            100, // min(1000, 300/3) = 100
		},
		{
			name:            "custom requests per PR",
			totalPRs:        1000,
			remainingBudget: 5000,
			reserveBuffer:   200,
			opts:            []ChunkOption{WithRequestsPerPR(5)},
			want:            960, // min(1000, 4800/5) = 960
		},
		{
			name:            "custom requests per PR with low budget",
			totalPRs:        1000,
			remainingBudget: 500,
			reserveBuffer:   200,
			opts:            []ChunkOption{WithRequestsPerPR(10)},
			want:            30, // min(1000, 300/10) = 30
		},
		{
			name:            "negative total PRs treated as zero",
			totalPRs:        -10,
			remainingBudget: 5000,
			reserveBuffer:   200,
			want:            0,
		},
		{
			name:            "negative remaining budget treated as zero",
			totalPRs:        100,
			remainingBudget: -100,
			reserveBuffer:   200,
			want:            0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateChunkSize(tt.totalPRs, tt.remainingBudget, tt.reserveBuffer, tt.opts...)
			if got != tt.want {
				t.Errorf("CalculateChunkSize(%d, %d, %d) = %d, want %d",
					tt.totalPRs, tt.remainingBudget, tt.reserveBuffer, got, tt.want)
			}
		})
	}
}

func TestWithRequestsPerPR(t *testing.T) {
	// Test that option is applied correctly
	cfg := defaultChunkConfig()
	WithRequestsPerPR(5)(&cfg)
	if cfg.requestsPerPR != 5 {
		t.Errorf("WithRequestsPerPR(5) failed: got %d, want 5", cfg.requestsPerPR)
	}

	// Test that non-positive values are ignored
	cfg = defaultChunkConfig()
	WithRequestsPerPR(0)(&cfg)
	if cfg.requestsPerPR != 3 {
		t.Errorf("WithRequestsPerPR(0) should not change value, got %d", cfg.requestsPerPR)
	}

	cfg = defaultChunkConfig()
	WithRequestsPerPR(-5)(&cfg)
	if cfg.requestsPerPR != 3 {
		t.Errorf("WithRequestsPerPR(-5) should not change value, got %d", cfg.requestsPerPR)
	}
}
