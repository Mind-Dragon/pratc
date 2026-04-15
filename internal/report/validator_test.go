package report

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateInputDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := ValidateInputDir(tmp); err != nil {
		t.Fatalf("ValidateInputDir(%q) = %v, want nil", tmp, err)
	}
	if err := ValidateInputDir(filepath.Join(tmp, "missing")); err == nil {
		t.Fatal("ValidateInputDir on missing dir = nil, want error")
	}
}

func TestValidateRequiredFilesAcceptsAnalyzeAlias(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	for _, name := range []string{"step-3-cluster.json", "step-4-graph.json", "step-5-plan.json"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte("{}"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "analyze.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write analyze alias: %v", err)
	}
	missing := ValidateRequiredFiles(tmp)
	if len(missing) != 0 {
		t.Fatalf("ValidateRequiredFiles() missing = %v, want none", missing)
	}
}

func TestValidateInputReportsMissingFiles(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "step-2-analyze.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write analyze: %v", err)
	}
	if err := ValidateInput(tmp); err == nil {
		t.Fatal("ValidateInput() = nil, want error for missing files")
	}
}
