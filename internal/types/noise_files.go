package types

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
