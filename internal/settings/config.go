package settings

import (
	"encoding/json"
	"time"
)

// AnalyzerConfig holds configuration for the PR analyzer.
// It controls analyzer behavior including timeouts and category-specific thresholds.
type AnalyzerConfig struct {
	// Enabled indicates whether the analyzer is active.
	Enabled bool `json:"enabled"`

	// Timeout specifies the maximum duration for analyzer operations.
	Timeout time.Duration `json:"timeout"`

	// Thresholds maps category names to confidence thresholds (0.0-1.0).
	// Categories might include "duplicate", "overlap", "conflict", etc.
	Thresholds map[string]float64 `json:"thresholds"`
}

// DefaultAnalyzerConfig returns a sensible default configuration.
func DefaultAnalyzerConfig() AnalyzerConfig {
	return AnalyzerConfig{
		Enabled: true,
		Timeout: 5 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.90,
			"overlap":   0.70,
		},
	}
}

// MarshalJSON implements custom JSON marshaling to handle duration serialization.
func (c AnalyzerConfig) MarshalJSON() ([]byte, error) {
	type Alias AnalyzerConfig
	return json.Marshal(&struct {
		Timeout string `json:"timeout"`
		*Alias
	}{
		Timeout: c.Timeout.String(),
		Alias:   (*Alias)(&c),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling to handle duration parsing.
func (c *AnalyzerConfig) UnmarshalJSON(data []byte) error {
	type Alias AnalyzerConfig
	aux := &struct {
		Timeout string `json:"timeout"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Timeout != "" {
		d, err := time.ParseDuration(aux.Timeout)
		if err != nil {
			return err
		}
		c.Timeout = d
	}
	return nil
}
