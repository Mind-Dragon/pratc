# prATC Production Resilience Fixes — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan.

**Goal:** Fix 9 production resilience issues across Docker, Go CLI, and docker-compose so the stack boots reliably, commands fail safe, and the system is ready for a live 5k-PR test.

**Architecture:** Changes are layered — Go fixes first (enabling `--host` flag), then Docker layer (using the new flag), then compose/healthchecks (no Go changes needed), then one ML fallback.

**Tech Stack:** Go (Cobra CLI), Docker, docker-compose, Python ML subprocess

---

## TL;DR

> **Quick Summary**: Fix Docker networking, command safety defaults, sync lifetime, and ML subprocess so the prATC stack starts properly, commands don't accidentally delete or execute, and the 5k-PR live test is safe to run.
>
> **Deliverables**:
> - `internal/cmd/root.go` — `mirror prune` dry-run default, sync CLI blocking, `--host` default
> - `Dockerfile.cli` — serves on `0.0.0.0`
> - `docker-compose.yml` — real HTTP healthchecks, `GH_TOKEN` env passthrough
> - `internal/ml/bridge.go` — `PYTHONPATH` for Python ML subprocess + graceful fallback
> - `.env.example` + README.md section for 5k test
>
> **Estimated Effort**: Short — 9 targeted tasks across 6 files
> **Parallel Execution**: YES — 4 waves
> **Critical Path**: Task 1 (Go `--host` default) → Task 4 (Dockerfile uses it)

---

## Context

### Original Request
"i have gh logged in here. i can't do tokens because i don't think they issue tokens anymore, create a plan to do all the rest of the fixes."

User has `gh` CLI authenticated locally. The Go code reads `GH_TOKEN` and `GITHUB_PAT` in `service.go:79-85`, and `GITHUB_TOKEN` in `default_runner.go:75-86`. `gh auth token` is referenced in error messages but never executed as a fallback.

### What This Plan Covers (and Does NOT Cover)

**Covers:**
1. API bind address (0.0.0.0 in container)
2. Mirror prune default safety (dry-run=true)
3. Sync CLI blocking wait
4. Docker healthchecks (real HTTP probes)
5. Docker GH_TOKEN passthrough
6. ML subprocess PYTHONPATH
7. ML graceful fallback
8. Env var documentation for 5k test

**NOT covered** (separate follow-up work):
- Rate limit jitter (exponential backoff without jitter)
- Middleware recovery/timeout layers
- Cancellation checkpoints in graph/formula loops
- SSE reconnection robustness (retry: id:, Last-Event-ID)

---

## Work Objectives

### Core Objective
Fix 9 blocking production resilience issues so the prATC stack is bootable, commands are safe-by-default, and the system survives a live 5k-PR test.

### Concrete Deliverables
- `internal/cmd/root.go` — `serve --host` defaults to `0.0.0.0`
- `internal/cmd/root.go` — `mirror prune` defaults to `dry-run=true`
- `internal/cmd/root.go` — `sync` CLI blocks until sync completes (adds `--no-wait` flag)
- `Dockerfile.cli` — serves on `0.0.0.0`
- `docker-compose.yml` — `GH_TOKEN` + `GITHUB_TOKEN` passthrough to pratc-cli
- `docker-compose.yml` — real HTTP healthchecks for both containers
- `internal/ml/bridge.go` — `PYTHONPATH` so `python -m pratc_ml.cli` works in container
- `internal/ml/bridge.go` — graceful fallback to local/heuristic when Python subprocess fails
- `.env.example` + README.md 5k test section

### Definition of Done
- [x] `pratc serve --host=0.0.0.0` binds to all interfaces
- [x] `docker-compose up` — both containers pass real HTTP healthchecks
- [x] `pratc mirror prune` prints plan, doesn't delete, unless `--confirm`
- [x] `pratc sync --repo=owner/repo` waits for completion before exiting
- [x] `python -m pratc_ml.cli` works inside Docker container
- [x] ML failures fall back to local/heuristic mode gracefully
- [x] `.env.example` documents all required env vars

### Must Have
- `mirror prune` is safe by default (dry-run=true)
- `sync` CLI completes synchronously
- Docker healthchecks verify actual HTTP readiness, not file existence
- API reachable from outside the container
- ML subprocess path is resilient

### Must NOT Have
- No changes to SQLite schema or migrations
- No changes to API response contracts
- No changes to existing test behavior (tests still pass)
- No GitHub token code changes (user uses `gh` CLI auth)

---

## Verification Strategy

- **Infrastructure exists**: YES (`go test -race -v ./...`)
- **Automated tests**: YES (tests-after — fixes are targeted, low-risk changes)
- **QA Policy**: Every task includes agent-executed QA scenarios. Evidence saved to `.sisyphus/evidence/`.

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Immediate — Go server and command safety):
├── Task 1: serve --host default to 0.0.0.0 [quick]
├── Task 2: mirror prune dry-run default [quick]
└── Task 3: sync CLI blocking wait [medium]

Wave 2 (Docker layer — uses new --host flag from Wave 1):
├── Task 4: Dockerfile.cli --host=0.0.0.0 [quick]
├── Task 5: docker-compose GH_TOKEN passthrough [quick]
└── Task 6: docker-compose HTTP healthchecks [quick]

Wave 3 (ML resilience — independent of Waves 1-2):
├── Task 7: PYTHONPATH + ML graceful fallback [medium]
└── Task 8: Document GH_TOKEN and 5k test commands [quick]

Wave FINAL:
└── Task 9: Verification run [quick]
```

---

## TODOs

- [x] 1. **serve --host defaults to 0.0.0.0**

  **What to do**: Modify `internal/cmd/root.go:432` — change the default value of `--host` from `"127.0.0.1"` to `"0.0.0.0"`. Update the flag description to reflect it now defaults to all interfaces. The flag already exists and `runServer` already uses it — just flip the default.

  **Must NOT do**: Do not remove the flag or change its type. Do not change the allowed bind hosts validation.

  **Recommended Agent Profile**:
  - **Category**: `quick` — one-line flag default change; no logic modification
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2, 3)
  - **Blocks**: Task 4 (Dockerfile uses the new default)
  - **Blocked By**: None

  **References**:
  - `internal/cmd/root.go:432` — current `--host` flag definition (default `"127.0.0.1"`)
  - `internal/cmd/root.go:429` — `runServer` function that uses the host variable
  - `internal/cmd/bind_policy_test.go:16-46` — tests for allowed bind hosts

  **Acceptance Criteria**:
  - [ ] `go test -race ./internal/cmd/... -run TestBindPolicy -v` → PASS
  - [ ] `go build -o /tmp/pratc ./cmd/pratc && /tmp/pratc serve --help | grep host` → shows `--host` with 0.0.0.0

  **QA Scenarios**:

  ```
  Scenario: serve binds to 0.0.0.0 by default
    Tool: Bash
    Preconditions: Built binary at /tmp/pratc
    Steps:
      1. Start server in background: `/tmp/pratc serve &`
      2. Wait 2s for startup
      3. Check what address is listening: `ss -tlnp | grep 8080`
    Expected Result: Shows `*:8080` or `0.0.0.0:8080` listening
    Failure Indicators: Still shows `127.0.0.1:8080`
    Evidence: .sisyphus/evidence/task-1-serve-bind.txt
  ```

  **Commit**: YES (group with Tasks 2, 3)
  - Message: `fix(cmd): default serve --host to 0.0.0.0 for container reachability`
  - Files: `internal/cmd/root.go`

---

- [x] 2. **mirror prune defaults to dry-run=true**

  **What to do**:
  - Modify `internal/cmd/root.go:1523` — change `--dry-run` default from `false` to `true`
  - Change flag description from `"Show what would be removed without deleting"` to `"Dry-run mode (default: true)"`
  - Add an explicit `--confirm` flag (negation of dry-run) for operators who want to actually delete

  **Must NOT do**: Do not change the prune logic itself. Do not add a confirmation prompt — use a flag approach.

  **Recommended Agent Profile**:
  - **Category**: `quick` — one-line flag change + one new flag definition
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `internal/cmd/root.go:1448-1523` — full prune command implementation
  - `internal/cmd/root.go:1523` — current flag definition (`dry-run` default false)
  - `internal/cmd/root.go:1505-1511` — dry-run branch that prints paths

  **Acceptance Criteria**:
  - [ ] `go test -race ./internal/cmd/... -run TestMirror -v` → PASS
  - [ ] `go build -o /tmp/pratc ./cmd/pratc && /tmp/pratc mirror prune --help` → shows dry-run defaulting to true

  **QA Scenarios**:

  ```
  Scenario: mirror prune is safe by default (prints paths without deleting)
    Tool: Bash
    Preconditions: Built binary, test mirrors directory with known repos
    Steps:
      1. Run prune on a known repo: `/tmp/pratc mirror prune --repo=openclaw/openclaw`
      2. Check output contains "would be removed" or lists paths
      3. Verify mirrors directory is unchanged
    Expected Result: Lists what would be removed, no files deleted
    Failure Indicators: Files are deleted
    Evidence: .sisyphus/evidence/task-2-prune-dryrun.txt

  Scenario: mirror prune --confirm actually deletes
    Tool: Bash
    Preconditions: Built binary, temp mirror directory created for a non-existent repo
    Steps:
      1. Run prune with --confirm for the temp repo
      2. Check the temp mirror directory is removed
    Expected Result: Directory removed after --confirm
    Failure Indicators: Directory still exists
    Evidence: .sisyphus/evidence/task-2-prune-confirm.txt
  ```

  **Commit**: YES (group with Tasks 1, 3)
  - Message: `fix(cmd): mirror prune defaults to dry-run, add --confirm flag`
  - Files: `internal/cmd/root.go`

---

- [x] 3. **sync CLI blocks until sync completes**

  **What to do**:
  - Modify `internal/cmd/root.go:438-479` (`RegisterSyncCommand` RunE)
  - When `--watch=false` (default), after calling `manager.Start(repo)`, also wait for completion
  - Simplest approach: after `manager.Start(repo)`, poll `analyzeSyncActive(repo)` until false or 30min timeout
  - Reuse the existing `waitForAnalyzeSyncCompletion(repo, timeout)` function from `root.go:165-174`
  - Add `--no-wait` flag for operators who want fire-and-forget behavior
  - Timeout error: `"sync for %s timed out after %v"`

  **Must NOT do**: Do not change watch mode behavior (already loops correctly). Do not break SSE streaming. Do not remove the goroutine in `sse.go Start()`.

  **Recommended Agent Profile**:
  - **Category**: `medium` — changes sync CLI exit behavior; requires understanding SSE/Manager interaction
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `internal/cmd/root.go:438-479` — current sync RunE (exits immediately after Start)
  - `internal/cmd/root.go:165-174` — `waitForAnalyzeSyncCompletion` polling function (reuse pattern)
  - `internal/sync/sse.go:38-73` — Manager.Start() launches goroutine
  - `internal/sync/sse.go:75-154` — Stream() SSE handler

  **Acceptance Criteria**:
  - [ ] `go test -race ./internal/cmd/... -run TestSync -v` → PASS
  - [ ] `go test -race ./internal/sync/... -v` → PASS
  - [ ] `pratc sync --repo=owner/repo` waits for completion before returning

  **QA Scenarios**:

  ```
  Scenario: sync CLI waits for completion and exits 0
    Tool: Bash
    Preconditions: Valid GitHub token in GH_TOKEN env, built binary
    Steps:
      1. Run sync for a small public repo: `GH_TOKEN=$GH_TOKEN /tmp/pratc sync --repo=go-git/go-git`
      2. Wait for exit (timeout 5min)
      3. Check exit code
    Expected Result: Exits 0 after sync completes (not immediately)
    Failure Indicators: Exits immediately, or error
    Evidence: .sisyphus/evidence/task-3-sync-wait.txt

  Scenario: sync --watch still loops correctly (no change to watch behavior)
    Tool: Bash
    Preconditions: Built binary, valid token
    Steps:
      1. Run sync in watch mode: `/tmp/pratc sync --repo=test/repo --watch --interval=1s`
      2. Send SIGINT after 3 seconds: `timeout 3s /tmp/pratc sync --watch --interval=1s --repo=test/repo`
    Expected Result: Runs ~3 seconds, exits cleanly on SIGINT
    Failure Indicators: Doesn't respect SIGINT or hangs
    Evidence: .sisyphus/evidence/task-3-sync-watch.txt

  Scenario: sync --no-wait returns immediately (fire-and-forget)
    Tool: Bash
    Preconditions: Built binary, valid token
    Steps:
      1. Run with --no-wait: `/tmp/pratc sync --no-wait --repo=go-git/go-git`
    Expected Result: Returns immediately with started=true
    Failure Indicators: Still blocks
    Evidence: .sisyphus/evidence/task-3-sync-nowait.txt
  ```

  **Commit**: YES (group with Tasks 1, 2)
  - Message: `fix(cmd): sync blocks until completion, add --no-wait for fire-and-forget`
  - Files: `internal/cmd/root.go`

---

- [x] 4. **Dockerfile.cli serves on 0.0.0.0**

  **What to do**:
  - Modify `Dockerfile.cli:22` — change `CMD ["pratc", "serve", "--port=8080"]` to `CMD ["pratc", "serve", "--host=0.0.0.0", "--port=8080"]`
  - No other changes needed

  **Must NOT do**: Do not change the port. Do not add extra environment variables here.

  **Recommended Agent Profile**:
  - **Category**: `quick` — one-line change to Dockerfile
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 5, 6)
  - **Blocks**: None
  - **Blocked By**: Task 1 (Go --host flag must be merged first so the flag exists)

  **References**:
  - `Dockerfile.cli:22` — current CMD line
  - `internal/cmd/root.go:432` — --host flag (Task 1 changes the default)

  **Acceptance Criteria**:
  - [ ] `docker build -f Dockerfile.cli -t pratc-cli:test .` → exit 0
  - [ ] `docker run --rm pratc-cli:test serve --help | grep host` → shows 0.0.0.0

  **QA Scenarios**:

  ```
  Scenario: Container serves on all interfaces
    Tool: Bash
    Preconditions: Image built with updated Dockerfile
    Steps:
      1. Run container: `docker run -d --name=test-cli pratc-cli:test serve`
      2. Wait 2s
      3. Inspect: `docker exec test-cli ss -tlnp | grep 8080`
      4. Cleanup: `docker rm -f test-cli`
    Expected Result: Listening on `0.0.0.0:8080` or `*:8080`
    Failure Indicators: Listening on 127.0.0.1:8080
    Evidence: .sisyphus/evidence/task-4-docker-host.txt
  ```

  **Commit**: YES (group with Tasks 5, 6)
  - Message: `fix(docker): serve on 0.0.0.0 inside container`
  - Files: `Dockerfile.cli`

---

- [x] 5. **docker-compose passes GH_TOKEN to pratc-cli**

  **What to do**:
  - Modify `docker-compose.yml:11-16` — add `GH_TOKEN: ${GH_TOKEN}` and `GITHUB_TOKEN: ${GH_TOKEN}` to the pratc-cli environment block
  - Add comment: `# GH_TOKEN/GITHUB_TOKEN: authenticated via host gh CLI (gh auth status)`
  - Keep existing `PRATC_ANALYSIS_BACKEND` as-is

  **Must NOT do**: Do not change port mappings. Do not add token to pratc-web.

  **Recommended Agent Profile**:
  - **Category**: `quick` — one-line addition to docker-compose environment block
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 6)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `docker-compose.yml:11-16` — current pratc-cli environment block
  - `internal/app/service.go:79-85` — reads GH_TOKEN via GITHUB_PAT/GH_TOKEN
  - `internal/sync/default_runner.go:75-86` — reads GITHUB_TOKEN

  **Acceptance Criteria**:
  - [ ] `docker-compose config` → valid YAML (no parse errors)
  - [ ] `docker-compose run --rm pratc-cli env | grep TOKEN` → shows GH_TOKEN value

  **QA Scenarios**:

  ```
  Scenario: docker-compose passes GH_TOKEN through to container
    Tool: Bash
    Preconditions: docker-compose.yml updated, GH_TOKEN exported or in .env
    Steps:
      1. Run in minimax-light profile: `docker-compose --profile minimax-light up -d pratc-cli`
      2. Inspect env inside container: `docker-compose exec pratc-cli env | grep TOKEN`
      3. Cleanup: `docker-compose down`
    Expected Result: GH_TOKEN and GITHUB_TOKEN are set inside container
    Failure Indicators: Env vars not present
    Evidence: .sisyphus/evidence/task-5-compose-token.txt
  ```

  **Commit**: YES (group with Tasks 4, 6)
  - Message: `fix(docker): thread GH_TOKEN through to pratc-cli container`
  - Files: `docker-compose.yml`

---

- [x] 6. **docker-compose uses real HTTP healthchecks**

  **What to do**:
  - Modify `docker-compose.yml:17-22` (pratc-cli healthcheck) — replace `test: ["CMD-SHELL", "test -x /usr/local/bin/pratc"]` with `test: ["CMD-SHELL", "curl -sf http://localhost:8080/api/health || exit 1"]`
  - Modify `docker-compose.yml:41-45` (pratc-web healthcheck) — replace `test: ["CMD-SHELL", "test -d /app"]` with `test: ["CMD-SHELL", "wget -qO- http://localhost:7788/ || exit 1"]`
  - Add `retries: 3`, `interval: 10s`, `start_period: 30s` for both
  - Note: python:3.11-slim-bookworm image already has curl available; alpine may need wget

  **Must NOT do**: Do not change port mappings. Do not change the depends_on condition (keep `service_healthy`).

  **Recommended Agent Profile**:
  - **Category**: `quick` — compose file healthcheck updates only
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 4, 5)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `docker-compose.yml:17-22` — current pratc-cli healthcheck
  - `docker-compose.yml:41-45` — current pratc-web healthcheck
  - `internal/cmd/root.go:534-547` — `/api/health` endpoint (returns JSON)
  - `web/next.config.js` — Next.js config (for web / endpoint)

  **Acceptance Criteria**:
  - [ ] `docker-compose config` → valid YAML
  - [ ] After startup: `docker-compose ps` shows both containers as "healthy"

  **QA Scenarios**:

  ```
  Scenario: pratc-cli healthcheck uses HTTP probe
    Tool: Bash
    Preconditions: Updated docker-compose.yml
    Steps:
      1. Start cli only: `docker-compose --profile minimax-light up -d pratc-cli`
      2. Wait for healthy: `docker-compose ps pratc-cli`
      3. Inspect: `docker inspect $(docker-compose ps -q pratc-cli) | jq '.[0].State.Health'`
      4. Cleanup: `docker-compose down`
    Expected Result: Health check status is "healthy"
    Failure Indicators: Still "starting" or unhealthy
    Evidence: .sisyphus/evidence/task-6-healthcheck.txt
  ```

  **Commit**: YES (group with Tasks 4, 5)
  - Message: `fix(docker): replace fake healthchecks with real HTTP probes`
  - Files: `docker-compose.yml`

---

- [x] 7. **ML subprocess PYTHONPATH and graceful fallback**

  **What to do**:
  - Modify `Dockerfile.cli:14` — add `ENV PYTHONPATH=/app/ml-service/src` after `COPY ml-service ./ml-service`
  - Add graceful fallback in `internal/ml/bridge.go` — when Python subprocess fails with an import-related error, retry once with `ML_BACKEND=local` injected into the subprocess environment
  - In `Bridge.invoke()`, if subprocess fails AND `ML_BACKEND` is not already `local`, retry once with `ML_BACKEND=local` in cmd.Env

  **Must NOT do**: Do not change ML response parsing logic. Do not change clustering/duplicate algorithms. Do not remove the Python subprocess path.

  **Recommended Agent Profile**:
  - **Category**: `medium` — modifies subprocess invocation logic; needs understanding of ML backend selection
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 8)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `internal/ml/bridge.go:126-149` — `invoke()` method (subprocess call site)
  - `internal/ml/bridge.go:204-221` — `findPython()` and `findMLServiceDir()` (auto-detection)
  - `ml-service/src/pratc_ml/providers/__init__.py:17-66` — `ML_BACKEND` and provider selection
  - `Dockerfile.cli:11-17` — current stage definition (python:3.11-slim-bookworm)
  - `internal/app/service.go:235-255` — ML fallback to heuristics in service layer

  **Acceptance Criteria**:
  - [ ] `go test -race ./internal/ml/... -v` → PASS
  - [ ] `python -m pratc_ml.cli --help` runs inside container (after PYTHONPATH fix)
  - [ ] Cluster analysis gracefully falls back to local when Python import fails

  **QA Scenarios**:

  ```
  Scenario: ML subprocess works inside container with PYTHONPATH
    Tool: Bash
    Preconditions: Updated Dockerfile.cli, PYTHONPATH set
    Steps:
      1. Build image: `docker build -f Dockerfile.cli -t pratc-cli:test .`
      2. Run python import test: `docker run --rm pratc-cli:test python -c "import pratc_ml; print('ok')"`
    Expected Result: Prints "ok" without error
    Failure Indicators: ModuleNotFoundError
    Evidence: .sisyphus/evidence/task-7-pythonpath.txt

  Scenario: ML call falls back to local when subprocess missing
    Tool: Bash
    Preconditions: Built binary, Python not in PATH
    Steps:
      1. Run analyze without Python: `PATH=/usr/bin PRATC_ANALYSIS_BACKEND=local /tmp/pratc analyze --repo=test/repo --format=json`
    Expected Result: Returns analyze result using heuristic clustering (no ML)
    Failure Indicators: Crashes or returns error
    Evidence: .sisyphus/evidence/task-7-ml-fallback.txt
  ```

  **Commit**: YES (group with Task 8)
  - Message: `fix(ml): set PYTHONPATH in container, add graceful ML fallback`
  - Files: `Dockerfile.cli`, `internal/ml/bridge.go`

---

- [x] 8. **Document required env vars and 5k test commands**

  **What to do**:
  - Create `.env.example` in repo root with all required env vars documented
  - Add to README.md a "5k PR Live Test" section with exact commands
  - Update top-level AGENTS.md env section to reflect GH_TOKEN as the primary var

  **Must NOT do**: Do not commit actual tokens or secrets. Do not change any Go code.

  **Recommended Agent Profile**:
  - **Category**: `writing` — documentation only
  - **Skills**: none required

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Task 7)
  - **Blocks**: None
  - **Blocked By**: None

  **References**:
  - `README.md:195-203` — current env var documentation
  - `internal/app/service.go:79-85` — GH_TOKEN, GITHUB_PAT, GH_TOKEN checked in order
  - `internal/sync/default_runner.go:75-86` — GITHUB_TOKEN for sync
  - `ml-service/src/pratc_ml/providers/__init__.py:17-66` — ML_BACKEND, MINIMAX_API_KEY, etc.

  **Acceptance Criteria**:
  - [ ] `.env.example` exists and documents `GH_TOKEN`, `ML_BACKEND=local`, `PRATC_DB_PATH`
  - [ ] README.md has a 5k test section with example commands

  **QA Scenarios**:

  ```
  Scenario: .env.example has correct token env var names
    Tool: Bash
    Preconditions: .env.example created
    Steps:
      1. Check file exists: `test -f .env.example && echo exists`
      2. Check content: `grep -E "GH_TOKEN|GITHUB_TOKEN|ML_BACKEND" .env.example`
    Expected Result: File exists, contains GH_TOKEN, GITHUB_TOKEN, ML_BACKEND
    Failure Indicators: File missing or wrong var names
    Evidence: .sisyphus/evidence/task-8-env-docs.txt
  ```

  **Commit**: YES (group with Task 7)
  - Message: `docs: add .env.example and 5k test guide`
  - Files: `.env.example`, `README.md`

---

## Final Verification Wave

- [x] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search codebase for forbidden patterns. Check evidence files exist.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | VERDICT: APPROVE/REJECT`

- [x] F2. **Code Quality Review** — `unspecified-high`
  Run `go build ./...` and `go vet ./...`. Review all changed files.
  Output: `Build [PASS/FAIL] | Vet [PASS/FAIL] | VERDICT`

- [x] F3. **Docker Integration Test** — `unspecified-high`
  Start full stack: `docker-compose --profile minimax-light up -d`. Wait for healthy. Verify:
  - `curl http://localhost:7780/api/health` returns 200 (ports changed to 7780/7781 to avoid host 8080 conflict)
  - `curl http://localhost:7781/` returns Next.js response
  - `docker-compose ps` shows both healthy
  Output: `API [PASS] | Web [PASS] | Health [PASS] | VERDICT: PASS`

- [x] F4. **CLI Smoke Test** — `unspecified-high`
  Run `pratc mirror --help`, `pratc sync --help`, `pratc serve --help`. Verify new flags present and defaults correct.
  Output: `Mirror [PASS/FAIL] | Sync [PASS/FAIL] | Serve [PASS/FAIL] | VERDICT`

---

## Commit Strategy

Grouped commits per wave:
- **Wave 1**: Tasks 1, 2, 3 — `fix(cmd): serve host, mirror prune safety, sync blocking`
- **Wave 2**: Tasks 4, 5, 6 — `fix(docker): serve on 0.0.0.0, token passthrough, HTTP healthchecks`
- **Wave 3**: Tasks 7, 8 — `fix(ml): PYTHONPATH, graceful fallback, env docs`

---

## Success Criteria

### Verification Commands
```bash
go build ./...                          # All packages compile
go test -race ./...                     # All tests pass
go vet ./...                            # No vet errors
docker build -f Dockerfile.cli -t pratc-cli:test .
docker-compose --profile minimax-light config  # Valid YAML
```

### Final Checklist
- [x] All "Must Have" present
- [x] All "Must NOT Have" absent
- [x] All tests pass
- [x] Evidence files captured for each task
