package review

import (
	"fmt"
	"sort"

	"github.com/jeffersonnunn/pratc/internal/settings"
)

// AnalyzerRegistry manages registration and retrieval of PR analyzers.
// It supports config-driven enablement where analyzers can be enabled or
// disabled based on the provided AnalyzerConfig.
type AnalyzerRegistry struct {
	analyzers map[string]Analyzer
}

// NewAnalyzerRegistry creates a new empty analyzer registry.
func NewAnalyzerRegistry() *AnalyzerRegistry {
	return &AnalyzerRegistry{
		analyzers: make(map[string]Analyzer),
	}
}

// Register adds an analyzer to the registry with the given name.
// If an analyzer with the same name already exists, it will be replaced.
func (r *AnalyzerRegistry) Register(name string, analyzer Analyzer) {
	r.analyzers[name] = analyzer
}

// Get retrieves an analyzer by name from the registry.
// Returns the analyzer and true if found, or nil and false if not found.
func (r *AnalyzerRegistry) Get(name string) (Analyzer, bool) {
	analyzer, ok := r.analyzers[name]
	return analyzer, ok
}

// GetEnabled returns a list of analyzers that should be enabled based on the
// provided config. Analyzers are enabled if:
// - The global Enabled field is true (default behavior), OR
// - The analyzer name appears in the thresholds map with a non-zero threshold
//
// When config.Enabled is false, only analyzers explicitly listed in thresholds
// with a value > 0 are enabled (opt-in mode).
// When config.Enabled is true, all registered analyzers are enabled by default
// unless explicitly excluded by a zero threshold.
func (r *AnalyzerRegistry) GetEnabled(config settings.AnalyzerConfig) []Analyzer {
	var enabled []Analyzer

	// If config is empty/default, return all analyzers
	if config.Enabled && len(config.Thresholds) == 0 {
		for _, a := range r.analyzers {
			enabled = append(enabled, a)
		}
		return enabled
	}

	// Config-driven enablement
	for name, analyzer := range r.analyzers {
		threshold, hasThreshold := config.Thresholds[name]

		if config.Enabled {
			// Global enabled: include if no threshold (default on) or threshold > 0
			if !hasThreshold || threshold > 0 {
				enabled = append(enabled, analyzer)
			}
		} else {
			// Global disabled: only include if explicitly opt-in via threshold > 0
			if hasThreshold && threshold > 0 {
				enabled = append(enabled, analyzer)
			}
		}
	}

	return enabled
}

// List returns a sorted list of all registered analyzer names.
func (r *AnalyzerRegistry) List() []string {
	names := make([]string, 0, len(r.analyzers))
	for name := range r.analyzers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Count returns the number of registered analyzers.
func (r *AnalyzerRegistry) Count() int {
	return len(r.analyzers)
}

// ValidateConfig validates that all analyzer names referenced in the config
// are actually registered. Returns an error if any referenced analyzer is missing.
func (r *AnalyzerRegistry) ValidateConfig(config settings.AnalyzerConfig) error {
	for name := range config.Thresholds {
		if _, ok := r.analyzers[name]; !ok {
			return fmt.Errorf("config references unknown analyzer: %q", name)
		}
	}
	return nil
}
