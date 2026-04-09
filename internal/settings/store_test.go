package settings

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGetReturnsGlobalAndRepoOverrides(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.Set(ctx, ScopeGlobal, "", "duplicate_threshold", 0.9); err != nil {
		t.Fatalf("set global: %v", err)
	}
	if err := store.Set(ctx, ScopeRepo, "octo/repo", "duplicate_threshold", 0.92); err != nil {
		t.Fatalf("set repo override: %v", err)
	}

	settingsMap, err := store.Get(ctx, "octo/repo")
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if settingsMap["duplicate_threshold"] != float64(0.92) {
		t.Fatalf("expected repo override 0.92, got %#v", settingsMap["duplicate_threshold"])
	}
}

func TestSetRejectsInvalidThreshold(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	err := store.Set(context.Background(), ScopeGlobal, "", "duplicate_threshold", 1.5)
	if err == nil {
		t.Fatalf("expected invalid threshold error")
	}
}

func TestSetRejectsInvalidPRWindow(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.Set(ctx, ScopeGlobal, "", "ending_pr_number", 100); err != nil {
		t.Fatalf("set ending pr number: %v", err)
	}
	err := store.Set(ctx, ScopeGlobal, "", "beginning_pr_number", 120)
	if err == nil || !strings.Contains(err.Error(), "beginning_pr_number") {
		t.Fatalf("expected beginning/ending validation error, got: %v", err)
	}
}

func TestSetRejectsUnknownKey(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	err := store.Set(context.Background(), ScopeGlobal, "", "surprise_key", true)
	if err == nil || !strings.Contains(err.Error(), "unknown setting key") {
		t.Fatalf("expected unknown-key validation error, got: %v", err)
	}
}

func TestExportImportYAML(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.Set(ctx, ScopeGlobal, "", "max_prs", 1000); err != nil {
		t.Fatalf("set max_prs: %v", err)
	}

	yamlBytes, err := store.ExportYAML(ctx, ScopeGlobal, "")
	if err != nil {
		t.Fatalf("export yaml: %v", err)
	}
	if !strings.Contains(string(yamlBytes), "max_prs") {
		t.Fatalf("expected max_prs in yaml output, got %s", string(yamlBytes))
	}

	store2 := openTestStore(t)
	defer store2.Close()
	if err := store2.ImportYAML(ctx, ScopeGlobal, "", yamlBytes); err != nil {
		t.Fatalf("import yaml: %v", err)
	}
	out, err := store2.Get(ctx, "")
	if err != nil {
		t.Fatalf("get after import: %v", err)
	}
	if out["max_prs"] != float64(1000) {
		t.Fatalf("expected max_prs=1000, got %#v", out["max_prs"])
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open("file::memory:")
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	return store
}

func TestGetAnalyzerConfig_GlobalScope(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()

	cfg, err := store.GetAnalyzerConfig(ctx, "")
	if err != nil {
		t.Fatalf("get global analyzer config: %v", err)
	}

	if !cfg.Enabled {
		t.Fatalf("expected default Enabled=true, got false")
	}
	if cfg.Timeout != 5*time.Minute {
		t.Fatalf("expected default Timeout=5m, got %v", cfg.Timeout)
	}
	if cfg.Thresholds["duplicate"] != 0.90 {
		t.Fatalf("expected default duplicate threshold 0.90, got %v", cfg.Thresholds["duplicate"])
	}
}

func TestSetAndGetAnalyzerConfig_Global(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()

	customCfg := AnalyzerConfig{
		Enabled: false,
		Timeout: 10 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.95,
			"overlap":   0.75,
		},
	}

	if err := store.SetAnalyzerConfig(ctx, "", customCfg); err != nil {
		t.Fatalf("set global analyzer config: %v", err)
	}

	cfg, err := store.GetAnalyzerConfig(ctx, "")
	if err != nil {
		t.Fatalf("get global analyzer config: %v", err)
	}

	if cfg.Enabled != false {
		t.Fatalf("expected Enabled=false, got true")
	}
	if cfg.Timeout != 10*time.Minute {
		t.Fatalf("expected Timeout=10m, got %v", cfg.Timeout)
	}
	if cfg.Thresholds["duplicate"] != 0.95 {
		t.Fatalf("expected duplicate threshold 0.95, got %v", cfg.Thresholds["duplicate"])
	}
	if cfg.Thresholds["overlap"] != 0.75 {
		t.Fatalf("expected overlap threshold 0.75, got %v", cfg.Thresholds["overlap"])
	}
}

func TestSetAndGetAnalyzerConfig_RepoScope(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()

	globalCfg := AnalyzerConfig{
		Enabled: true,
		Timeout: 5 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.90,
		},
	}
	if err := store.SetAnalyzerConfig(ctx, "", globalCfg); err != nil {
		t.Fatalf("set global analyzer config: %v", err)
	}

	repoCfg := AnalyzerConfig{
		Enabled: false,
		Timeout: 15 * time.Minute,
		Thresholds: map[string]float64{
			"duplicate": 0.85,
			"custom":    0.50,
		},
	}
	if err := store.SetAnalyzerConfig(ctx, "octo/repo", repoCfg); err != nil {
		t.Fatalf("set repo analyzer config: %v", err)
	}

	cfg, err := store.GetAnalyzerConfig(ctx, "octo/repo")
	if err != nil {
		t.Fatalf("get repo analyzer config: %v", err)
	}

	if cfg.Enabled != false {
		t.Fatalf("expected repo Enabled=false to override global, got true")
	}
	if cfg.Timeout != 15*time.Minute {
		t.Fatalf("expected repo Timeout=15m to override global, got %v", cfg.Timeout)
	}
	if cfg.Thresholds["duplicate"] != 0.85 {
		t.Fatalf("expected repo duplicate threshold 0.85 to override global, got %v", cfg.Thresholds["duplicate"])
	}
	if cfg.Thresholds["custom"] != 0.50 {
		t.Fatalf("expected repo custom threshold 0.50, got %v", cfg.Thresholds["custom"])
	}
}

func TestGetAnalyzerConfig_InvalidRepoFormat(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()

	_, err := store.GetAnalyzerConfig(ctx, "invalid-repo-format")
	if err == nil {
		t.Fatalf("expected error for invalid repo format")
	}
	if !strings.Contains(err.Error(), "invalid repo format") {
		t.Fatalf("expected 'invalid repo format' in error, got: %v", err)
	}
}

func TestSetAnalyzerConfig_InvalidRepoFormat(t *testing.T) {
	t.Parallel()
	store := openTestStore(t)
	defer store.Close()

	ctx := context.Background()
	cfg := DefaultAnalyzerConfig()

	err := store.SetAnalyzerConfig(ctx, "invalid", cfg)
	if err == nil {
		t.Fatalf("expected error for invalid repo format")
	}
	if !strings.Contains(err.Error(), "invalid repo format") {
		t.Fatalf("expected 'invalid repo format' in error, got: %v", err)
	}
}
