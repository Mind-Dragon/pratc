package settings

import (
	"errors"
	"fmt"
	"math"
)

// Sentinel errors for validation wrapping
var (
	ErrNotWholeNumber  = errors.New("value is not a whole number")
	ErrUnsupportedType = errors.New("unsupported type")
)

var allowedKeys = map[string]struct{}{
	"duplicate_threshold": {},
	"overlap_threshold":   {},
	"beginning_pr_number": {},
	"ending_pr_number":    {},
	"max_prs":             {},
	"analyzer_config":     {},
}

// globalOnlyKeys are keys that cannot be set at repo scope.
var globalOnlyKeys = map[string]struct{}{}

func ValidateSettings(values map[string]any) error {
	return ValidateSettingsWithScope(values, ScopeGlobal)
}

func ValidateSettingsWithScope(values map[string]any, scope string) error {
	for key := range values {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("unknown setting key %q", key)
		}
		if scope == ScopeRepo {
			if _, ok := globalOnlyKeys[key]; ok {
				return fmt.Errorf("setting %q is not allowed at repo scope", key)
			}
		}
	}

	for _, key := range []string{"duplicate_threshold", "overlap_threshold"} {
		if v, ok := values[key]; ok {
			f, err := toFloat(v)
			if err != nil {
				return fmt.Errorf("%s must be numeric: %w", key, err)
			}
			if f < 0 || f > 1 {
				return fmt.Errorf("%s must be between 0 and 1", key)
			}
		}
	}

	for _, key := range []string{"beginning_pr_number", "ending_pr_number", "max_prs"} {
		if v, ok := values[key]; ok {
			i, err := toInt(v)
			if err != nil {
				return fmt.Errorf("%s must be an integer: %w", key, err)
			}
			if i < 0 {
				return fmt.Errorf("%s must be non-negative", key)
			}
		}
	}

	b, hasBeginning := values["beginning_pr_number"]
	e, hasEnding := values["ending_pr_number"]
	if hasBeginning && hasEnding {
		beginning, err := toInt(b)
		if err != nil {
			return fmt.Errorf("beginning_pr_number must be an integer: %w", err)
		}
		ending, err := toInt(e)
		if err != nil {
			return fmt.Errorf("ending_pr_number must be an integer: %w", err)
		}
		if beginning > ending {
			return fmt.Errorf("beginning_pr_number must be less than or equal to ending_pr_number")
		}
	}

	return nil
}

func toFloat(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case int32:
		return float64(typed), nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

func toInt(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case float64:
		if math.Mod(typed, 1) != 0 {
			return 0, fmt.Errorf("expected whole number, got %v: %w", typed, ErrNotWholeNumber)
		}
		return int(typed), nil
	case float32:
		if math.Mod(float64(typed), 1) != 0 {
			return 0, fmt.Errorf("expected whole number, got %v: %w", typed, ErrNotWholeNumber)
		}
		return int(typed), nil
	default:
		return 0, fmt.Errorf("unsupported type %T: %w", value, ErrUnsupportedType)
	}
}
