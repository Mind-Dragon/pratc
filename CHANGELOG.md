# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Version shift note (2026-04-16):** v1.4.2 is the shipped managed-service triage release. `version1.4.2.md` covers resumable sync, explicit states, and crash-safe background operation.
> Dashboard polish remains slated for v1.5.
> Evidence enrichment remains v1.6 work.
> Analyst PDF reporting and decision-trail output are part of the v1.4.2 operator packet.

> **Phase renaming (2026-04-15):** v1.4.2 restructured as full-corpus triage engine.
> Previous Phase A (Planning Integration) and Phase B (Analyst PDF) are now Phase 0 (Foundation).
> New Phases A–F cover corpus coverage, outer peel, substance scoring, routing, deep judgment, and report.
> See ROADMAP.md for the current phase definitions.

## [Unreleased]

### v1.4.2 — Full-Corpus Triage Engine (COMPLETED)

#### Added

- **Risk buckets added to the review contract** — `review_payload.risk_buckets` now flows through the API and web type surface alongside `buckets` and `priority_tiers`.
- **Risk bucket rendering in the analyst packet** — Review PDFs now show `security_risk`, `reliability_risk`, and `performance_risk` alongside operational priority tiers.
- **Decision trail section in the analyst packet** — The report now includes a readable per-PR decision ladder view instead of burying layers inside raw reasons.
- **Analyst loader compatibility wrapper** — Public `LoadAnalystDataset(...)` entrypoint keeps tests and report sections aligned on the same assembly path.
- **Web type sync for review payloads** — The frontend types now match the enriched review payload contract.

### v1.4.2 Phase 0 — Foundation (COMPLETED)

_Previously labeled Phase A (Planning Integration) and Phase B (Analyst PDF Report)._

#### Added

- **Planning strategy selection** — `planning_strategy` field added to `OperationTelemetry`. Configurable via `Config.PlanningStrategy` ("formula" or "hierarchical").
- **PoolSelector integration** — Weighted priority scoring replaces the legacy heuristic pre-filter. Produces `PoolStrategy = "weighted_priority"` and `PoolStrategy = "exponential_decay"` telemetry. Reason codes attached to each candidate (staleness, ci, security, cluster, recency).
- **HierarchicalPlanner integration** — 3-level cluster-based planning: cluster identification, tier assignment (critical/high/medium/low), and topological cluster ordering. Records `HierarchicalComplexityReduction` in telemetry.
- **PairwiseExecutor sharded conflict detection** — Sharded pairwise PR conflict detection with early-exit on first conflict found. Reports `PairwiseShards`, `PairwiseEarlyExits`, and `PairwiseWorkersActive` in telemetry.
- **TimeDecayWindow with protected lane** — Exponential time-decay scoring with configurable `HalfLifeHours`, `WindowHours`, `ProtectedHours`, and `MinScore` floor. Protected PRs (recent critical PRs) are preserved in the candidate pool.
- **PriorityWeights configuration** — `PriorityWeights` struct (`StalenessWeight`, `CIStatusWeight`, `SecurityLabelWeight`, `ClusterCoherenceWeight`, `TimeDecayWeight`) exposed in `Config`. Round-trip serialisation via settings.
- **`--planning-strategy` CLI flag** — `pratc plan --planning-strategy hierarchical` to use the new hierarchical planner.

#### Changed

- **Planning telemetry fields added** — `OperationTelemetry` gains `PlanningStrategy`, `HierarchicalComplexityReduction`, `PairwiseEarlyExits`, `PairwiseWorkersActive`.
- **`MergePlanCandidate` gains `Reasons` field** — `[]string` holding PoolSelector reason codes alongside `Rationale`.

#### Performance

- **PoolSelector benchmarks** — `BenchmarkPoolSelector_SelectCandidates`: ~16ms / 1.8MB / 4,080 allocs for fixture corpus. `BenchmarkPoolSelector_SelectCandidatesWithClusterCoherence`: ~22ms / 776KB / 3,437 allocs.
- **End-to-end Plan() benchmarks (fixture corpus)** — formula strategy: ~319ms/op; hierarchical strategy: ~325ms/op. Both well under 90s SLO.

### v1.4.2 Phase 0 — Analyst PDF Report (IN PROGRESS → absorbed into Phase 0)

#### Added

- **Full PR analysis table** — Paginated table covering all PRs with columns: PR number, title, author, age, cluster, score, reason codes, classification, recommended action. Each PR has at least one reason code.
- **Per-PR reason codes in analyze output** — PoolSelector reason codes (`staleness_high`, `ci_pass`, `security_label`, `has_cluster`, `in_recency_window`, `trivial_change`, `bot_generated`, `spam_pattern`, etc.) wired into the `analyze` endpoint response.
- **Spam/junk classification** — Review classifier extended with spam/bot/noise detection. Classification labels: `trivial_change`, `bot_generated`, `spam_pattern`, `malformed`, `promotional`, `abandoned_no_activity`.
- **Duplicate detail list in report** — Report section listing canonical PRs and their duplicates with similarity scores and grouping reasons.
- **Spam/junk list in report** — Dedicated report section listing low-value PRs with classification and supporting evidence.
- **Category summary section** — Rollup counts by classification bucket (high_value, merge_candidate, duplicate, spam, stale, blocked, needs_review, re_engage) with representative examples.
- **Recommendations section in report** — Practical triage plan: top-N to inspect, to-close duplicates, to-close spam, to-re-engage, clusters to merge together.

### Security

- **Added X-API-Key authentication middleware** — All non-health API endpoints now require `X-API-Key` header. Key stored in `~/.pratc/api-key` with mode 0600. Configurable via `PRATC_API_KEY` environment variable.
- **Sanitized error messages** — Client-facing errors no longer leak internal paths, SQL details, or stack traces. Full errors logged server-side with request ID correlation.
- **Rate limiting middleware** — Per-IP rate limits: 100 req/min general, 10 req/min for `/analyze` and `/sync`. Returns 429 with `Retry-After` header. Configurable via `PRATC_RATE_LIMIT_*` environment variables.
- **WebSocket origin validation** — Origins now validated with proper URL parsing (`net/url`). Rejects non-http schemes (data:, javascript:, file:). Startup validation for configured origins with logging.
- **Removed token injection** — GitHub tokens no longer injected into `os.environ`. Passed explicitly via struct fields to prevent inheritance by subprocesses.

### Performance

- **Graph build O(n²) → O(n)** — Replaced pairwise PR comparison with hash map indexing. For 5500 PRs: ~15M → ~5500 comparisons. Uses `BaseBranch → []PR` index for dependency detection and file inverted index for conflict detection.
- **Topological sort O(V×E log V) → O(V log V)** — Replaced repeated `sort.Ints()` with `container/heap` for ready queue. Single heap operation per PR instead of re-sorting on every iteration.
- **String concatenation optimized** — Replaced `result += sep + s` loops with `strings.Builder` across planner, planning, and report packages. Eliminates O(n²) string copying.
- **HTTP connection pooling** — Configured `http.Transport` with `MaxIdleConns: 100`, `MaxIdleConnsPerHost: 10`, `IdleConnTimeout: 90s`. Request timeout: 30s. Configurable via `PRATC_HTTP_*` environment variables.
- **ML service vectorization** — Python ML service now uses:
  - MinHash/LSH (datasketch) for O(n) duplicate detection instead of O(n²) brute force
  - Inverted file index for sparse overlap detection
  - NumPy matrix multiplication for batch cosine similarity
  - LRU cache for tokenization (1024 entries)

### Code Quality

- **Extracted shared utilities** — Created `internal/util/strings.go` with `Tokenize()` and `Jaccard()` functions. Removed duplicate implementations from `app/service.go` and `planner/planner.go`.
- **Constants package** — Created `internal/types/constants.go` with named constants for thresholds, weights, and defaults:
  - `DuplicateThreshold = 0.90`
  - `OverlapThreshold = 0.70`
  - `TitleWeight = 0.6`, `FileWeight = 0.3`, `BodyWeight = 0.1`
  - `HighRiskConfidenceCap = 0.79`
  - `DefaultTarget = 20`, `DefaultCandidatePoolCap = 100`, `MaxTarget = 1000`
- **Silent error logging** — Added logging to previously silent error paths in WebSocket upgrade, JSON marshal, and time parsing.
- **GitHub URL prefix constant** — Defined `GitHubURLPrefix` constant and replaced 10 scattered occurrences across test and source files.

### Documentation

- **Added RATELIMITS.md** — Comprehensive guide to GitHub API rate limits, prATC budget management, retry logic, and best practices.
- **Updated AGENTS.md** — Corrected package index (telemetry/ratelimit has 1821 LOC, not empty). Marked bubble sort and app/planner duplication as FIXED.
- **Added installer script** — `scripts/install.sh` for one-line installation on Linux/macOS with prerequisite checks, binary download, and PATH setup.

### Changed

- **License changed to FSL-1.1-Apache-2.0** — Non-commercial use only. Automatically converts to Apache 2.0 after 2 years. Commercial licenses available.
- **Updated .gitignore** — Excludes `.pratc/`, `.pratc-tree/`, workflow outputs, and analysis results.

### Fixed

- **AGENTS.md accuracy** — Removed references to non-existent empty packages (models/, mq/, search/, config/). Updated filter/AGENTS.md to reflect bubble sort fix and correct pool cap (100, not 64).

## [1.3.0] - 2026-04-09

### Added

- **Omni batch planning** — Select PRs by ID ranges and boolean expressions via selector syntax (`1-100 AND 50-150`). Endpoint: `GET /api/repos/:owner/:repo/plan/omni`
- **Review pipeline** — Advisory analyzers for security, reliability, performance, and quality with confidence scores and evidence references
- **Evidence-backed review output** — PRs categorized into buckets: `merge_now`, `focused_review`, `duplicate/superseded`, `problematic`, `escalate`
- **Selector parser** — Expression language for PR selection with AND, OR, parentheses, and range support

### Changed

- **Default API port** — Changed from 8080 to 7400 (reserved prATC range: 7400-7500)
- **Review engine** — Evidence-backed PR review workflows with merged/open duplicate detection

## [0.2.0] - 2026-03-23

### Added

- `PRATC_CACHE_DIR` environment variable for mirror storage location
- `mirror list`, `mirror info`, `mirror clean` commands
- `--use-cache-first` flag for cache-first analysis workflow
- Background sync auto-trigger for first-time analysis
- Batch git fetch operations (100 refs per call)
- Parallel file extraction with worker pool (10 concurrent)

### Changed

- Default mirror storage: `~/.cache/pratc/repos/` (XDG cache directory)
- Reduced git fetch from N sequential to ceil(N/100) batched calls

## [0.1.0] - 2026-03-15

### Added

- Initial release of prATC (PR Air Traffic Control)
- CLI commands: `analyze`, `cluster`, `graph`, `plan`, `serve`, `sync`, `audit`
- Web dashboard with ATC overview, triage inbox, dependency graphs
- Python ML service for PR clustering and duplicate detection
- SQLite cache with incremental GitHub sync
- GitHub GraphQL client with rate limiting and retry logic
- Pre-filter pipeline for draft, conflict, CI status, and bot detection
- Combinatorial formula engine for merge planning
- Docker Compose profiles for local-ml and minimax-light deployments

[Unreleased]: https://github.com/Mind-Dragon/pratc/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/Mind-Dragon/pratc/compare/v0.2.0...v1.3.0
[0.2.0]: https://github.com/Mind-Dragon/pratc/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Mind-Dragon/pratc/releases/tag/v0.1.0
