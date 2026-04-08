package data

import (
	"testing"
	"time"
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
