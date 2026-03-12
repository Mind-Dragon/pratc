# Swarm Board — PRATC — 2026-03-12

> Shared coordination hub. Read before starting. Update after completing.

**Goal:** You are an execution agent working on `prATC` in this repository.

  Source of truth:
  - Build plan: `.sisyphus/plans/pratc.md`
  - Execution contract: `AGENTS.md`

  Your job:
  - Help complete the full scoped plan without reducing scope.
  - Follow the dependency graph, acceptance criteria, workflow rules, testing rules, and merge rules defined in those files.
  - Keep `main` green. No task is done until it is merged into `main` and verified on `main`.

  Universal rules for every agent:
  - Read `AGENTS.md` and `.sisyphus/plans/pratc.md` before taking substantial action.
  - Treat `.sisyphus/plans/pratc.md` as the source of truth for what to build.
  - Treat `AGENTS.md` as the source of truth for how to work safely.
  - Use root-relative paths. Do not create a nested `pratc/` directory.
  - Default to one worktree or feature branch per task.
  - Do not silently change scope, reorder dependencies incorrectly, or skip required verification.
  - If you discover a contradiction, ambiguity, or plan defect, record it explicitly and resolve it before continuing.
  - Preserve all requested end-state deliverables.

  Task execution rules:
  - Only work on tasks whose dependencies are satisfied.
  - Own one task, or one tightly-coupled task bundle, unless explicitly instructed otherwise.
  - Keep changes bounded to the task’s owned files or subsystem.
  - Add or update tests as required by the plan.
  - Produce evidence under `.sisyphus/evidence/`.
  - Update task state under `.sisyphus/status/` using one file per task.

  Task status rules:
  - Status values are: `todo`, `in_progress`, `blocked`, `code_complete`, `merged`, `verified`.
  - Each task status file should include:
    - task id
    - title
    - owner
    - status
    - branch or worktree
    - dependencies
    - changed files
    - evidence paths
    - tests run
    - merge commit
    - residual risks

  Swarm rules:
  - The swarm may include 1 coordinator and up to 3 builders, but every agent must act safely even if role boundaries are imperfect.
  - If you are acting as a builder:
    - work only on dependency-ready assigned work
    - report changed files, commands run, tests run, evidence paths, and blockers
    - do not declare success before merge and verification on `main`
  - If you are acting as coordinator or integrator:
    - dispatch only dependency-ready tasks
    - maintain `.sisyphus/status/`
    - merge completed work into `main`
    - run post-merge verification on `main`
    - fix forward immediately if `main` fails

  Verification rules:
  - Minimum verification gate on `main`:
    - `make build`
    - `make test`
  - Also run any task-specific checks required by the plan.
  - Do not leave `main` in a failing state.
  - If post-merge verification fails, fix forward on `main` and rerun until green.

  Implementation rules:
  - Contracts under `contracts/` are generated from canonical Go and TypeScript models.
  - Fixtures policy:
    - keep a small sanitized committed fixture set in `fixtures/`
    - do not commit large raw captures as normal source files
    - document capture source, date, command, and sanitization in evidence
  - Hosted AI mode uses retry and timeout controls plus explicit fallback behavior.
  - SQLite migrations are forward-only and versioned.

  Critical path priority:
  - `0 -> 1 -> 3 -> 13 -> 14 -> 18 -> 19 -> 20`

  Output expectations:
  - Be concise and operational.
  - State what task you are working on.
  - State what dependencies are satisfied.
  - State what files you changed.
  - State what tests you ran.
  - State what evidence you produced.
  - State whether the task is only `code_complete` or fully `merged` and `verified`.

  Completion rule:
  - Work is incomplete until it is merged into `main` and verified on `main`.

  Small recommendation: if all agents truly get the same prompt, make the coordinator identify itself by writing/owning .sisyphus/status/coordination.md first. That creates a concrete leadership signal and reduces
  duplicate dispatch.

---

## Active Skills

The following behavioral directives apply to ALL agents in this swarm:

- **Incremental Commits:** Make small, atomic git commits after each meaningful change. Each commit should be independently valid and have a clear, descriptive message. Never batch unrelated changes into a single commit.
- **Test-Driven:** Follow test-driven development: write failing tests first, then implement the minimum code to make them pass. Ensure all new code has corresponding test coverage.
- **Code Review:** Before finalizing any changes, perform a thorough self-review: check for bugs, security issues, edge cases, and adherence to project conventions. Leave inline comments explaining non-obvious decisions.
- **Keep CI Green:** After every change, run the project linter, type checker, and test suite. Do not proceed to the next task until all CI checks pass. Fix any failures immediately.
- **Performance:** Optimize for performance: minimize unnecessary re-renders, avoid N+1 queries, use appropriate data structures, lazy-load where possible, and profile before and after changes when feasible.
- **DRY Principle:** Aggressively eliminate code duplication. Extract shared logic into reusable utilities, hooks, or base classes. When you see similar patterns repeated, consolidate them into a single source of truth.

---

## Task Breakdown

> Coordinator: fill this table. Each task must list owned files — no overlaps between tasks.
> Status lifecycle: OPEN → ASSIGNED → PLANNING → BUILDING → REVIEW → DONE

| ID | Task | Owner | Owned Files | Depends On | Status |
|----|------|-------|-------------|------------|--------|
| 3 | GitHub API client + SQLite cache (TDD) | Builder 2 | internal/github/, internal/cache/ | 1 | ASSIGNED |
| 8 | Formula engine — P(n,k), C(n,k), n^k (TDD) | Builder 3 | internal/formula/ | 2, 6 | ASSIGNED |
| 9 | Graph engine — dependency/conflict + topo sort (TDD) | Builder 4 | internal/graph/ | 2, 6 | DONE |
| 13 | Pre-filter pipeline (TDD) | Builder 4 | internal/filter/ | 2, 6, 3 | ASSIGNED |

---

## Coordinator 1

**Role:** coordinator
**Status:** DISPATCHING
**Assigned Task:** —
**Owned Files:** —
**Progress:**
Wave 2 dispatch in progress. Critical path: T3 → T13 → T14.

---

## Builder 2

**Role:** builder
**Status:** DONE
**Assigned Task:** Task 3 — GitHub API client + SQLite cache (TDD)
**Owned Files:** internal/github/, internal/cache/
**Progress:**
- Implemented SQLite-backed cache with WAL mode, PR persistence/querying, merged PR metadata, and sync job/progress persistence.
- Implemented rate-limit-aware GitHub GraphQL client with paginated PR fetch, file/review/CI queries, low-budget backoff, and descriptive limit errors.
- Validation complete: `go test -race -v ./internal/cache/...` and `go test -race -v ./internal/github/...` both passing.

---

## Builder 3

**Role:** builder
**Status:** DONE
**Assigned Task:** Task 8 — Formula engine P(n,k), C(n,k), n^k (TDD)
**Owned Files:** internal/formula/
**Progress:**
- Implemented `internal/formula/` with overflow-safe combinatorial counts, direct index-based candidate generation, deterministic scoring, and tiered search.
- Validation passed with `go test -race -v ./internal/formula/...`; evidence written to `.sisyphus/evidence/task-8-formula-validation.txt`, `.sisyphus/evidence/task-8-index-generation.txt`, and `.sisyphus/evidence/task-8-edge-cases.txt`.
- Task is `code_complete`; merge and verification on `main` are still pending coordinator integration.

---

## Builder 4

**Role:** builder
**Status:** ASSIGNED
**Assigned Task:** Task 13 — Pre-filter pipeline (TDD)
**Owned Files:** internal/filter/
**Progress:**
Task 13 assigned. Dependencies: T2, T6 complete. Write tests in internal/filter/*_test.go.

---

## Reviewer 5

**Role:** reviewer
**Status:** REVIEWING
**Assigned Task:** Task 9 — Graph engine review
**Owned Files:** —
**Progress:**
- Completed review of Task 9 (Graph engine) — APPROVED
- Verified: 4 tests pass with race detection, clean implementation, proper DOT output, branch-universe conflict isolation
- Awaiting next task from Coordinator

---

## Completed Work Log

| Task | Agent | Summary | Files Changed |
|------|-------|---------|---------------|
| 3 | Builder 2 | Implemented SQLite cache and GitHub GraphQL client with rate-limit handling, pagination, merged PR metadata, and sync job persistence. | `internal/cache/models.go`, `internal/cache/sqlite.go`, `internal/cache/sqlite_test.go`, `internal/github/client.go`, `internal/github/queries.go`, `internal/github/client_test.go`, `go.mod`, `go.sum` |
| 9 | Builder 4 | Implemented graph engine package with dependency/conflict edges, topological sort, cycle detection, and DOT output. | `internal/graph/graph.go`, `internal/graph/graph_test.go` |
| 8 | Builder 3 | Implemented formula engine with MAG40-validated counts, direct index generation, scoring, and tiered search. | `internal/formula/config.go`, `internal/formula/engine.go`, `internal/formula/modes.go`, `internal/formula/scoring.go`, `internal/formula/tiers.go`, `internal/formula/formula_test.go` |
