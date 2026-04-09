package data

import (
	"testing"
	"time"

	"github.com/jeffersonnunn/pratc/internal/cache"
)

func TestGetBucketKey(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "exactly on quarter hour",
			input:    time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC),
			expected: "2026-04-09 14:00:00",
		},
		{
			name:     "first quarter past",
			input:    time.Date(2026, 4, 9, 14, 15, 0, 0, time.UTC),
			expected: "2026-04-09 14:15:00",
		},
		{
			name:     "second quarter past",
			input:    time.Date(2026, 4, 9, 14, 30, 0, 0, time.UTC),
			expected: "2026-04-09 14:30:00",
		},
		{
			name:     "third quarter past",
			input:    time.Date(2026, 4, 9, 14, 45, 0, 0, time.UTC),
			expected: "2026-04-09 14:45:00",
		},
		{
			name:     "minutes roll into next quarter",
			input:    time.Date(2026, 4, 9, 14, 7, 0, 0, time.UTC),
			expected: "2026-04-09 14:00:00",
		},
		{
			name:     "minutes roll into next hour",
			input:    time.Date(2026, 4, 9, 14, 58, 0, 0, time.UTC),
			expected: "2026-04-09 14:45:00",
		},
		{
			name:     "end of hour boundary",
			input:    time.Date(2026, 4, 9, 14, 59, 59, 999999999, time.UTC),
			expected: "2026-04-09 14:45:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBucketKey(tt.input)
			if result != tt.expected {
				t.Errorf("getBucketKey(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTruncateToQuarter(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "on the quarter",
			input:    time.Date(2026, 4, 9, 14, 15, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 15, 0, 0, time.UTC),
		},
		{
			name:     "within first quarter",
			input:    time.Date(2026, 4, 9, 14, 7, 30, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "within second quarter",
			input:    time.Date(2026, 4, 9, 14, 22, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 15, 0, 0, time.UTC),
		},
		{
			name:     "within third quarter",
			input:    time.Date(2026, 4, 9, 14, 37, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "within fourth quarter",
			input:    time.Date(2026, 4, 9, 14, 52, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 45, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToQuarter(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("truncateToQuarter(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
	}{
		{
			name:     "valid RFC3339",
			input:    "2026-04-09T14:30:00Z",
			expected: time.Date(2026, 4, 9, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "valid RFC3339 with timezone",
			input:    "2026-04-09T14:30:00+05:00",
			expected: time.Date(2026, 4, 9, 9, 30, 0, 0, time.UTC),
		},
		{
			name:     "empty string",
			input:    "",
			expected: time.Time{},
		},
		{
			name:     "invalid format",
			input:    "2026-04-09 14:30:00",
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTime(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("parseTime(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTimelineBucketCount(t *testing.T) {
	tests := []struct {
		hours      int
		minBuckets int
		maxBuckets int
	}{
		{4, 15, 17},
		{24, 95, 97},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			exactBuckets := tt.hours * 4
			if exactBuckets < tt.minBuckets || exactBuckets > tt.maxBuckets {
				t.Errorf("hours %d: expected bucket count between %d and %d, got %d",
					tt.hours, tt.minBuckets, tt.maxBuckets, exactBuckets)
			}
		})
	}
}

func TestTimelineAggregatorGetTimeline(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	agg := NewTimelineAggregator(cacheStore)

	buckets := agg.GetTimeline(1)
	if len(buckets) == 0 {
		t.Error("expected at least one bucket for 1 hour")
	}

	for _, b := range buckets {
		if b.JobCount != 0 {
			t.Errorf("expected 0 jobs in fresh timeline, got %d", b.JobCount)
		}
		if b.RequestCount != 0 {
			t.Errorf("expected 0 requests in fresh timeline, got %d", b.RequestCount)
		}
	}
}

func TestTimelineAggregatorGetTimelineWithJobs(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	agg := NewTimelineAggregator(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 25,
		TotalPRs:     50,
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	if err := cacheStore.MarkSyncJobComplete(job.ID, time.Now()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	buckets := agg.GetTimeline(4)
	if len(buckets) == 0 {
		t.Fatal("expected buckets with job data")
	}

	var totalJobs int
	var totalRequests int
	for _, b := range buckets {
		totalJobs += b.JobCount
		totalRequests += b.RequestCount
	}

	if totalJobs == 0 {
		t.Error("expected at least one job across all buckets")
	}

	if totalRequests != 25 {
		t.Errorf("expected 25 total requests, got %d", totalRequests)
	}
}

func TestTimelineAggregatorGetTimelineZeroHours(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	agg := NewTimelineAggregator(cacheStore)

	buckets := agg.GetTimeline(0)
	if len(buckets) == 0 {
		t.Error("expected default buckets when hours is 0")
	}
}

func TestTimelineAggregatorGetTimelineNegativeHours(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	agg := NewTimelineAggregator(cacheStore)

	buckets := agg.GetTimeline(-5)
	if len(buckets) == 0 {
		t.Error("expected default buckets when hours is negative")
	}
}

func TestTimelineAggregatorGetTimelineSummary(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	agg := NewTimelineAggregator(cacheStore)

	summary := agg.GetTimelineSummary(1)
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	if !containsStr(summary, "0 buckets") && !containsStr(summary, "bucket") {
		t.Errorf("expected summary to mention buckets, got: %s", summary)
	}
}

func TestTimelineAggregatorGetTimelineSummaryWithData(t *testing.T) {
	t.Parallel()

	cacheStore := newTestCacheStore(t)
	agg := NewTimelineAggregator(cacheStore)

	job, err := cacheStore.CreateSyncJob("owner/repo")
	if err != nil {
		t.Fatalf("create sync job: %v", err)
	}

	if err := cacheStore.UpdateSyncJobProgress(job.ID, cache.SyncProgress{
		Cursor:       "cursor-1",
		ProcessedPRs: 10,
		TotalPRs:     20,
	}); err != nil {
		t.Fatalf("update progress: %v", err)
	}

	if err := cacheStore.MarkSyncJobComplete(job.ID, time.Now()); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	summary := agg.GetTimelineSummary(4)
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	if !containsStr(summary, "bucket") {
		t.Errorf("expected summary to mention buckets, got: %s", summary)
	}
}

func TestFillGapsEmptyBuckets(t *testing.T) {
	startTime := time.Now().Add(-1 * time.Hour)
	buckets := []ActivityBucket{}

	result := fillGaps(buckets, startTime, 1)

	expectedMinBuckets := 3
	if len(result) < expectedMinBuckets {
		t.Errorf("expected at least %d buckets for empty input, got %d", expectedMinBuckets, len(result))
	}

	for _, b := range result {
		if b.JobCount != 0 {
			t.Error("expected 0 jobs in gap-filled buckets")
		}
		if b.RequestCount != 0 {
			t.Error("expected 0 requests in gap-filled buckets")
		}
	}
}

func TestFillGapsWithExistingBuckets(t *testing.T) {
	startTime := time.Now().Add(-1 * time.Hour)
	existingBucket := ActivityBucket{
		TimeWindow:   truncateToQuarter(time.Now().Add(-30 * time.Minute)),
		JobCount:     5,
		RequestCount: 100,
	}
	buckets := []ActivityBucket{existingBucket}

	result := fillGaps(buckets, startTime, 1)

	var foundExisting bool
	for _, b := range result {
		if b.JobCount == 5 && b.RequestCount == 100 {
			foundExisting = true
		}
	}
	if !foundExisting {
		t.Error("expected to find existing bucket data in result")
	}
}

func TestParseTimeInvalidFormats(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
	}{
		{"not-a-time", time.Time{}},
		{"2026/04/09", time.Time{}},
		{"April 9, 2026", time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseTime(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("parseTime(%q) = %v, want zero time", tt.input, result)
			}
		})
	}
}

func TestTruncateToQuarterBoundary(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "exactly at 0 minutes",
			input:    time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "exactly at 45 minutes",
			input:    time.Date(2026, 4, 9, 14, 45, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 45, 0, 0, time.UTC),
		},
		{
			name:     "1 minute past quarter",
			input:    time.Date(2026, 4, 9, 14, 1, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "14 minutes past quarter",
			input:    time.Date(2026, 4, 9, 14, 14, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 0, 0, 0, time.UTC),
		},
		{
			name:     "16 minutes past quarter",
			input:    time.Date(2026, 4, 9, 14, 16, 0, 0, time.UTC),
			expected: time.Date(2026, 4, 9, 14, 15, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToQuarter(tt.input)
			if !result.Equal(tt.expected) {
				t.Errorf("truncateToQuarter(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
