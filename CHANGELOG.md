# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.7.0] — 2026-04-23

### Added

- **WS1 — Diff Analysis Evidence**: Subsystem detection fields (` subsystem`, `riskyPatterns`, `deepScore`) wired into `AnalyzeResponse`; evidence classifier produces structured diff-grounded signals for security, reliability, and quality gates.
- **WS2 — Dependency Impact**: API surface change detection, shared module boundary analysis, and schema change tracking integrated into the analyze path.
- **WS3 — Test Evidence**: Test movement tracking and coverage estimation added to the analyze pipeline; tests that moved or changed alongside code are surfaced in the evidence record.
- **WS4 — PlanOptions Contract Widening**: `PlanOptions` struct with 8 fields (`Target`, `Mode`, `IncludeBots`, `ScoreMin`, `StaleDays`, `StaleScoreThreshold`, `CandidatePoolCap`, `ConflictFilterMode`) wired through CLI (`--target`, `--mode`, `--include-bots`, `--score-min`, etc.) and HTTP (`?target=`, `?mode=`, `?include_bots=`, etc.); existing `Plan()` call in service contract updated to `PlanWithOptions()`.
- **WS4 — Explicit Review Failure Semantics**: `ReviewResponse` gains `Partial`, `Errors`, and `FailedPRs` fields so partial failures during multi-PR review are distinguishable from complete successes; per-PR error capture in `reviewErrorsByPR`.
- **WS4 — Unified Token Source**: `TokenSource` interface replaces separate GraphQL/REST token acquisition paths; `singleTokenSource` used for both; `IsRetryableError` tightened so generic 403 responses are no longer retryable.
- **WS5 — Release Hardening**: All v1.7 TODO items closed; remediation note finalized; full green suite confirmed across Go tests (race), vet, build, and Python tests (19/19).

### Fixed

- **Rate-limit token fallback**: `github` client stopped retrying token acquisition after first successful resolution, preventing unnecessary `gh auth token` invocations on every request.
- **Budget remaining clamp**: `CanAfford` now correctly uses `>=` at the reserve boundary; `RecordResponse` clamps negative remaining values to 0.
- **Engine empty tier skip**: Planner correctly skips empty tier results instead of producing malformed output.
- **Cache legacy paused sync**: Resumed legacy paused sync jobs now proceed normally instead of stalling.

### Changed

- **Plan dry-run default**: `PlanWithOptions` dry-run defaults to `true` (safe); callers must explicitly pass `false` to execute.
- **Conflict noise filtering**: Expanded noise file list includes `schema.base.generated.ts`, `schema.help.ts`, `schema.labels.ts`, `docs/docs.json`, and `docs/.generated/*` paths.

## [1.6.0] — 2026-04-21

### Fixed

- Workflow now carries a full artifact contract end-to-end: `sync.json`, `analyze.json`, step-numbered artifacts, and a real `report.pdf` generated before workflow completion.
- Post-sync workflow phases (`cluster`, `graph`, `plan`) now preserve the workflow snapshot truthfully even when `analyze` runs longer than cache TTL.
- Duplicate classification now preserves truthful `0.80` duplicate groups on cache-backed fresh reruns as well as cached reloads, restoring duplicate presence on full-corpus cached analysis.
- Large-corpus duplicate detection now uses MinHash/LSH candidate generation with exact rescoring, keeping the sparse 6k synthetic benchmark around `~90ms/op` instead of `~21s/op` for exact pairwise comparison.
- Conflict-noise filtering now suppresses additional generated/docs-heavy OpenClaw paths (`docs/.generated/*`, `docs/docs.json`, `schema.base.generated.ts`, `schema.help.ts`, `schema.labels.ts`).
- Substance scoring now uses a wider and more informative composite (source-file impact, test signal, freshness, diff footprint, and clean findings) instead of a compressed 30–70 band.
- Audit smoke runs under 150 PRs no longer hard-fail duplicate/junk presence checks; those now downgrade to manual when the sample is intentionally too small.
- Public docs now describe the cache-first default, the full workflow artifact contract, and the current release-ready validation state without stale installer/version examples.

### Removed

- Stale root-level planning docs and scratch release notes (`pratc.md`, `prATC _ App Architecture Plan.md`, `REPORT_TODO.md`, `v1.5-triage-engine-plan.md`, `version1.4.1.md`) were dropped from the release surface.

### Verified

- Cache-backed full workflow against `openclaw/openclaw` (`6,632` PRs) is audit-green with `17` passing checks, `0` failures, and a generated `report.pdf`.
- Explicit live validation (`--refresh-sync --force-live`) against `openclaw/openclaw` is also audit-green with the same `17` passing checks, `0` failures, and a generated `report.pdf`.

## [1.5.0] — 2026-04-18

### Triage Engine Fixes (from live openclaw/openclaw production run)

#### Fixed

- **BUG-1: GitHub auth token passthrough** — `defaultWorker()` was using `os.Getenv("GITHUB_TOKEN")` directly instead of the token resolved by `github.ResolveToken()`. Sync fell to unauthenticated rate limit (60 req/hr) when env var was not set. Now uses resolved token from `gh auth token` fallback.
- **BUG-2: Repo name case sensitivity** — `OpenClaw/OpenClaw` and `openclaw/openclaw` were treated as different repos, fragmenting the cache. Added `NormalizeRepoName()` (lowercase + trim) at all CLI entry points. Migration v7 deduplicates and lowercases all repo columns.
- **BUG-3: No pre-flight check** — Running sync on large repos could take hours with no warning. Added `pratc preflight --repo=X` command showing cached PR count, GitHub delta, estimated API calls, time, and rate limit status.
- **BUG-3: No singleton lock** — Multiple prATC instances could run against the same repo, wasting rate limit. Added file-based lock at `~/.pratc/locks/<md5(repo)>.lock` with PID check and `ps -ef | grep pratc` fallback for stale lock cleanup.
- **BUG-4: Duplicate threshold unreachable** — `DuplicateThreshold` was 0.90 but scoring formula maxes at 0.85 (file>0.8 + title>0.05 boost path). Lowered threshold to 0.85.
- **BUG-5: Conflict count 92,715** — `buildConflicts()` created pairs for any shared file. Now requires 2+ shared signal files. Expanded noise list: go.mod, go.sum, Cargo.toml/lock, pyproject.toml, setup.py, requirements.txt, Makefile, Dockerfile, README.md, LICENSE.
- **BUG-6: No intermediate caching** — Pipeline recomputed duplicates and conflicts from scratch every run. Wired `CorpusFingerprint` + cache load/save into `classifyDuplicates` and `buildConflicts` paths. Second run on same corpus skips O(n^2) recomputation.

#### Added

- **`pratc preflight` command** — Pre-flight check showing cache delta, GitHub open PR count, estimated API calls, time, and lock status
- **Singleton lock** — File-based per-repo lock (`~/.pratc/locks/<md5(repo)>.lock`) with `--force` override
- **Repo normalization** — `NormalizeRepoName()` in `internal/types/repo_normalize.go`, applied at all 8 CLI entry points
- **Intermediate cache** — Schema v7 with `duplicate_groups`, `conflict_cache`, `substance_cache` tables and `CorpusFingerprint()` for invalidation
- **Near-duplicate section in PDF** — Overlap groups (0.70-0.85) rendered as distinct section with orange header
- **Garbage classifier tests** — 9 tests in `internal/app/garbage_test.go`
- **Conflict noise tests** — 12 tests in `internal/app/conflict_noise_test.go`
- **Deep judgment tests** — 12 tests in `internal/review/deep_judgment_test.go`
- **Intermediate cache tests** — 8 tests in `internal/cache/intermediate_cache_test.go`
- **Pipeline cache tests** — 4 tests in `internal/app/pipeline_cache_test.go`
- **Auth tests** — 3 tests in `internal/sync/auth_test.go`
- **Repo normalization tests** — 5 tests in `internal/types/repo_normalize_test.go`
- **Lock tests** — 8 tests in `internal/cmd/lock_test.go`
- **Preflight tests** — 3 tests in `internal/cmd/preflight_test.go`
- **Duplicate threshold tests** — 2 tests in `internal/app/duplicate_threshold_test.go`

#### Changed

- `DuplicateThreshold` constant: 0.90 → 0.85
- `buildConflicts()` minimum shared file threshold: 0 → 2
- `noiseFiles` map expanded with 11 new entries
- Auth: all commands now pass resolved token to worker constructors
- CLI: all 8 commands normalize repo names at entry point

#### Performance

- Full corpus analysis: 6,632 PRs in 28.5 min (exceeded 5-min SLO, expected for O(n^2))
- Intermediate caching: second run skips duplicate detection and conflict graph recomputation

## [1.4.2] — 2026-04-16

### Full-Corpus Triage Engine

- Local-first sync with explicit sync ceilings
- Resumable sync from SQLite checkpoints
- Rate-limit pause/restart with auto-resume
- Managed-service states (queued/running/paused/resuming/completed/failed/canceled)
- Planning integration (PoolSelector, HierarchicalPlanner, PairwiseExecutor, TimeDecayWindow)
- Review pipeline with security/reliability/performance/quality analyzers
- PDF analyst report with PR table, bucket counts, recommendations
- 16-layer decision ladder documented and partially implemented
- Doc synchronization across README, ROADMAP, version docs, CHANGELOG

## [1.3.0] — 2026-04-09

- Omni batch planning with selector expressions
- Review pipeline with evidence-backed output
- PR buckets: merge_now, focused_review, duplicate/superseded, problematic, escalate
- Default API port changed to 7400

## [0.2.0] — 2026-03-23

- Mirror storage, cache-first workflow, batch git fetch
- Parallel file extraction with worker pool

## [0.1.0] — 2026-03-15

- Initial release: CLI commands, web dashboard, ML service, SQLite cache
