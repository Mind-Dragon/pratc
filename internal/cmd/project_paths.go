package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func projectsRootDir() string {
	if override := strings.TrimSpace(os.Getenv("PRATC_PROJECTS_DIR")); override != "" {
		return override
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		return filepath.Join(cwd, "projects")
	}
	return filepath.Join(".", "projects")
}

func projectSlug(repo string) string {
	slug := strings.NewReplacer("/", "_", string(os.PathSeparator), "_", " ", "_").Replace(strings.TrimSpace(repo))
	if slug == "" {
		return "repo"
	}
	return slug
}

func projectRunDir(repo string, ts time.Time) string {
	return filepath.Join(projectsRootDir(), projectSlug(repo), "runs", ts.UTC().Format("20060102-150405"))
}

func projectRunsDir(repo string) string {
	return filepath.Join(projectsRootDir(), projectSlug(repo), "runs")
}

func writeProjectManifest(dir, repo string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	manifest := fmt.Sprintf("# prATC project run\n\n- Repository: %s\n- Created: %s\n- Purpose: workflow artifacts for a single production run\n\nArtifacts in this directory are treated as persistent documents, not scratch files.\n\nFiles:\n- sync.json\n- analyze.json\n- step-2-analyze.json\n- step-3-cluster.json\n- step-4-graph.json\n- step-5-plan.json\n", repo, time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("write project manifest: %w", err)
	}
	return nil
}
