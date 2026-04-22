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

// TestValidateTargetRatio_Negative tests that negative target-ratio values
// are rejected as invalid. A negative ratio is nonsensical for a target.
// This is a contract test: the current implementation only checks ratio > 1.0,
// so negative values pass silently — this test SHOULD FAIL until the bug is fixed.
func TestValidateTargetRatio_Negative(t *testing.T) {
	err := validateTargetRatio(-0.5)
	if err == nil {
		t.Fatalf("expected error for negative target-ratio -0.5, got nil")
	}
	if !strings.Contains(err.Error(), "target-ratio") {
		t.Fatalf("expected error containing 'target-ratio', got %q", err.Error())
	}
}
