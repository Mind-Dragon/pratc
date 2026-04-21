# Contributing to prATC

Thank you for your interest in contributing to prATC (PR Air Traffic Control). This document outlines how to set up your development environment, follow our coding conventions, and submit changes.

## Development Setup

### Prerequisites

You will need the following tools installed:

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.23+ | Backend CLI and API |
| Python | 3.11+ | ML service |
| Node.js | Latest LTS | Optional: web UI development only |
| Bun | Latest | Optional: web UI development only |
| Docker | Latest | Containerized services |
| uv | Latest | Python package management |

### Verify Your Environment

Run the following command to check that all required tools are installed:

```bash
make verify-env
```

This validates that Go, Python 3.11+, uv, Node.js, and Bun are available on your system.

### Build the Project

```bash
make build
```

This compiles the Go binary to `./bin/pratc`.

## Testing

All changes must include tests and pass the existing test suite.

### Run All Tests

```bash
make test
```

This runs the full test suite across all three components:

### Component-Specific Tests

**Go tests:**
```bash
make test-go
```
Runs `go test -race -v ./...`

**Python tests:**
```bash
make test-python
```
Runs `uv run pytest -v` in the `ml-service/` directory

**Web tests:**
```bash
make test-web
```
Runs `bun run test` (vitest) in the `web/` directory

### Linting

```bash
make lint
```
Runs `go vet ./...` on the Go codebase.

## Code Style

### Go Conventions

Follow these conventions for all Go code in `internal/` and `cmd/`:

**Error Wrapping**
Always wrap errors with context using `fmt.Errorf`:
```go
// Correct
return fmt.Errorf("failed to load config: %w", err)

// Incorrect
return err
```

**Interfaces**
Keep interfaces small (1-3 methods) and define them at the point of consumption, not at the implementation.

**Constructors**
Use `New()` with functional options for configurable types:
```go
func New(opts ...Option) *Type { ... }

func WithTimeout(d time.Duration) Option { ... }
```

**Tests**
Write table-driven tests with `t.Run` subtests. Do not use testify/assert.
```go
func TestSomething(t *testing.T) {
    cases := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid input", "foo", "bar"},
        {"empty input", "", ""},
    }
    
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            result := DoSomething(tc.input)
            if result != tc.expected {
                t.Errorf("expected %q, got %q", tc.expected, result)
            }
        })
    }
}
```

**init() Functions**
Only use `init()` in `cmd/pratc/` for Cobra command registration. Never use `init()` in `internal/` packages.

**Sorting**
Always use stable + deterministic sorting. Use PR number as a tiebreaker wherever applicable.

### Type Consistency

Go, Python, and TypeScript share identical type definitions with `snake_case` JSON keys:

- **Go:** `internal/types/models.go`
- **Python:** `ml-service/src/pratc_ml/models.py`
- **TypeScript:** `web/src/types/api.ts`

When adding new fields, update all three files to maintain consistency.

## How to Report Bugs

1. **Check existing issues** first to avoid duplicates
2. **Use the issue template** if available
3. **Include reproduction steps:**
   - Command or API call that triggered the bug
   - Expected behavior
   - Actual behavior (with full error message)
   - Environment details (OS, Go version, etc.)

## How to Request Features

1. **Open an issue** describing the feature
2. **Explain the use case** — what problem does it solve?
3. **Reference the scope guardrails** in AGENTS.md — features outside v0.1 scope may be deferred

## Pull Request Process

1. **Create a feature branch** from `main`
2. **Make your changes** following the code style guidelines above
3. **Add tests** for new functionality
4. **Run the full test suite** with `make test` — all tests must pass
5. **Run linting** with `make lint` — no errors allowed
6. **Update documentation** if your change affects APIs or behavior
7. **Submit the PR** with a clear description of:
   - What changed and why
   - How to test the changes
   - Any breaking changes

### PR Requirements

- All tests must pass (`make test`)
- No linting errors (`make lint`)
- Code follows Go conventions from AGENTS.md
- New features include tests
- Documentation updated as needed

### Post-Merge Verification

After your PR is merged, verify that `main` remains healthy:

```bash
git checkout main
make build && make test
```

## Anti-Patterns to Avoid

- Never read raw secret values; use `psst SECRET_NAME -- <command>`
- Never store GitHub PAT in SQLite or config files; only use runtime env vars
- Never run combinatorial planning on raw PR universe; always pre-filter first
- Never commit secrets (GITHUB_PAT, OPENROUTER_API_KEY, etc.)
- Never leave `main` red; post-merge verification is mandatory

## Project Structure Quick Reference

```
cmd/pratc/          # CLI entrypoints
internal/           # Go packages
  app/              # Service layer
  cache/            # SQLite persistence
  github/           # GitHub client
  filter/           # Pre-filter pipeline
  formula/          # Combinatorial engine
  graph/            # Dependency graph
  planner/          # Merge planning
  settings/         # Settings management
  types/            # Shared types
ml-service/         # Python ML service
web/                # TypeScript Next.js dashboard (deprecated)
```

## Getting Help

- Check `AGENTS.md` for detailed project conventions
- Review existing code in the relevant package for patterns
- Open an issue for questions not covered here

## License

By contributing to prATC, you agree that your contributions will be licensed under the MIT License.
