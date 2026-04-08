package ratelimit

import (
	"testing"
	"time"
)

func TestEstimateRequests(t *testing.T) {
	tests := []struct {
		name     string
		totalPRs int
		options  EstimateOptions
		want     int
	}{
		{
			name:     "zero PRs",
			totalPRs: 0,
			options:  DefaultEstimateOptions(),
			want:     0,
		},
		{
			name:     "100 PRs no options",
			totalPRs: 100,
			options:  DefaultEstimateOptions(),
			want:     1,
		},
		{
			name:     "100 PRs with all options",
			totalPRs: 100,
			options: EstimateOptions{
				FetchFiles:   true,
				FetchReviews: true,
				FetchCI:      true,
				PerPage:      100,
			},
			want: 301,
		},
		{
			name:     "1000 PRs no options",
			totalPRs: 1000,
			options:  DefaultEstimateOptions(),
			want:     10,
		},
		{
			name:     "1000 PRs with files only",
			totalPRs: 1000,
			options: EstimateOptions{
				FetchFiles: true,
				PerPage:    100,
			},
			want: 1010,
		},
		{
			name:     "6646 PRs openclaw scale all options",
			totalPRs: 6646,
			options: EstimateOptions{
				FetchFiles:   true,
				FetchReviews: true,
				FetchCI:      true,
				PerPage:      100,
			},
			want: 20005,
		},
		{
			name:     "6646 PRs openclaw scale no options",
			totalPRs: 6646,
			options: EstimateOptions{
				PerPage: 100,
			},
			want: 67,
		},
		{
			name:     "custom per page",
			totalPRs: 250,
			options: EstimateOptions{
				FetchReviews: true,
				PerPage:      50,
			},
			want: 255,
		},
		{
			name:     "negative PRs treated as zero",
			totalPRs: -10,
			options:  DefaultEstimateOptions(),
			want:     0,
		},
		{
			name:     "zero per page defaults to 100",
			totalPRs: 100,
			options: EstimateOptions{
				FetchCI: true,
				PerPage: 0,
			},
			want: 101,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateRequests(tt.totalPRs, tt.options)
			if got != tt.want {
				t.Errorf("EstimateRequests(%d, %+v) = %d, want %d", tt.totalPRs, tt.options, got, tt.want)
			}
		})
	}
}

func TestEstimateSyncCost(t *testing.T) {
	tests := []struct {
		name    string
		prCount int
		want    int
	}{
		{
			name:    "zero PRs",
			prCount: 0,
			want:    0,
		},
		{
			name:    "100 PRs",
			prCount: 100,
			want:    301, // 1 page + 100 files + 100 reviews + 100 CI
		},
		{
			name:    "1000 PRs",
			prCount: 1000,
			want:    3010, // 10 pages + 1000*3
		},
		{
			name:    "6646 PRs openclaw scale",
			prCount: 6646,
			want:    20005, // 67 pages + 6646*3
		},
		{
			name:    "negative PRs treated as zero",
			prCount: -10,
			want:    0,
		},
		{
			name:    "1 PR minimum",
			prCount: 1,
			want:    4, // 1 page + 1 file + 1 review + 1 CI
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateSyncCost(tt.prCount)
			if got != tt.want {
				t.Errorf("EstimateSyncCost(%d) = %d, want %d", tt.prCount, got, tt.want)
			}
		})
	}
}

func TestEstimateCompletionTime(t *testing.T) {
	tests := []struct {
		name          string
		budget        *BudgetManager
		remainingWork int
		want          time.Duration
	}{
		{
			name:          "zero remaining work",
			budget:        nil,
			remainingWork: 0,
			want:          0,
		},
		{
			name:          "100 PRs with default budget",
			budget:        nil,
			remainingWork: 100,
			want:          1 * time.Hour, // 301 requests / 4800 = 1 hour
		},
		{
			name:          "6646 PRs openclaw scale",
			budget:        nil,
			remainingWork: 6646,
			want:          5 * time.Hour, // 20005 requests / 4800 = 5 hours
		},
		{
			name:          "with fresh budget",
			budget:        NewBudgetManager(),
			remainingWork: 1000,
			want:          1 * time.Hour, // 3010 requests / 4800 = 1 hour
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateCompletionTime(tt.budget, tt.remainingWork)
			if got != tt.want {
				t.Errorf("EstimateCompletionTime(%+v, %d) = %v, want %v", tt.budget, tt.remainingWork, got, tt.want)
			}
		})
	}
}

func TestEstimateETA(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		budget        *BudgetManager
		totalWork     int
		completedWork int
	}{
		{
			name:          "all work completed",
			budget:        nil,
			totalWork:     100,
			completedWork: 100,
		},
		{
			name:          "no work completed",
			budget:        nil,
			totalWork:     100,
			completedWork: 0,
		},
		{
			name:          "half completed",
			budget:        nil,
			totalWork:     1000,
			completedWork: 500,
		},
		{
			name:          "openclaw scale no progress",
			budget:        nil,
			totalWork:     6646,
			completedWork: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateETA(tt.budget, tt.totalWork, tt.completedWork)

			// ETA should be in the future (or now if complete)
			if got.Before(now) && tt.completedWork < tt.totalWork {
				t.Errorf("EstimateETA() = %v, should be in the future", got)
			}

			// If all work is done, ETA should be now or very close
			if tt.completedWork >= tt.totalWork {
				diff := got.Sub(now)
				if diff > time.Second {
					t.Errorf("EstimateETA() with complete work = %v, should be close to now (diff: %v)", got, diff)
				}
			}
		})
	}
}

func TestEstimateSyncDuration(t *testing.T) {
	tests := []struct {
		name     string
		requests int
		budget   *BudgetManager
		want     time.Duration
	}{
		{
			name:     "zero requests",
			requests: 0,
			budget:   nil,
			want:     0,
		},
		{
			name:     "100 requests with default budget",
			requests: 100,
			budget:   nil,
			want:     1 * time.Hour,
		},
		{
			name:     "4800 requests exact capacity",
			requests: 4800,
			budget:   nil,
			want:     1 * time.Hour,
		},
		{
			name:     "4801 requests needs 2 hours",
			requests: 4801,
			budget:   nil,
			want:     2 * time.Hour,
		},
		{
			name:     "20005 requests openclaw scale",
			requests: 20005,
			budget:   nil,
			want:     5 * time.Hour,
		},
		{
			name:     "20005 requests with fresh budget",
			requests: 20005,
			budget:   NewBudgetManager(),
			want:     5 * time.Hour,
		},
		{
			name:     "low remaining budget",
			requests: 5000,
			budget: func() *BudgetManager {
				bm := NewBudgetManager()
				bm.RecordResponse(400, time.Now().Add(1*time.Hour).Unix())
				return bm
			}(),
			want: 2 * time.Hour,
		},
		{
			name:     "nil budget uses worst case",
			requests: 9600,
			budget:   nil,
			want:     2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateSyncDuration(tt.requests, tt.budget)
			if got != tt.want {
				t.Errorf("EstimateSyncDuration(%d, %+v) = %v, want %v", tt.requests, tt.budget, got, tt.want)
			}
		})
	}
}

func TestEstimateRequestsAndDuration_Integration(t *testing.T) {
	prs := 6646
	options := EstimateOptions{
		FetchFiles:   true,
		FetchReviews: true,
		FetchCI:      true,
		PerPage:      100,
	}

	requests := EstimateRequests(prs, options)
	duration := EstimateSyncDuration(requests, nil)

	if requests < 20000 {
		t.Errorf("Expected ~20005 requests for openclaw scale, got %d", requests)
	}

	if duration != 5*time.Hour {
		t.Errorf("Expected 5h for %d requests, got %v", requests, duration)
	}
}

func TestFullEstimateOptions(t *testing.T) {
	opts := FullEstimateOptions()

	if !opts.FetchFiles {
		t.Error("FullEstimateOptions should have FetchFiles=true")
	}
	if !opts.FetchReviews {
		t.Error("FullEstimateOptions should have FetchReviews=true")
	}
	if !opts.FetchCI {
		t.Error("FullEstimateOptions should have FetchCI=true")
	}
	if opts.PerPage != 100 {
		t.Errorf("FullEstimateOptions PerPage = %d, want 100", opts.PerPage)
	}
}
