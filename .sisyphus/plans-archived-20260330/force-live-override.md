# Live Override for Cluster/Graph/Plan Commands

## TL;DR

> **Quick Summary**: `pratc analyze --force-live` works against live GitHub repos without a prior sync. `cluster`, `graph`, and `plan` do not — they fail with `no fixture data for repo ... and no live snapshot available`. Add `--force-live` support to all three commands so they can run end-to-end on any repo without a persisted sync snapshot.
>
> **Deliverables**:
> - `internal/cmd/root.go`: add `--force-live` flag to cluster, graph, plan commands
> - Live data fetch wiring in each command handler
> - Tests for the new flag across all three commands
>
> **Estimated Effort**: Short (single file, mechanical change)
> **Parallel Execution**: NO (single file, sequential edits)
> **Critical Path**: root.go edit → test → live verify

---

## Context

### Original Request
User ran full production PR-ATC pipeline on `openclaw/openclaw`. Sync succeeded, analyze worked with `--force-live`, but cluster/graph/plan all failed with `no fixture data for repo "openclaw/openclaw" and no live snapshot available`.

### Interview Summary
- Sync completes (~6 min) but persist job gets stuck `in_progress` in this environment
- `analyze --force-live` fetches 6500+ PRs and processes 1000 — fully working
- `cluster/graph/plan` have no `--force-live` flag and always require a persisted snapshot
- The service layer (`app.Config.AllowLive`) already handles live fetch — just not plumbed from CLI

### Research Findings
- `RegisterAnalyzeCommand` (root.go:224-270): has `forceLive` var, passes via `buildAnalyzeConfig(useCacheFirst, forceLive)` → `app.Config{AllowLive: forceLive}`
- `RegisterClusterCommand` (root.go:307-335): uses `buildCacheFirstConfig(useCacheFirst)` → `app.Config{UseCacheFirst}` — missing `AllowLive`
- `RegisterGraphCommand` (root.go:337-368): same pattern as cluster
- `RegisterPlanCommand` (root.go:370-410): same pattern as cluster
- `buildCacheFirstConfig` (root.go:276-278): `app.Config{UseCacheFirst: useCacheFirst}`
- `buildAnalyzeConfig` (root.go:272-274): `app.Config{AllowLive: forceLive, UseCacheFirst: useCacheFirst}`
- `app.Config.AllowLive` (app/service.go:33): existing field, already wired in `Analyze` path

### Metis Review
**Identified Gaps** (addressed):
- Changing from `buildCacheFirstConfig` to `buildAnalyzeConfig` is safe — `buildAnalyzeConfig` is a superset
- No interface changes needed — `app.Config` already has `AllowLive`
- No test doubles break since config change is internal to command registration
- The `forceLive` variable name collision across functions is safe (Go function-scoped vars)

---

## Work Objectives

### Core Objective
Add `--force-live` flag to `RegisterClusterCommand`, `RegisterGraphCommand`, and `RegisterPlanCommand` so each passes `AllowLive: true` to the app service, mirroring `RegisterAnalyzeCommand`.

### Concrete Deliverables
- `internal/cmd/root.go`: 3 functions updated (cluster/graph/plan) — add `forceLive` var, use `buildAnalyzeConfig`, register flag

### Definition of Done
- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `./bin/pratc cluster --help` shows `--force-live` flag
- [ ] `./bin/pratc graph --help` shows `--force-live` flag
- [ ] `./bin/pratc plan --help` shows `--force-live` flag

### Must Have
- `--force-live` flag on all three commands
- `AllowLive: true` passed to service when flag set
- Default behavior unchanged (snapshot-first when flag absent)

### Must NOT Have (Guardrails)
- Don't change `buildCacheFirstConfig` or `buildAnalyzeConfig` — reuse existing
- Don't change `app/service.go` — `AllowLive` already works
- Don't change default behavior of any command
- Don't break existing tests

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (`go test -race ./...`)
- **Automated tests**: YES (tests-after — existing tests validate no regression)
- **Framework**: Go standard `testing`

### QA Policy
- Task: verify `--force-live` flag appears in `--help` output
- Task: verify `go test ./internal/cmd/...` still passes (no regression)
- Final: live test all three commands against openclaw/openclaw

---

## Execution Strategy

```
Wave 1 (single file, sequential):
├── Task 1: Add --force-live to RegisterClusterCommand (root.go)
├── Task 2: Add --force-live to RegisterGraphCommand (root.go)
├── Task 3: Add --force-live to RegisterPlanCommand (root.go)
├── Task 4: Verify --help shows flag on all three commands
└── Task 5: Run go test -race ./...
```

---

## TODOs

- [x] 1. **Add --force-live to RegisterClusterCommand**

  **What to do**:
  - In `internal/cmd/root.go`, in `RegisterClusterCommand()` (line 307):
    - Add `var forceLive bool` after `var useCacheFirst bool` (line 310)
    - Change `service := app.NewService(buildCacheFirstConfig(useCacheFirst))` (line 316) to `service := app.NewService(buildAnalyzeConfig(useCacheFirst, forceLive))`
    - Add `command.Flags().BoolVar(&forceLive, "force-live", false, "Skip cache check and force live fetch")` after the `--use-cache-first` flag registration (after line 332)

  **Must NOT do**:
  - Don't change `buildCacheFirstConfig` or `buildAnalyzeConfig` functions
  - Don't change `app/service.go`
  - Don't break existing tests

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: none needed
  - **Reason**: Single-file, 3-line mechanical change

  **Parallelization**:
  - **Can Run In Parallel**: NO (same file as Tasks 2-3)
  - **Blocks**: Tasks 2-3

  **References** (CRITICAL - Be Exhaustive):
  - `internal/cmd/root.go:307-335` — RegisterClusterCommand current implementation
  - `internal/cmd/root.go:224-270` — RegisterAnalyzeCommand pattern to follow
  - `internal/cmd/root.go:272-274` — buildAnalyzeConfig to reuse
  - `internal/cmd/root.go:276-278` — buildCacheFirstConfig to replace with buildAnalyzeConfig

  **WHY Each Reference Matters**:
  - RegisterAnalyzeCommand shows the exact pattern: `var forceLive bool` → `buildAnalyzeConfig(useCacheFirst, forceLive)` → `Flags().BoolVar(&forceLive, ...)`
  - buildAnalyzeConfig already exists and returns `app.Config{AllowLive: forceLive, UseCacheFirst: useCacheFirst}`

  **Acceptance Criteria**:
  - [ ] `go test ./internal/cmd/... -v` passes

  **QA Scenarios**:

  Scenario: cluster --help shows --force-live flag
    Tool: Bash
    Preconditions: binary built
    Steps:
      1. `./bin/pratc cluster --help`
      2. Assert output contains `--force-live`
      3. Assert output contains `Skip cache check and force live fetch`
    Expected Result: help text shows the new flag
    Evidence: terminal output

  Scenario: cluster default behavior unchanged
    Tool: Bash
    Preconditions: existing tests pass
    Steps:
      1. `go test ./internal/cmd/... -v`
      2. Assert 0 failures
    Expected Result: all existing tests pass
    Evidence: test output

  **Commit**: NO (combine with Tasks 2-3)

---

- [x] 2. **Add --force-live to RegisterGraphCommand**

  **What to do**:
  - In `internal/cmd/root.go`, in `RegisterGraphCommand()` (line 337):
    - Add `var forceLive bool` after `var useCacheFirst bool` (line 340)
    - Change `service := app.NewService(buildCacheFirstConfig(useCacheFirst))` (line 346) to `service := app.NewService(buildAnalyzeConfig(useCacheFirst, forceLive))`
    - Add `command.Flags().BoolVar(&forceLive, "force-live", false, "Skip cache check and force live fetch")` after the `--use-cache-first` flag registration (after line 365)

  **Must NOT do**: Same as Task 1

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (same file as Tasks 1,3)
  - **Blocks**: Task 4

  **References**:
  - `internal/cmd/root.go:337-368` — RegisterGraphCommand current implementation
  - `internal/cmd/root.go:307-335` — RegisterClusterCommand (already modified by Task 1)

  **Acceptance Criteria**:
  - [ ] `go test ./internal/cmd/... -v` passes

  **QA Scenarios**:

  Scenario: graph --help shows --force-live flag
    Tool: Bash
    Preconditions: binary built
    Steps:
      1. `./bin/pratc graph --help`
      2. Assert output contains `--force-live`
    Expected Result: help text shows the new flag
    Evidence: terminal output

  **Commit**: NO (combine with Tasks 1,3)

---

- [x] 3. **Add --force-live to RegisterPlanCommand**

  **What to do**:
  - In `internal/cmd/root.go`, in `RegisterPlanCommand()` (line 370):
    - Add `var forceLive bool` after `var useCacheFirst bool` (line 377)
    - Change `service := app.NewService(buildCacheFirstConfig(useCacheFirst))` (line 392) to `service := app.NewService(buildAnalyzeConfig(useCacheFirst, forceLive))`
    - Add `command.Flags().BoolVar(&forceLive, "force-live", false, "Skip cache check and force live fetch")` after the `--use-cache-first` flag registration (after line 377 area — find existing flag registrations in that function)

  **Must NOT do**: Same as Task 1

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (same file as Tasks 1-2)
  - **Blocks**: Task 4

  **References**:
  - `internal/cmd/root.go:370-410` — RegisterPlanCommand current implementation

  **Acceptance Criteria**:
  - [ ] `go test ./internal/cmd/... -v` passes

  **QA Scenarios**:

  Scenario: plan --help shows --force-live flag
    Tool: Bash
    Preconditions: binary built
    Steps:
      1. `./bin/pratc plan --help`
      2. Assert output contains `--force-live`
    Expected Result: help text shows the new flag
    Evidence: terminal output

  **Commit**: YES (with Tasks 1-2)
  - Message: `feat(cmd): add --force-live to cluster/graph/plan commands`
  - Files: `internal/cmd/root.go`
  - Pre-commit: `go test -race ./...`

---

- [x] 4. **Verify --help and run full test suite**

  **What to do**:
  - Build binary: `make build`
  - Run `./bin/pratc cluster --help`, `./bin/pratc graph --help`, `./bin/pratc plan --help` — confirm `--force-live` appears
  - Run `go build ./...`, `go vet ./...`, `go test -race ./...`

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Blocked By**: Tasks 1-3

  **References**:
  - `Makefile` — build target

  **Acceptance Criteria**:
  - [ ] All three `--help` outputs show `--force-live`
  - [ ] `go build ./...` passes
  - [ ] `go vet ./...` passes
  - [ ] `go test -race ./...` passes

  **QA Scenarios**:

  Scenario: all help outputs show --force-live
    Tool: Bash
    Preconditions: binary built
    Steps:
      1. `./bin/pratc cluster --help | grep force-live`
      2. `./bin/pratc graph --help | grep force-live`
      3. `./bin/pratc plan --help | grep force-live`
    Expected Result: all three commands show the flag
    Evidence: terminal output

  Scenario: full test suite passes
    Tool: Bash
    Preconditions: none
    Steps:
      1. `go test -race ./...`
      2. Assert 0 failures across all packages
    Expected Result: all packages pass
    Evidence: test output

  **Commit**: NO

---

## Final Verification Wave

- [x] F1: `go build ./...` passes
- [x] F2: `go vet ./...` passes
- [x] F3: `go test -race ./...` passes
- [x] F4: Live test: `./bin/pratc cluster --repo=openclaw/openclaw --force-live --format=json` returns cluster JSON
- [x] F5: Live test: `./bin/pratc graph --repo=openclaw/openclaw --force-live --format=dot` returns DOT graph
- [x] F6: Live test: `./bin/pratc plan --repo=openclaw/openclaw --force-live --target=50 --format=json` returns plan JSON

---

## Commit Strategy

- **Single commit**: `feat(cmd): add --force-live to cluster/graph/plan commands`
- **Files**: `internal/cmd/root.go`
- **Pre-commit**: `go test -race ./...`

---

## Success Criteria

```bash
go build ./...                    # exit 0
go vet ./...                      # exit 0
go test -race ./...               # all pass
./bin/pratc cluster --repo=openclaw/openclaw --force-live --format=json  # returns cluster JSON
./bin/pratc graph --repo=openclaw/openclaw --force-live --format=dot     # returns DOT graph
./bin/pratc plan --repo=openclaw/openclaw --force-live --target=50 --format=json  # returns plan JSON
```
