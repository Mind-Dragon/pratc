// Package ratelimit provides budget tracking and estimation for GitHub API rate limits.
package ratelimit

import (
	"math"
	"time"
)

type EstimateOptions struct {
	FetchFiles   bool
	FetchReviews bool
	FetchCI      bool
	PerPage      int
}

// DefaultEstimateOptions returns the default estimation options.
func DefaultEstimateOptions() EstimateOptions {
	return EstimateOptions{
		FetchFiles:   false,
		FetchReviews: false,
		FetchCI:      false,
		PerPage:      100,
	}
}

func EstimateRequests(totalPRs int, options EstimateOptions) int {
	if totalPRs <= 0 {
		return 0
	}
	if options.PerPage <= 0 {
		options.PerPage = 100
	}

	// PR list pages: ceil(totalPRs / PerPage)
	pageCount := (totalPRs + options.PerPage - 1) / options.PerPage

	total := pageCount

	if options.FetchFiles {
		total += totalPRs
	}
	if options.FetchReviews {
		total += totalPRs
	}
	if options.FetchCI {
		total += totalPRs
	}

	return total
}

func EstimateSyncDuration(requests int, budget *BudgetManager) time.Duration {
	if requests <= 0 {
		return 0
	}

	usablePerHour := 4800
	if budget != nil && budget.Remaining > 0 {
		usablePerHour = budget.Limit - 200
		if usablePerHour <= 0 {
			usablePerHour = 4800
		}
	}

	hours := math.Ceil(float64(requests) / float64(usablePerHour))
	return time.Duration(hours) * time.Hour
}
