# prATC Testing Protocol

## Overview
This protocol defines the CI-stable verification sequence for all PRs to this repository.
Tests not listed here are excluded from the default merge gate.

## Verification Sequence

Run in order. All must exit 0:

```bash
make verify-env    # Check toolchain (go, python3.11+, uv, node, bun, docker)
make build         # Compile Go binary to ./bin/pratc
make test          # Run all tests (go + python + web)
make lint          # go vet ./...
cd web && bun run build   # Web dashboard build
```

## Evidence Format

Every task must produce evidence files under `.sisyphus/evidence/`:

```
.sisyphus/evidence/task-{N}-{slug}.txt   # Text evidence
.sisyphus/evidence/task-{N}-{slug}.md     # Markdown evidence
```

Evidence files must capture:
- Command outputs (stdout/stderr)
- Exit codes
- Timestamps
- Verification pass/fail status

## Failure Triage

### make build fails
1. Run `go build ./...` directly to isolate the package
2. Check for syntax errors in recently modified files
3. Run `go vet ./...` to find type errors
4. Fix the broken package, rebuild

### make test fails
1. Run `go test ./...` directly to see specific failures
2. Run targeted test: `go test ./internal/pkg -count=1` for the failing package
3. Check `go test -race ./internal/pkg` for race conditions
4. Fix tests or code as needed

### make lint fails
1. Run `go vet ./...` to see warnings/errors
2. Address each warning (usually type errors or missing error checks)
3. Re-run `make lint`

### cd web && bun run build fails
1. Run `cd web && bun run build` directly to see TypeScript errors
2. Check for type mismatches in recently modified components
3. Run `cd web && bun run typecheck` if available
4. Fix type errors

## CI-Stable Only Policy

**Default merge gate excludes:**
- Live/provider tests (require actual GitHub API credentials)
- Non-deterministic tests (flaky by design)
- Integration tests requiring external services

**Default merge gate includes:**
- `make build`
- `make test`
- `make lint`
- `cd web && bun run build`

## Dry-Run Verification

To verify the full protocol locally:

```bash
cd /home/agent/pratc
git status  # Should be clean or only expected changes
make verify-env
make build
make test
make lint
cd web && bun run build && cd ..
echo "All gates passed"
```

If any gate fails, do NOT proceed. Fix the failure first.
