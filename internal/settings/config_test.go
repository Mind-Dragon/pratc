package settings

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAnalyzerConfig_JSON(t *testing.T) {
	config := AnalyzerConfig{
		Enabled: true,
		Timeout: 5 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.90,
			"overlap":   0.70,
		},
	}

	// Test marshaling
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify snake_case keys
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"enabled"`) {
		t.Errorf("expected snake_case 'enabled' key, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"timeout"`) {
		t.Errorf("expected snake_case 'timeout' key, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"thresholds"`) {
		t.Errorf("expected snake_case 'thresholds' key, got: %s", jsonStr)
	}

	// Test round-trip
	var parsed AnalyzerConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed.Enabled != config.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", parsed.Enabled, config.Enabled)
	}
	if parsed.Timeout != config.Timeout {
		t.Errorf("Timeout mismatch: got %v, want %v", parsed.Timeout, config.Timeout)
	}
	if len(parsed.Thresholds) != len(config.Thresholds) {
		t.Errorf("Thresholds length mismatch: got %d, want %d", len(parsed.Thresholds), len(config.Thresholds))
	}
}

func TestAnalyzerConfig_Defaults(t *testing.T) {
	config := DefaultAnalyzerConfig()

	if !config.Enabled {
		t.Error("expected Enabled to be true by default")
	}
	if config.Timeout != 5*time.Minute {
		t.Errorf("expected Timeout to be 5m by default, got %v", config.Timeout)
	}
	if config.Thresholds["duplicate"] != 0.90 {
		t.Errorf("expected duplicate threshold 0.90, got %v", config.Thresholds["duplicate"])
	}
	if config.Thresholds["overlap"] != 0.70 {
		t.Errorf("expected overlap threshold 0.70, got %v", config.Thresholds["overlap"])
	}
}
