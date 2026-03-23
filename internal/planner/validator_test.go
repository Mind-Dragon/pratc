package planner

import (
	"testing"
)

func TestValidatePlanInput_InvalidTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "target zero",
			target:  0,
			wantErr: true,
			errMsg:  "target must be positive",
		},
		{
			name:    "target negative",
			target:  -5,
			wantErr: true,
			errMsg:  "target must be positive",
		},
		{
			name:    "target positive valid",
			target:  20,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewPlanInputValidator()
			err := v.ValidatePlanInput(tt.target, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePlanInput(target=%d) error = %v, wantErr %v", tt.target, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && err.Error() != tt.errMsg {
				t.Errorf("ValidatePlanInput(target=%d) error message = %q, want %q", tt.target, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidatePlanInput_EmptyPool(t *testing.T) {
	// Empty pool should be valid - the planner should handle it gracefully
	v := NewPlanInputValidator()
	err := v.ValidatePlanInput(20, nil)
	if err != nil {
		t.Errorf("ValidatePlanInput with nil pool should not error, got %v", err)
	}

	// Empty slice should also be valid
	emptyPool := []int{}
	err = v.ValidatePlanInput(20, emptyPool)
	if err != nil {
		t.Errorf("ValidatePlanInput with empty pool should not error, got %v", err)
	}
}

func TestPlanInputValidator_Options(t *testing.T) {
	t.Run("custom min target", func(t *testing.T) {
		v := NewPlanInputValidator(WithMinTarget(5))
		err := v.ValidatePlanInput(3, nil)
		if err == nil {
			t.Error("expected error for target below minimum")
		}
	})

	t.Run("custom max target", func(t *testing.T) {
		v := NewPlanInputValidator(WithMaxTarget(50))
		err := v.ValidatePlanInput(100, nil)
		if err == nil {
			t.Error("expected error for target above maximum")
		}
	})
}
