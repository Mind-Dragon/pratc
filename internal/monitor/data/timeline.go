// Package data provides data models and storage abstractions for the monitor package.
package data

import (
	"fmt"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

// TimelineAggregator provides historical activity aggregation for sync jobs.
// It reads from sync_jobs and sync_progress tables and aggregates metrics
// into 15-minute buckets for timeline display.
type TimelineAggregator struct {
	store *cache.Store
}

// NewTimelineAggregator creates a new TimelineAggregator backed by the given store.
func NewTimelineAggregator(store *cache.Store) *TimelineAggregator {
	return &TimelineAggregator{store: store}
}

// GetTimeline returns activity buckets for the specified number of hours.
// Each bucket represents a 15-minute window with aggregated metrics.
// Buckets are sorted by TimeWindow ascending.
func (ta *TimelineAggregator) GetTimeline(hours int) []ActivityBucket {
	if hours <= 0 {
		hours = 4 // default to 4 hours
	}

	startTime := time.Now().Add(-time.Duration(hours) * time.Hour)
	buckets := make(map[string]*ActivityBucket)

	// Query sync_jobs for historical data within the time range
	rows, err := ta.store.DB().Query(`
		SELECT
			j.id,
			j.repo,
			j.status,
			j.created_at,
			j.updated_at,
			COALESCE(p.processed_prs, 0) as processed_prs,
			COALESCE(p.total_prs, 0) as total_prs
		FROM sync_jobs j
		LEFT JOIN sync_progress p ON p.repo = j.repo AND p.job_id = j.id
		WHERE j.created_at >= ?
		   OR j.updated_at >= ?
		ORDER BY j.created_at ASC
	`, startTime.Format(time.RFC3339), startTime.Format(time.RFC3339))
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var id, repo, status string
		var createdAtStr, updatedAtStr string
		var processedPRs, totalPRs int

		if err := rows.Scan(&id, &repo, &status, &createdAtStr, &updatedAtStr, &processedPRs, &totalPRs); err != nil {
			continue
		}

		createdAt := parseTime(createdAtStr)
		updatedAt := parseTime(updatedAtStr)

		// Skip invalid timestamps
		if createdAt.IsZero() {
			continue
		}

		// Calculate the 15-minute bucket window
		bucketKey := getBucketKey(createdAt)
		bucketTime, _ := time.Parse("2006-01-02 15:04:00", bucketKey)

		if _, ok := buckets[bucketKey]; !ok {
			buckets[bucketKey] = &ActivityBucket{
				TimeWindow:   bucketTime,
				RequestCount: 0,
				JobCount:     0,
				AvgDuration:  0,
			}
		}

		bucket := buckets[bucketKey]
		bucket.JobCount++
		bucket.RequestCount += processedPRs

		// Calculate duration if we have both timestamps
		if !updatedAt.IsZero() && updatedAt.After(createdAt) {
			duration := updatedAt.Sub(createdAt)
			// Running average calculation
			if bucket.JobCount > 1 {
				prevTotal := bucket.AvgDuration * time.Duration(bucket.JobCount-1)
				bucket.AvgDuration = (prevTotal + duration) / time.Duration(bucket.JobCount)
			} else {
				bucket.AvgDuration = duration
			}
		}
	}

	// Convert map to sorted slice
	result := make([]ActivityBucket, 0, len(buckets))
	for _, bucket := range buckets {
		result = append(result, *bucket)
	}

	// Sort by TimeWindow ascending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].TimeWindow.Before(result[i].TimeWindow) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Ensure we have the expected number of buckets for the requested hours
	// Fill in any gaps with empty buckets
	result = fillGaps(result, startTime, hours)

	return result
}

// getBucketKey returns the 15-minute bucket key for a given time.
// Uses format: "2006-01-02 15:04:00" truncated to 15-minute intervals.
func getBucketKey(t time.Time) string {
	// Truncate to 15-minute interval
	minutes := t.Minute()
	bucketMinute := (minutes / 15) * 15
	truncated := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), bucketMinute, 0, 0, t.Location())
	return truncated.Format("2006-01-02 15:04:00")
}

// fillGaps ensures the result has consecutive 15-minute buckets from startTime to now.
func fillGaps(buckets []ActivityBucket, startTime time.Time, hours int) []ActivityBucket {
	if len(buckets) == 0 {
		// Return empty buckets for the entire range
		result := make([]ActivityBucket, 0, hours*4)
		current := truncateToQuarter(startTime)
		now := time.Now()
		for current.Before(now) {
			result = append(result, ActivityBucket{
				TimeWindow:   current,
				RequestCount: 0,
				JobCount:     0,
				AvgDuration:  0,
			})
			current = current.Add(15 * time.Minute)
		}
		return result
	}

	// Build a map of existing buckets for quick lookup
	existing := make(map[string]ActivityBucket)
	for _, b := range buckets {
		key := b.TimeWindow.Format("2006-01-02 15:04:00")
		existing[key] = b
	}

	// Generate full range
	result := make([]ActivityBucket, 0, hours*4)
	current := truncateToQuarter(startTime)
	now := time.Now()
	for current.Before(now) {
		key := current.Format("2006-01-02 15:04:00")
		if b, ok := existing[key]; ok {
			result = append(result, b)
		} else {
			result = append(result, ActivityBucket{
				TimeWindow:   current,
				RequestCount: 0,
				JobCount:     0,
				AvgDuration:  0,
			})
		}
		current = current.Add(15 * time.Minute)
	}

	return result
}

// truncateToQuarter truncates a time to the nearest 15-minute interval.
func truncateToQuarter(t time.Time) time.Time {
	minutes := t.Minute()
	bucketMinute := (minutes / 15) * 15
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), bucketMinute, 0, 0, t.Location())
}

// parseTime parses a time string in RFC3339 format or returns zero time.
func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// GetTimelineSummary returns a summary of the timeline data.
func (ta *TimelineAggregator) GetTimelineSummary(hours int) string {
	buckets := ta.GetTimeline(hours)
	if len(buckets) == 0 {
		return fmt.Sprintf("No activity in the last %d hours", hours)
	}

	var totalJobs, totalRequests int
	for _, b := range buckets {
		totalJobs += b.JobCount
		totalRequests += b.RequestCount
	}

	return fmt.Sprintf("%d buckets, %d jobs, %d requests over %d hours",
		len(buckets), totalJobs, totalRequests, hours)
}
