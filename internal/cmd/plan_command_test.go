package cmd

import (
	"strings"
	"testing"
)

// TestTargetRatioUpperBoundValidation tests that --target-ratio values
// greater than 1.0 are rejected as invalid.
func TestTargetRatioUpperBoundValidation(t *testing.T) {
	tests := []struct {
		name        string
		targetRatio float64
		wantErr     bool
		errContains string
	}{
		{
			name:        "target-ratio 1.5 exceeds maximum of 1.0",
			targetRatio: 1.5,
			wantErr:     true,
			errContains: "target-ratio",
		},
		{
			name:        "target-ratio 2.0 exceeds maximum of 1.0",
			targetRatio: 2.0,
			wantErr:     true,
			errContains: "target-ratio",
		},
		{
			name:        "target-ratio at boundary 1.0 is valid",
			targetRatio: 1.0,
			wantErr:     false,
		},
		{
			name:        "target-ratio below 1.0 is valid",
			targetRatio: 0.5,
			wantErr:     false,
		},
		{
			name:        "default target-ratio 0.05 is valid",
			targetRatio: 0.05,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTargetRatio(tt.targetRatio)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for target-ratio %.2f, got nil", tt.targetRatio)
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error for target-ratio %.2f, got %v", tt.targetRatio, err)
				}
			}
		})
	}
}