# prATC — PR Air Traffic Control

## TL;DR

> **Quick Summary**: Build prATC, an AI-powered, repo-agnostic, self-hostable open-source tool that brings combinatorial optimization to PR management — using a MAG40-inspired formula engine for merge plan selection, graph algorithms for merge ordering, NLP clustering for duplicate detection, and an Outlook-style web dashboard for triage.
> 
> **Deliverables**:
> - Go CLI (`pratc`) with `analyze`, `cluster`, `graph`, `plan`, `serve` commands
> - Python ML service (NLP clustering, duplicate detection, overlap heuristics)
> - TypeScript web dashboard (air traffic control view + Outlook inbox for triage)
> - Docker Compose for self-hosting
> - Formula engine (P(n,k), C(n,k), n^k modes) for merge plan candidate generation
> - Dependency graph engine with topological sort + DOT output
> - GitHub API client with rate-limit-aware incremental sync + SQLite cache
> 
> **Estimated Effort**: XL (polyglot greenfield — Go + Python + TypeScript)
> **Parallel Execution**: YES — 6 waves
> **Critical Path**: Task 0 → Task 1 → Task 3 → Task 13 → Task 14 → Task 18 → Task 19 → Task 20 → Final

---

## Context

### Original Request
User manages repositories with 5,500+ open PRs and no effective way to triage, deduplicate, or plan merges at scale. After analyzing 5,674 PRs across openclaw, opencode, and oh-my-openagent, the core problem was identified: PR management at scale is a combinatorial optimization problem (like air traffic control), not a linear queue. No production tool addresses this.

### Interview Summary
**Key Discussions**:
- **MAG40 connection**: User's BIP38 wallet recovery tool uses formula-based combinatorial generation (P(n,k), C(n,k), n^k) with tiered search. Direct architectural parallel to PR merge planning.
- **Architecture**: Hybrid — formula engine (MAG40-style) for subset selection + graph engine for merge ordering + learning component (v0.2)
- **Deployment**: Layered — CLI first → Web dashboard → GitHub App (v0.2+)
- **Tech stack**: Go CLI + Python ML service + TypeScript web dashboard
- **MVP scope**: All 4 capabilities at once (analysis/clustering, dependency graph, merge planner, dashboard)
- **UX**: Outlook-style inbox view for PR triage with action buttons + air traffic control dashboard
- **Actions**: Configurable — read-only default, action buttons log intent in v0.1
- **Test strategy**: TDD (go test / pytest / vitest)
- **GitHub API**: PAT via psst (5,000 req/hr), GraphQL preferred, incremental sync
- **Install policy**: User explicitly approved installing any required toolchains, runtimes, CLI tools, and optional QA utilities during execution
- **Sync execution**: GitHub sync should run as an in-process background worker, not a foreground blocking flow
- **ML backend**: Configurable — local ML or hosted OpenRouter-backed analysis
- **Hosted AI shape**: If hosted mode is enabled, use hosted embeddings for primary analysis plus GPT-5.4 for reasoning/explanations

**Research Findings**:
- No production tool exists for PR deduplication or conflict prediction
- Weave (tree-sitter semantic merge) achieves 31/31 vs Git's 15/31
- HDBSCAN + sentence-transformers validated for text clustering
- OR-Tools CP-SAT available for constraint satisfaction in merge ordering
- PR-DupliChecker (2024): BERT-based, 92% accuracy on 3,328 PRs validates approach

### Metis Review
**Identified Gaps** (addressed):
- **Data persistence**: SQLite for local cache (queryable, portable, handles concurrent reads with WAL mode)
- **Destructive action safety**: Dry-run mode mandatory, audit log, confirmation prompts
- **Duplicate/overlapping/conflicting definitions**: Explicit thresholds set (>90% embedding similarity = duplicate, 70-90% = overlapping, conflict edges in v0.1 come from GitHub mergeability + file-overlap heuristics)
- **First-run experience**: Progressive loading with partial results during initial sync
- **Rate limit exhaustion**: Incremental sync, REST fallback, progress persistence
- **Pre-filter before formula engine**: Never run combinatorial engine on raw 5,500 PR set — cluster → filter → score → then formula
- **Branch-aware analysis**: PRs targeting different base branches form separate merge universes
- **Bot PR handling**: Auto-detect Dependabot/Renovate PRs, mark as batch-merge-safe
- **Tooling may be missing locally**: Plan now includes explicit bootstrap/install + verification steps for required and optional tooling

---

## Work Objectives

### Core Objective
Build prATC v0.1 — a self-hostable, repo-agnostic CLI + web dashboard that applies combinatorial optimization to PR management, enabling solo maintainers to effectively triage, deduplicate, and plan merges for repositories with thousands of open PRs.

### Concrete Deliverables
- `pratc` Go binary with 5 commands: `analyze`, `cluster`, `graph`, `plan`, `serve`
- `pratc-ml` Python service callable via subprocess (JSON stdin/stdout)
- Web dashboard (Next.js) served separately in dev (`bun run dev`) or self-hosted via Docker Compose
- Docker Compose stack for self-hosting
- SQLite-based local cache with incremental sync
- In-process background sync worker with persisted job/progress state
- Formula engine implementing P(n,k), C(n,k), n^k modes
- Dependency graph engine with DOT output
- Configurable analysis backend: local embeddings/HDBSCAN or hosted embeddings + GPT-5.4 reasoning via OpenRouter
- Frozen test fixtures from real PR data

### Execution Constraints
- The current repository root is the project root. All implementation paths are root-relative unless explicitly stated otherwise.
- Do not create a nested `pratc/` directory inside the current repository.
- `.sisyphus/plans/pratc.md` is the source-of-truth build plan.
- `AGENTS.md` is the execution contract for agent workflow, testing discipline, merge discipline, and reporting.
- Multi-agent execution uses one worktree or feature branch per task by default.
- No task is complete until it is merged into `main` and verification passes on `main`.
- If post-merge verification fails, agents must fix forward immediately and rerun verification on `main`.

### Definition of Done
- [ ] `pratc analyze --repo=owner/repo` outputs JSON with PR categories, clusters, duplicates, conflicts
- [ ] `pratc cluster --repo=owner/repo` outputs cluster assignments with similarity scores
- [ ] `pratc graph --repo=owner/repo` outputs DOT graph of PR dependencies/conflicts
- [ ] `pratc plan --repo=owner/repo --target=20` outputs ranked merge plan for top 20 PRs
- [ ] `pratc serve --port=8080` launches the Go API on localhost:8080
- [ ] Web dashboard is reachable on localhost:3000 via `bun run dev` or either Docker profile
- [ ] `docker compose --profile local-ml up` starts full local stack (`pratc-cli` API + local ML runtime + `pratc-web` dashboard)
- [ ] `docker compose --profile minimax-light up` starts lighter hosted-AI stack (`pratc-cli` API + `pratc-web` dashboard, no local model download required)
- [ ] All tests pass: `make test` (go test + pytest + vitest)
- [ ] Formula engine produces correct values validated against MAG40 hand-calculations

### Output Contracts
- `pratc analyze --repo=owner/repo --format=json` exits `0` and returns JSON with keys: `repo`, `generatedAt`, `counts`, `clusters`, `duplicates`, `overlaps`, `conflicts`, `stalenessSignals`.
- `pratc cluster --repo=owner/repo --format=json` exits `0` and returns JSON with keys: `repo`, `generatedAt`, `model`, `thresholds`, `clusters`.
- `pratc graph --repo=owner/repo --format=dot` exits `0`, emits non-empty DOT, and includes `digraph`.
- `pratc plan --repo=owner/repo --target=20 --format=json` exits `0` and returns JSON with keys: `repo`, `generatedAt`, `target`, `candidatePoolSize`, `strategy`, `selected`, `ordering`, `rejections`.
- `pratc serve --port=8080` must expose `/healthz` with HTTP `200` and JSON keys `status` and `version`.
- Invalid arguments must exit `2`. Runtime failures must exit `1`.
- Warm-cache runtime targets on the `~5,500` PR fixture-scale dataset: `analyze <= 300s`, `cluster <= 180s`, `graph <= 120s`, `plan <= 90s`.

### Must Have
- Rate-limit-aware GitHub API client with incremental sync
- In-process background sync jobs with persisted progress + resumability
- SQLite cache for PR data persistence
- Formula engine with 3 modes (permutation, combination, with_replacement)
- Pre-filter pipeline before formula engine (cluster → CI-status → conflict-check → score)
- Configurable ML backend (local subprocess by default; hosted OpenRouter mode optional)
- Dependency graph with topological sort
- Merge plan generation combining formula + graph engines
- Web dashboard with table view (6 columns) + 3 action buttons
- Outlook-style inbox view for sequential PR triage
- DOT graph output for CLI
- D3.js interactive graph for web dashboard
- Docker Compose with health checks
- Dry-run mode for all actions
- Frozen JSON fixtures for unit tests
- Makefile as unified build orchestrator
- Explicit output contracts, exit codes, and runtime budgets for CLI/API deliverables
- SQLite schema versioning with forward migrations
- Post-merge verification on `main` for all agent-delivered work

### Must NOT Have (Guardrails)
- ❌ GitHub App / OAuth flow (v0.2+)
- ❌ Webhook/real-time event processing (v0.2+)
- ❌ ML feedback/learning loop (v0.2+ — needs usage data first)
- ❌ Multi-repo aggregate view (v0.2+)
- ❌ gRPC (use subprocess JSON in v0.1)
- ❌ Keyboard shortcuts in dashboard
- ❌ Saved views / advanced filtering / search in dashboard
- ❌ User authentication in dashboard (single-user, localhost)
- ❌ CI/CD pipeline (GitHub Actions)
- ❌ Automatic PR action execution — dashboard buttons log intent, CLI has `--dry-run` default
- ❌ Tree-sitter semantic merge resolution (detection only via Weave CLI as optional dep)
- ❌ Custom scoring function configuration (hardcode sensible defaults)
- ❌ Nx, Turborepo, or any JS-based monorepo tool
- ❌ Over-abstraction, premature generalization, or excessive interface layering
- ❌ Console.log in production code, empty catch blocks, `as any` / `@ts-ignore`

---

## Verification Strategy

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.
> Acceptance criteria requiring "user manually tests/confirms" are FORBIDDEN.

### Test Decision
- **Infrastructure exists**: NO (greenfield project)
- **Automated tests**: YES (TDD — RED → GREEN → REFACTOR)
- **Frameworks**: Go: `go test -race -v` / Python: `uv run pytest -v` / TypeScript: `vitest`
- **TDD Protocol**: Test file written and failing BEFORE implementation. Every task includes test cases.
- **ML test tiers**: Fast unit tests (mocked models, `@pytest.mark.unit`) + slow integration tests (real models, `@pytest.mark.slow`)
- **Install policy**: Executor may install anything required to satisfy the plan (Go, Python, uv, Node, bun, Docker, psst, gh, jq, tmux, Playwright browsers, optional graphviz)

### Non-Functional Targets
- Initial sync on a cold cache for a repository with `~5,500` open PRs should complete in `<= 20 min`.
- Incremental refresh on a warm cache with `<5%` changed PRs should complete in `<= 3 min`.
- Planner candidate generation and final ordering for `target=20` should complete in `<= 90s`.
- API p95 latency on a warm cache should be `<= 5s` for `/analyze`, `<= 3s` for `/cluster`, `<= 2s` for `/graph`, and `<= 2s` for `/plan`.
- CLI `analyze` memory ceiling on the fixture-scale dataset should be `<= 2.5 GB RSS`.

### Fixture Policy
- A small, sanitized, stable fixture set must be committed under `fixtures/` for deterministic unit and integration tests.
- Large raw captures from live repositories must not be committed as normal source files; they should be reproducible via documented capture commands and stored as generated artifacts or regenerated on demand.
- Fixture capture must record source repo, capture date, capture command, and sanitization rules in evidence.
- Secrets, tokens, and private-only metadata must never be stored in fixtures.
- Fixture refreshes must create versioned snapshots rather than mutating historical fixtures in place.

### Hosted AI Operational Policy
- Hosted mode must define request timeouts, retry limits, and explicit degraded behavior when the provider is unavailable.
- Hosted analysis must fail closed into a documented fallback path rather than hanging indefinitely.
- Local mode remains the default path and must support all v0.1 capabilities.
- Hosted mode must log provider, model, timeout, retry count, and fallback decision for each run.

### Contract Generation Policy
- `contracts/` is generated from the canonical Go and TypeScript response/request models rather than hand-authored first.
- Generated contracts must be reproducible from checked-in source code and committed so CLI, API, and web agents share a stable interface.
- If a generated contract changes, the owning task must update all affected tests and consumers in the same change set.

### SQLite Migration Policy
- SQLite must include a `schema_migrations` table with columns `version`, `name`, and `applied_at`.
- Migrations must be forward-only.
- `PRAGMA user_version` must match the latest applied migration.
- Startup must fail fast if the on-disk schema version is newer than the running binary supports.
- Migration tests must verify upgrades from fresh DB, `N-1`, and `N-2`.

### QA Policy
Every task MUST include agent-executed QA scenarios (see TODO template below).
Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **CLI**: Use Bash — run command, assert exit code, parse JSON output with `jq`
- **Go tests**: Use Bash — `go test -race -v ./internal/{pkg}/...`, assert PASS
- **Python tests**: Use Bash — `uv run pytest ml-service/tests/ -v`, assert passed
- **Web dashboard**: Use Playwright (playwright skill) — Navigate, interact, assert DOM, screenshot
- **API endpoints**: Use Bash (curl) — Send requests, assert status + response fields
- **Docker**: Use Bash — `docker compose up -d`, health check endpoints, `docker compose down`
- **Post-merge gate**: After merge to `main`, rerun required verification on `main`; if anything fails, fix forward immediately and rerun until green

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 0 (Preflight — toolchain bootstrap):
├── Task 0: Toolchain bootstrap + environment preflight [quick]

Wave 1 (Foundation — scaffolding, types, config):
├── Task 1: Monorepo scaffold + Makefile + Docker Compose skeleton [quick]
├── Task 2: Go CLI skeleton (Cobra commands, internal/ layout) [quick]
├── Task 3: GitHub API client + SQLite cache (TDD) [deep]
├── Task 4: Python ML service scaffold (pyproject.toml, uv, pytest) [quick]
├── Task 5: TypeScript web dashboard scaffold (Next.js, vitest) [quick]
├── Task 6: Shared types + JSON interface contracts [quick]
└── Task 7: Frozen test fixtures from real PR data [quick]

Wave 2 (Core Engines — MAX PARALLEL):
├── Task 8: Formula engine — P(n,k), C(n,k), n^k modes (TDD) [deep] (depends: 2, 6)
├── Task 9: Graph engine — dependency/conflict graph + topological sort (TDD) [deep] (depends: 2, 6)
├── Task 10: NLP clustering — configurable local or hosted backend (TDD) [deep] (depends: 4, 6, 7)
├── Task 11: Duplicate detection — embedding similarity + thresholds (TDD) [deep] (depends: 4, 6, 7)
├── Task 12: Staleness analyzer — superseded change detection (TDD) [unspecified-high] (depends: 2, 6, 7)
└── Task 13: Pre-filter pipeline — cluster→CI→conflict→score (TDD) [deep] (depends: 2, 6)

Wave 3 (Integration — engines combine):
├── Task 14: Merge planner — formula + graph + pre-filter integration (TDD) [deep] (depends: 8, 9, 13)
├── Task 15: CLI `analyze` command — full analysis pipeline [unspecified-high] (depends: 3, 10, 11, 12)
├── Task 16: CLI `cluster` command — clustering output [quick] (depends: 3, 10)
├── Task 17: CLI `graph` command — DOT output [quick] (depends: 3, 9)
├── Task 18: CLI `plan` command — merge plan output [unspecified-high] (depends: 3, 14)
└── Task 19: CLI `serve` command — HTTP API mode [unspecified-high] (depends: 3, 15, 16, 17, 18)

Wave 4 (Dashboard — UI):
├── Task 20: Dashboard layout + API integration [visual-engineering] (depends: 19)
├── Task 21: Air traffic control view — PR cluster visualization [visual-engineering] (depends: 20)
├── Task 22: Outlook inbox view — sequential triage with action buttons [visual-engineering] (depends: 20)
├── Task 23: Interactive dependency graph (D3.js) [visual-engineering] (depends: 20)
└── Task 24: Merge plan view — recommended merge set + ordering [visual-engineering] (depends: 20)

Wave 5 (Polish — Docker, docs, edge cases):
├── Task 25: Docker Compose full stack + health checks [quick] (depends: 19, 20)
├── Task 26: First-run experience — progressive loading + progress UI [unspecified-high] (depends: 3, 20)
├── Task 27: Edge case handling — bot PRs, draft PRs, branch-awareness [unspecified-high] (depends: 15)
├── Task 28: Dry-run mode + audit log for actions [unspecified-high] (depends: 19, 22)
└── Task 29: README + setup documentation [writing] (depends: 25)

Wave FINAL (Verification — 4 parallel review agents):
├── Task F1: Plan compliance audit (oracle)
├── Task F2: Code quality review (unspecified-high)
├── Task F3: Real QA — Playwright + CLI (unspecified-high + playwright)
└── Task F4: Scope fidelity check (deep)

Critical Path: Task 0 → Task 1 → Task 3 → Task 13 → Task 14 → Task 18 → Task 19 → Task 20 → F1-F4
Parallel Speedup: ~65% faster than sequential
Max Concurrent: 6 (Wave 2)
```

### Agent Coordination Rules
- Coordinator agents may only dispatch tasks whose dependencies are already complete.
- Each task should have a single owning worker agent unless the plan explicitly defines a split.
- Workers must report changed files, commands run, tests run, evidence paths, and residual risks.
- Integrator agents own merge-to-`main`, post-merge verification on `main`, and any required fix-forward work.
- Review agents verify plan compliance, regression risk, and evidence quality rather than re-implementing features.

### Swarm Operating Model (`1` Coordinator + up to `3` Builders)
- The coordinator also acts as integrator by default.
- Builder allocation should prioritize the critical path first, then fill remaining capacity with disjoint supporting tasks.
- Do not use all 3 builders on overlapping integration-heavy phases unless file ownership is explicit and conflict risk is low.
- Recommended execution pattern:
  - Phase A: Builder 1 -> Task 1, Builder 2 -> Task 4 or 5, Builder 3 -> Task 6
  - Phase B: Builder 1 -> Task 3, Builder 2 -> Task 2, Builder 3 -> Task 7
  - Phase C: Builder 1 -> Task 13, Builder 2 -> Task 8 or 9, Builder 3 -> Task 10 or 11
  - Phase D: Builder 1 -> Task 14 then 18, Builder 2 -> Task 15 or 17, Builder 3 -> Task 16 or Task 20 prep if dependencies are satisfied
- During high-conflict phases, cap active builders at `2` if needed to preserve merge velocity and mainline stability.
- The coordinator merges after each meaningful completed task rather than allowing large unintegrated branch queues to accumulate.
- Every builder handoff must include changed files, tests run, evidence paths, known merge risks, and any dependency assumptions.

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 0 | — | 1-7,25,29,F3 | 0 |
| 1 | 0 | 2,4,5,25 | 1 |
| 2 | 0,1 | 8,9,12,13,15-19 | 1 |
| 3 | 0,1 | 15,16,17,18,19,26,28 | 1 |
| 4 | 0,1 | 10,11 | 1 |
| 5 | 0,1 | 20 | 1 |
| 6 | 0 | 8,9,10,11,12,13 | 1 |
| 7 | 0 | 10,11,12 | 1 |
| 8 | 2,6 | 14 | 2 |
| 9 | 2,6 | 14,17 | 2 |
| 10 | 4,6,7 | 15,16 | 2 |
| 11 | 4,6,7 | 15 | 2 |
| 12 | 2,6,7 | 15 | 2 |
| 13 | 2,6 | 14 | 2 |
| 14 | 8,9,13 | 18 | 3 |
| 15 | 3,10,11,12 | 19,27 | 3 |
| 16 | 3,10 | 19 | 3 |
| 17 | 3,9 | 19 | 3 |
| 18 | 3,14 | 19 | 3 |
| 19 | 3,15,16,17,18 | 20,25,28 | 3 |
| 20 | 5,19 | 21,22,23,24,26 | 4 |
| 21 | 20 | F3 | 4 |
| 22 | 20 | 28,F3 | 4 |
| 23 | 20 | F3 | 4 |
| 24 | 20 | F3 | 4 |
| 25 | 1,19,20 | 29 | 5 |
| 26 | 3,20 | F3 | 5 |
| 27 | 15 | F4 | 5 |
| 28 | 19,22 | F4 | 5 |
| 29 | 25 | F1 | 5 |
| F1-F4 | ALL | — | FINAL |

### Agent Dispatch Summary

- **Wave 0**: 1 task — T0→`quick`
- **Wave 1**: 7 tasks — T1→`quick`, T2→`quick`, T3→`deep`, T4→`quick`, T5→`quick`, T6→`quick`, T7→`quick`
- **Wave 2**: 6 tasks — T8→`deep`, T9→`deep`, T10→`deep`, T11→`deep`, T12→`unspecified-high`, T13→`deep`
- **Wave 3**: 6 tasks — T14→`deep`, T15→`unspecified-high`, T16→`quick`, T17→`quick`, T18→`unspecified-high`, T19→`unspecified-high`
- **Wave 4**: 5 tasks — T20-T24→`visual-engineering`
- **Wave 5**: 5 tasks — T25→`quick`, T26→`unspecified-high`, T27→`unspecified-high`, T28→`unspecified-high`, T29→`writing`
- **FINAL**: 4 tasks — F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`+`playwright`, F4→`deep`

---

## TODOs

- [ ] 0. Toolchain Bootstrap + Environment Preflight

  **What to do**:
  - Install and verify required tooling before any code work begins:
    - Go 1.23+
    - Python 3.11+
    - `uv`
    - Node 20+
    - `bun`
    - Docker + Docker Compose plugin
    - `psst`
    - GitHub CLI (`gh`)
    - `jq`
  - Install and verify optional-but-planned QA utilities:
    - `tmux` (for CLI/TUI QA scenarios)
    - Playwright browser binaries (`bunx playwright install --with-deps` or platform equivalent)
    - `graphviz` (`dot`) if available; if unavailable, use syntax validation path already described in graph QA
  - Initialize secrets/tool auth needed by later tasks:
    - `psst init` if required by the local setup
    - `psst set GITHUB_PAT` with a valid GitHub PAT
    - `gh auth status` succeeds (for Task 7 fixture capture)
  - Capture and verify versions with explicit commands before entering Wave 1

  **Must NOT do**:
  - No application code changes
  - No repo scaffolding yet
  - No fallback to manually pasting secrets in chat or files

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Deterministic environment setup and verification, no product logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 0 (must complete before all implementation waves)
  - **Blocks**: Tasks 1-7, 25, 29, F3
  - **Blocked By**: None (can start immediately)

  **References**:
  - `/Users/jeffersonnunn/AGENTS.md` — `psst` usage rules and secret handling expectations
  - `https://docs.github.com/en/get-started/git-basics/caching-your-github-credentials-in-git` — PAT/CLI auth expectations
  - `https://playwright.dev/docs/browsers` — Browser binary installation for QA

  **Acceptance Criteria**:
  - [ ] `go version` shows 1.23+
  - [ ] `python3 --version` shows 3.11+
  - [ ] `uv --version`, `node --version`, `bun --version`, `docker --version`, `docker compose version`, `psst list`, `gh auth status`, `jq --version` all succeed
  - [ ] `tmux -V` succeeds
  - [ ] `bunx playwright --version` succeeds after browser install

  **QA Scenarios**:

  ```
  Scenario: Required toolchain is installed and reachable
    Tool: Bash
    Preconditions: Fresh machine or newly prepared dev environment
    Steps:
      1. Run `go version && python3 --version && uv --version && node --version && bun --version`
      2. Run `docker --version && docker compose version && psst list && gh auth status && jq --version`
      3. Assert each command exits 0 and prints a version or auth summary
    Expected Result: All required tools are installed, on PATH, and usable
    Failure Indicators: command not found, auth failure, unsupported version
    Evidence: .sisyphus/evidence/task-0-toolchain.txt

  Scenario: Optional QA tooling is ready
    Tool: Bash
    Preconditions: Toolchain install complete
    Steps:
      1. Run `tmux -V`
      2. Run `bunx playwright --version`
      3. Run `dot -V` if graphviz was installed; otherwise record explicit skip note in evidence
    Expected Result: tmux and Playwright are ready; graphviz status is explicitly known
    Failure Indicators: command not found, Playwright missing browsers, ambiguous graphviz availability
    Evidence: .sisyphus/evidence/task-0-qa-tooling.txt
  ```

  **Commit**: NO

- [ ] 1. Monorepo Scaffold + Makefile + Docker Compose Skeleton

  **What to do**:
  - Create repository-root directory structure:
    ```
    .
    ├── cmd/pratc/main.go          # Minimal main → cmd.Execute()
    ├── internal/                  # Go packages (follow crush pattern)
    ├── ml-service/                # Python ML service
    │   ├── pyproject.toml
    │   ├── src/pratc_ml/
    │   └── tests/
    ├── web/                       # Next.js dashboard
    │   ├── package.json
    │   └── src/
    ├── fixtures/                  # Frozen test data
    ├── Makefile                   # Unified build orchestrator
    ├── docker-compose.yml         # Full stack
    ├── Dockerfile.cli             # Go CLI + Python runtime for subprocess ML
    ├── Dockerfile.web             # Next.js
    └── README.md
    ```
  - Makefile targets: `deps`, `verify-env`, `dev`, `build`, `test`, `test-go`, `test-python`, `test-web`, `lint`, `docker-up`, `docker-down`, `clean`
  - Docker Compose skeleton with 2 services: `pratc-cli`, `pratc-web` — placeholder configs, health check stubs
  - Define 2 runtime profiles from the start:
    - `local-ml` — default self-hosted local model path
    - `minimax-light` — hosted analysis path with lighter container/runtime requirements
  - `go mod init github.com/jeffersonnunn/pratc`
  - Verify: `make build` succeeds (even if binaries do nothing yet)

  **Must NOT do**:
  - No Nx, Turborepo, or JS-based monorepo tooling
  - No actual business logic — scaffold only
  - No CI/CD pipeline files

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Scaffolding with no business logic, file creation only
  - **Skills**: [`git-master`]
    - `git-master`: Atomic initial commit with proper .gitignore

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 2-7)
  - **Blocks**: Tasks 2, 4, 5, 25
  - **Blocked By**: Task 0 (toolchain preflight)

  **References**:
  - `/Users/jeffersonnunn/crush/` — Go project layout convention (minimal main.go, internal/ packages, Cobra CLI). Copy the `internal/` directory structure pattern.
  - `/Users/jeffersonnunn/crush/internal/cmd/` — How cmd.Execute() is wired from main.go
  - `/Users/jeffersonnunn/hermes-agent/pyproject.toml` — Python project setup with uv, optional dep groups, CLI entry points
  - `/Users/jeffersonnunn/zackor/docker-compose.yml` — Docker Compose patterns with health checks, named volumes, profiles
  - `/Users/jeffersonnunn/mag40/FORMULA.md:317-330` — JSON config schema pattern for formula definitions

  **Acceptance Criteria**:
  - [ ] `make verify-env` exits 0
  - [ ] `make build` exits 0 (all three ecosystems compile/install)
  - [ ] `make test` exits 0 (placeholder tests pass)
  - [ ] `docker compose config` validates without errors
  - [ ] Directory structure matches spec above
  - [ ] `go vet ./...` passes in the repository root

  **QA Scenarios**:

  ```
  Scenario: Makefile targets work
    Tool: Bash
    Preconditions: Fresh clone of the repository root, Task 0 complete
    Steps:
      1. Run `make verify-env` — expect exit code 0
      2. Run `make build` — expect exit code 0
      3. Run `make test` — expect exit code 0
      4. Run `make lint` — expect exit code 0
      5. Run `make clean` — expect exit code 0, verify bin/ removed
    Expected Result: All make targets exit 0
    Failure Indicators: Non-zero exit code, missing targets, env verification missing required tools
    Evidence: .sisyphus/evidence/task-1-makefile-targets.txt

  Scenario: Docker Compose validates
    Tool: Bash
    Preconditions: Docker running
    Steps:
      1. Run `docker compose config` in the repository root
      2. Verify 2 services listed: pratc-cli, pratc-web
    Expected Result: Valid compose file with 2 services
    Failure Indicators: YAML parse errors, missing services
    Evidence: .sisyphus/evidence/task-1-docker-compose-validate.txt
  ```

  **Commit**: YES
  - Message: `chore(infra): scaffold monorepo with Makefile and Docker Compose`
  - Files: repository root scaffolding files
  - Pre-commit: `make lint`

- [ ] 2. Go CLI Skeleton (Cobra Commands + internal/ Layout)

  **What to do**:
  - Set up Cobra CLI with root command and 5 subcommands: `analyze`, `cluster`, `graph`, `plan`, `serve`
  - Each command has placeholder Run function that prints "not implemented yet"
  - Follow crush pattern: `cmd/pratc/main.go` → `internal/cmd/root.go` → `internal/cmd/analyze.go`, etc.
  - Add global flags: `--repo` (required for analysis commands), `--format` (json/table/dot), `--verbose`, `--dry-run`
  - Add `internal/` package stubs: `github/`, `cache/`, `analysis/`, `formula/`, `graph/`, `filter/`, `planner/`, `staleness/`, `actions/`, `server/`
  - Add `go.sum` with Cobra dependency

  **Must NOT do**:
  - No actual command implementation — stubs only
  - No GitHub API calls
  - No tests yet (Task-specific tests come with implementation)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Scaffolding with Cobra boilerplate, no logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 3-7)
  - **Blocks**: Tasks 8, 9, 12, 13, 15-19
  - **Blocked By**: Task 1 (needs go.mod)

  **References**:
  - `/Users/jeffersonnunn/crush/internal/cmd/` — Cobra command structure, root.go pattern
  - `/Users/jeffersonnunn/crush/internal/` — Package layout convention (27+ packages)
  - `/Users/jeffersonnunn/crush/main.go` — Minimal main.go delegating to cmd.Execute()

  **Acceptance Criteria**:
  - [ ] `go build ./cmd/pratc/` produces binary
  - [ ] `./pratc --help` shows 5 subcommands
  - [ ] `./pratc analyze --help` shows `--repo` flag
  - [ ] `./pratc analyze --repo=test` prints "not implemented yet" and exits 0

  **QA Scenarios**:

  ```
  Scenario: CLI help output shows all commands
    Tool: Bash
    Preconditions: Binary built
    Steps:
      1. Run `./bin/pratc --help`
      2. Assert output contains "analyze", "cluster", "graph", "plan", "serve"
      3. Run `./bin/pratc analyze --help`
      4. Assert output contains "--repo"
    Expected Result: All 5 commands visible, --repo flag documented
    Failure Indicators: Missing commands, missing flags
    Evidence: .sisyphus/evidence/task-2-cli-help.txt

  Scenario: Stub commands exit cleanly
    Tool: Bash
    Preconditions: Binary built
    Steps:
      1. Run `./bin/pratc analyze --repo=test` — expect exit 0
      2. Run `./bin/pratc cluster --repo=test` — expect exit 0
      3. Run `./bin/pratc graph --repo=test` — expect exit 0
      4. Run `./bin/pratc plan --repo=test` — expect exit 0
    Expected Result: All exit 0 with "not implemented" message
    Failure Indicators: Non-zero exit, panic, missing subcommand
    Evidence: .sisyphus/evidence/task-2-cli-stubs.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): scaffold Go CLI with Cobra commands`
  - Files: `cmd/pratc/`, `internal/cmd/`, `internal/*/`
  - Pre-commit: `go vet ./...`

- [ ] 3. GitHub API Client + SQLite Cache (TDD)

  **What to do**:
  - **RED**: Write tests first for:
    - `github.Client`: Fetch PRs (paginated), fetch PR files, fetch PR reviews, fetch CI status
    - Rate limit handling: back-off when near limit, resume after reset
    - Incremental sync: only fetch PRs updated since last sync timestamp
    - `cache.Store`: SQLite operations — upsert PR, query by filter, get last sync time
    - Minimal merged-PR metadata cache needed for staleness signals: merged PR number, mergedAt, touched files
    - Background sync job state: create job, persist cursor/progress, resume job, mark complete/failed
    - GraphQL query construction with cursor pagination
  - **GREEN**: Implement:
    - `internal/github/client.go` — GitHub GraphQL API client with rate-limit awareness
    - `internal/github/queries.go` — GraphQL query templates for PRs, files, reviews, CI
    - `internal/cache/sqlite.go` — SQLite store with WAL mode, PR table, sync metadata
    - `internal/cache/models.go` — Go structs matching JSON contracts from Task 6
    - REST fallback for initial bulk fetch (1 req per page of 100, within rate limits)
    - Progress reporting callback for long syncs
    - Explicit SQLite initialization including `PRAGMA journal_mode=WAL;`
    - Minimal merged-PR metadata persistence to support Task 12 superseded-change detection without git checkout
    - Background sync job persistence (`sync_jobs` + progress metadata) for in-process worker execution
  - **REFACTOR**: Extract rate-limit logic into reusable middleware

  **Must NOT do**:
  - No actual GitHub API calls in unit tests — use httptest mock server
  - No gRPC
  - No GitHub App auth — PAT only via `GITHUB_PAT` env var (psst-compatible)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Rate-limit-aware API client + SQLite cache is complex, needs careful TDD, concurrent access patterns
  - **Skills**: [`superpowers/test-driven-development`]
    - `test-driven-development`: Formula-critical module, must follow strict RED→GREEN→REFACTOR

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1, 2, 4-7)
  - **Blocks**: Tasks 15, 16, 17, 18, 19, 26
  - **Blocked By**: Task 1 (needs go.mod + SQLite dependency)

  **References**:
  - GitHub GraphQL API: `https://docs.github.com/en/graphql/reference/objects#pullrequest` — PR fields: `number`, `title`, `body`, `state`, `mergeable`, `changedFiles`, `additions`, `deletions`, `createdAt`, `updatedAt`, `headRefName`, `baseRefName`, `author`, `labels`, `reviewDecision`, `statusCheckRollup`
  - GitHub REST API: `https://docs.github.com/en/rest/pulls/pulls#list-pull-requests` — Fallback for bulk fetch, 100 per page
  - SQLite WAL mode: `https://www.sqlite.org/wal.html` — Concurrent read/write pattern
  - `/Users/jeffersonnunn/crush/internal/db/` — If exists, follow SQLite patterns from crush
  - Rate limit headers: `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/github/...` → PASS (8+ tests)
  - [ ] `go test -race -v ./internal/cache/...` → PASS (6+ tests)
  - [ ] Tests cover: pagination, rate limiting, incremental sync, SQLite CRUD, WAL mode
  - [ ] Cache supports merged PR metadata required by Task 12 (`mergedAt` + touched files)
  - [ ] Cache supports persisted sync job progress/resume state for background worker mode
  - [ ] Mock server tests don't make real HTTP calls

  **QA Scenarios**:

  ```
  Scenario: SQLite cache persists and retrieves PR data
    Tool: Bash
    Preconditions: Go tests passing
    Steps:
      1. Run `go test -race -v ./internal/cache/... -run TestCacheUpsertAndQuery`
      2. Assert test creates temp SQLite file, upserts 10 PRs, queries by filter, gets correct count
    Expected Result: PASS — 10 PRs inserted, queries return correct subsets
    Failure Indicators: SQLite lock errors, wrong counts, WAL mode failures
    Evidence: .sisyphus/evidence/task-3-cache-tests.txt

  Scenario: Rate limit backoff triggers correctly
    Tool: Bash
    Preconditions: Mock server configured to return rate-limit headers
    Steps:
      1. Run `go test -race -v ./internal/github/... -run TestRateLimitBackoff`
      2. Assert client waits when X-RateLimit-Remaining < 10
      3. Assert client retries after X-RateLimit-Reset timestamp
    Expected Result: PASS — backoff and retry logic verified
    Failure Indicators: No backoff, infinite loop, wrong timing
    Evidence: .sisyphus/evidence/task-3-rate-limit-tests.txt

  Scenario: Rate limit exceeded returns error (not hang)
    Tool: Bash
    Preconditions: Mock server returns 403 with rate limit exceeded
    Steps:
      1. Run `go test -race -v ./internal/github/... -run TestRateLimitExceeded`
      2. Assert client returns descriptive error, does not hang
    Expected Result: Error returned with remaining time info
    Failure Indicators: Hang, panic, generic error without context
    Evidence: .sisyphus/evidence/task-3-rate-limit-exceeded.txt
  ```

  **Commit**: YES
  - Message: `feat(github): add rate-limit-aware API client with SQLite cache`
  - Files: `internal/github/`, `internal/cache/`
  - Pre-commit: `go test -race ./internal/github/... ./internal/cache/...`

- [ ] 4. Python ML Service Scaffold

  **What to do**:
  - Set up Python project with `uv`:
    - `pyproject.toml` with project metadata, dependencies, optional groups `[dev]`, `[ml]`
    - Dependencies: `sentence-transformers`, `hdbscan`, `scikit-learn`, `numpy`, `pydantic`
    - Dev dependencies: `pytest`, `pytest-cov`, `ruff`
    - Run `uv sync` after declaring dependencies so the environment is reproducible immediately
    - Add provider configuration surface for `ML_BACKEND=local|openrouter`, `OPENROUTER_API_KEY`, `OPENROUTER_EMBED_MODEL`, and `OPENROUTER_REASON_MODEL`
  - Create package structure:
    ```
    ml-service/
    ├── pyproject.toml
    ├── src/pratc_ml/
    │   ├── __init__.py
    │   ├── cli.py              # JSON stdin/stdout interface
    │   ├── clustering.py       # NLP clustering orchestration (stub)
    │   ├── duplicates.py       # Duplicate detection orchestration (stub)
    │   ├── providers/          # backend adapters (local/openrouter)
    │   ├── overlap.py          # file-overlap heuristics (stub)
    │   └── models.py           # Pydantic models matching JSON contracts
    └── tests/
        ├── conftest.py
        ├── test_clustering.py  # Placeholder
        ├── test_duplicates.py  # Placeholder
        └── test_overlap.py     # Placeholder
    ```
  - `cli.py` reads JSON from stdin, routes to appropriate function, writes JSON to stdout
  - Add pytest markers: `@pytest.mark.unit`, `@pytest.mark.slow` (for ML model tests)
  - Verify: `uv run pytest` passes (placeholder tests)

  **Must NOT do**:
  - No actual ML model loading yet — stubs only
  - No gRPC or HTTP server — subprocess JSON only
  - No FastAPI

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Python scaffolding with stubs, no business logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-3, 5-7)
  - **Blocks**: Tasks 10, 11
  - **Blocked By**: Task 1 (needs monorepo directory)

  **References**:
  - `/Users/jeffersonnunn/hermes-agent/pyproject.toml` — Python project setup with uv, optional dep groups, pytest config
  - JSON interface contract from Task 6 — Pydantic models must match Go structs
  - OpenRouter docs: `https://openrouter.ai/docs` — Hosted provider env/config expectations for optional hosted mode

  **Acceptance Criteria**:
  - [ ] `uv sync` exits 0
  - [ ] `uv run pytest ml-service/tests/ -v` → all placeholder tests PASS
  - [ ] `echo '{"action":"health"}' | uv run python -m pratc_ml.cli` → `{"status":"ok"}`
  - [ ] `uv run ruff check ml-service/` → no errors

  **QA Scenarios**:

  ```
  Scenario: ML service responds to health check via stdin
    Tool: Bash
    Preconditions: Task 0 complete, `uv sync` already run
    Steps:
      1. Run `echo '{"action":"health"}' | uv run python -m pratc_ml.cli`
      2. Parse output as JSON
      3. Assert `.status` == "ok"
    Expected Result: JSON response with status ok
    Failure Indicators: Import errors, non-JSON output, missing cli.py
    Evidence: .sisyphus/evidence/task-4-ml-health.txt

  Scenario: ML service returns error for unknown action
    Tool: Bash
    Preconditions: Task 0 complete, `uv sync` already run
    Steps:
      1. Run `echo '{"action":"nonexistent"}' | uv run python -m pratc_ml.cli`
      2. Assert exit code is non-zero OR JSON contains `.error`
    Expected Result: Graceful error message, not a stack trace
    Failure Indicators: Unhandled exception, stack trace on stdout
    Evidence: .sisyphus/evidence/task-4-ml-unknown-action.txt
  ```

  **Commit**: YES
  - Message: `chore(ml): scaffold Python ML service with uv + pytest`
  - Files: `ml-service/`
  - Pre-commit: `uv run pytest ml-service/tests/`

- [ ] 5. TypeScript Web Dashboard Scaffold (Next.js + Vitest)

  **What to do**:
  - Initialize Next.js 14+ app in `web/`:
    - App Router, TypeScript, Tailwind CSS
    - Vitest + React Testing Library for tests
    - TanStack Table dependency (for Outlook inbox view)
    - D3.js dependency (for graph visualization)
    - Run `bun install` after package scaffolding so all dependencies are present before tests/build
  - Create page stubs:
    ```
    web/src/app/
    ├── layout.tsx              # Root layout with nav sidebar
    ├── page.tsx                # Dashboard overview (redirect to /analysis)
    ├── analysis/page.tsx       # Air traffic control view (stub)
    ├── inbox/page.tsx          # Outlook triage view (stub)
    ├── graph/page.tsx          # Dependency graph view (stub)
    └── plan/page.tsx           # Merge plan view (stub)
    ```
  - Add API client stub in `web/src/lib/api.ts` — typed fetch wrapper for Go API
  - Add placeholder tests for each page (renders without crash)
  - Configure vitest with `@testing-library/react`

  **Must NOT do**:
  - No actual API calls — stubs with mock data
  - No keyboard shortcuts, saved views, search, or advanced filtering
  - No user authentication
  - No SSR data fetching (static stubs)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Next.js scaffolding with stubs
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Need clean layout structure even for stubs

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-4, 6-7)
  - **Blocks**: Task 20
  - **Blocked By**: Task 1 (needs monorepo directory)

  **References**:
  - Next.js App Router docs: `https://nextjs.org/docs/app` — routing, layouts, server components
  - TanStack Table: `https://tanstack.com/table` — headless table for inbox view
  - D3.js: `https://d3js.org/` — graph visualization

  **Acceptance Criteria**:
  - [ ] `bun install` exits 0
  - [ ] `bun test` → all placeholder tests PASS
  - [ ] `bun run build` → build succeeds
  - [ ] `bun run dev` → localhost:3000 renders dashboard skeleton
  - [ ] 4 page routes accessible: /analysis, /inbox, /graph, /plan

  **QA Scenarios**:

  ```
  Scenario: Dashboard renders on localhost
    Tool: Playwright (playwright skill)
    Preconditions: `bun run dev` running on localhost:3000
    Steps:
      1. Navigate to http://localhost:3000
      2. Assert page title contains "prATC"
      3. Assert nav sidebar visible with 4 links
      4. Navigate to /inbox — assert page renders
      5. Navigate to /graph — assert page renders
      6. Take screenshot
    Expected Result: All 4 pages render without errors
    Failure Indicators: 404, React error boundary, blank page
    Evidence: .sisyphus/evidence/task-5-dashboard-scaffold.png

  Scenario: Build produces static output
    Tool: Bash
    Preconditions: Dependencies installed
    Steps:
      1. Run `bun run build` in web/
      2. Assert exit code 0
      3. Assert .next/ directory created
    Expected Result: Successful build with no TypeScript errors
    Failure Indicators: Type errors, missing imports, build failure
    Evidence: .sisyphus/evidence/task-5-build-output.txt
  ```

  **Commit**: YES
  - Message: `chore(web): scaffold Next.js dashboard with vitest`
  - Files: `web/`
  - Pre-commit: `bun test`

- [ ] 6. Shared Types + JSON Interface Contracts

  **What to do**:
  - Define the JSON schema that Go CLI and Python ML service communicate over:
    - **Request types**: `ClusterRequest`, `DuplicateDetectionRequest`, `SemanticAnalysisRequest`
    - **Response types**: `ClusterResponse`, `DuplicateResponse`, `SemanticConflictResponse`
    - **Shared models**: `PR`, `PRCluster`, `DuplicateGroup`, `ConflictPair`, `MergePlan`, `MergePlanCandidate`, `StalenessReport`
  - Implement in Go: `internal/types/models.go` — Go structs with `json:` tags
  - Implement in Python: `ml-service/src/pratc_ml/models.py` — Pydantic models
  - Implement in TypeScript: `web/src/types/api.ts` — TypeScript interfaces
  - All three must produce/consume identical JSON shapes
  - Add a shared `contracts/` directory with JSON Schema files (`.json`) as the source of truth
  - Write a validation test that serializes Go → JSON → Python → JSON → compare

  **Must NOT do**:
  - No protobuf/gRPC — plain JSON
  - No runtime schema validation in production (only in tests)

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Type definitions across 3 languages, no business logic
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-5, 7)
  - **Blocks**: Tasks 8, 9, 10, 11, 12, 13
  - **Blocked By**: None (can start immediately)

  **References**:
  - `/Users/jeffersonnunn/mag40/FORMULA.md:317-330` — JSON config schema pattern
  - JSON Schema spec: `https://json-schema.org/` — for contracts/
  - GitHub PR API response shape: `https://docs.github.com/en/graphql/reference/objects#pullrequest`

  **Acceptance Criteria**:
  - [ ] `contracts/*.json` files validate with JSON Schema validator
  - [ ] Go structs serialize to JSON matching schema
  - [ ] Python Pydantic models serialize to JSON matching schema
  - [ ] TypeScript interfaces compile against schema
  - [ ] Cross-language round-trip test passes

  **QA Scenarios**:

  ```
  Scenario: Go and Python produce identical JSON for same data
    Tool: Bash
    Preconditions: Both Go and Python stubs exist
    Steps:
      1. Run Go test that serializes a sample PR to JSON
      2. Run Python test that serializes same data to JSON
      3. Compare outputs with `jq --sort-keys`
      4. Assert identical
    Expected Result: Byte-identical JSON after key sorting
    Failure Indicators: Missing fields, type mismatches, naming differences
    Evidence: .sisyphus/evidence/task-6-cross-language-json.txt

  Scenario: Invalid data rejected by schema
    Tool: Bash
    Preconditions: JSON Schema files exist
    Steps:
      1. Feed malformed JSON to schema validator
      2. Assert validation fails with specific error
    Expected Result: Schema catches missing required fields
    Failure Indicators: Silent acceptance of invalid data
    Evidence: .sisyphus/evidence/task-6-schema-validation.txt
  ```

  **Commit**: YES
  - Message: `feat(types): define shared JSON interface contracts`
  - Files: `contracts/`, `internal/types/`, `ml-service/src/pratc_ml/models.py`, `web/src/types/`
  - Pre-commit: `go vet ./...`

- [ ] 7. Frozen Test Fixtures from Real PR Data

  **What to do**:
  - Create frozen JSON fixture files from real PR data (from our analysis):
    - `fixtures/opencode-42prs.json` — All 42 opencode PRs with full metadata
    - `fixtures/openclaw-sample-200.json` — 200 representative openclaw PRs (sampled across categories)
    - `fixtures/small-5prs.json` — 5 PRs for unit test fast execution
    - `fixtures/edge-cases.json` — Edge cases: bot PR (Dependabot), draft PR, conflicting PR, stale PR (6+ months), PR targeting non-main branch
  - Each fixture includes: PR number, title, body, author, labels, files changed, review status, CI status, created/updated dates, base/head branches, mergeable state
  - Fetch real data via `gh api` and freeze it (strip any sensitive data)
  - Add Go helper: `internal/testutil/fixtures.go` — load fixtures as typed structs
  - Add Python helper: `ml-service/tests/conftest.py` — pytest fixtures loading JSON

  **Must NOT do**:
  - No live API calls in unit tests — fixtures only
  - No fixture files >10MB (keep manageable)
  - No credentials or tokens in fixture data

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Data fetching + file creation, straightforward
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Tasks 1-6)
  - **Blocks**: Tasks 10, 11, 12
  - **Blocked By**: None (can start immediately — uses gh CLI directly)

  **References**:
  - `/Users/jeffersonnunn/.sisyphus/plans/pr-review-analysis.md` — Complete categorized PR data from our analysis. Use PR numbers listed here to fetch specific PRs for fixtures.
  - GitHub CLI: `gh api repos/opencode-ai/opencode/pulls --paginate` — fetch PR data
  - Edge cases identified by Metis: bot PRs (Dependabot), draft PRs, multi-branch PRs, stale PRs

  **Acceptance Criteria**:
  - [ ] All 4 fixture files exist and contain valid JSON
  - [ ] `fixtures/opencode-42prs.json` has exactly 42 PR objects
  - [ ] `fixtures/edge-cases.json` has PRs for each edge case type
  - [ ] Go testutil loads fixtures successfully: `go test -v ./internal/testutil/...`
  - [ ] Python conftest loads fixtures: `uv run pytest --co ml-service/tests/` (collect only)

  **QA Scenarios**:

  ```
  Scenario: Fixture files are valid and complete
    Tool: Bash
    Preconditions: Fixture files exist
    Steps:
      1. Run `jq '. | length' fixtures/opencode-42prs.json` — assert 42
      2. Run `jq '.[0] | keys' fixtures/opencode-42prs.json` — assert contains "number", "title", "body", "files", "author"
      3. Run `jq '. | length' fixtures/edge-cases.json` — assert >= 5
      4. Run `jq '.[] | select(.author.login == "dependabot[bot]")' fixtures/edge-cases.json` — assert 1 result
    Expected Result: All fixtures valid, complete, contain expected data
    Failure Indicators: Wrong count, missing fields, no bot PR in edge cases
    Evidence: .sisyphus/evidence/task-7-fixture-validation.txt

  Scenario: Go and Python can load fixtures
    Tool: Bash
    Preconditions: Fixture files + loader code exist
    Steps:
      1. Run `go test -v ./internal/testutil/... -run TestLoadFixtures`
      2. Run `uv run pytest ml-service/tests/ -k test_load_fixtures -v`
    Expected Result: Both pass, correct PR counts loaded
    Failure Indicators: Deserialization errors, wrong types, file not found
    Evidence: .sisyphus/evidence/task-7-fixture-loaders.txt
  ```

  **Commit**: YES
  - Message: `test(fixtures): add frozen PR data fixtures from real repos`
  - Files: `fixtures/`, `internal/testutil/`, `ml-service/tests/conftest.py`
  - Pre-commit: —

- [ ] 8. Formula Engine — P(n,k), C(n,k), n^k Modes (TDD)

  **What to do**:
  - **RED**: Write exhaustive tests first in `internal/formula/formula_test.go`:
    - `TestPermutation`: P(54,4) = 7,590,024 (from MAG40 FORMULA.md)
    - `TestCombination`: C(54,5) = 3,162,510 (from MAG40 FORMULA.md)
    - `TestWithReplacement`: 54^4 = 8,503,056
    - `TestGenerateByIndex`: Given index N, generate the Nth candidate without enumerating 0..N-1
    - `TestScoringFunction`: Given a merge plan candidate (set of PRs), compute fitness score
    - `TestTieredSearch`: Tier 1 (independent, no conflicts) → Tier 2 (dependent chains) → Tier 3 (conflict resolution)
    - `TestPreFilteredInput`: Formula engine receives pre-filtered candidate pool, not raw PR set
    - Edge cases: n=0, k=0, k>n, single PR, all PRs conflicting
  - **GREEN**: Implement `internal/formula/`:
    - `engine.go` — Core formula engine with mode selection
    - `modes.go` — P(n,k), C(n,k), n^k implementations with index-based generation
    - `scoring.go` — Fitness function: weight(age, size, CI_status, review_status, conflict_count, cluster_coherence)
    - `tiers.go` — Tiered search strategy (quick → thorough → exhaustive)
    - `config.go` — Formula configuration (matches MAG40 JSON schema pattern)
  - **REFACTOR**: Ensure all math uses big.Int where overflow possible

  **Must NOT do**:
  - No GitHub API calls — formula engine is pure math + scoring
  - No ML model dependencies — scoring uses pre-computed features only
  - No storage — generate candidates on-the-fly by index

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Math-heavy combinatorial engine, core novel component, needs meticulous TDD
  - **Skills**: [`superpowers/test-driven-development`]
    - `test-driven-development`: Formula correctness is critical — validated against MAG40 hand-calculations

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 9-13)
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 2 (Go CLI skeleton), 6 (shared types)

  **References**:
  - `/Users/jeffersonnunn/mag40/FORMULA.md` — **PRIMARY REFERENCE**: Formula definitions for P(n,k), C(n,k), n^k. JSON config schema at lines 317-330. Expected values for validation: P(54,4)=7,590,024, C(54,5)=3,162,510. Tier structure at lines 103-163.
  - `/Users/jeffersonnunn/mag40/FORMULA.md:36-44` — P(n,k) formula: n!/(n-k)! = n×(n-1)×...×(n-k+1)
  - `/Users/jeffersonnunn/mag40/FORMULA.md:55-63` — C(n,k) formula: n!/(k!×(n-k)!)
  - `/Users/jeffersonnunn/mag40/FORMULA.md:68-79` — n^k formula with replacement
  - `/Users/jeffersonnunn/mag40/FORMULA.md:270-298` — Quick reference table and total formula

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/formula/...` → PASS (12+ tests)
  - [ ] P(54,4) == 7,590,024 (exact match)
  - [ ] C(54,5) == 3,162,510 (exact match)
  - [ ] Index-based generation: `GenerateByIndex(mode, n, k, idx)` returns correct candidate
  - [ ] Scoring function produces deterministic output for same inputs
  - [ ] Tiered search explores Tier 1 before Tier 2 before Tier 3

  **QA Scenarios**:

  ```
  Scenario: Formula engine matches MAG40 hand-calculations
    Tool: Bash
    Preconditions: Formula engine implemented
    Steps:
      1. Run `go test -race -v ./internal/formula/... -run TestPermutation`
      2. Assert P(54,4) == 7,590,024
      3. Run `go test -race -v ./internal/formula/... -run TestCombination`
      4. Assert C(54,5) == 3,162,510
      5. Run `go test -race -v ./internal/formula/... -run TestWithReplacement`
      6. Assert 54^4 == 8,503,056
    Expected Result: All three modes produce exact expected values
    Failure Indicators: Off-by-one, overflow, wrong formula
    Evidence: .sisyphus/evidence/task-8-formula-validation.txt

  Scenario: Index-based generation avoids enumeration
    Tool: Bash
    Preconditions: Formula engine implemented
    Steps:
      1. Run test that generates candidate at index 1,000,000 directly
      2. Assert it completes in <1ms (no enumeration of 0..999,999)
      3. Assert candidate at index 0 differs from index 1
      4. Assert total count matches expected formula value
    Expected Result: O(1) index lookup, not O(n) enumeration
    Failure Indicators: Slow execution, memory allocation proportional to index
    Evidence: .sisyphus/evidence/task-8-index-generation.txt

  Scenario: Edge case — k > n returns zero candidates
    Tool: Bash
    Steps:
      1. Run `go test -race -v ./internal/formula/... -run TestEdgeCases`
      2. Assert P(3,5) == 0 (can't pick 5 from 3 without replacement)
      3. Assert C(3,5) == 0
      4. Assert 0^k == 0 for any k>0
    Expected Result: Graceful handling of impossible combinations
    Failure Indicators: Panic, negative numbers, division by zero
    Evidence: .sisyphus/evidence/task-8-edge-cases.txt
  ```

  **Commit**: YES
  - Message: `feat(formula): implement combinatorial merge plan engine`
  - Files: `internal/formula/`
  - Pre-commit: `go test -race ./internal/formula/...`

- [ ] 9. Graph Engine — PR Dependency/Conflict Graph + Topological Sort (TDD)

  **What to do**:
  - **RED**: Write tests first in `internal/graph/graph_test.go`:
    - `TestBuildGraph`: Given PRs with file overlap data, build adjacency graph
    - `TestFileOverlap`: Jaccard similarity calculation for changed files
    - `TestConflictEdges`: PRs modifying same files get conflict edges with weight
    - `TestDependencyEdges`: Stacked PRs (branch ancestry) get dependency edges
    - `TestTopologicalSort`: Given dependency graph, produce valid merge ordering
    - `TestCycleDetection`: Circular dependencies detected and reported
    - `TestIndependentSubgraphs`: Identify groups of PRs with no inter-dependencies
    - `TestDOTOutput`: Graph serializes to valid DOT format
    - `TestBranchAwareness`: PRs targeting different base branches form separate subgraphs
  - **GREEN**: Implement `internal/graph/`:
    - `graph.go` — Graph data structure (adjacency list, weighted edges)
    - `builder.go` — Build graph from PR data (file overlap, branch ancestry)
    - `similarity.go` — Jaccard similarity, file-path overlap scoring
    - `topo.go` — Topological sort (Kahn's algorithm) with priority tie-breaking
    - `dot.go` — DOT format serializer for Graphviz
    - `subgraph.go` — Connected component extraction, branch-aware partitioning
  - **REFACTOR**: Optimize for large graphs (5,500 nodes)

  **Must NOT do**:
  - No AST/semantic analysis — file-path level only; deeper semantic conflict detection is deferred beyond v0.1
  - No Graphviz rendering — output DOT text only (rendering is user's responsibility or web dashboard)
  - No external graph library — implement with Go stdlib

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Graph algorithms (topological sort, cycle detection, connected components) require careful implementation
  - **Skills**: [`superpowers/test-driven-development`]
    - `test-driven-development`: Graph correctness is critical for merge ordering safety

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8, 10-13)
  - **Blocks**: Tasks 14, 17
  - **Blocked By**: Tasks 2 (Go CLI skeleton), 6 (shared types)

  **References**:
  - Kahn's algorithm for topological sort: `https://en.wikipedia.org/wiki/Topological_sorting#Kahn's_algorithm`
  - DOT language spec: `https://graphviz.org/doc/info/lang.html`
  - Jaccard similarity: `https://en.wikipedia.org/wiki/Jaccard_index` — |A∩B|/|A∪B| for file sets
  - Metis research findings: Depviz (153 stars) for graph visualization patterns, pr-graph-generator for branch relationship patterns

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/graph/...` → PASS (10+ tests)
  - [ ] Topological sort produces valid ordering (every edge goes from earlier to later)
  - [ ] Cycle detection identifies and reports circular dependencies
  - [ ] DOT output is valid (parseable by `dot -Tsvg`)
  - [ ] 5,500-node graph completes in <5 seconds

  **QA Scenarios**:

  ```
  Scenario: Topological sort produces valid merge ordering
    Tool: Bash
    Steps:
      1. Run `go test -race -v ./internal/graph/... -run TestTopologicalSort`
      2. Test with known graph: A→B, A→C, B→D, C→D
      3. Assert A appears before B and C, B and C appear before D
    Expected Result: Valid topological ordering respecting all edges
    Failure Indicators: Dependency violation, missing nodes
    Evidence: .sisyphus/evidence/task-9-topo-sort.txt

  Scenario: DOT output is valid Graphviz
    Tool: Bash
    Steps:
      1. Run `go test -race -v ./internal/graph/... -run TestDOTOutput`
      2. Pipe DOT output to `dot -Tsvg` (if graphviz installed) or validate format
      3. Assert starts with "digraph {" and ends with "}"
    Expected Result: Valid DOT format
    Failure Indicators: Parse errors, missing edges
    Evidence: .sisyphus/evidence/task-9-dot-output.txt

  Scenario: Cycle detection catches circular dependencies
    Tool: Bash
    Steps:
      1. Run `go test -race -v ./internal/graph/... -run TestCycleDetection`
      2. Test with cycle: A→B, B→C, C→A
      3. Assert error returned with cycle path
    Expected Result: Error identifies cycle [A, B, C, A]
    Failure Indicators: No error (infinite loop), wrong cycle path
    Evidence: .sisyphus/evidence/task-9-cycle-detection.txt
  ```

  **Commit**: YES
  - Message: `feat(graph): implement PR dependency graph + topological sort`
  - Files: `internal/graph/`
  - Pre-commit: `go test -race ./internal/graph/...`

- [ ] 10. NLP Clustering — Configurable Local or Hosted Backend (TDD)

  **What to do**:
  - **RED**: Write tests in `ml-service/tests/test_clustering.py`:
    - `test_embed_pr_titles`: Embedding generation for PR title+body text
    - `test_cluster_similar_prs`: Known-similar PRs cluster together
    - `test_cluster_dissimilar_prs`: Known-different PRs end up in different clusters
    - `test_noise_handling`: Unique PRs labeled as noise (-1), not forced into clusters
    - `test_empty_input`: Empty PR list returns empty clusters
    - `test_large_input`: 500 PRs clustered in <30 seconds
    - `test_cluster_labels`: Each cluster gets a descriptive label (most common words)
  - **GREEN**: Implement `ml-service/src/pratc_ml/clustering.py`:
    - Add provider abstraction: `local` or `openrouter`
    - **Local mode**: load `all-MiniLM-L6-v2` model (80MB, fast), embed PR titles + first 200 chars of body, cluster with HDBSCAN
    - **Hosted mode**: call hosted embeddings provider for vectors, then cluster locally with HDBSCAN; use GPT-5.4 via OpenRouter for cluster labeling/explanations
    - Reduce dimensions with UMAP (optional, for >1000 PRs)
    - Return JSON: `{clusters: [{id, label, prs: [pr_numbers], centroid_similarity, backend}]}`
  - **REFACTOR**: Cache embeddings to disk (avoid re-computing on repeat runs)

  **Must NOT do**:
  - No fine-tuning or custom model training
  - No file-content analysis (NLP is title+body only; overlap heuristics stay in Task 11)
  - No GraphCodeBERT (overkill for v0.1 — use all-MiniLM-L6-v2)
  - No provider-specific business logic leaking into CLI/UI — keep provider choice behind a stable interface

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: ML pipeline with model loading, embedding, clustering — needs careful testing
  - **Skills**: [`superpowers/test-driven-development`]
    - `test-driven-development`: Clustering correctness validated against known-similar PR pairs

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8, 9, 11-13)
  - **Blocks**: Tasks 15, 16
  - **Blocked By**: Tasks 4 (Python scaffold), 6 (shared types), 7 (test fixtures)

  **References**:
  - sentence-transformers: `https://www.sbert.net/` — `all-MiniLM-L6-v2` model for local mode
  - HDBSCAN: `https://hdbscan.readthedocs.io/` — density-based clustering
  - HuggingFace text-clustering: `https://github.com/huggingface/text-clustering` — UMAP + HDBSCAN pattern
  - Librarian research: PR-DupliChecker (2024) achieved 92% accuracy with BERT embeddings on 3,328 PRs
  - OpenRouter docs: `https://openrouter.ai/docs` — hosted model routing for GPT-5.4 reasoning path

  **Acceptance Criteria**:
  - [ ] `uv run pytest ml-service/tests/test_clustering.py -v` → PASS (7+ tests)
  - [ ] First real clustering run documents or proves model download/cache path behavior
  - [ ] `ML_BACKEND=local` and `ML_BACKEND=openrouter` both produce contract-valid output using mocked provider tests
  - [ ] Known-similar PRs (e.g., 3 "fix cron false positive" PRs from openclaw) cluster together
  - [ ] Clustering 500 PRs completes in <30 seconds
  - [ ] Each cluster has a descriptive label
  - [ ] Output JSON matches contract from Task 6

  **QA Scenarios**:

  ```
  Scenario: Known-duplicate PRs cluster together
    Tool: Bash
    Preconditions: ML dependencies installed, fixtures loaded, backend configured
    Steps:
      1. Feed fixtures/openclaw-sample-200.json to clustering endpoint
      2. Run `echo '{"action":"cluster","prs":[...]}' | uv run python -m pratc_ml.cli`
      3. Find cluster containing "cron false positive" PRs
      4. Assert all 3 cron fix PRs are in same cluster
    Expected Result: Known-similar PRs grouped, dissimilar PRs separated
    Failure Indicators: Similar PRs in different clusters, all PRs in one cluster
    Evidence: .sisyphus/evidence/task-10-clustering-known-dupes.txt

  Scenario: Empty input returns empty clusters
    Tool: Bash
    Steps:
      1. Run `echo '{"action":"cluster","prs":[]}' | uv run python -m pratc_ml.cli`
      2. Assert output is `{"clusters":[]}`
    Expected Result: Graceful empty response
    Failure Indicators: Error, null, crash
    Evidence: .sisyphus/evidence/task-10-empty-input.txt

  Scenario: Hosted backend returns contract-valid clusters
    Tool: Bash
    Preconditions: `ML_BACKEND=openrouter`, provider calls mocked in test environment
    Steps:
      1. Run `uv run pytest ml-service/tests/test_clustering.py -k openrouter -v`
      2. Assert mocked hosted embeddings path is exercised
      3. Assert output JSON still matches local-mode contract
    Expected Result: Hosted mode works behind same interface without changing callers
    Failure Indicators: Provider-specific response leaks, contract mismatch, missing backend label
    Evidence: .sisyphus/evidence/task-10-openrouter-backend.txt
  ```

  **Commit**: YES
  - Message: `feat(ml): implement configurable clustering backend (local or hosted)`
  - Files: `ml-service/src/pratc_ml/clustering.py`, `ml-service/tests/test_clustering.py`
  - Pre-commit: `uv run pytest ml-service/tests/test_clustering.py`

- [ ] 11. Duplicate Detection — Embedding Similarity + Thresholds (TDD)

  **What to do**:
  - **RED**: Write tests in `ml-service/tests/test_duplicates.py`:
    - `test_exact_duplicate`: PRs with identical titles flagged as duplicates (>90% similarity)
    - `test_near_duplicate`: PRs with similar titles+bodies flagged (>80% similarity)
    - `test_overlapping_not_duplicate`: PRs modifying same files but different intent (50-80% similarity)
    - `test_unrelated`: Clearly different PRs score <50% similarity
    - `test_threshold_configurable`: Threshold can be adjusted
    - `test_duplicate_groups`: Multiple duplicates form groups (not pairs)
    - `test_file_overlap_augmentation`: File overlap boosts similarity score
  - **GREEN**: Implement `ml-service/src/pratc_ml/duplicates.py`:
    - Compute pairwise cosine similarity of PR embeddings (reuse from clustering provider abstraction)
    - Augment with file-path Jaccard overlap (weighted blend: 70% NLP + 30% file overlap)
    - Apply thresholds: >90% = DUPLICATE, 70-90% = OVERLAPPING, <70% = INDEPENDENT
    - Group duplicates transitively (if A≈B and B≈C, group {A,B,C})
    - In hosted mode, use hosted embeddings for primary similarity and GPT-5.4 only for explanation/tie-break text, not pairwise brute force reasoning
    - Return JSON: `{duplicate_groups: [{prs: [numbers], similarity, recommendation: "close"|"review", backend}]}`
  - **REFACTOR**: Use sparse matrix for pairwise similarity (memory efficient for 5,500 PRs)

  **Must NOT do**:
  - No automatic closing of duplicates — detection and recommendation only
  - No AST-level duplicate detection (that's semantic conflict territory)

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Pairwise similarity at scale needs efficient implementation + careful thresholding
  - **Skills**: [`superpowers/test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8-10, 12-13)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 4 (Python scaffold), 6 (shared types), 7 (test fixtures)

  **References**:
  - Cosine similarity: `https://scikit-learn.org/stable/modules/generated/sklearn.metrics.pairwise.cosine_similarity.html`
  - Jaccard index: |A∩B|/|A∪B| for file path sets
  - Librarian research: PR-DupliChecker (2024) — BERT-based, 92% accuracy. Our approach augments NLP with file overlap.
  - `/Users/jeffersonnunn/.sisyphus/plans/pr-review-analysis.md` — Documented duplicate patterns: "3x cron false positive fixes, 3x tool card sidebar fixes, 3x Matrix allowFrom fixes" in openclaw

  **Acceptance Criteria**:
  - [ ] `uv run pytest ml-service/tests/test_duplicates.py -v` → PASS (7+ tests)
  - [ ] Known duplicate PRs from openclaw analysis correctly grouped
  - [ ] Threshold boundaries produce expected classifications
  - [ ] Local and hosted backends produce the same response shape and threshold semantics
  - [ ] Output JSON matches contract from Task 6

  **QA Scenarios**:

  ```
  Scenario: Known openclaw duplicates detected
    Tool: Bash
    Preconditions: Fixtures loaded with known duplicate sets
    Steps:
      1. Feed openclaw fixture data with known "cron false positive" duplicate PRs
      2. Assert duplicate group contains all 3 PRs
      3. Assert similarity score >0.90
      4. Assert recommendation is "close" (keep newest, close older)
    Expected Result: All known duplicates detected with correct recommendations
    Failure Indicators: Missed duplicates, false positives on unrelated PRs
    Evidence: .sisyphus/evidence/task-11-known-duplicates.txt

  Scenario: Unrelated PRs not flagged as duplicates
    Tool: Bash
    Steps:
      1. Feed PRs with clearly different titles/files (bug fix vs new feature)
      2. Assert similarity <0.50
      3. Assert no duplicate groups formed
    Expected Result: No false positive duplicate flags
    Failure Indicators: Unrelated PRs grouped together
    Evidence: .sisyphus/evidence/task-11-no-false-positives.txt
  ```

  **Commit**: YES
  - Message: `feat(ml): implement duplicate PR detection via embedding similarity`
  - Files: `ml-service/src/pratc_ml/duplicates.py`, `ml-service/tests/test_duplicates.py`
  - Pre-commit: `uv run pytest ml-service/tests/test_duplicates.py`

- [ ] 12. Staleness Analyzer — Superseded Change Detection (TDD)

  **What to do**:
  - **RED**: Write tests in `internal/staleness/staleness_test.go`:
    - `TestTimeBasedStaleness`: PR not updated in >30 days flagged stale
    - `TestSupersededByMergedPR`: PR's changed files overlap with a merged PR since creation using cached merged PR metadata from Task 3
    - `TestLinkedIssueClosed`: PR references an issue that was closed by another PR
    - `TestConflictStaleness`: PR has merge conflicts that weren't present at creation
    - `TestBotPRStaleness`: Dependabot PR for old version when newer version available
    - `TestNotStale`: Active PR with recent updates, no conflicts, issue still open
    - `TestStalenessScore`: Combined staleness score (0-100) from multiple signals
  - **GREEN**: Implement `internal/staleness/`:
    - `analyzer.go` — Main staleness analysis engine
    - `signals.go` — Individual staleness signals (time, superseded via cached merged PR metadata, issue, conflict, bot)
    - `scorer.go` — Combine signals into 0-100 staleness score
    - `report.go` — Generate staleness report with reasons and recommendations
  - Each signal has a weight and produces a sub-score

  **Must NOT do**:
  - No automatic closing of stale PRs — scoring and recommendations only
  - No git clone or checkout — work from cached PR metadata only
  - No ML/NLP — rule-based analysis using PR metadata

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Multi-signal analysis with scoring, moderate complexity
  - **Skills**: [`superpowers/test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8-11, 13)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 2 (Go CLI skeleton), 6 (shared types), 7 (test fixtures)

  **References**:
  - GitHub Actions/stale: `https://github.com/actions/stale` — Time-based staleness detection (what we're improving upon)
  - Librarian research: Superseded change detection via entity tracking, problem resolution checks
  - `/Users/jeffersonnunn/.sisyphus/plans/pr-review-analysis.md` — "oldest PRs from April 2025 (10+ months stale)" in opencode

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/staleness/...` → PASS (7+ tests)
  - [ ] Time-based staleness correctly identifies PRs >30 days old
  - [ ] Superseded detection works when cached data includes merged PR file lists
  - [ ] Staleness score is 0-100 with deterministic calculation

  **QA Scenarios**:

  ```
  Scenario: Stale PR correctly scored
    Tool: Bash
    Steps:
      1. Run staleness test with PR from April 2025 fixture (10+ months old)
      2. Assert staleness score >80 (very stale)
      3. Assert reasons include "inactive for N days"
    Expected Result: High staleness score with specific reasons
    Failure Indicators: Low score for clearly stale PR, missing reasons
    Evidence: .sisyphus/evidence/task-12-stale-scoring.txt

  Scenario: Active PR not flagged stale
    Tool: Bash
    Steps:
      1. Run staleness test with recently updated PR (updated today)
      2. Assert staleness score <20
      3. Assert no staleness reasons
    Expected Result: Low staleness score
    Failure Indicators: Active PR flagged as stale
    Evidence: .sisyphus/evidence/task-12-active-pr.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement staleness analyzer for superseded PRs`
  - Files: `internal/staleness/`
  - Pre-commit: `go test -race ./internal/staleness/...`

- [ ] 13. Pre-Filter Pipeline — Cluster → CI → Conflict → Score (TDD)

  **What to do**:
  - **RED**: Write tests in `internal/filter/filter_test.go`:
    - `TestFilterByCIStatus`: Only CI-passing PRs included in candidate pool
    - `TestFilterByConflict`: PRs with merge conflicts excluded from default pool
    - `TestFilterByCluster`: Filter to specific cluster ID
    - `TestFilterByStaleness`: PRs with staleness >80 excluded by default
    - `TestScoring`: Multi-factor score: age_weight×age + ci_weight×ci + review_weight×reviews + size_weight×size
    - `TestPipelineChaining`: Filters chain: raw → cluster_filter → ci_filter → conflict_filter → staleness_filter → score → sort
    - `TestCandidatePoolSize`: Output pool capped at configurable max (default 200)
    - `TestEmptyAfterFilter`: All PRs filtered out → return empty pool with explanation
  - **GREEN**: Implement `internal/filter/`:
    - `pipeline.go` — Composable filter pipeline (functional chain pattern)
    - `filters.go` — Individual filters: CI status, conflict state, staleness threshold, cluster ID, base branch
    - `scorer.go` — Multi-factor PR scoring function
    - `pool.go` — Candidate pool with cap, sorting, and metadata
  - Pre-filter reduces raw PR set (5,500) to tractable candidate pool (50-200) for formula engine

  **Must NOT do**:
  - No ML/NLP calls — uses pre-computed cluster assignments and cached metadata
  - No GitHub API calls — works from cached data only
  - No actual merging or conflict resolution

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Pipeline architecture with composable filters, critical path for merge planner
  - **Skills**: [`superpowers/test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 8-12)
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 2 (Go CLI skeleton), 6 (shared types)

  **References**:
  - Metis directive: "MUST pre-filter candidate pool before combinatorial engine. Never run formula engine on raw 5,500 PR set."
  - Pipeline pattern: Go functional options / middleware chaining
  - Scoring function weights: age(0.2) + CI(0.3) + reviews(0.2) + size(0.1) + staleness_inverse(0.2) — configurable

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/filter/...` → PASS (8+ tests)
  - [ ] Pipeline reduces 200-PR fixture to <50 candidates after all filters
  - [ ] Scoring produces deterministic ordering for same inputs
  - [ ] Empty result includes explanation of what was filtered and why

  **QA Scenarios**:

  ```
  Scenario: Pipeline reduces PR set to candidate pool
    Tool: Bash
    Steps:
      1. Run filter pipeline test with 200-PR fixture
      2. Assert output pool size <50
      3. Assert all pool PRs have CI passing (or unknown)
      4. Assert no pool PR has staleness >80
      5. Assert pool is sorted by score descending
    Expected Result: Dramatically reduced, scored, sorted candidate pool
    Failure Indicators: Pool too large (>200), stale PRs included, unsorted
    Evidence: .sisyphus/evidence/task-13-pipeline-reduction.txt

  Scenario: All PRs filtered out returns explanation
    Tool: Bash
    Steps:
      1. Run filter pipeline with all PRs having merge conflicts
      2. Assert empty pool returned
      3. Assert explanation contains "all PRs filtered: conflict"
    Expected Result: Empty pool with human-readable explanation
    Failure Indicators: Panic, null pointer, no explanation
    Evidence: .sisyphus/evidence/task-13-empty-pool.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement pre-filter pipeline (cluster→CI→conflict→score)`
  - Files: `internal/filter/`
  - Pre-commit: `go test -race ./internal/filter/...`

- [ ] 14. Merge Planner — Formula + Graph + Pre-filter Integration (TDD)

  **What to do**:
  - **RED**: Write tests in `internal/planner/planner_test.go`:
    - `TestPlanGeneration`: Given pre-filtered pool of 50 PRs and target of 10, generate optimal merge plan
    - `TestPlanRespectsOrdering`: Plan's merge order respects dependency graph (topological constraints)
    - `TestPlanAvoidConflicts`: Plan excludes mutually conflicting PRs (picks higher-scored one)
    - `TestPlanTiered`: Tier 1 (independent, CI-green) → Tier 2 (dependent chains) → Tier 3 (needs resolution)
    - `TestPlanScoring`: Plan has aggregate score (sum of individual PR scores × ordering bonus)
    - `TestMultiplePlans`: Generate top-5 plans ranked by aggregate score
    - `TestTargetCount`: Plan contains exactly `--target` PRs (or fewer if not enough pass filters)
    - `TestBranchAware`: Plans only include PRs targeting same base branch
  - **GREEN**: Implement `internal/planner/`:
    - `planner.go` — Orchestrates: pre-filter → formula engine → graph ordering → plan scoring
    - `plan.go` — MergePlan struct with PRs, ordering, scores, conflict warnings
    - `optimizer.go` — Generate candidate plans via formula engine, score each, rank top-N
    - `validator.go` — Validate plan against dependency graph constraints
  - This is the integration point: formula engine (Task 8) + graph engine (Task 9) + pre-filter (Task 13)

  **Must NOT do**:
  - No actual merging — plan generation only
  - No GitHub API calls — works from cached data + pre-computed analyses
  - No ML calls — uses pre-computed cluster/duplicate data

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Integration of 3 subsystems (formula, graph, filter) — the core novel feature
  - **Skills**: [`superpowers/test-driven-development`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 15-18 if Task 3 done)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 18
  - **Blocked By**: Tasks 8 (formula engine), 9 (graph engine), 13 (pre-filter)

  **References**:
  - `/Users/jeffersonnunn/mag40/FORMULA.md:103-163` — Tiered search strategy from MAG40
  - Tasks 8, 9, 13 — The three engines being integrated
  - Metis directive: "Pre-filter to candidate pool BEFORE combinatorial selection. Realistic merge batch = 5-20 PRs from a pool of 50-200 pre-filtered candidates."

  **Acceptance Criteria**:
  - [ ] `go test -race -v ./internal/planner/...` → PASS (8+ tests)
  - [ ] Plan with target=10 produces exactly 10 PRs (or fewer with explanation)
  - [ ] Plan ordering respects all dependency edges
  - [ ] Top-5 plans have different PR selections, all valid
  - [ ] Plan JSON matches contract from Task 6

  **QA Scenarios**:

  ```
  Scenario: Generate merge plan for 10 PRs from 42-PR fixture
    Tool: Bash
    Steps:
      1. Run planner test with opencode-42prs fixture, target=10
      2. Assert plan contains <=10 PRs
      3. Assert plan has valid topological ordering
      4. Assert plan.score > 0
      5. Assert no two PRs in plan are marked as conflicting
    Expected Result: Valid, conflict-free, ordered merge plan
    Failure Indicators: Dependency violation, conflicting PRs included, wrong count
    Evidence: .sisyphus/evidence/task-14-merge-plan.txt

  Scenario: Multiple plans differ in PR selection
    Tool: Bash
    Steps:
      1. Generate top-5 plans for same input
      2. Assert at least 3 of 5 have different PR sets
      3. Assert all 5 respect dependency constraints
      4. Assert plans are ranked by descending score
    Expected Result: Multiple valid alternatives with different trade-offs
    Failure Indicators: All identical plans, invalid ordering in any plan
    Evidence: .sisyphus/evidence/task-14-multiple-plans.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): integrate merge planner (formula + graph + pre-filter)`
  - Files: `internal/planner/`
  - Pre-commit: `go test -race ./internal/planner/...`

- [ ] 15. CLI `analyze` Command — Full Analysis Pipeline

  **What to do**:
  - Wire the `analyze` command to execute the full pipeline:
    1. Use last-complete cache by default; if `--api` targets a running `pratc serve`, enqueue or attach to its background sync job for GitHub refresh
    2. Call Python ML service for clustering + duplicate detection
    3. Run staleness analysis
    4. Run pre-filter pipeline
    5. Aggregate results into analysis report
  - Output formats: `--format=json` (machine-readable), `--format=table` (human-readable)
  - JSON output matches the `AnalysisReport` contract from Task 6
  - Table output shows: summary stats, top clusters, duplicate groups, stalest PRs, merge-readiness breakdown
  - Add `--cache-only` flag to skip API sync and use existing cache
  - Add `--api=http://localhost:8080` flag to delegate sync/job orchestration to a running server when desired
  - Add `--no-ml` flag to skip NLP analysis (faster, uses file-path only)
  - Add `--ml-backend=local|openrouter` flag (default local)
  - Handle first-run: if no cache exists, return structured `sync_required` / `job_id` guidance unless a running server is available to own the background job

  **Must NOT do**:
  - No web dashboard rendering
  - No action execution (close/merge/label)
  - No blocking full-repo sync in the foreground if background worker path is available

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Integration command wiring multiple subsystems together
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 16-18)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 3 (GitHub API), 10 (clustering), 11 (duplicates), 12 (staleness)

  **References**:
  - `/Users/jeffersonnunn/crush/internal/cmd/` — Cobra command implementation patterns
  - Tasks 3, 10, 11, 12 — Subsystems being composed
  - JSON contracts from Task 6

  **Acceptance Criteria**:
  - [ ] `./bin/pratc analyze --repo=fixture/test --format=json | jq '.pr_count'` → correct count
  - [ ] `./bin/pratc analyze --repo=fixture/test --format=table` → human-readable table output
  - [ ] `./bin/pratc analyze --repo=fixture/test --cache-only` → uses cache, no API calls
  - [ ] JSON output validates against contract schema

  **QA Scenarios**:

  ```
  Scenario: Analyze with fixture data produces complete report
    Tool: Bash
    Steps:
      1. Pre-load cache with fixture data
      2. Run `./bin/pratc analyze --repo=fixture/test --cache-only --format=json`
      3. Assert JSON has: .pr_count, .clusters, .duplicate_groups, .staleness_report
      4. Assert .pr_count == 42 (opencode fixture)
      5. Assert .clusters is array with length >0
    Expected Result: Complete analysis JSON with all sections populated
    Failure Indicators: Missing sections, wrong PR count, empty clusters
    Evidence: .sisyphus/evidence/task-15-analyze-output.json

  Scenario: Analyze without ML falls back gracefully
    Tool: Bash
    Steps:
      1. Run `./bin/pratc analyze --repo=fixture/test --cache-only --no-ml --format=json`
      2. Assert .clusters is empty or file-path-based only
      3. Assert .duplicate_groups uses file-overlap only (no NLP)
      4. Assert no Python process was spawned
    Expected Result: Degraded but functional analysis without ML
    Failure Indicators: Error when ML unavailable, crash
    Evidence: .sisyphus/evidence/task-15-no-ml-fallback.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement analyze command`
  - Files: `internal/cmd/analyze.go`
  - Pre-commit: `go test -race ./internal/cmd/...`

- [ ] 16. CLI `cluster` Command — Clustering Output

  **What to do**:
  - Wire `cluster` command: use cache by default → call ML clustering → output
  - If `--api` is set, use server-managed background sync/job state; otherwise operate on cache only
  - Output formats: `--format=json`, `--format=table`
  - JSON output: `{clusters: [{id, label, pr_count, prs: [{number, title, similarity}]}]}`
  - Table output: cluster table with ID, label, PR count, sample titles
  - Add `--min-cluster-size` flag (default 3, passthrough to HDBSCAN)
  - Add `--ml-backend=local|openrouter` flag (default local)

  **Must NOT do**:
  - No duplicate detection (that's `analyze`)
  - No action execution

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple command wiring, most logic in Tasks 10 + 3
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 15, 17, 18)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 3 (GitHub API), 10 (clustering)

  **References**:
  - Task 10 output format, Task 6 JSON contracts

  **Acceptance Criteria**:
  - [ ] `./bin/pratc cluster --repo=fixture/test --format=json | jq '.clusters | length'` → >0
  - [ ] Table output shows cluster labels and PR counts

  **QA Scenarios**:

  ```
  Scenario: Cluster command outputs valid clusters
    Tool: Bash
    Steps:
      1. Run `./bin/pratc cluster --repo=fixture/test --cache-only --format=json`
      2. Assert .clusters is non-empty array
      3. Assert each cluster has .id, .label, .prs
    Expected Result: Valid cluster JSON
    Failure Indicators: Empty clusters, missing labels
    Evidence: .sisyphus/evidence/task-16-cluster-output.json
  ```

  **Commit**: YES
  - Message: `feat(cli): implement cluster command`
  - Files: `internal/cmd/cluster.go`
  - Pre-commit: `go test -race ./internal/cmd/...`

- [ ] 17. CLI `graph` Command — DOT Output

  **What to do**:
  - Wire `graph` command: use cache by default → build dependency graph → output
  - If `--api` is set, use server-managed background sync/job state; otherwise operate on cache only
  - Output formats: `--format=dot` (default), `--format=json` (adjacency list)
  - DOT output: nodes = PRs (colored by cluster), edges = dependencies/conflicts (styled by type)
  - Add `--filter-cluster` flag to show graph for specific cluster only
  - Add `--max-nodes` flag (default 100) to limit visualization complexity

  **Must NOT do**:
  - No SVG rendering (user pipes to `dot -Tsvg` or views in web dashboard)
  - No interactive features

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Simple command wiring, graph logic in Task 9
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Tasks 15, 16, 18)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 3 (GitHub API), 9 (graph engine)

  **References**:
  - Task 9 DOT output, DOT language spec

  **Acceptance Criteria**:
  - [ ] `./bin/pratc graph --repo=fixture/test --format=dot | head -1` → `digraph {`
  - [ ] DOT output parseable by graphviz (if installed)

  **QA Scenarios**:

  ```
  Scenario: Graph command outputs valid DOT
    Tool: Bash
    Steps:
      1. Run `./bin/pratc graph --repo=fixture/test --cache-only --format=dot`
      2. Assert first line is "digraph {"
      3. Assert contains node definitions and edge definitions
      4. Assert last line is "}"
    Expected Result: Valid DOT format
    Failure Indicators: Invalid DOT syntax, empty graph
    Evidence: .sisyphus/evidence/task-17-graph-dot.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement graph command with DOT output`
  - Files: `internal/cmd/graph.go`
  - Pre-commit: `go test -race ./internal/cmd/...`

- [ ] 18. CLI `plan` Command — Merge Plan Output

  **What to do**:
  - Wire `plan` command: use cache by default → pre-filter → formula engine → graph ordering → output
  - If `--api` is set, use server-managed background sync/job state; otherwise operate on cache only
  - Key flags: `--target=N` (how many PRs to include), `--tier=1|2|3` (search depth), `--top=5` (number of plans)
  - Output formats: `--format=json`, `--format=table`
  - JSON: `{plans: [{rank, score, prs: [{number, title, position, reason}], warnings: []}]}`
  - Table: ranked plans with PR lists, scores, and any warnings
  - Add `--base-branch=main` flag to filter by target branch (default: repo default branch)

  **Must NOT do**:
  - No actual merging
  - No action execution

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Wires the core novel feature (formula + graph integration)
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 15 if all deps met)
  - **Parallel Group**: Wave 3
  - **Blocks**: Task 19
  - **Blocked By**: Tasks 3 (GitHub API), 14 (merge planner)

  **References**:
  - Task 14 planner output, Task 6 JSON contracts

  **Acceptance Criteria**:
  - [ ] `./bin/pratc plan --repo=fixture/test --target=10 --format=json | jq '.plans[0].prs | length'` → <=10
  - [ ] Plans have valid topological ordering
  - [ ] `--top=5` produces 5 different plans

  **QA Scenarios**:

  ```
  Scenario: Plan command generates valid merge plans
    Tool: Bash
    Steps:
      1. Run `./bin/pratc plan --repo=fixture/test --target=10 --top=3 --cache-only --format=json`
      2. Assert .plans has length 3
      3. Assert each plan has .score > 0
      4. Assert each plan.prs has length <= 10
      5. Assert plans are ranked by descending score
    Expected Result: 3 valid, ranked merge plans
    Failure Indicators: Wrong plan count, zero scores, no PR data
    Evidence: .sisyphus/evidence/task-18-plan-output.json

  Scenario: Plan with impossible target returns partial plan
    Tool: Bash
    Steps:
      1. Run `./bin/pratc plan --repo=fixture/test --target=1000 --cache-only --format=json`
      2. Assert .plans[0].prs has length < 1000 (can't exceed available PRs)
      3. Assert .plans[0].warnings contains explanation
    Expected Result: Partial plan with warning about insufficient candidates
    Failure Indicators: Error, attempt to create 1000-PR plan
    Evidence: .sisyphus/evidence/task-18-impossible-target.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement plan command`
  - Files: `internal/cmd/plan.go`
  - Pre-commit: `go test -race ./internal/cmd/...`

- [ ] 19. CLI `serve` Command — HTTP API Mode

  **What to do**:
  - Implement HTTP API server in Go (stdlib `net/http` + lightweight router):
    - `pratc serve` is the owner of the in-process background sync worker for v0.1
    - `GET /api/health` → `{"status":"healthy"}`
    - `GET /api/repos/:owner/:repo/analysis` → full analysis report
    - `GET /api/repos/:owner/:repo/clusters` → cluster data
    - `GET /api/repos/:owner/:repo/graph` → graph data (JSON adjacency)
    - `GET /api/repos/:owner/:repo/plans?target=N&top=5&mode=combination|permutation|with_replacement&require_ci=true|false&exclude_stale=true|false&exclude_drafts=true|false` → merge plans
      - Pre-filter pipeline controls for Task 13 MUST be first-class query params (cluster -> CI -> conflict -> score) so callers can configure candidate reduction before combinatorial selection
      - Extend `/plans` query surface (v0.1) with:
        - `cluster_id=<id>` (optional) -> only evaluate PRs in one cluster
        - `exclude_conflicts=true|false` (default true) -> include/exclude merge-conflicting PRs from candidate pool
        - `stale_score_threshold=0..100` (default 80) -> threshold used by stale filter when `exclude_stale=true`
        - `candidate_pool_cap=1..500` (default 200) -> max pre-filtered pool handed to formula engine
        - `score_min=0..1` (optional) -> drop PRs below minimum pre-filter score
      - API handler must build explicit `PreFilterConfig` from query params and pass it into Task 13 pipeline (no hidden hardcoded filter behavior)
    - `GET /api/repos/:owner/:repo/prs` → paginated PR list with filters
    - `GET /api/repos/:owner/:repo/prs/:number` → single PR detail
    - `POST /api/repos/:owner/:repo/sync` → enqueue/attach to in-process background sync job and return job metadata immediately
    - `POST /api/repos/:owner/:repo/actions` → log action intent (dry-run)
  - CORS headers for localhost:3000 (dashboard)
  - Server-Sent Events endpoint for sync progress: `GET /api/repos/:owner/:repo/sync/stream`
  - `GET /api/repos/:owner/:repo/sync/status` → current job state/progress for background sync worker
  - `pratc serve --port=8080` starts server

  **Must NOT do**:
  - No authentication — localhost single-user only
  - No actual PR action execution via API (log intent only)
  - No WebSocket — use SSE for simplicity

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: HTTP server with multiple endpoints, SSE, CORS — moderate complexity
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on all CLI commands)
  - **Parallel Group**: Sequential (end of Wave 3)
  - **Blocks**: Tasks 20-24, 25, 28
  - **Blocked By**: Tasks 3, 15, 16, 17, 18

  **References**:
  - Go stdlib net/http: `https://pkg.go.dev/net/http`
  - SSE spec: `https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events`
  - Task 6 JSON contracts — API responses match these schemas

  **Acceptance Criteria**:
  - [ ] `curl -s http://localhost:8080/api/health | jq .status` → `"healthy"`
  - [ ] All endpoints return valid JSON matching contracts
  - [ ] `/plans` accepts and validates pre-filter params (`mode`, `require_ci`, `exclude_stale`, `exclude_drafts`, `cluster_id`, `exclude_conflicts`, `stale_score_threshold`, `candidate_pool_cap`, `score_min`) with deterministic defaults
  - [ ] `/plans` applies pre-filter params before formula engine invocation; formula engine never receives raw unfiltered 5,500-PR pool
  - [ ] CORS allows requests from localhost:3000
  - [ ] SSE endpoint streams sync progress events
  - [ ] Sync trigger endpoint returns without blocking on full repo sync
  - [ ] Background sync worker lifecycle is owned by `pratc serve`, not one-shot CLI invocation
  - [ ] `go test -race -v ./internal/server/...` → PASS

  **QA Scenarios**:

  ```
  Scenario: Health endpoint returns healthy
    Tool: Bash
    Preconditions: `pratc serve --port=8080` running
    Steps:
      1. Run `curl -s http://localhost:8080/api/health`
      2. Parse JSON, assert .status == "healthy"
    Expected Result: {"status":"healthy"}
    Failure Indicators: Connection refused, non-JSON, wrong status
    Evidence: .sisyphus/evidence/task-19-health.txt

  Scenario: Analysis endpoint returns full report
    Tool: Bash
    Preconditions: Server running, cache pre-loaded with fixture
    Steps:
      1. Run `curl -s http://localhost:8080/api/repos/fixture/test/analysis`
      2. Assert JSON has .pr_count, .clusters, .duplicate_groups
      3. Assert Content-Type: application/json
    Expected Result: Full analysis JSON matching CLI output
    Failure Indicators: 404, 500, empty response
    Evidence: .sisyphus/evidence/task-19-analysis-api.json

  Scenario: Plans endpoint honors pre-filter query params
    Tool: Bash
    Preconditions: Server running with fixture repo containing mixed CI states, drafts, stale PRs, and conflicting PRs
    Steps:
      1. Run `curl -s "http://localhost:8080/api/repos/fixture/test/plans?target=10&top=2&mode=combination&require_ci=true&exclude_stale=true&exclude_drafts=true&exclude_conflicts=true&stale_score_threshold=80&candidate_pool_cap=120"`
      2. Assert response includes `.candidatePoolSize` <= 120
      3. Assert `.rejections` contains reasons for CI/stale/draft/conflict exclusions when such PRs exist
      4. Assert returned selected PRs do not include drafts or conflicting PRs
    Expected Result: Pre-filter contract is applied before combinatorial planning and reflected in output
    Failure Indicators: Params ignored, conflicting/draft/stale PRs present despite exclusions, pool cap exceeded
    Evidence: .sisyphus/evidence/task-19-plans-prefilter-params.json

  Scenario: Plans endpoint rejects invalid pre-filter params
    Tool: Bash
    Preconditions: Server running
    Steps:
      1. Run `curl -s -o /tmp/pratc-invalid.json -w "%{http_code}" "http://localhost:8080/api/repos/fixture/test/plans?target=10&stale_score_threshold=200&candidate_pool_cap=0&score_min=1.5"`
      2. Assert status code is 400
      3. Assert `/tmp/pratc-invalid.json` includes field-level validation errors for each invalid query param
    Expected Result: API fails fast on invalid pipeline config and documents why
    Failure Indicators: 200 with silent coercion, generic/unhelpful error, missing parameter-specific validation
    Evidence: .sisyphus/evidence/task-19-plans-prefilter-invalid.json

  Scenario: CORS allows dashboard origin
    Tool: Bash
    Steps:
      1. Run `curl -s -H "Origin: http://localhost:3000" -I http://localhost:8080/api/health`
      2. Assert Access-Control-Allow-Origin header present
    Expected Result: CORS headers allow localhost:3000
    Failure Indicators: Missing CORS headers, wrong origin
    Evidence: .sisyphus/evidence/task-19-cors.txt
  ```

  **Commit**: YES
  - Message: `feat(cli): implement serve command with HTTP API`
  - Files: `internal/server/`, `internal/cmd/serve.go`
  - Pre-commit: `go test -race ./internal/cmd/... ./internal/server/...`

- [ ] 20. Dashboard Layout + API Integration Layer

  **What to do**:
  - Build the root dashboard layout in `web/src/app/layout.tsx`:
    - Left sidebar nav with 4 links: Analysis, Inbox, Graph, Plan
    - Active route highlighting
    - Collapsible sidebar (icon-only mode)
    - Top bar with repo selector dropdown + sync status indicator
  - Implement API client in `web/src/lib/api.ts`:
    - Typed fetch wrapper hitting Go API at `http://localhost:8080/api/`
    - Endpoints: `/repos/:owner/:repo/analysis`, `/repos/:owner/:repo/clusters`, `/repos/:owner/:repo/graph`, `/repos/:owner/:repo/plans`, `/health`
    - Error handling: connection refused → "Server not running" banner
    - Loading states: skeleton loaders per page
  - Create shared UI components in `web/src/components/`:
    - `PRBadge.tsx` — colored badge for PR status (open/draft/bot/stale)
    - `ClusterTag.tsx` — colored tag for cluster assignment
    - `ActionButton.tsx` — configurable action button (Approve/Close/Skip) with intent logging
    - `SyncStatus.tsx` — real-time sync progress indicator
    - `EmptyState.tsx` — zero-data state with helpful messaging
  - Configure TanStack Query for server state management:
    - Query client with stale time = 30s
    - Automatic refetch on window focus
  - Write tests: layout renders, nav links work, API client handles errors
  - **TDD**: Write tests first for API client error handling and component rendering

  **Must NOT do**:
  - No authentication/login
  - No dark mode toggle
  - No keyboard shortcuts
  - No WebSocket real-time updates (polling only in v0.1)
  - No animations beyond CSS transitions

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Layout, navigation, shared UI components — core frontend architecture
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Clean dashboard layout with professional visual hierarchy
  - **Skills Evaluated but Omitted**:
    - `playwright`: Not needed yet — QA scenarios use Playwright but that's the executing agent's concern

  **Parallelization**:
  - **Can Run In Parallel**: NO (first task of Wave 4)
  - **Parallel Group**: Wave 4 (must complete before Tasks 21-24)
  - **Blocks**: Tasks 21, 22, 23, 24, 26
  - **Blocked By**: Task 5 (web scaffold), Task 19 (serve command provides API)

  **References**:

  **Pattern References** (existing code to follow):
  - `web/src/app/layout.tsx` — Stub layout from Task 5 to extend
  - `web/src/lib/api.ts` — Stub API client from Task 5 to implement

  **API/Type References** (contracts to implement against):
  - `contracts/*.json` — JSON Schema contracts defining all API response shapes
  - `web/src/types/api.ts` — TypeScript interfaces from Task 6
  - `internal/server/` — Go HTTP handlers from Task 19 defining endpoints

  **External References**:
  - TanStack Query: `https://tanstack.com/query` — React data fetching with caching
  - TanStack Table: `https://tanstack.com/table` — Headless table for inbox (dependency installed in Task 5)
  - Tailwind CSS: `https://tailwindcss.com/docs` — Utility-first styling

  **WHY Each Reference Matters**:
  - `contracts/*.json` — The API client MUST match these shapes exactly or TypeScript types will diverge from Go responses
  - `internal/server/` — Defines the actual endpoints, status codes, and error shapes the client must handle
  - `api.ts` (Task 6) — These are the TypeScript types the API client returns; implement fetch functions that return these types

  **Acceptance Criteria**:
  - [ ] `bun test` → layout + API client tests PASS
  - [ ] `bun run build` → zero TypeScript errors
  - [ ] Dashboard loads with sidebar nav showing 4 routes
  - [ ] Repo selector dropdown renders (with mock data)
  - [ ] API client returns typed responses from Go server
  - [ ] Connection error shows "Server not running" banner

  **QA Scenarios**:

  ```
  Scenario: Dashboard layout renders with navigation
    Tool: Playwright (playwright skill)
    Preconditions: `bun run dev` on localhost:3000, Go API on localhost:8080
    Steps:
      1. Navigate to http://localhost:3000
      2. Assert sidebar nav visible: `nav[data-testid="sidebar"]`
      3. Assert 4 nav links: `.sidebar-link` count === 4
      4. Click "Inbox" link: `a[href="/inbox"]`
      5. Assert URL changed to /inbox
      6. Assert active link highlighted: `.sidebar-link.active` text === "Inbox"
      7. Take screenshot
    Expected Result: Sidebar with 4 links, route navigation works, active state shown
    Failure Indicators: Missing sidebar, broken links, no active highlighting
    Evidence: .sisyphus/evidence/task-20-layout-nav.png

  Scenario: API connection error shows banner
    Tool: Playwright (playwright skill)
    Preconditions: `bun run dev` on localhost:3000, Go API NOT running
    Steps:
      1. Navigate to http://localhost:3000/analysis
      2. Wait for error state (timeout: 5s)
      3. Assert error banner visible: `[data-testid="connection-error"]`
      4. Assert banner text contains "Server not running" or "Connection refused"
      5. Take screenshot
    Expected Result: User-friendly error banner, no unhandled exception
    Failure Indicators: Blank page, React error boundary, console errors without UI feedback
    Evidence: .sisyphus/evidence/task-20-api-error.png

  Scenario: Repo selector dropdown renders options
    Tool: Playwright (playwright skill)
    Preconditions: `bun run dev` + Go API running with at least 1 synced repo
    Steps:
      1. Navigate to http://localhost:3000
      2. Click repo selector: `[data-testid="repo-selector"]`
      3. Assert dropdown menu visible with at least 1 option
      4. Select first repo option
      5. Assert selected repo name shown in selector
    Expected Result: Dropdown opens, shows repos, selection persists
    Failure Indicators: Dropdown empty, click does nothing, selection resets
    Evidence: .sisyphus/evidence/task-20-repo-selector.png
  ```

  **Commit**: YES
  - Message: `feat(web): implement dashboard layout, API client, and shared components`
  - Files: `web/src/app/layout.tsx`, `web/src/lib/api.ts`, `web/src/components/`, `web/src/lib/query.ts`
  - Pre-commit: `bun test`

- [ ] 21. Air Traffic Control View — PR Cluster Visualization

  **What to do**:
  - Build the Analysis page at `web/src/app/analysis/page.tsx`:
    - Hero section: Repository stats summary (total PRs, clusters found, duplicates detected, stale count)
    - Cluster cards grid: each cluster as a card showing:
      - Cluster label (auto-generated from common file paths or PR title keywords)
      - PR count badge
      - Merge readiness indicator (% of PRs with passing CI)
      - Top 3 PR titles as preview
      - Expandable: full PR list within cluster
    - Cluster health indicators using color coding:
      - Green: All PRs have passing CI, no conflicts
      - Yellow: Some CI failures or minor conflicts
      - Red: Major conflicts between PRs in cluster, stale PRs
    - Filter bar: filter clusters by health status, sort by PR count / readiness / recency
    - Summary panel: "Recommended Action" per cluster (merge candidates, needs review, close as stale)
  - Fetch data from `/api/repos/:owner/:repo/analysis` endpoint
  - **TDD**: Write component tests first for cluster card rendering, health indicators, filter logic

  **Must NOT do**:
  - No real-time cluster updates (refresh button only)
  - No drag-and-drop cluster reassignment
  - No cluster editing (rename, merge clusters)
  - No inline PR diff viewer
  - No custom color schemes

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Data visualization with color-coded health indicators, card grid layout
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Dashboard needs clear information hierarchy and visual design
  - **Skills Evaluated but Omitted**:
    - `playwright`: Executing agent uses for QA, not needed as loaded skill

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 22, 23, 24)
  - **Blocks**: F3
  - **Blocked By**: Task 20 (layout + API client)

  **References**:

  **Pattern References**:
  - `web/src/components/PRBadge.tsx` — PR status badge from Task 20
  - `web/src/components/ClusterTag.tsx` — Cluster tag component from Task 20
  - `web/src/lib/api.ts` — API client with typed responses from Task 20

  **API/Type References**:
  - `web/src/types/api.ts` — `PRCluster`, `PR`, `AnalysisResponse` types from Task 6
  - `contracts/analysis-response.json` — JSON Schema for analysis endpoint response

  **External References**:
  - Tailwind CSS grid: `https://tailwindcss.com/docs/grid-template-columns` — Card grid layout
  - HuggingFace text-clustering UI: `https://huggingface.co/spaces/lm-cluster/text-clustering` — Inspiration for cluster visualization

  **WHY Each Reference Matters**:
  - `api.ts` types — Cluster cards must render data matching the exact `PRCluster` shape (cluster_label, pr_ids, health_status)
  - `PRBadge.tsx` — Reuse for individual PR status within expanded cluster view
  - `contracts/analysis-response.json` — Defines the data shape; analysis page MUST handle all fields

  **Acceptance Criteria**:
  - [ ] `bun test` → analysis page component tests PASS
  - [ ] Cluster cards render with correct color coding (green/yellow/red)
  - [ ] Filter bar filters clusters by health status
  - [ ] Expanded cluster shows full PR list
  - [ ] Stats summary shows correct counts

  **QA Scenarios**:

  ```
  Scenario: Analysis page renders cluster cards with health colors
    Tool: Playwright (playwright skill)
    Preconditions: Go API running with synced repo data (at least 3 clusters)
    Steps:
      1. Navigate to http://localhost:3000/analysis
      2. Wait for data load: `[data-testid="cluster-grid"]` visible (timeout: 10s)
      3. Assert stats summary visible: `[data-testid="stats-summary"]`
      4. Assert at least 3 cluster cards: `.cluster-card` count >= 3
      5. Assert at least one card has health indicator: `.health-indicator` exists
      6. Assert each card shows PR count badge: `.cluster-card .pr-count` text matches \d+
      7. Take screenshot
    Expected Result: Grid of colored cluster cards with stats, PR counts, health indicators
    Failure Indicators: Empty grid, missing colors, "0 PRs" on all cards
    Evidence: .sisyphus/evidence/task-21-cluster-grid.png

  Scenario: Filter clusters by health status
    Tool: Playwright (playwright skill)
    Preconditions: Analysis page loaded with mixed health clusters
    Steps:
      1. Navigate to http://localhost:3000/analysis
      2. Wait for cluster grid loaded
      3. Count total clusters: store `.cluster-card` count as N
      4. Click filter "Red" (unhealthy): `[data-testid="filter-red"]`
      5. Assert `.cluster-card` count < N (filtered)
      6. Assert all visible cards have red indicator: `.cluster-card .health-indicator.red`
      7. Click "Clear filters": `[data-testid="clear-filters"]`
      8. Assert `.cluster-card` count === N (restored)
    Expected Result: Filtering reduces visible cards, clear restores all
    Failure Indicators: Filter doesn't reduce count, clear doesn't restore, wrong colors after filter
    Evidence: .sisyphus/evidence/task-21-filter-health.png

  Scenario: Expand cluster to see PR list
    Tool: Playwright (playwright skill)
    Preconditions: Analysis page loaded with clusters containing multiple PRs
    Steps:
      1. Navigate to http://localhost:3000/analysis
      2. Wait for cluster grid
      3. Click first cluster card: `.cluster-card:first-child`
      4. Assert expanded view visible: `[data-testid="cluster-expanded"]`
      5. Assert PR list items visible: `.pr-list-item` count > 0
      6. Assert each PR item shows title, author, status badge
      7. Take screenshot of expanded view
    Expected Result: Clicking card expands to show full PR list with details
    Failure Indicators: Click does nothing, expanded view empty, PR data missing
    Evidence: .sisyphus/evidence/task-21-cluster-expand.png
  ```

  **Commit**: YES
  - Message: `feat(web): implement air traffic control analysis view with cluster cards`
  - Files: `web/src/app/analysis/page.tsx`, `web/src/app/analysis/`, `web/src/components/ClusterCard.tsx`
  - Pre-commit: `bun test`

- [ ] 22. Outlook Inbox View — Sequential PR Triage with Action Buttons

  **What to do**:
  - Build the Inbox page at `web/src/app/inbox/page.tsx`:
    - **Outlook-style 3-pane layout**:
      - Left pane: PR list with TanStack Table — sortable columns: #, Title, Author, Cluster, CI Status, Age
      - Right pane: PR detail view (selected PR's full info)
      - Bottom pane (optional, collapsible): Recommended action reasoning
    - **6-column table** (TanStack Table):
      - `#` — PR number (link to GitHub)
      - `Title` — truncated with full tooltip
      - `Author` — avatar + username
      - `Cluster` — colored cluster tag
      - `CI Status` — pass/fail/pending icon
      - `Age` — relative time ("3 months ago") with color (green < 30d, yellow < 90d, red > 90d)
    - **3 action buttons** per PR (configurable):
      - ✅ Approve — logs intent "approve PR #X" to audit log
      - ❌ Close — logs intent "close PR #X as stale/duplicate" to audit log
      - ⏭️ Skip — moves to next PR without action
    - **Triage workflow**: After action, auto-advance to next PR in list
    - **Batch mode**: Checkbox column for multi-select + batch actions
    - **Sorting**: Click column headers to sort; default sort by "recommended action priority"
    - **Pagination**: Virtual scroll for 5,000+ PRs (TanStack Virtual)
    - Fetch data from `/api/repos/:owner/:repo/prs` for the inbox list and `/api/repos/:owner/:repo/prs/:number` for detail pane
  - **TDD**: Write tests for table rendering, action button click handlers, auto-advance logic, sort

  **Must NOT do**:
  - Action buttons DO NOT actually call GitHub API — they log intent only (dry-run default)
  - No inline diff viewer
  - No PR commenting
  - No drag-and-drop reordering
  - No saved filters/views
  - No email-style read/unread states

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Complex table layout with TanStack Table, 3-pane Outlook design, interaction design
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Outlook-style inbox is a high-fidelity UI pattern requiring careful layout
  - **Skills Evaluated but Omitted**:
    - `playwright`: QA tool, not implementation skill

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 21, 23, 24)
  - **Blocks**: Task 28, F3
  - **Blocked By**: Task 20 (layout + API client)

  **References**:

  **Pattern References**:
  - `web/src/components/ActionButton.tsx` — Action button component from Task 20
  - `web/src/components/PRBadge.tsx` — PR status badge from Task 20
  - `web/src/components/ClusterTag.tsx` — Cluster tag from Task 20
  - `web/src/lib/api.ts` — API client from Task 20

  **API/Type References**:
  - `web/src/types/api.ts` — `PR`, `PRCluster`, `ActionIntent` types from Task 6
  - `contracts/analysis-response.json` — PR list data shape

  **External References**:
  - TanStack Table docs: `https://tanstack.com/table/latest/docs/guide/introduction` — Column definitions, sorting, pagination
  - TanStack Virtual: `https://tanstack.com/virtual` — Virtual scrolling for large lists
  - Outlook Web App: Visual reference for 3-pane layout pattern

  **WHY Each Reference Matters**:
  - TanStack Table — This page is 80% table; column definitions, sorting callbacks, and pagination MUST follow TanStack API
  - TanStack Virtual — With 5,000+ PRs, virtual scrolling is not optional; without it the page will freeze
  - `ActionButton.tsx` — Reuse the configurable action button; don't create a separate button component
  - `PR` type — Table columns must map directly to PR type fields (number, title, author, cluster_id, ci_status, created_at)

  **Acceptance Criteria**:
  - [ ] `bun test` → inbox page tests PASS (table render, sort, action clicks, auto-advance)
  - [ ] Table renders 6 columns with correct headers
  - [ ] Clicking column header sorts data
  - [ ] Action buttons log intent (visible in browser console or audit panel)
  - [ ] Auto-advance selects next PR after action
  - [ ] Virtual scroll handles 1000+ rows without lag

  **QA Scenarios**:

  ```
  Scenario: Inbox table renders with 6 columns and PR data
    Tool: Playwright (playwright skill)
    Preconditions: Go API running with synced repo (50+ PRs)
    Steps:
      1. Navigate to http://localhost:3000/inbox
      2. Wait for table: `[data-testid="pr-table"]` visible (timeout: 10s)
      3. Assert 6 column headers: `th` count === 6
      4. Assert column headers text: "#", "Title", "Author", "Cluster", "CI", "Age"
      5. Assert at least 10 rows visible: `tbody tr` count >= 10
      6. Assert first row has PR number: `tbody tr:first-child td:first-child` text matches #\d+
      7. Take screenshot
    Expected Result: Full 6-column table with PR data, sortable headers
    Failure Indicators: Missing columns, empty table, wrong column order
    Evidence: .sisyphus/evidence/task-22-inbox-table.png

  Scenario: Action buttons log intent and auto-advance
    Tool: Playwright (playwright skill)
    Preconditions: Inbox loaded with multiple PRs
    Steps:
      1. Navigate to http://localhost:3000/inbox
      2. Wait for table loaded
      3. Click first PR row to select: `tbody tr:first-child`
      4. Assert detail pane shows PR info: `[data-testid="pr-detail"]` visible
      5. Note current PR number from detail pane
      6. Click "Skip" button: `[data-testid="action-skip"]`
      7. Assert detail pane now shows DIFFERENT PR (auto-advanced)
      8. Assert previous PR number !== current PR number
    Expected Result: Skip advances to next PR, detail pane updates
    Failure Indicators: Same PR still shown, detail pane blank, button unresponsive
    Evidence: .sisyphus/evidence/task-22-auto-advance.png

  Scenario: Column sorting works for Age column
    Tool: Playwright (playwright skill)
    Preconditions: Inbox loaded with PRs of different ages
    Steps:
      1. Navigate to http://localhost:3000/inbox
      2. Wait for table loaded
      3. Note first row's age text: `tbody tr:first-child td:nth-child(6)` → store as AGE_BEFORE
      4. Click "Age" column header: `th:nth-child(6)`
      5. Wait 500ms for re-sort
      6. Note first row's age text → store as AGE_AFTER
      7. Assert AGE_BEFORE !== AGE_AFTER (sort changed order)
      8. Assert sort indicator visible on Age column header
    Expected Result: Clicking header re-sorts table, sort indicator shown
    Failure Indicators: Order unchanged, no sort indicator, table flickers
    Evidence: .sisyphus/evidence/task-22-sort-age.png

  Scenario: Virtual scroll handles large dataset without freezing
    Tool: Playwright (playwright skill)
    Preconditions: Go API with repo containing 500+ PRs
    Steps:
      1. Navigate to http://localhost:3000/inbox
      2. Wait for table loaded
      3. Scroll table to bottom: `evaluate(() => { document.querySelector('[data-testid="pr-table"]').scrollTop = 99999 })`
      4. Wait 1s for virtual scroll to render
      5. Assert new rows rendered (different PR numbers than initial view)
      6. Measure page responsiveness: no "page unresponsive" dialog
    Expected Result: Smooth scroll through 500+ PRs, rows render on demand
    Failure Indicators: Page freezes, blank rows, scroll stutters, memory spike
    Evidence: .sisyphus/evidence/task-22-virtual-scroll.png
  ```

  **Commit**: YES
  - Message: `feat(web): implement Outlook-style inbox view with TanStack Table and action buttons`
  - Files: `web/src/app/inbox/page.tsx`, `web/src/app/inbox/`, `web/src/components/InboxTable.tsx`
  - Pre-commit: `bun test`

- [ ] 23. Interactive Dependency Graph (D3.js Force-Directed)

  **What to do**:
  - Build the Graph page at `web/src/app/graph/page.tsx`:
    - **D3.js force-directed graph** rendering PR dependency/conflict relationships:
      - Nodes = PRs (sized by lines changed, colored by cluster)
      - Edges = relationships:
        - Solid blue: file overlap dependency
        - Dashed red: conflict (GitHub mergeability or high-overlap file conflict heuristic)
        - Dotted green: duplicate (>90% similarity)
      - Node labels: PR number + short title (truncated)
    - **Interactive features**:
      - Zoom + pan (D3 zoom behavior)
      - Hover node: tooltip with PR title, author, CI status, cluster
      - Click node: highlight connected edges + neighbor nodes
      - Click edge: show relationship detail (which files overlap, conflict type)
    - **Controls panel**:
      - Filter by cluster (toggle clusters on/off)
      - Filter by edge type (dependencies, conflicts, duplicates)
      - Layout reset button
      - Node size: toggle between "lines changed" and "uniform"
    - **Legend**: Color + line style legend for node/edge types
    - Fetch graph data from `/api/repos/:owner/:repo/graph` endpoint (JSON adjacency list)
    - **Performance**: Canvas rendering for 500+ nodes, SVG for <500
  - **TDD**: Write tests for graph data transformation, filter logic, tooltip content

  **Must NOT do**:
  - No 3D graph
  - No graph editing (add/remove nodes)
  - No export to image/PDF
  - No animated transitions between layouts
  - No search within graph (use inbox for search)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Complex D3.js data visualization with interaction patterns
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Graph needs clear visual hierarchy, accessible color choices
  - **Skills Evaluated but Omitted**:
    - `playwright`: QA tool only

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 21, 22, 24)
  - **Blocks**: F3
  - **Blocked By**: Task 20 (layout + API client)

  **References**:

  **Pattern References**:
  - `web/src/lib/api.ts` — API client for `/api/repos/:owner/:repo/graph` endpoint from Task 20
  - `internal/graph/` — Go graph engine from Task 9 (produces DOT + JSON adjacency list)

  **API/Type References**:
  - `web/src/types/api.ts` — `GraphNode`, `GraphEdge`, `GraphResponse` types from Task 6
  - `contracts/graph-response.json` — JSON Schema for graph API response

  **External References**:
  - D3.js force simulation: `https://d3js.org/d3-force` — Force-directed layout API
  - D3 zoom: `https://d3js.org/d3-zoom` — Pan and zoom behavior
  - Observable D3 force graph examples: `https://observablehq.com/@d3/force-directed-graph` — Reference implementation

  **WHY Each Reference Matters**:
  - `graph-response.json` — The D3 graph MUST consume the exact JSON adjacency format the Go API produces; mismatched node/edge shapes = broken graph
  - D3 force simulation — Core rendering engine; follow `d3-force` API for node positioning, link forces, collision
  - Go graph engine — Understand what data the backend produces (node: {id, pr_number, cluster_id, lines_changed}, edge: {source, target, type, files})

  **Acceptance Criteria**:
  - [ ] `bun test` → graph page tests PASS
  - [ ] D3 graph renders with nodes and edges
  - [ ] Nodes colored by cluster, edges styled by relationship type
  - [ ] Zoom and pan work
  - [ ] Hover tooltip shows PR info
  - [ ] Filter controls toggle clusters/edge types

  **QA Scenarios**:

  ```
  Scenario: Graph renders with nodes and colored edges
    Tool: Playwright (playwright skill)
    Preconditions: Go API running with repo that has clustered PRs with dependencies
    Steps:
      1. Navigate to http://localhost:3000/graph
      2. Wait for graph canvas/SVG: `[data-testid="graph-container"] svg` or `canvas` visible (timeout: 15s)
      3. Assert nodes exist: `circle.graph-node` count > 0 (SVG) or canvas is non-empty
      4. Assert edges exist: `line.graph-edge` or `path.graph-edge` count > 0
      5. Assert legend visible: `[data-testid="graph-legend"]`
      6. Take screenshot
    Expected Result: Force-directed graph with colored nodes and styled edges, legend visible
    Failure Indicators: Blank canvas, nodes all same color, no edges, legend missing
    Evidence: .sisyphus/evidence/task-23-graph-render.png

  Scenario: Hover tooltip shows PR details
    Tool: Playwright (playwright skill)
    Preconditions: Graph loaded with visible nodes
    Steps:
      1. Navigate to http://localhost:3000/graph
      2. Wait for graph loaded
      3. Hover over first node: `circle.graph-node:first-child` (or equivalent canvas coordinate)
      4. Assert tooltip visible: `[data-testid="graph-tooltip"]`
      5. Assert tooltip contains PR number: text matches /#\d+/
      6. Assert tooltip contains author name
      7. Move mouse away — assert tooltip hidden
    Expected Result: Tooltip appears on hover with PR info, disappears on leave
    Failure Indicators: No tooltip, tooltip in wrong position, tooltip doesn't hide
    Evidence: .sisyphus/evidence/task-23-tooltip.png

  Scenario: Cluster filter hides/shows nodes
    Tool: Playwright (playwright skill)
    Preconditions: Graph loaded with 2+ clusters visible
    Steps:
      1. Navigate to http://localhost:3000/graph
      2. Wait for graph loaded
      3. Count total nodes: store count as N
      4. Toggle first cluster off in filter panel: `[data-testid="cluster-filter"] input:first-child`
      5. Wait 500ms for graph re-render
      6. Count visible nodes: assert count < N
      7. Toggle cluster back on
      8. Assert node count === N (restored)
    Expected Result: Toggling cluster filter hides/shows cluster's nodes
    Failure Indicators: Filter has no effect, nodes disappear permanently, graph crashes
    Evidence: .sisyphus/evidence/task-23-cluster-filter.png
  ```

  **Commit**: YES
  - Message: `feat(web): implement interactive D3.js dependency graph with zoom and filters`
  - Files: `web/src/app/graph/page.tsx`, `web/src/app/graph/`, `web/src/components/ForceGraph.tsx`
  - Pre-commit: `bun test`

- [ ] 24. Merge Plan View — Recommended Merge Set + Ordering

  **What to do**:
  - Build the Plan page at `web/src/app/plan/page.tsx`:
    - **Merge plan display**:
      - Ordered list of recommended PRs to merge (topological order from graph engine)
      - Each entry shows: position #, PR number + title, merge readiness score (0-100), reason for inclusion
      - Color-coded readiness: green ≥ 80, yellow 50-79, red < 50
    - **Plan configuration panel** (left sidebar):
      - Target PR count slider (e.g., "merge top 10 / 20 / 50")
      - Formula mode selector: Permutation, Combination, With Replacement (radio buttons)
      - Constraint toggles: require passing CI, exclude stale, exclude drafts, exclude conflicting PRs
      - Advanced pre-filter controls: cluster selector, stale score threshold slider, candidate pool cap, minimum score
      - "Generate Plan" button → calls `/api/repos/:owner/:repo/plans` with config params
    - **Plan summary stats**:
      - Total PRs in plan, estimated conflict count, formula space explored
      - Formula display: "C(142, 20) = X candidates evaluated"
    - **Merge order timeline**: vertical timeline showing merge sequence
      - Each step: PR title, files touched, potential conflicts with previous merges
      - Conflict warnings highlighted in red between steps
    - **Export**: "Copy as Markdown" button — generates checklist for manual merging
  - Fetch from `/api/repos/:owner/:repo/plans` endpoint with query params (target, top, mode, require_ci, exclude_stale, exclude_drafts, exclude_conflicts, cluster_id, stale_score_threshold, candidate_pool_cap, score_min)
  - **TDD**: Write tests for plan config → API request mapping, timeline rendering, markdown export

  **Must NOT do**:
  - No "Execute merge" button (actions are read-only in v0.1)
  - No merge simulation/preview
  - No branch creation
  - No plan saving/history
  - No comparison between plan configurations

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Timeline visualization, configuration panel, data-rich display
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Merge plan needs clear visual hierarchy to convey complex ordering info
  - **Skills Evaluated but Omitted**:
    - `playwright`: QA tool only

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Tasks 21, 22, 23)
  - **Blocks**: F3
  - **Blocked By**: Task 20 (layout + API client)

  **References**:

  **Pattern References**:
  - `web/src/lib/api.ts` — API client for `/api/repos/:owner/:repo/plans` endpoint from Task 20
  - `internal/planner/` — Go merge planner from Task 14 (formula + graph integration)
  - `internal/formula/` — Formula engine from Task 8 (P/C/n^k modes)

  **API/Type References**:
  - `web/src/types/api.ts` — `MergePlan`, `MergePlanCandidate`, `MergePlanConfig` types from Task 6
  - `contracts/plan-response.json` — JSON Schema for plan API response

  **External References**:
  - MAG40 FORMULA.md: `/Users/jeffersonnunn/mag40/FORMULA.md` — Formula mode naming and behavior reference (P(n,k), C(n,k), n^k)

  **WHY Each Reference Matters**:
  - `planner/` (Task 14) — Defines what the API actually returns: ordered PR list, scores, conflict predictions, formula stats
  - `formula/` (Task 8) — The mode selector (Permutation/Combination/WithReplacement) MUST use the same naming as the Go formula engine
  - MAG40 FORMULA.md — The formula display ("C(142, 20) = X") should match the notation used in the original MAG40 project for consistency
  - `plan-response.json` — The timeline visualization must render every field in the plan response (order, score, conflicts, files_touched)

  **Acceptance Criteria**:
  - [ ] `bun test` → plan page tests PASS
  - [ ] Config panel lets user set target count, formula mode, and pipeline constraints
  - [ ] "Generate Plan" button calls API with correct params
  - [ ] Timeline shows ordered merge steps with conflict warnings
  - [ ] Formula stats display shows correct notation (e.g., "C(142, 20) = ...")
  - [ ] "Copy as Markdown" produces valid checklist

  **QA Scenarios**:

  ```
  Scenario: Generate merge plan with default config
    Tool: Playwright (playwright skill)
    Preconditions: Go API running with synced repo (50+ PRs, analysis complete)
    Steps:
      1. Navigate to http://localhost:3000/plan
      2. Wait for config panel: `[data-testid="plan-config"]` visible
      3. Assert target slider visible: `[data-testid="target-slider"]`
      4. Assert formula mode radio buttons: `[data-testid="mode-combination"]` checked by default
      5. Click "Generate Plan": `[data-testid="generate-plan-btn"]`
      6. Wait for plan results: `[data-testid="plan-timeline"]` visible (timeout: 30s)
      7. Assert at least 1 merge step: `.timeline-step` count >= 1
      8. Assert formula stats visible: `[data-testid="formula-stats"]` text matches /C\(\d+,\s*\d+\)/
      9. Take screenshot
    Expected Result: Plan generated with timeline steps, formula stats, and readiness scores
    Failure Indicators: "Generate" spinner never stops, empty timeline, formula stats show "N/A"
    Evidence: .sisyphus/evidence/task-24-plan-default.png

  Scenario: Change formula mode and regenerate
    Tool: Playwright (playwright skill)
    Preconditions: Plan page loaded with initial plan generated
    Steps:
      1. Navigate to http://localhost:3000/plan
      2. Generate initial plan (click Generate Plan)
      3. Wait for results
      4. Note formula stats text → store as STATS_BEFORE
      5. Select "Permutation" mode: `[data-testid="mode-permutation"]`
      6. Click "Generate Plan" again
      7. Wait for new results
      8. Note formula stats text → store as STATS_AFTER
      9. Assert STATS_BEFORE !== STATS_AFTER (different formula)
      10. Assert STATS_AFTER text matches /P\(\d+,\s*\d+\)/ (permutation notation)
    Expected Result: Different formula mode produces different stats and potentially different plan
    Failure Indicators: Same stats after mode change, P() notation not shown, API error
    Evidence: .sisyphus/evidence/task-24-plan-permutation.png

  Scenario: Copy as Markdown produces valid checklist
    Tool: Playwright (playwright skill)
    Preconditions: Plan generated with at least 3 merge steps
    Steps:
      1. Generate a plan on /plan page
      2. Click "Copy as Markdown": `[data-testid="copy-markdown-btn"]`
      3. Read clipboard content (or assert success toast)
      4. Assert clipboard contains "- [ ]" (checkbox format)
      5. Assert clipboard contains PR numbers matching timeline
    Expected Result: Markdown checklist copied with PR numbers in merge order
    Failure Indicators: Empty clipboard, missing checkboxes, wrong PR order
    Evidence: .sisyphus/evidence/task-24-copy-markdown.txt
  ```

  **Commit**: YES
  - Message: `feat(web): implement merge plan view with formula config and timeline`
  - Files: `web/src/app/plan/page.tsx`, `web/src/app/plan/`, `web/src/components/MergeTimeline.tsx`
  - Pre-commit: `bun test`

- [ ] 25. Docker Compose Full Stack + Health Checks

  **What to do**:
- Create `docker-compose.yml` with 2 services:
    - **pratc-cli** (Go): Builds from `Dockerfile.cli`
      - Runs `pratc serve` on port 8080
      - Health check: `curl -f http://localhost:8080/api/health`
      - Volume mount: `./data:/app/data` (SQLite DB persistence)
      - Environment: `PRATC_DB_PATH=/app/data/pratc.db`
      - Includes Python 3.11 runtime + `uv` + `ml-service/` code so Go can spawn the ML subprocess locally inside the same container
      - Volume mount: `./models:/app/models` (cached sentence-transformers model)
      - Environment: `HF_HOME=/app/models`, `GITHUB_PAT` passed through at runtime when needed, `ML_BACKEND=local|openrouter`, `OPENROUTER_API_KEY` optional when hosted mode enabled
      - Compose profiles:
        - `local-ml`: `ML_BACKEND=local`, model cache volume enabled, local embeddings/runtime enabled
        - `minimax-light`: `ML_BACKEND=openrouter`, skip local model prefetch, require `OPENROUTER_API_KEY`, `OPENROUTER_EMBED_MODEL`, `OPENROUTER_REASON_MODEL`
    - **pratc-web** (Next.js): Builds from `Dockerfile.web`
      - Runs on port 3000
      - Health check: `curl -f http://localhost:3000`
      - Environment: `NEXT_PUBLIC_API_URL=http://pratc-cli:8080`
      - Depends on: pratc-cli
  - Create 2 Dockerfiles:
    - `Dockerfile.cli`: Multi-stage Go build + Python 3.11 runtime image with `uv sync` for `ml-service/` dependencies, supporting both `local-ml` and `minimax-light` profiles
    - `Dockerfile.web`: Node 20 slim, bun install, next build + start
  - Add explicit profile notes in Docker comments/docs:
    - `local-ml`: `sentence-transformers` pulls torch; expect larger images/longer builds, pin CPU-only wheels where possible, and cache `HF_HOME` via `./models`
    - `minimax-light`: skip local model downloads and rely on provider credentials/config instead
  - Create `Makefile` targets:
    - `make docker-build` — builds both images
    - `make docker-up` — docker compose up -d
    - `make docker-down` — docker compose down
    - `make docker-logs` — docker compose logs -f
  - Follow patterns from user's zackor project: `~/zackor/docker-compose.yml`
  - **TDD**: Not applicable (infrastructure) — QA scenarios verify instead

  **Must NOT do**:
  - No Kubernetes manifests
  - No CI/CD pipeline files
  - No Docker Swarm config
  - No nginx reverse proxy (direct port mapping)
  - No production TLS/HTTPS config

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: Docker Compose is straightforward config files following established patterns
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Not relevant to Docker infrastructure

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 26, 27, 28, 29)
  - **Blocks**: Task 29
  - **Blocked By**: Task 1 (monorepo structure), Task 19 (serve command), Task 20 (web dashboard)

  **References**:

  **Pattern References**:
  - `~/zackor/docker-compose.yml` — User's existing Docker Compose pattern with health checks and multi-service stack
  - `Makefile` — Build targets from Task 1 monorepo scaffold

  **API/Type References**:
  - `internal/server/` — Health endpoint path from Task 19 (`/api/health`)
  - `ml-service/pyproject.toml` — Python dependencies that Dockerfile.cli must install for subprocess ML

  **External References**:
  - Docker multi-stage builds: `https://docs.docker.com/build/building/multi-stage/`
  - Python official images: `https://hub.docker.com/_/python`

  **WHY Each Reference Matters**:
  - `~/zackor/docker-compose.yml` — Follow the EXACT health check pattern (interval, timeout, retries) the user already uses
  - `internal/server/` — Health endpoint path must match; if Go serves on `/api/health`, Docker health check must hit that exact path
  - `pyproject.toml` — Dockerfile.cli must install the exact ML dependencies listed here or subprocess spawning will fail in-container

  **Acceptance Criteria**:
  - [ ] `docker compose build` → both images build successfully
  - [ ] `docker compose --profile local-ml up -d` → both services start and pass health checks within 60s
  - [ ] `docker compose --profile minimax-light up -d` → both services start and pass health checks within 60s
  - [ ] `curl http://localhost:8080/api/health` → 200 OK from Go CLI
  - [ ] `curl http://localhost:3000` → 200 OK from web dashboard
  - [ ] `docker compose down` → clean shutdown, no orphan containers

  **QA Scenarios**:

  ```
  Scenario: Local-ML profile starts with all health checks passing
    Tool: Bash
    Preconditions: Docker Desktop running, no port conflicts on 3000/8080
    Steps:
      1. Run `docker compose build` in the repository root — assert exit code 0
      2. Run `docker compose --profile local-ml up -d` — assert exit code 0
      3. Wait 60s for health checks
      4. Run `docker compose ps` — assert both services show "healthy"
      5. Run `curl -s http://localhost:8080/api/health` — assert contains "ok"
      6. Run `curl -s -o /dev/null -w "%{http_code}" http://localhost:3000` — assert 200
      7. Run `docker compose down` — assert clean shutdown
    Expected Result: Both services build, start, pass health checks, serve traffic in local-ml mode
    Failure Indicators: Build failure, service exit, unhealthy status, connection refused
    Evidence: .sisyphus/evidence/task-25-docker-stack.txt

  Scenario: OpenRouter-light profile starts without local model dependency
    Tool: Bash
    Preconditions: Docker Desktop running, `OPENROUTER_API_KEY` available
    Steps:
      1. Run `docker compose --profile minimax-light up -d`
      2. Run `docker compose ps` — assert both services show healthy/running
      3. Run `curl -s http://localhost:8080/api/health` — confirm running
      4. Assert startup did not require local model download or populated `./models`
      5. Run `docker compose down`
    Expected Result: Hosted profile boots in lighter mode using provider credentials only
    Failure Indicators: startup blocked on local model fetch, missing provider config handling, unhealthy services
    Evidence: .sisyphus/evidence/task-25-minimax-light.txt

  Scenario: Data persistence survives restart
    Tool: Bash
    Preconditions: Docker stack running with synced data in SQLite
    Steps:
      1. Run `docker compose --profile local-ml up -d`
      2. Run `curl -s http://localhost:8080/api/health` — confirm running
      3. Run `docker compose down`
      4. Run `docker compose --profile local-ml up -d`
      5. Wait for health checks (30s)
      6. Run `curl -s http://localhost:8080/api/repos/opencode-ai/opencode/analysis` — assert data still present (non-empty response)
    Expected Result: SQLite data persists through container restart via volume mount
    Failure Indicators: Empty response after restart, "no data" error, missing volume
    Evidence: .sisyphus/evidence/task-25-data-persistence.txt
  ```

  **Commit**: YES
  - Message: `chore(docker): add Docker Compose stack with health checks for full prATC deployment`
  - Files: `docker-compose.yml`, `Dockerfile.cli`, `Dockerfile.web`, `Makefile` (updated)
  - Pre-commit: `docker compose build`

- [ ] 26. First-Run Experience — Progressive Loading + Progress UI

  **What to do**:
- Implement progressive first-run flow in both CLI and web dashboard:
  - **CLI first-run**:
    - `pratc analyze --repo=owner/repo` on fresh install detects empty cache
    - Starts or attaches to in-process background sync worker with progress bar: `Syncing PRs... [####----] 2,340/5,506 (42%)`
    - Shows partial results as they arrive: "Found 12 clusters so far..."
    - Rate limit awareness: "Rate limit: 4,200/5,000 remaining. ETA: 8 min"
    - If interrupted (Ctrl+C): saves progress, resumes on next run
    - Progress persistence: `data/sync_progress.json` tracks last cursor/page
  - **Web dashboard first-run**:
      - Empty state page when no repos synced: `EmptyState.tsx` with "Add a repository" CTA
      - During sync: progress bar in top banner showing sync status
      - Partial data rendering: show whatever clusters/analysis exist so far
      - Auto-refresh: TanStack Query refetches every 10s during active sync
  - **API support**:
    - Add `/api/repos/:owner/:repo/sync/status` endpoint returning: `{syncing: bool, job_id, progress: {current, total, eta_seconds}, rate_limit: {remaining, reset_at}}`
    - Go server tracks in-process background worker state in memory + SQLite-backed resume metadata
  - **TDD**: Write tests for progress calculation, partial result rendering, resume logic

  **Must NOT do**:
  - No webhook-based sync (polling only)
  - No separate daemon/cron/queue infrastructure in v0.1
  - No multi-repo parallel sync
  - No OAuth flow for token setup (PAT only via env/psst)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Crosses Go (sync progress, API) + TypeScript (progress UI) — needs careful coordination
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Progress UI and empty states need good visual design
  - **Skills Evaluated but Omitted**:
    - `playwright`: QA tool only

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25, 27, 28, 29)
  - **Blocks**: F3
  - **Blocked By**: Task 3 (GitHub API client with sync), Task 20 (web layout for progress UI)

  **References**:

  **Pattern References**:
  - `internal/github/` — GitHub API client from Task 3 (incremental sync, pagination, rate limiting)
  - `web/src/components/EmptyState.tsx` — Empty state component from Task 20
  - `web/src/components/SyncStatus.tsx` — Sync status component from Task 20
  - `internal/server/` — HTTP server from Task 19 (add sync status endpoint)

  **API/Type References**:
  - `web/src/types/api.ts` — Add `SyncStatus` type to existing type definitions

  **External References**:
  - TanStack Query refetch: `https://tanstack.com/query/latest/docs/react/guides/important-defaults` — refetchInterval for polling

  **WHY Each Reference Matters**:
  - `internal/github/` — The sync progress tracking MUST integrate with the existing pagination cursor logic; don't create a separate sync mechanism
  - `SyncStatus.tsx` — Task 20 created a stub; this task makes it functional with real data
  - `EmptyState.tsx` — Task 20 created the component; this task wires it to detect actual empty state

  **Acceptance Criteria**:
  - [ ] `make test` → progress + resume tests PASS
  - [ ] Sync starts as background job and returns control immediately to CLI/API caller
  - [ ] CLI shows progress bar during initial sync
  - [ ] CLI resumes from checkpoint after interruption
  - [ ] Web shows empty state when no repos synced
  - [ ] Web shows progress bar during active sync
  - [ ] `/api/repos/:owner/:repo/sync/status` returns current sync state

  **QA Scenarios**:

  ```
  Scenario: CLI first-run shows progress bar and partial results
    Tool: interactive_bash (tmux)
    Preconditions: Fresh install, no SQLite cache, PAT configured via psst
    Steps:
      1. Create tmux session: `new-session -d -s first-run`
      2. Send command: `send-keys -t first-run "psst GITHUB_PAT -- pratc analyze --repo=opencode-ai/opencode" Enter`
      3. Wait 10s for sync to begin
      4. Capture pane: `capture-pane -t first-run -p`
      5. Assert output contains progress indicator: "Syncing" or progress bar characters
      6. Assert output contains rate limit info: "Rate limit" or "remaining"
      7. Wait 30s more, capture again
      8. Assert PR count increased from first capture
    Expected Result: Progress bar updates, rate limit shown, partial results appear
    Failure Indicators: No progress output, immediate error, stuck at 0%
    Evidence: .sisyphus/evidence/task-26-cli-first-run.txt

  Scenario: CLI resumes sync after interruption
    Tool: interactive_bash (tmux)
    Preconditions: Sync started but not complete (interrupt after partial sync)
    Steps:
      1. Start sync in tmux session
      2. Wait 15s for partial sync
      3. Send Ctrl+C: `send-keys -t first-run C-c`
      4. Wait 2s for graceful shutdown
      5. Assert `data/sync_progress.json` exists
      6. Restart sync: `send-keys -t first-run "psst GITHUB_PAT -- pratc analyze --repo=opencode-ai/opencode" Enter`
      7. Wait 5s, capture pane
      8. Assert output shows "Resuming from..." or starts from non-zero offset
    Expected Result: Sync resumes from last checkpoint, doesn't re-fetch all PRs
    Failure Indicators: Starts from 0, no progress file saved, crash on resume
    Evidence: .sisyphus/evidence/task-26-resume-sync.txt

  Scenario: Web dashboard shows empty state then progress during sync
    Tool: Playwright (playwright skill)
    Preconditions: Fresh install, Go API running, no data synced
    Steps:
      1. Navigate to http://localhost:3000
      2. Assert empty state visible: `[data-testid="empty-state"]`
      3. Assert CTA button: `[data-testid="add-repo-cta"]` or setup instructions visible
      4. Trigger sync via CLI (in background)
      5. Wait 10s, refresh page
      6. Assert progress bar visible: `[data-testid="sync-progress"]`
      7. Assert progress shows percentage or PR count
      8. Take screenshot
    Expected Result: Empty state shown before data, progress bar during sync
    Failure Indicators: Blank page instead of empty state, no progress during sync
    Evidence: .sisyphus/evidence/task-26-web-first-run.png
  ```

  **Commit**: YES
  - Message: `feat: implement first-run experience with progressive sync and progress UI`
  - Files: `internal/github/sync.go`, `internal/server/sync_handler.go`, `web/src/app/`, `web/src/components/SyncStatus.tsx`
  - Pre-commit: `make test`

- [ ] 27. Edge Case Handling — Bot PRs, Draft PRs, Branch-Awareness

  **What to do**:
  - Add edge case handling across the analysis pipeline:
    - **Bot PR detection** in `internal/analysis/bots.go`:
      - Auto-detect by author: `dependabot[bot]`, `renovate[bot]`, `github-actions[bot]`, `snyk-bot`
      - Auto-detect by title pattern: `^Bump `, `^chore\(deps\)`, `^Update dependency`
      - Tag as `bot_pr: true` in PR metadata
      - Cluster bot PRs separately: "Dependency Updates" auto-cluster
      - Mark as `batch_merge_safe: true` (dependencies can merge in batch)
    - **Draft PR handling** in analysis pipeline:
      - Detect via GitHub API `draft` field
      - Exclude from merge plans by default (configurable)
      - Show in inbox with "Draft" badge but lower priority
      - Don't count in duplicate detection (drafts are WIP, may match completed PRs)
    - **Branch-awareness** in `internal/analysis/branches.go`:
      - Group PRs by base branch (`main`, `develop`, `release/v2`, etc.)
      - PRs targeting different base branches = separate merge universes
      - CLI output sections by base branch: "=== PRs targeting main (4,200) === ... === PRs targeting develop (1,300) ==="
      - Graph engine: no edges between PRs targeting different branches
      - Merge planner: only plan within same base branch
    - Add bot/draft/branch filters to CLI commands (flags: `--include-bots`, `--include-drafts`, `--base-branch=main`)
  - **TDD**: Write tests for bot detection patterns, draft filtering, branch grouping

  **Must NOT do**:
  - No custom bot detection rules (hardcoded patterns only in v0.1)
  - No cross-branch merge planning
  - No branch creation/management
  - No stacked PR detection (v0.2)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Touches analysis engine, CLI flags, and pipeline logic — moderate complexity
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: No UI changes in this task (inbox already has badge support from Task 22)

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25, 26, 28, 29)
  - **Blocks**: F4
  - **Blocked By**: Task 15 (CLI analyze command — adds flags to existing command)

  **References**:

  **Pattern References**:
  - `internal/analysis/` — Analysis pipeline from Tasks 10-12 (staleness, clustering, duplicates)
  - `internal/cmd/analyze.go` — CLI analyze command from Task 15 (add new flags)
  - `internal/github/client.go` — GitHub API client from Task 3 (PR model includes `draft` field)

  **API/Type References**:
  - `internal/types/models.go` — `PR` struct from Task 6 (add `IsDraft`, `IsBot`, `BaseBranch` fields if not present)

  **External References**:
  - Dependabot docs: `https://docs.github.com/en/code-security/dependabot` — Bot author naming patterns
  - Renovate docs: `https://docs.renovatebot.com/` — Bot PR title patterns

  **WHY Each Reference Matters**:
  - `analysis/` — Bot detection and branch grouping insert INTO the existing analysis pipeline; must understand pipeline stages
  - `cmd/analyze.go` — New flags (`--include-bots`, `--include-drafts`, `--base-branch`) add to existing Cobra command
  - `models.go` — The `PR` struct may already have `Draft` from GitHub API; check before adding fields

  **Acceptance Criteria**:
  - [ ] `go test ./internal/analysis/...` → bot + branch tests PASS
  - [ ] Bot PRs auto-detected and clustered separately
  - [ ] Draft PRs excluded from merge plans by default
  - [ ] PRs grouped by base branch in analysis output
  - [ ] `--include-bots` flag includes bot PRs in analysis
  - [ ] `--base-branch=main` filters to single branch

  **QA Scenarios**:

  ```
  Scenario: Bot PRs detected and clustered separately
    Tool: Bash
    Preconditions: Synced repo with known Dependabot PRs (openclaw has many)
    Steps:
      1. Run `psst GITHUB_PAT -- pratc analyze --repo=openclaw/openclaw --format=json | jq '.clusters[] | select(.label | contains("Dependency"))'`
      2. Assert output contains a "Dependency Updates" cluster
      3. Assert cluster PRs have `is_bot: true`
      4. Assert bot PR titles match known patterns (^Bump, ^chore(deps))
    Expected Result: Bot PRs auto-clustered, tagged correctly
    Failure Indicators: No dependency cluster, bot PRs mixed with feature clusters
    Evidence: .sisyphus/evidence/task-27-bot-detection.txt

  Scenario: Branch-aware analysis separates merge universes
    Tool: Bash
    Preconditions: Repo with PRs targeting multiple base branches
    Steps:
      1. Run `psst GITHUB_PAT -- pratc analyze --repo=openclaw/openclaw --format=json | jq '.branches | keys'`
      2. Assert output contains "main" (or default branch)
      3. If multiple branches exist, assert PR counts differ between branches
      4. Run `psst GITHUB_PAT -- pratc plan --repo=openclaw/openclaw --base-branch=main --target=5 --format=json`
      5. Assert all PRs in plan target "main" branch
    Expected Result: PRs grouped by base branch, plan restricted to single branch
    Failure Indicators: All PRs in one group, plan mixes branches
    Evidence: .sisyphus/evidence/task-27-branch-awareness.txt

  Scenario: Draft PRs excluded from merge plan by default
    Tool: Bash
    Preconditions: Repo with at least 1 draft PR
    Steps:
      1. Run `psst GITHUB_PAT -- pratc plan --repo=openclaw/openclaw --target=10 --format=json | jq '[.plans[0].prs[] | select(.is_draft == true)] | length'`
      2. Assert output === 0 (no drafts in plan)
      3. Run `psst GITHUB_PAT -- pratc plan --repo=openclaw/openclaw --target=10 --include-drafts --format=json | jq '[.plans[0].prs[] | select(.is_draft == true)] | length'`
      4. Assert output >= 0 (drafts now included if any qualify)
    Expected Result: Drafts excluded by default, included with flag
    Failure Indicators: Drafts in default plan, --include-drafts flag not recognized
    Evidence: .sisyphus/evidence/task-27-draft-exclusion.txt
  ```

  **Commit**: YES
  - Message: `feat(analysis): add bot PR detection, draft filtering, and branch-aware merge universes`
  - Files: `internal/analysis/bots.go`, `internal/analysis/branches.go`, `internal/cmd/analyze.go`
  - Pre-commit: `go test -race ./internal/analysis/... ./internal/cmd/...`

- [ ] 28. Dry-Run Mode + Audit Log for Actions

  **What to do**:
  - Implement action safety layer across CLI and web:
    - **Dry-run mode** (default for all actions):
      - `pratc plan --dry-run` → shows "DRY RUN: Would merge PR #123, PR #456, PR #789"
      - If user passes an execution-style flag in v0.1, return a clear error: "Execution disabled in v0.1 — audit/log only"
      - All CLI commands default to `--dry-run=true`
      - Web action buttons default to logging intent only
    - **Audit log** in `internal/audit/log.go`:
      - SQLite table: `audit_log (id, timestamp, action, pr_number, repo, user, dry_run, details)`
      - Log every action: approve, close, skip, merge-plan-generate, sync-start, sync-complete
      - CLI: `pratc audit` command shows recent audit entries
      - API: `/api/audit` endpoint returns paginated audit log
    - **Web audit panel**:
      - Collapsible bottom panel on all pages showing recent actions
      - Each entry: timestamp, action icon, PR reference, dry-run badge
      - "Undo" concept: for logged intents, show "Remove from queue" button
    - **Confirmation dialog** for destructive intents (logging only in v0.1):
      - "Log close intent for PR #123? This will NOT call GitHub in v0.1. Confirm?"
      - Must type repo name to confirm batch intent logging (>5 PRs)
  - **TDD**: Write tests for audit log recording, dry-run flag propagation, confirmation logic

  **Must NOT do**:
  - No actual GitHub API mutations in v0.1 under any flag
  - No audit log export (CSV/JSON export in v0.2)
  - No audit log retention policy (keep all in v0.1)
  - No user authentication for audit (single-user tool)
  - No rollback/undo for executed actions

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Crosses Go (audit log, CLI flags) + TypeScript (audit panel) — safety-critical logic
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Audit panel is simple list UI, not complex design

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 5 (with Tasks 25, 26, 27, 29)
  - **Blocks**: F4
  - **Blocked By**: Task 19 (serve command — API endpoints), Task 22 (inbox — action buttons to connect)

  **References**:

  **Pattern References**:
  - `internal/cache/` — SQLite patterns from Task 3 (database initialization, WAL mode)
  - `internal/server/` — HTTP handlers from Task 19 (add audit endpoint)
  - `web/src/components/ActionButton.tsx` — Action button from Task 20 (connect to audit logging)

  **API/Type References**:
  - `internal/types/models.go` — Add `AuditEntry` struct
  - `web/src/types/api.ts` — Add `AuditEntry`, `AuditLogResponse` types

  **External References**:
  - SQLite datetime functions: `https://www.sqlite.org/lang_datefunc.html` — For audit timestamps

  **WHY Each Reference Matters**:
  - `internal/cache/` — Audit log uses the same SQLite database; follow the same connection pattern, WAL mode, migration approach
  - `ActionButton.tsx` — The inbox action buttons from Task 22 must call the audit API; this task wires them together
  - `server/` — Audit endpoint follows same handler pattern as health/analysis/plan endpoints

  **Acceptance Criteria**:
  - [ ] `make test` → audit log + dry-run tests PASS
  - [ ] CLI defaults to dry-run mode (no GitHub mutations)
  - [ ] `pratc audit` shows recent actions
  - [ ] `/api/audit` returns paginated audit entries
  - [ ] Web action buttons log to audit (visible in audit panel)
  - [ ] Confirmation dialog appears for destructive actions when action mode on

  **QA Scenarios**:

  ```
  Scenario: CLI dry-run shows what would happen without acting
    Tool: Bash
    Preconditions: Synced repo, merge plan generated
    Steps:
      1. Run `psst GITHUB_PAT -- pratc plan --repo=openclaw/openclaw --target=5 --dry-run`
      2. Assert output contains "DRY RUN" or "dry_run"
      3. Assert output lists PR numbers that would be merged
      4. Assert NO actual GitHub API mutation occurred (check PR status unchanged)
      5. Run `pratc audit --format=json | jq '.[0]'`
      6. Assert audit entry has `dry_run: true`
    Expected Result: Dry run shows plan without executing, audit logged
    Failure Indicators: Missing "DRY RUN" label, actual PR merged, no audit entry
    Evidence: .sisyphus/evidence/task-28-dry-run.txt

  Scenario: Web action button logs to audit panel
    Tool: Playwright (playwright skill)
    Preconditions: Web dashboard running with inbox loaded
    Steps:
      1. Navigate to http://localhost:3000/inbox
      2. Wait for table loaded
      3. Select first PR, click "Approve" action button: `[data-testid="action-approve"]`
      4. Assert audit panel visible (bottom): `[data-testid="audit-panel"]`
      5. Assert latest audit entry shows: action="approve", PR number matches, "dry-run" badge visible
      6. Take screenshot
    Expected Result: Action logged in audit panel with dry-run indicator
    Failure Indicators: No audit panel, missing entry, no dry-run badge
    Evidence: .sisyphus/evidence/task-28-audit-panel.png

  Scenario: Audit CLI command shows history
    Tool: Bash
    Preconditions: Multiple actions performed (from previous scenarios)
    Steps:
      1. Run `pratc audit --limit=10 --format=json`
      2. Assert output is valid JSON array
      3. Assert at least 1 entry exists
      4. Assert each entry has: timestamp, action, pr_number, dry_run fields
      5. Assert entries sorted by timestamp descending (newest first)
    Expected Result: Audit log returns structured history of all actions
    Failure Indicators: Empty log, invalid JSON, missing fields, wrong sort order
    Evidence: .sisyphus/evidence/task-28-audit-cli.txt
  ```

  **Commit**: YES
  - Message: `feat: implement dry-run mode and audit log for action safety`
  - Files: `internal/audit/log.go`, `internal/cmd/audit.go`, `internal/server/audit_handler.go`, `web/src/components/AuditPanel.tsx`
  - Pre-commit: `make test`

- [ ] 29. README + Setup Documentation

  **What to do**:
  - Create `README.md` with:
    - **Project overview**: "PR Air Traffic Control — AI-powered management for repositories with thousands of open PRs"
    - **Quick start** (3 steps):
      1. `git clone && cd pratc`
      2. `psst set GITHUB_PAT` (preferred) or `export GITHUB_PAT=<your-token>`
      3. Choose a runtime profile:
         - `docker compose --profile local-ml up` → fully local/self-hosted ML path
         - `docker compose --profile minimax-light up` → lighter hosted-analysis path (requires OpenRouter env vars)
    - **CLI usage**:
      - `pratc analyze --repo=owner/repo` — full analysis with clustering + duplicates + conflicts
      - `pratc cluster --repo=owner/repo` — NLP clustering only
      - `pratc graph --repo=owner/repo` — dependency graph (DOT output)
      - `pratc plan --repo=owner/repo --target=20` — merge plan for top 20 PRs
      - `pratc serve` — start Go API server on localhost:8080
      - `pratc audit` — view action audit log
    - **Architecture overview**: ASCII diagram showing Go CLI ↔ Python ML ↔ Web Dashboard
    - **Configuration**: environment variables, CLI flags, formula mode selection
      - `ML_BACKEND=local|openrouter`
      - `OPENROUTER_API_KEY`, `OPENROUTER_EMBED_MODEL`, `OPENROUTER_REASON_MODEL` for hosted mode
    - **Development setup**:
      - Prerequisites: Go 1.23+, Python 3.11+, uv, Node 20+, bun, Docker, psst, gh, jq
      - `make deps` → install all dependencies
      - `make test` → run all test suites
      - `make dev` → start all services in dev mode
      - Docker runtime profiles: `local-ml` and `minimax-light`
    - **How it works** (brief):
      - Formula engine: explains P(n,k)/C(n,k)/n^k modes
      - Clustering: configurable local embeddings/HDBSCAN or hosted embeddings + GPT-5.4 reasoning
      - Graph: file overlap + mergeability heuristics → topological sort
    - **License**: MIT (or user's preference)
  - Create `web/README.md` — brief web-specific dev instructions
  - **NOT** a full documentation site — just a quality README

  **Must NOT do**:
  - No API documentation (auto-generate from code in v0.2)
  - No contribution guide (CONTRIBUTING.md in v0.2)
  - No changelog
  - No badges (build status, coverage) — no CI yet
  - No marketing copy or feature comparison tables

  **Recommended Agent Profile**:
  - **Category**: `writing`
    - Reason: Technical documentation writing
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Documentation, not UI

  **Parallelization**:
  - **Can Run In Parallel**: YES (but should wait for Docker to verify quick start steps)
  - **Parallel Group**: Wave 5 (with Tasks 25, 26, 27, 28)
  - **Blocks**: F1
  - **Blocked By**: Task 25 (Docker Compose — README references the runtime profiles)

  **References**:

  **Pattern References**:
  - `Makefile` — Build targets from Task 1 (document exact make targets)
  - `docker-compose.yml` — Docker setup from Task 25 (quick start steps)

  **API/Type References**:
  - `internal/cmd/` — All CLI commands from Tasks 15-19 (document flags and usage)

  **External References**:
  - MAG40 FORMULA.md: `/Users/jeffersonnunn/mag40/FORMULA.md` — Formula notation for "How it works" section

  **WHY Each Reference Matters**:
  - `Makefile` — README must list the EXACT make targets that exist; wrong target names = broken quick start
  - `internal/cmd/` — Each CLI command's flags and output format should be documented accurately
  - `docker-compose.yml` — Quick start profile commands must match the actual file; port numbers must be correct

  **Acceptance Criteria**:
  - [ ] `README.md` exists with all required sections
  - [ ] Quick start steps are copy-pasteable and work on clean clone
  - [ ] CLI command examples match actual implementation
  - [ ] Architecture diagram is accurate ASCII art
  - [ ] No broken links or references to non-existent files

  **QA Scenarios**:

  ```
  Scenario: Quick start steps work on fresh clone
    Tool: Bash
    Preconditions: Docker running, GITHUB_PAT available via psst
    Steps:
      1. Read README.md quick start section
      2. Follow steps exactly as written in a temp directory
      3. Assert one of the documented profile commands succeeds (from Task 25 verification)
      4. Assert `curl http://localhost:3000` returns 200
      5. Assert `curl http://localhost:8080/api/health` returns 200
    Expected Result: A user following the README can get prATC running
    Failure Indicators: Missing step, wrong port, failed docker build, missing env var
    Evidence: .sisyphus/evidence/task-29-quickstart.txt

  Scenario: CLI examples in README match actual output
    Tool: Bash
    Preconditions: prATC built and synced with a repo
    Steps:
      1. Extract code blocks from README.md that show CLI usage
      2. Run `pratc --help` — assert subcommands match README
      3. Run `pratc analyze --help` — assert flags match README description
      4. Run `pratc plan --help` — assert --target flag documented correctly
    Expected Result: README documentation matches actual CLI interface
    Failure Indicators: Undocumented flags, wrong command names, missing subcommands
    Evidence: .sisyphus/evidence/task-29-cli-docs.txt
  ```

  **Commit**: YES
  - Message: `docs: add README with quick start, CLI usage, and architecture overview`
  - Files: `README.md`, `web/README.md`
  - Pre-commit: `make test`

---

## Final Verification Wave

> 4 review agents run in PARALLEL. ALL must APPROVE. Rejection → fix → re-run.

- [ ] F1. **Plan Compliance Audit** — `oracle`
  Read the plan end-to-end. For each "Must Have": verify implementation exists (read file, run command, curl endpoint). For each "Must NOT Have": search codebase for forbidden patterns — reject with file:line if found. Check evidence files exist in .sisyphus/evidence/. Compare deliverables against plan.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

- [ ] F2. **Code Quality Review** — `unspecified-high`
  Run `go vet ./...` + `go test -race ./...` + `uv run pytest` + `bun test`. Review all changed files for: `as any`/`@ts-ignore`, empty catches, console.log in prod, commented-out code, unused imports. Check AI slop: excessive comments, over-abstraction, generic names.
  Output: `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | Files [N clean/N issues] | VERDICT`

- [ ] F3. **Real QA — Full E2E** — `unspecified-high` (+ `playwright` skill)
  Start from clean state. Execute EVERY QA scenario from EVERY task. Test cross-task integration (CLI → API → dashboard). Test edge cases: empty repo, huge repo, rate limit recovery, bot PRs. Save to `.sisyphus/evidence/final-qa/`.
  Output: `Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **Scope Fidelity Check** — `deep`
  For each task: read "What to do", read actual diff. Verify 1:1 — everything in spec was built, nothing beyond spec was built. Check "Must NOT do" compliance. Detect cross-task contamination. Flag unaccounted changes.
  Output: `Tasks [N/N compliant] | Contamination [CLEAN/N issues] | Unaccounted [CLEAN/N files] | VERDICT`

---

## Commit Strategy

| Task(s) | Commit Message | Pre-commit |
|---------|---------------|------------|
| 0 | `—` | `—` |
| 1 | `chore(infra): scaffold monorepo with Makefile and Docker Compose` | `make lint` |
| 2 | `feat(cli): scaffold Go CLI with Cobra commands` | `go vet ./...` |
| 3 | `feat(github): add rate-limit-aware API client with SQLite cache` | `go test -race ./internal/github/...` |
| 4 | `chore(ml): scaffold Python ML service with uv + pytest` | `uv run pytest` |
| 5 | `chore(web): scaffold Next.js dashboard with vitest` | `bun test` |
| 6 | `feat(types): define shared JSON interface contracts` | `go vet ./...` |
| 7 | `test(fixtures): add frozen PR data fixtures from real repos` | — |
| 8 | `feat(formula): implement combinatorial merge plan engine` | `go test -race ./internal/formula/...` |
| 9 | `feat(graph): implement PR dependency graph + topological sort` | `go test -race ./internal/graph/...` |
| 10 | `feat(ml): implement configurable clustering backend (local or hosted)` | `uv run pytest ml-service/tests/test_clustering.py` |
| 11 | `feat(ml): implement duplicate PR detection via embedding similarity` | `uv run pytest ml-service/tests/test_duplicates.py` |
| 12 | `feat(cli): implement staleness analyzer for superseded PRs` | `go test -race ./internal/staleness/...` |
| 13 | `feat(cli): implement pre-filter pipeline (cluster→CI→conflict→score)` | `go test -race ./internal/filter/...` |
| 14 | `feat(cli): integrate merge planner (formula + graph + pre-filter)` | `go test -race ./internal/planner/...` |
| 15 | `feat(cli): implement analyze command` | `go test -race ./internal/cmd/...` |
| 16 | `feat(cli): implement cluster command` | `go test -race ./internal/cmd/...` |
| 17 | `feat(cli): implement graph command with DOT output` | `go test -race ./internal/cmd/...` |
| 18 | `feat(cli): implement plan command` | `go test -race ./internal/cmd/...` |
| 19 | `feat(cli): implement serve command with HTTP API` | `go test -race ./internal/cmd/... && curl localhost:8080/api/health` |
| 20 | `feat(web): implement dashboard layout + API integration` | `bun test` |
| 21 | `feat(web): implement air traffic control cluster view` | `bun test` |
| 22 | `feat(web): implement Outlook inbox triage view` | `bun test` |
| 23 | `feat(web): implement interactive D3.js dependency graph` | `bun test` |
| 24 | `feat(web): implement merge plan recommendation view` | `bun test` |
| 25 | `chore(infra): finalize Docker Compose with health checks` | `docker compose --profile local-ml up -d && sleep 5 && curl localhost:8080/api/health` |
| 26 | `feat(web): implement progressive loading for first-run sync` | `bun test` |
| 27 | `feat(cli): handle edge cases (bot PRs, drafts, multi-branch)` | `go test -race ./...` |
| 28 | `feat(cli): add dry-run mode + audit log` | `go test -race ./internal/actions/...` |
| 29 | `docs: add README with setup instructions` | — |

---

## Success Criteria

### Verification Commands
```bash
# Build all
make build                    # Expected: binaries in ./bin/, no errors

# Test all
make test                     # Expected: all pass (go + pytest + vitest)

# CLI smoke tests
./bin/pratc analyze --repo="fixture/test" --format=json | jq '.pr_count'     # Expected: 42
./bin/pratc cluster --repo="fixture/test" --format=json | jq '.clusters | length'  # Expected: >0
./bin/pratc graph --repo="fixture/test" --format=dot | head -1               # Expected: "digraph {"
./bin/pratc plan --repo="fixture/test" --target=10 --format=json | jq '.plans[0].prs | length'  # Expected: <=10

# API smoke test
./bin/pratc serve &
curl -s http://localhost:8080/api/health | jq .status                   # Expected: "healthy"
curl -s http://localhost:8080/api/repos/fixture/test/analysis | jq .pr_count # Expected: 42

# Docker smoke test
docker compose --profile local-ml up -d
curl -s http://localhost:3000                                           # Expected: 200 OK (dashboard)
curl -s http://localhost:8080/api/health                                # Expected: {"status":"healthy"}
docker compose down

# Formula engine validation (from MAG40)
go test -race -v ./internal/formula/... -run TestPermutation            # Expected: P(54,4) = 7,590,024
go test -race -v ./internal/formula/... -run TestCombination            # Expected: C(54,5) = 3,162,510
```

### Final Checklist
- [ ] All "Must Have" items present and functional
- [ ] All "Must NOT Have" items absent (verified by grep)
- [ ] All Go tests pass with `-race` flag
- [ ] All Python tests pass (unit + integration)
- [ ] All TypeScript tests pass
- [ ] CLI outputs valid JSON for all commands
- [ ] Web dashboard renders on localhost:3000
- [ ] Docker Compose starts both services with health checks passing in both documented profiles
- [ ] Formula engine produces mathematically correct values
- [ ] No `as any`, `@ts-ignore`, empty catches, or console.log in production code
