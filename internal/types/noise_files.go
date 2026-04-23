package types

import "strings"

// Conflict detection defaults and shared noise filters.
const MinSharedSignalFilesForConflict = 2

// ConflictNoiseFiles are filenames that commonly appear across many PRs and add
// little value to conflict-pair reporting.
var ConflictNoiseFiles = map[string]bool{
	// JavaScript/TypeScript package managers
	"package.json":      true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"package-lock.json": true,
	"bun.lockb":         true,
	// Go modules
	"go.mod": true,
	"go.sum": true,
	// Rust
	"Cargo.toml": true,
	"Cargo.lock": true,
	// Python
	"pyproject.toml":   true,
	"setup.py":         true,
	"requirements.txt": true,
	// Build
	"Makefile":   true,
	"Dockerfile": true,
	// VCS/config
	".gitignore":    true,
	".editorconfig": true,
	// Linters/formatters
	".eslintrc":        true,
	".eslintrc.json":   true,
	".prettierrc":      true,
	".prettierrc.json": true,
	// TypeScript
	"tsconfig.json":      true,
	"tsconfig.base.json": true,
	"vitest.config.ts":   true,
	"jest.config.ts":     true,
	// Documentation
	"CHANGELOG.md": true,
	"README.md":    true,
	"LICENSE":      true,
}

// ConflictNoiseExtensions are extensions that are always treated as conflict noise.
var ConflictNoiseExtensions = []string{
	".lock", ".lockb", ".lock.json",
}

var ConflictNoisePathPrefixes = []string{
	"docs/.generated/",
}

var ConflictNoisePathExact = map[string]bool{
	"docs/docs.json": true,
}

var ConflictNoisePathSuffixes = []string{
	"/schema.base.generated.ts",
	"/schema.help.ts",
	"/schema.labels.ts",
}

// FilterNoiseFiles returns only the signal files from the input set.
// Noise files are: exact name matches, extension matches, path prefix matches,
// path exact matches, and path suffix matches defined in this package.
// mergeability_signal is always preserved as a valid signal.
func FilterNoiseFiles(files []string) []string {
	var signal []string
	for _, f := range files {
		base := f
		if idx := strings.LastIndex(f, "/"); idx >= 0 {
			base = f[idx+1:]
		}
		if ConflictNoiseFiles[base] || ConflictNoiseFiles[f] {
			continue
		}
		if ConflictNoisePathExact[f] {
			continue
		}
		prefixNoise := false
		for _, prefix := range ConflictNoisePathPrefixes {
			if strings.HasPrefix(f, prefix) {
				prefixNoise = true
				break
			}
		}
		if prefixNoise {
			continue
		}
		suffixNoise := false
		for _, suffix := range ConflictNoisePathSuffixes {
			if strings.HasSuffix(f, suffix) {
				suffixNoise = true
				break
			}
		}
		if suffixNoise {
			continue
		}
		isNoise := false
		for _, ext := range ConflictNoiseExtensions {
			if strings.HasSuffix(f, ext) {
				isNoise = true
				break
			}
		}
		if isNoise {
			continue
		}
		// Skip files in .github/ unless they are in /src/ or /actions/
		if strings.HasPrefix(f, ".github/") && !strings.Contains(f, "/src/") && !strings.Contains(f, "/actions/") {
			continue
		}
		signal = append(signal, f)
	}
	return signal
}
