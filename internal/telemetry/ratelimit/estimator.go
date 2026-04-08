// Package ratelimit provides budget tracking and estimation for GitHub API rate limits.
package ratelimit

import (
	"math"
	"time"
)

// EstimateOptions configures the request estimation.
type EstimateOptions struct {
	FetchFiles   bool
	FetchReviews bool
	FetchCI      bool
	PerPage      int
}

// DefaultEstimateOptions returns the default estimation options.
// By default, only PR list is fetched (no files, reviews, or CI).
func DefaultEstimateOptions() EstimateOptions {
	return EstimateOptions{
		FetchFiles:   false,
		FetchReviews: false,
		FetchCI:      false,
		PerPage:      100,
	}
}

// FullEstimateOptions returns options for full PR enrichment.
// This includes files, reviews, and CI status - approximately 3 requests per PR.
func FullEstimateOptions() EstimateOptions {
	return EstimateOptions{
		FetchFiles:   true,
		FetchReviews: true,
		FetchCI:      true,
		PerPage:      100,
	}
}

// EstimateRequests calculates the total API requests needed for a sync operation.
// Formula:
//   - PR list pages: ceil(totalPRs / PerPage)
//   - If FetchFiles: +totalPRs (one request per PR for files)
//   - If FetchReviews: +totalPRs (one request per PR for reviews)
//   - If FetchCI: +totalPRs (one request per PR for CI status)
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

// EstimateSyncCost estimates the API calls needed to sync N PRs with full enrichment.
// This is a convenience wrapper that assumes ~3 requests per PR (list + files + reviews).
// Formula: ceil(prCount / 100) + prCount * 3
//   - 1 request per 100 PRs for the list
//   - 1 request per PR for files
//   - 1 request per PR for reviews
//   - 1 request per PR for CI (optional, included in full enrichment)
func EstimateSyncCost(prCount int) int {
	if prCount <= 0 {
		return 0
	}
	// Use full enrichment options (~4 requests per PR including CI)
	return EstimateRequests(prCount, FullEstimateOptions())
}

// EstimateSyncDuration calculates how long a sync will take given the request count and budget.
// Assumes 4,800 usable requests per hour (5,000 limit minus 200 reserve buffer).
// If budget is provided and has limited remaining requests, accounts for wait time until reset.
func EstimateSyncDuration(requests int, budget *BudgetManager) time.Duration {
	if requests <= 0 {
		return 0
	}

	usablePerHour := 4800
	if budget != nil && budget.Remaining() > 0 {
		usablePerHour = budget.Limit - 200
		if usablePerHour <= 0 {
			usablePerHour = 4800
		}
	}

	hours := math.Ceil(float64(requests) / float64(usablePerHour))
	return time.Duration(hours) * time.Hour
}

// EstimateCompletionTime calculates the time duration needed to complete remaining work.
// This is a convenience wrapper that first estimates requests from PR count, then calculates duration.
// Uses full enrichment options (~4 requests per PR).
func EstimateCompletionTime(budget *BudgetManager, remainingWork int) time.Duration {
	requests := EstimateSyncCost(remainingWork)
	return EstimateSyncDuration(requests, budget)
}

// EstimateETA calculates the estimated time of arrival (completion) for a sync operation.
// Returns the current time plus the estimated completion duration.
// Parameters:
//   - budget: current rate limit budget (nil uses default assumptions)
//   - totalWork: total number of PRs to process
//   - completedWork: number of PRs already processed
func EstimateETA(budget *BudgetManager, totalWork, completedWork int) time.Time {
	remainingWork := totalWork - completedWork
	if remainingWork <= 0 {
		return time.Now()
	}

	duration := EstimateCompletionTime(budget, remainingWork)
	return time.Now().Add(duration)
}
