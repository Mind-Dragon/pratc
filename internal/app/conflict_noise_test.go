package app

import "testing"

// =============================================================================
// filterNoiseFiles tests
// =============================================================================

func TestFilterNoiseFiles_PackageJSON(t *testing.T) {
	t.Parallel()
	files := []string{"package.json"}
	got := filterNoiseFiles(files)
	if len(got) != 0 {
		t.Errorf("package.json should be filtered, got %v", got)
	}
}

func TestFilterNoiseFiles_LockFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file string
	}{
		{"pnpm-lock.yaml", "pnpm-lock.yaml"},
		{"yarn.lock", "yarn.lock"},
		{"package-lock.json", "package-lock.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterNoiseFiles([]string{tt.file})
			if len(got) != 0 {
				t.Errorf("%s should be filtered, got %v", tt.file, got)
			}
		})
	}
}

func TestFilterNoiseFiles_GithubConfig(t *testing.T) {
	t.Parallel()
	files := []string{".github/workflows/ci.yml"}
	got := filterNoiseFiles(files)
	if len(got) != 0 {
		t.Errorf(".github/workflows/ci.yml should be filtered, got %v", got)
	}
}

func TestFilterNoiseFiles_SourceCode(t *testing.T) {
	t.Parallel()
	files := []string{"src/main.go"}
	got := filterNoiseFiles(files)
	if len(got) != 1 || got[0] != "src/main.go" {
		t.Errorf("src/main.go should NOT be filtered, got %v", got)
	}
}

func TestFilterNoiseFiles_TestFiles(t *testing.T) {
	t.Parallel()
	files := []string{"internal/app/service_test.go"}
	got := filterNoiseFiles(files)
	if len(got) != 1 || got[0] != "internal/app/service_test.go" {
		t.Errorf("internal/app/service_test.go should NOT be filtered, got %v", got)
	}
}

func TestFilterNoiseFiles_MixedBatch(t *testing.T) {
	t.Parallel()
	files := []string{
		"package.json",
		"pnpm-lock.yaml",
		"yarn.lock",
		".github/workflows/ci.yml",
		"src/main.go",
		"internal/app/service_test.go",
		"src/utils/helper.py",
	}
	got := filterNoiseFiles(files)
	// Expected: src/main.go, internal/app/service_test.go, src/utils/helper.py (README.md is now noise)
	if len(got) != 3 {
		t.Errorf("expected 3 signal files, got %d: %v", len(got), got)
	}
	for _, f := range []string{"src/main.go", "internal/app/service_test.go", "src/utils/helper.py"} {
		found := false
		for _, g := range got {
			if g == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in signal files, got %v", f, got)
		}
	}
	// Verify noise files are NOT present
	for _, f := range []string{"package.json", "pnpm-lock.yaml", "yarn.lock", ".github/workflows/ci.yml"} {
		for _, g := range got {
			if g == f {
				t.Errorf("noise file %q should not be in signal files", f)
			}
		}
	}
}

// =============================================================================
// isSourceFile tests
// =============================================================================

func TestIsSourceFile_Go(t *testing.T) {
	t.Parallel()
	if !isSourceFile("main.go") {
		t.Errorf("main.go should be a source file")
	}
}

func TestIsSourceFile_Python(t *testing.T) {
	t.Parallel()
	if !isSourceFile("app.py") {
		t.Errorf("app.py should be a source file")
	}
}

func TestIsSourceFile_TypeScript(t *testing.T) {
	t.Parallel()
	if !isSourceFile("index.ts") {
		t.Errorf("index.ts should be a source file")
	}
}

func TestIsSourceFile_Config(t *testing.T) {
	t.Parallel()
	if isSourceFile("tsconfig.json") {
		t.Errorf("tsconfig.json should NOT be a source file")
	}
}

func TestIsSourceFile_Lock(t *testing.T) {
	t.Parallel()
	if isSourceFile("yarn.lock") {
		t.Errorf("yarn.lock should NOT be a source file")
	}
}

func TestIsSourceFile_Markdown(t *testing.T) {
	t.Parallel()
	if isSourceFile("README.md") {
		t.Errorf("README.md should NOT be a source file")
	}
}

// =============================================================================
// Expanded noise files tests (monorepo noise)
// =============================================================================

func TestNoiseFiles_ExpandedList(t *testing.T) {
	t.Parallel()
	// These files should be filtered as noise
	noiseFiles := []string{
		// Go modules
		"go.mod",
		"go.sum",
		// Documentation
		"README.md",
		"LICENSE",
		"CHANGELOG.md", // already existed
		// Build
		"Makefile",
		"Dockerfile",
		// Rust
		"Cargo.toml",
		"Cargo.lock",
		// Python
		"pyproject.toml",
		"setup.py",
		"requirements.txt",
	}
	for _, f := range noiseFiles {
		got := filterNoiseFiles([]string{f})
		if len(got) != 0 {
			t.Errorf("%s should be in noise files, got %v", f, got)
		}
	}
}

func TestFilterNoiseFiles_MixedBatch_Updated(t *testing.T) {
	t.Parallel()
	// Updated test with expanded noise list
	files := []string{
		"package.json",
		"pnpm-lock.yaml",
		"yarn.lock",
		".github/workflows/ci.yml",
		"src/main.go",
		"internal/app/service_test.go",
		"go.mod",         // now noise
		"README.md",      // now noise
		"src/utils/helper.py",
	}
	got := filterNoiseFiles(files)
	// Expected: src/main.go, internal/app/service_test.go, src/utils/helper.py (3, not 4)
	if len(got) != 3 {
		t.Errorf("expected 3 signal files, got %d: %v", len(got), got)
	}
	for _, f := range []string{"src/main.go", "internal/app/service_test.go", "src/utils/helper.py"} {
		found := false
		for _, g := range got {
			if g == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in signal files, got %v", f, got)
		}
	}
	// Verify noise files are NOT present
	for _, f := range []string{"package.json", "pnpm-lock.yaml", "yarn.lock", ".github/workflows/ci.yml", "go.mod", "README.md"} {
		for _, g := range got {
			if g == f {
				t.Errorf("noise file %q should not be in signal files", f)
			}
		}
	}
}
