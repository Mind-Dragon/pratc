package settings

import (
	"context"
	"strings"
	"testing"
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
	store, err := Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	return store
}
