package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/jeffersonnunn/pratc/internal/types"
)

type Manifest struct {
	Repo        string `json:"repo"`
	FetchedAt   string `json:"fetched_at"`
	Count       int    `json:"count"`
	PRNumbers   []int  `json:"pr_numbers"`
	Command     string `json:"command"`
	Sanitized   bool   `json:"sanitized"`
	Description string `json:"description"`
}

func LoadFixturePRs() ([]types.PR, error) {
	pattern := filepath.Join(repoRoot(), "fixtures", "prs", "pr-*.json")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob fixtures: %w", err)
	}

	sort.Strings(paths)

	prs := make([]types.PR, 0, len(paths))
	for _, path := range paths {
		pr, err := loadFixtureFromPath(path)
		if err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}

	return prs, nil
}

func LoadFixtureByNumber(number int) (types.PR, error) {
	path := filepath.Join(repoRoot(), "fixtures", "prs", fmt.Sprintf("pr-%d.json", number))
	return loadFixtureFromPath(path)
}

func LoadManifest() (Manifest, error) {
	bytes, err := os.ReadFile(filepath.Join(repoRoot(), "fixtures", "manifest.json"))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}

	return manifest, nil
}

func loadFixtureFromPath(path string) (types.PR, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return types.PR{}, fmt.Errorf("read fixture %s: %w", path, err)
	}

	var pr types.PR
	if err := json.Unmarshal(bytes, &pr); err != nil {
		return types.PR{}, fmt.Errorf("decode fixture %s: %w", path, err)
	}

	return pr, nil
}

func repoRoot() string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}
