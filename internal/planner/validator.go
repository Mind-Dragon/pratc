package planner

import (
	"errors"
	"fmt"
)

// PlanInputValidator validates input for plan operations.
type PlanInputValidator struct {
	minTarget int
	maxTarget int
}

// ValidatorOption configures a PlanInputValidator.
type ValidatorOption func(*PlanInputValidator)

// WithMinTarget sets the minimum allowed target value.
func WithMinTarget(min int) ValidatorOption {
	return func(v *PlanInputValidator) {
		v.minTarget = min
	}
}

// WithMaxTarget sets the maximum allowed target value.
func WithMaxTarget(max int) ValidatorOption {
	return func(v *PlanInputValidator) {
		v.maxTarget = max
	}
}

// NewPlanInputValidator creates a new validator with optional configuration.
func NewPlanInputValidator(opts ...ValidatorOption) *PlanInputValidator {
	v := &PlanInputValidator{
		minTarget: 1,
		maxTarget: 1000,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// ValidatePlanInput validates the plan input parameters.
func (v *PlanInputValidator) ValidatePlanInput(target int, pool []int) error {
	return validatePlanInput(target, pool, v.minTarget, v.maxTarget)
}

// validatePlanInput validates target and pool parameters.
func validatePlanInput(target int, pool []int, minTarget, maxTarget int) error {
	if target <= 0 {
		return errors.New("target must be positive")
	}
	if target < minTarget {
		return fmt.Errorf("target %d is below minimum %d", target, minTarget)
	}
	if target > maxTarget {
		return fmt.Errorf("target %d exceeds maximum %d", target, maxTarget)
	}
	// Empty pool is valid - will result in empty plan
	_ = pool
	return nil
}

// Error definitions.
var (
	ErrInvalidTarget      = errors.New("invalid target value")
	ErrTargetBelowMinimum = errors.New("target below minimum")
	ErrTargetAboveMaximum = errors.New("target above maximum")
)
