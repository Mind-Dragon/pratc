package cmd

import (
	"testing"
)

func TestPreflight_EstimateAPICalls(t *testing.T) {
	tests := []struct {
		name     string
		deltaPRs int
		expected int
	}{
		{"zero delta", 0, 0},
		{"one PR", 1, 3},      // 1*2 + (1+99)/100 = 2 + 1 = 3
		{"100 PRs exactly", 100, 202}, // 100*2 + (100+99)/100 = 200 + 1 = 201... wait let me recalculate
		{"101 PRs", 101, 204},  // 101*2 + (101+99)/100 = 202 + 2 = 204
		{"1000 PRs", 1000, 2010},     // 1000*2 + (1000+99)/100 = 2000 + 10 = 2010
		{"5000 PRs", 5000, 10050},     // 5000*2 + (5000+99)/100 = 10000 + 50 = 10050
	}

	// Recalculate expected values
	// estimateAPICalls = deltaPRs*2 + (deltaPRs+99)/100
	tests[1] = struct{ name string; deltaPRs int; expected int }{"one PR", 1, 1*2 + (1+99)/100}
	tests[2] = struct{ name string; deltaPRs int; expected int }{"100 PRs exactly", 100, 100*2 + (100+99)/100}
	tests[3] = struct{ name string; deltaPRs int; expected int }{"101 PRs", 101, 101*2 + (101+99)/100}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := estimateAPICalls(tc.deltaPRs)
			if result != tc.expected {
				t.Errorf("estimateAPICalls(%d) = %d, want %d", tc.deltaPRs, result, tc.expected)
			}
		})
	}
}

func TestPreflight_FormatNumber(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{100, "100"},
		{999, "999"},
		{1000, "1,000"},
		{1234, "1,234"},
		{9999, "9,999"},
		{10000, "10,000"},
		{123456, "123,456"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := formatNumber(tc.input)
			if result != tc.expected {
				t.Errorf("formatNumber(%d) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestPreflight_GenerateRecommendation(t *testing.T) {
	tests := []struct {
		name           string
		delta          int
		rateLimit      int
		estMinutes     float64
		shouldContain  string
		shouldNotContain string
	}{
		{
			name:          "up to date",
			delta:         0,
			rateLimit:     5000,
			estMinutes:    0,
			shouldContain: "up-to-date",
		},
		{
			name:          "low rate limit",
			delta:         100,
			rateLimit:     100,
			estMinutes:    10,
			shouldContain: "Low rate limit",
		},
		{
			name:          "large delta",
			delta:         2000,
			rateLimit:     5000,
			estMinutes:    10,
			shouldContain: "Large delta",
		},
		{
			name:          "long sync time",
			delta:         100,
			rateLimit:     5000,
			estMinutes:    120,
			shouldContain: "long",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generateRecommendation(tc.delta, tc.rateLimit, tc.estMinutes)
			if tc.shouldContain != "" && !containsString(result, tc.shouldContain) {
				t.Errorf("generateRecommendation(%d, %d, %.0f) = %q, want to contain %q",
					tc.delta, tc.rateLimit, tc.estMinutes, result, tc.shouldContain)
			}
			if tc.shouldNotContain != "" && containsString(result, tc.shouldNotContain) {
				t.Errorf("generateRecommendation(%d, %d, %.0f) = %q, should NOT contain %q",
					tc.delta, tc.rateLimit, tc.estMinutes, result, tc.shouldNotContain)
			}
		})
	}
}

func TestPreflight_Min(t *testing.T) {
	tests := []struct {
		a, b    int
		expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{0, 5, 0},
		{5, 0, 0},
		{-1, 1, -1},
		{1, -1, -1},
	}

	for _, tc := range tests {
		result := min(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("min(%d, %d) = %d, want %d", tc.a, tc.b, result, tc.expected)
		}
	}
}
