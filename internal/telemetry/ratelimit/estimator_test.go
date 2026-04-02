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
			budget: &BudgetManager{
				Limit:     5000,
				Remaining: 5000,
				ResetTime: time.Now().Add(1 * time.Hour),
			},
			want: 5 * time.Hour,
		},
		{
			name:     "low remaining budget",
			requests: 5000,
			budget: &BudgetManager{
				Limit:     5000,
				Remaining: 400,
				ResetTime: time.Now().Add(1 * time.Hour),
			},
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
