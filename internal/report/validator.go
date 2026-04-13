// Package report provides PDF report generation for prATC scalability reports.
package report

import (
	"fmt"
	"os"
	"path/filepath"
)

// RequiredFiles lists the JSON files required to generate a complete PDF report.
var RequiredFiles = []string{
	"step-2-analyze.json",
	"step-3-cluster.json",
	"step-4-graph.json",
	"step-5-plan.json",
}

// ValidateInputDir checks if the input directory exists and is accessible.
// Returns an error if the directory does not exist or is not a directory.
func ValidateInputDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("input directory path is empty")
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("input directory does not exist: %s", dir)
		}
		return fmt.Errorf("failed to access input directory %s: %w", dir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", dir)
	}

	return nil
}

// ValidateRequiredFiles checks if all required JSON files exist in the input directory.
// Returns a list of missing file paths (relative to the input directory).
// Returns an empty slice if all required files are present.
func ValidateRequiredFiles(dir string) []string {
	var missing []string

	for _, file := range RequiredFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if file == "step-2-analyze.json" {
				alt := filepath.Join(dir, "analyze.json")
				if _, altErr := os.Stat(alt); !os.IsNotExist(altErr) {
					continue
				}
			}
			missing = append(missing, file)
		}
	}

	return missing
}

// ValidateInput performs complete input validation:
// 1. Checks directory exists
// 2. Checks all required files are present
// Returns a formatted error message listing all missing files, or nil if validation passes.
func ValidateInput(dir string) error {
	if err := ValidateInputDir(dir); err != nil {
		return err
	}

	missing := ValidateRequiredFiles(dir)
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("missing required input files: %v", missing)
}
