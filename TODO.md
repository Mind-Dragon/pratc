# prATC v1.5 — Triage Engine Development Plan

Single source of truth for all v1.5 development. Requirements, implementation, and tests live here.

## Execution status (last updated: 2026-04-18)

Most v1.5 features were already implemented in the codebase. This execution session identified and completed the genuine gaps:

**Completed this session:**
- [x] Garbage classifier: added tests (9 tests in garbage_test.go). Abandoned detection deferred to buildStaleness() which has merged PR history access.
- [x] Duplicate detection: threshold 0.03, body 20%, file 40%, parallelized — already implemented.
- [x] Conflict noise filtering: filterNoiseFiles + isSourceFile + severity scoring — already implemented, added 12 tests.
- [x] Substance scoring: computeSubstanceScore + temporal routing — already implemented.
- [x] Deep judgment layers: computeBlastRadius, computeLeverage, computeHasOwner, computeReversible, computeStrategicWeight, bucketForAttentionCost, bucketForSignalQuality — already implemented, added 12 tests.
- [x] Near-duplicate bucket: overlaps (0.70-0.90) now rendered as distinct section in PDF report with orange header.
- [x] Intermediate result caching: duplicate_groups, conflict_cache, substance_cache tables with corpus fingerprinting (schema v6). 8 tests added.
- [x] All tests pass: `go test ./...` + `go vet ./...` clean (27 packages, 0 failures).

**Deferred (requires production run verification):**
- [ ] Garbage classifier catches > 80% of bad PRs (needs live corpus run)
- [ ] Duplicate detection finds > 10 groups (needs live corpus run)
- [ ] Conflict count < 5,000 (needs live corpus run)
- [ ] Full analysis < 15 minutes (needs live corpus run)

## Critical bugs found during live run (2026-04-18)

### BUG-1: GitHub auth token not passed to sync worker

**Root cause:** `defaultWorker()` in `internal/sync/default_runner.go:150` creates the GitHub client with `os.Getenv("GITHUB_TOKEN")` directly. The sync command calls `github.ResolveToken(ctx)` at line 145 of `sync.go`, but the resolved token is discarded — it's never passed to the worker.

**Impact:** When `GITHUB_TOKEN` env var is not set, prATC uses unauthenticated GitHub API rate limit (60 req/hr) even though `gh auth token` returns a valid token. This makes any sync of >60 PRs impossible without manually exporting `GITHUB_TOKEN`.

**Fix:**
- [ ] Change `defaultWorker()` to call `github.ResolveToken(context.Background())` instead of `os.Getenv("GITHUB_TOKEN")`
- [ ] Pass the resolved token (or error) into the worker constructor
- [ ] Add test: `TestDefaultWorker_UsesResolvedToken` — verify worker gets token from `gh auth token` fallback
- [ ] Audit all other `os.Getenv("GITHUB_TOKEN")` usages to ensure they also use `ResolveToken`

### BUG-2: Repo name case sensitivity causes cache fragmentation

**Root cause:** `openclaw/openclaw` and `OpenClaw/OpenClaw` are treated as different repos. The SQLite cache stores PRs, sync progress, and artifacts under the exact string the user typed. A user who runs `pratc analyze --repo=OpenClaw/OpenClaw` gets a different (empty) cache than `--repo=openclaw/openclaw` which has 6,646 PRs.

**Impact:** Wasted sync time, duplicate data, confusing behavior. User may think the tool is broken when they just used different casing.

**Fix:**
- [ ] Add `NormalizeRepoName(repo string) string` function that lowercases the owner/repo
- [ ] Apply normalization at every entry point: sync, analyze, workflow, serve, cluster, graph, plan, report
- [ ] Migrate existing cache entries: add a migration that lowercases all `repo` columns in: pull_requests, sync_progress, sync_jobs, audit_log, duplicate_groups, conflict_cache, substance_cache
- [ ] Add test: `TestNormalizeRepoName` — "OpenClaw/OpenClaw", "openclaw/openclaw", "oPeNcLaW/oPeNcLaW" all → "openclaw/openclaw"
- [ ] Add test: `TestCacheRepoNormalization` — same PR stored under different casings resolves to one entry
- [ ] Update CLI help text to note repo names are case-insensitive

### BUG-3: No pre-flight check or singleton lock for long-running operations

**Problem:** Running `pratc sync` or `pratc workflow` against a large repo (19K+ PRs) can take hours. There's no way to:
1. Estimate how long a sync will take before committing to it
2. Detect if another prATC instance is already running against the same repo (causing duplicate API calls, rate limit waste, and timeouts)

**Fix — Pre-flight check:**
- [ ] Add `preflight` subcommand or `--preflight` flag to sync/workflow
- [ ] Pre-flight checks:
  - Cached PR count vs live open PR count (shows delta)
  - Estimated API calls needed (based on delta + rate limit remaining)
  - Estimated time (based on API calls / rate limit)
  - Rate limit status (remaining requests, reset time)
  - Whether a fresh sync is even needed (cache is recent enough)
- [ ] Output: human-readable summary like "60 new PRs to fetch, ~120 API calls, ~2 minutes at current rate. Proceed? [Y/n]"
- [ ] Add test: `TestPreflight_DeltaEstimate` — cached 1000 PRs, live 1060 PRs → estimate 60 PRs to fetch

**Fix — Singleton lock:**
- [ ] Add a file-based lock (`~/.pratc/locks/<repo-hash>.lock`) acquired before any sync/workflow/analyze
- [ ] Lock file contains: PID, start time, command
- [ ] If lock exists and process is still running → error "another prATC instance is running for this repo (PID X, started Y)"
- [ ] If lock exists but process is dead → stale lock, auto-clean and proceed
- [ ] Lock released on exit (defer) or on signal (SIGTERM/SIGINT)
- [ ] Add `--force` flag to override lock (with warning)
- [ ] Add test: `TestSingletonLock_AcquireAndRelease` — acquire, verify lock, release, verify clean
- [ ] Add test: `TestSingletonLock_BlockedByActive` — second acquire fails while first holds lock
- [ ] Add test: `TestSingletonLock_StaleLockCleanup` — dead PID lock is auto-cleaned

## Source of truth hierarchy

1. `GUIDELINE.md` — bucket vocabulary, layer ordering, non-negotiables (authority)
2. `ARCHITECTURE.md` — system shape, data flow, packages, SLOs
3. `ROADMAP.md` — milestone sequence and release-line intent
4. `v1.5-triage-engine-plan.md` — design rationale and production failure analysis
5. This document — actionable development plan (execution authority)

If this document conflicts with GUIDELINE.md, GUIDELINE.md wins.

## Release contract

**Goal:** The triage engine produces actionable output from a 6,646+ PR corpus. Every PR lands in a defensible bucket. The report tells a human what to merge, what to close, and what to ignore.

**Baseline (from openclaw/openclaw production run):**
- 6,646 PRs ingested
- 0 duplicate groups detected (broken)
- 92,911 "conflicts" produced (useless noise)
- 95% of PRs land in "needs_review" with no differentiation
- No garbage/junk pre-filter exists
- Deep judgment layers 6-16 documented but not computed

**Release success criteria:**
- [ ] Garbage classifier catches > 80% of obviously bad PRs
- [ ] Duplicate detection finds > 10 duplicate groups
- [ ] Conflict count < 5,000 actionable conflicts
- [ ] Review pipeline differentiates high_value / needs_review / low_value
- [ ] Deep judgment layers 6-16 produce scores for every PR
- [ ] PDF report executive summary tells the operator what to do
- [ ] Full analysis completes in < 15 minutes for 6,000+ PRs
- [ ] All tests pass (zero failures on main before release)

## Pre-work: Fix existing test debt

Before v1.5 feature work begins, the 5 pre-existing test failures must be green.

### PW-1: Fix TestHandleAnalyze (x3 failures)

- **Files:** `internal/cmd/analyze_command_test.go`, `internal/app/service_test.go`
- **Root cause:** TBD — run `go test -run TestHandleAnalyze -v ./internal/cmd/ ./internal/app/` to isolate
- **Acceptance:** `TestHandleAnalyze` passes in both `internal/cmd/` and `internal/app/`
- **Test:** `go test -count=1 -run TestHandleAnalyze ./internal/cmd/ ./internal/app/`

### PW-2: Fix TestCorsMiddleware (x2 failures)

- **Files:** `internal/cmd/cors_test.go`
- **Root cause:** TBD — run `go test -run TestCorsMiddleware -v ./internal/cmd/`
- **Acceptance:** `TestCorsMiddleware` passes
- **Test:** `go test -count=1 -run TestCorsMiddleware ./internal/cmd/`

### PW-3: Verify all tests green

- **Acceptance:** `go test ./...` exits 0
- **Gate:** No feature work begins until this is confirmed

---

## Phase 1: Outer Peel (layers 1-3)

Strip obvious noise before deeper reasoning consumes time. Order: 1.1 → 1.2 → 1.3 (sequential dependency).

### 1.1 Garbage classifier

**Requirement:** Every PR that is abandoned, bot-generated, empty, spam, or malformed gets classified into the `junk` bucket with a reason code before it enters duplicate detection or review.

**Implementation:**

1.1.1 Create `internal/app/garbage.go`
- New function: `classifyGarbage(prs []types.PR) []types.PR, []types.GarbageReason`
- Returns filtered PRs (non-garbage) and garbage reasons for classified PRs
- Each reason has: `PRNumber int`, `Bucket string` ("junk"), `ReasonCode string`, `Details string`

1.1.2 Implement abandoned detection
- **Rule:** No commits, no comments in > 90 days AND author has no other recent PRs (last 30 days)
- **Reason code:** `abandoned`
- **Data source:** PR metadata (updated_at, author activity from cache)

1.1.3 Implement bot PR detection
- **Rule:** Author login matches known bot patterns (`[bot]`, `dependabot`, `renovate`, `github-actions`, etc.)
- **Reason code:** `bot_generated`
- **Data source:** existing `internal/analysis/bots.go` — extend `IsBot()` if needed
- **File:** `internal/analysis/bots.go` (extend), `internal/app/garbage.go` (call)

1.1.4 Implement empty PR detection
- **Rule:** 0 additions AND 0 deletions AND 0 changed files (or only metadata changes)
- **Reason code:** `empty_pr`
- **Data source:** PR additions/deletions/changed_files from cache

1.1.5 Implement spam pattern detection
- **Rule:** Title or body matches known signatures: "test", "asdf", single-character titles, URL-only bodies, "WIP" with no description > 90 days old
- **Reason code:** `spam_pattern`
- **Data source:** PR title, body from cache

1.1.6 Implement malformed detection
- **Rule:** Missing title OR (missing body AND missing files) OR git error metadata
- **Reason code:** `malformed`
- **Data source:** PR title, body, files from cache

1.1.7 Wire into service pipeline
- **File:** `internal/app/service.go` — call `classifyGarbage()` before `classifyDuplicates()`
- **Integration:** Garbage PRs get assigned `junk` bucket in the analysis response
- **Progress emit:** emit `garbage` phase with count of classified PRs

1.1.8 Add garbage reason to analysis response
- **File:** `internal/types/models.go` — add `GarbageReasons []GarbageReason` to `AnalysisResponse`
- **File:** `internal/types/models.go` — add `GarbageReason` struct: `PRNumber int`, `ReasonCode string`, `Details string`

**Tests:**

1.1.T1 Create `internal/app/garbage_test.go`
- `TestClassifyGarbage_AbandonedPR` — PR with no activity > 90 days, author inactive
- `TestClassifyGarbage_BotPR` — PR by `dependabot[bot]`
- `TestClassifyGarbage_EmptyPR` — PR with 0 additions, 0 deletions
- `TestClassifyGarbage_SpamPR` — PR with title "test" and empty body
- `TestClassifyGarbage_MalformedPR` — PR with empty title
- `TestClassifyGarbage_ActivePRNotGarbage` — fresh PR by active author should NOT be classified
- `TestClassifyGarbage_BoundaryConditions` — PR at exactly 90 days, PR at 89 days
- `TestClassifyGarbage_ReasonCodesValid` — all reason codes match GUIDELINE.md vocabulary

1.1.T2 Extend `internal/analysis/bots_test.go`
- `TestIsBot_KnownBotPatterns` — dependabot, renovate, github-actions, [bot] suffix
- `TestIsBot_HumanAuthor` — normal username should NOT match

**Acceptance:**
- [ ] Garbage classifier runs before duplicate detection
- [ ] Each garbage PR has a reason code from: abandoned, bot_generated, empty_pr, spam_pattern, malformed
- [ ] Non-garbage PRs pass through unchanged
- [ ] Progress emits show garbage phase with count
- [ ] All tests in 1.1.T1 and 1.1.T2 pass

---

### 1.2 Fix duplicate detection

**Requirement:** Duplicate detection finds > 10 duplicate groups in a 6,646-PR corpus. Currently returns 0 groups.

**Root cause (from production analysis):**
- Quick-skip threshold (`titleScore < 0.1`) too aggressive — drops valid pairs like "fix: update deps" / "chore: bump dependencies"
- Body contributes only 10% to score — insufficient to overcome low title Jaccard
- File-overlap signal not used

**Implementation:**

1.2.1 Lower quick-skip threshold
- **File:** `internal/app/service.go` or wherever duplicate comparison happens
- **Change:** Lower `titleScore < 0.1` quick-skip to `titleScore < 0.03` (or remove entirely for cache-mode runs)
- **Rationale:** Preserve pairs with low title Jaccard but meaningful body/file overlap

1.2.2 Re-weight body similarity
- **File:** same as above
- **Change:** Body weight from 10% → 25% of composite score
- **Rationale:** "chore: bump dependencies" and "fix: update deps" have different titles but similar bodies

1.2.3 Add file-overlap as primary signal
- **File:** same as above
- **Change:** If two PRs share > 50% of changed files AND have ANY title similarity (Jaccard > 0), flag as potential duplicate
- **Rationale:** File overlap is the strongest signal for functional duplicates

1.2.4 Add near-duplicate threshold
- **File:** same as above
- **Change:** Score 0.70-0.90 → `near_duplicate` bucket (distinct from `duplicate` at 0.90+)
- **Data source:** existing overlap threshold at 0.70

1.2.5 Update bucket assignment
- **File:** `internal/types/models.go` — ensure `near_duplicate` bucket exists
- **File:** `internal/app/service.go` — assign `duplicate` or `near_duplicate` based on score

**Tests:**

1.2.T1 Create or extend duplicate detection tests
- `TestDuplicateDetection_SimilarTitlesDifferentPhrasing` — "fix: update deps" / "chore: bump dependencies" → duplicate
- `TestDuplicateDetection_HighBodyOverlap` — different titles, identical bodies → duplicate
- `TestDuplicateDetection_FileOverlapSignal` — same files, any title similarity → duplicate
- `TestDuplicateDetection_NearDuplicateThreshold` — score 0.75 → near_duplicate bucket
- `TestDuplicateDetection_NoFalsePositive` — unrelated PRs should NOT be flagged
- `TestDuplicateDetection_QuickSkipPreservesValidPairs` — pairs that old threshold would drop

1.2.T2 Benchmark test
- `BenchmarkDuplicateDetection_6000PRs` — completes in < 30s

**Acceptance:**
- [ ] Quick-skip threshold lowered to 0.03 (or removed)
- [ ] Body weight ≥ 20% of composite score
- [ ] File-overlap > 50% with any title similarity triggers duplicate consideration
- [ ] Near-duplicate bucket (0.70-0.90) exists and is assigned
- [ ] Finds > 10 duplicate groups in openclaw/openclaw corpus
- [ ] All tests in 1.2.T1 pass
- [ ] Benchmark < 30s for 6,000 PRs

---

### 1.3 Fix conflict graph

**Requirement:** Conflict count drops from 92,911 to < 5,000 actionable conflicts.

**Root cause:** `buildConflicts()` checks if any two PRs share a file. In a monorepo, every PR touches `package.json`, `pnpm-lock.yaml`, shared configs.

**Implementation:**

1.3.1 Define noise file list
- **File:** `internal/graph/noise_files.go` (new)
- **Contents:** constant list of noise file patterns: `package.json`, `pnpm-lock.yaml`, `yarn.lock`, `package-lock.json`, `.github/`, `.eslintrc`, `tsconfig.json`, `.gitignore`, `*.lock`, CI config patterns
- **Function:** `IsNoiseFile(path string) bool`

1.3.2 Filter noise from conflict detection
- **File:** `internal/graph/graph.go` or `internal/app/service.go`
- **Change:** Before counting conflicts, filter out files matching noise patterns

1.3.3 Add conflict severity scoring
- **File:** `internal/graph/conflict_severity.go` (new)
- **Scoring:**
  - 1 shared signal file = low severity
  - 2-4 shared signal files = medium
  - 5+ shared signal files = high
  - Source code overlap > config file overlap = higher severity
  - Same subsystem/directory overlap = higher severity
- **Struct:** `ConflictSeverity { Level string, SharedFileCount int, SignalFiles []string, SameSubsystem bool }`

1.3.4 Separate conflict types
- **Merge-blocking:** high severity, same subsystem, source code overlap
- **Attention-needed:** medium severity, config overlap, different subsystems

1.3.5 Update analysis response
- **File:** `internal/types/models.go` — add `ConflictSeverity` to conflict entries if not already present

**Tests:**

1.3.T1 Create `internal/graph/noise_files_test.go`
- `TestIsNoiseFile_PackageJSON` — true
- `TestIsNoiseFile_PnpmLock` — true
- `TestIsNoiseFile_SourceCode` — false
- `TestIsNoiseFile_GithubActions` — true
- `TestIsNoiseFile_TestFile` — false

1.3.T2 Create or extend conflict graph tests
- `TestConflictGraph_NoiseFilesFiltered` — conflict count drops when noise files are filtered
- `TestConflictGraph_SignalFilesPreserved` — source code conflicts still detected
- `TestConflictGraph_SeverityScoring_High` — 5+ shared source files → high
- `TestConflictGraph_SeverityScoring_Low` — 1 shared config file → low
- `TestConflictGraph_MergeBlockingVsAttentionNeeded` — correct classification

1.3.T3 Integration test
- `TestConflictGraph_OpenclawCorpus` — on cached corpus, conflict count < 5,000

**Acceptance:**
- [ ] Noise file patterns defined and applied before conflict counting
- [ ] Conflict severity scoring implemented (low/med/high)
- [ ] Merge-blocking vs attention-needed separation works
- [ ] Conflict count < 5,000 on openclaw/openclaw corpus
- [ ] All tests in 1.3.T1, 1.3.T2 pass

---

## Phase 2: Substance Scoring (layers 4-5)

Score remaining work against things that actually matter. Depends on Phase 1 completing.

### 2.1 Substance score calculation

**Requirement:** Every non-garbage, non-duplicate PR gets a substance score (0-100) that differentiates high_value (>= 70), needs_review (30-69), and low_value (< 30).

**Implementation:**

2.1.1 Create `internal/app/substance.go`
- Function: `ComputeSubstanceScore(pr types.PR, findings []types.AnalyzerFinding) int`
- Inputs: PR metadata, review analyzer findings from `internal/review/`

2.1.2 Scoring dimensions (weighted):
- **File depth** (25%): PRs touching many files in core subsystems score higher
  - Source: PR changed_files, file paths
  - Core subsystems configurable via settings
- **Test coverage delta** (20%): PRs that add/update tests alongside source score higher
  - Source: detect test files in changed_files
- **Review findings** (25%): security/risk findings lower score, quality findings raise it
  - Source: existing `internal/review/` analyzer output
- **Author activity** (15%): first-time contributors get review flag (not penalty); active authors score higher
  - Source: author history from cache
- **PR age** (15%): fresh PRs score higher than 6-month-old with no activity
  - Source: PR created_at, updated_at

2.1.3 Wire into service pipeline
- **File:** `internal/app/service.go` — call `ComputeSubstanceScore()` after garbage/duplicate phases
- **Integration:** score flows into bucket assignment

2.1.4 Bucket routing from substance score
- **File:** `internal/app/service.go`
- Rules:
  - `high_value` (score >= 70) + `mergeable` + not stale → `now` temporal bucket
  - `high_value` + not mergeable or stale → `future`
  - `needs_review` (score 30-69) → `needs_review` quality bucket
  - `low_value` (< 30) → `low_value` quality bucket, `future` temporal bucket

**Tests:**

2.1.T1 Create `internal/app/substance_test.go`
- `TestSubstanceScore_HighValuePR` — many files, core subsystem, tests included, active author → score >= 70
- `TestSubstanceScore_LowValuePR` — 1 file, no tests, old, inactive author → score < 30
- `TestSubstanceScore_BoundaryConditions` — score at exactly 70, 69, 30, 29
- `TestSubstanceScore_SecurityFindingsLowerScore` — PR with security findings scores lower
- `TestSubstanceScore_TestFilesRaiseScore` — PR with test changes scores higher
- `TestSubstanceScore_FirstTimeContributor` — gets review flag, not penalty
- `TestSubstanceScore_DeterministicScore` — same inputs always produce same score

2.1.T2 Bucket routing tests
- `TestBucketRouting_HighValueMergeableNow` — high_value + mergeable + fresh → now
- `TestBucketRouting_HighValueStaleFuture` — high_value + stale → future
- `TestBucketRouting_LowValueFuture` — low_value → future + low_value
- `TestSubstanceScore_RoutingIntegration` — end-to-end from score to bucket

**Acceptance:**
- [ ] Substance score computed for every non-garbage, non-duplicate PR
- [ ] Score is deterministic (same inputs → same output)
- [ ] Bucket routing correctly maps score ranges to quality/temporal buckets
- [ ] Review pipeline differentiates high_value / needs_review / low_value
- [ ] All tests in 2.1.T1, 2.1.T2 pass

---

## Phase 3: Deep Judgment Layers (layers 6-16)

Apply deeper scoring to decide what deserves human time. Depends on Phase 2 completing.

### 3.1 Add fields to types

**Requirement:** AnalysisResponse carries all 16 layer scores per PR.

**Implementation:**

3.1.1 Extend `internal/types/models.go`
- Add to `AnalysisResponse` (or per-PR analysis result):
  - `Confidence float64` — 0-1.0
  - `BlockedBy []int` — PR numbers that must merge first
  - `BlastRadius string` — "low" | "med" | "high"
  - `Leverage float64` — 0-1.0
  - `HasOwner bool`
  - `StrategicWeight float64` — 0-1.0
  - `AttentionCost string` — "low" | "med" | "high"
  - `Reversible bool`
  - `SignalQuality string` — "noise" | "signal"

3.1.2 Ensure JSON serialization
- All new fields must serialize correctly for API and report output
- Omit empty fields where appropriate

**Tests:**

3.1.T1 Extend `internal/types/models_test.go`
- `TestAnalysisResponse_DeepJudgmentFieldsPresent` — all new fields exist
- `TestAnalysisResponse_DeepJudgmentJSON` — serializes/deserializes correctly
- `TestAnalysisResponse_FieldDefaults` — zero values are sensible

### 3.2 Compute each layer

**Implementation:**

3.2.1 Create `internal/app/deep_judgment.go`
- Function: `ComputeDeepJudgments(prs []types.PR, conflictGraph types.ConflictGraph, reviewResults []types.ReviewResult) []types.DeepJudgment`

3.2.2 Layer 6: Confidence
- **Rule:** Count of review findings with confidence > 0.7 / total findings
- **If < 0.5 findings:** mark as `needs_review`
- **Data source:** existing `internal/review/` confidence calculation
- **File:** may extend `internal/review/confidence.go`

3.2.3 Layer 7: Dependency (BlockedBy)
- **Rule:** From conflict graph — PRs that must merge before this one
- **Data source:** `internal/graph/` conflict graph, merge-blocking conflicts
- **Mark as `blocked`** if any blocker exists

3.2.4 Layer 8: Blast radius
- **Rule:** Number of downstream files affected + subsystem criticality
- **Low:** touches only tests/docs/config
- **Med:** touches source code in non-critical subsystems
- **High:** touches core subsystems (auth, security, api, data layer)

3.2.5 Layer 9: Leverage
- **Rule:** Number of PRs this unblocks / total blocked PRs
- **Data source:** dependency graph inverse
- **High leverage (> 0.5):** bump to `high_value`

3.2.6 Layer 10: Ownership (HasOwner)
- **Rule:** Author has commits in last 30 days AND PR is not abandoned
- **No owner:** assign `re_engage` or `stale` bucket

3.2.7 Layer 11: Stability
- **Rule:** From existing staleness analysis
- **Already partially implemented** — wire into new field

3.2.8 Layer 12: Mergeability
- **Rule:** From GitHub API `mergeable` field
- **Already captured** — use for routing decisions

3.2.9 Layer 13: Strategic weight
- **Rule:** Subsystem match to known project priorities (from settings)
- **Configurable per-repo** via `internal/settings/`

3.2.10 Layer 14: Attention cost
- **Rule:** file count + diff size + number of review findings
- **High cost:** defer to `future`

3.2.11 Layer 15: Reversibility
- **Rule:** Touches only test files, docs, or config → reversible
- **Reversible PRs** get lower risk weighting

3.2.12 Layer 16: Signal quality
- **Rule:** Composite of layers 6-15
- **Noise:** multiple layers flag concerns
- **Signal:** passes most layers cleanly
- **Final gate** before human review bucket assignment

**Tests:**

3.2.T1 Create `internal/app/deep_judgment_test.go`
- `TestLayer6_Confidence_LowFindings` — < 0.5 findings → needs_review
- `TestLayer6_Confidence_HighFindings` — > 0.7 findings → high confidence
- `TestLayer7_Dependency_BlockedPR` — has blockers → blocked
- `TestLayer7_Dependency_UnblockedPR` — no blockers → not blocked
- `TestLayer8_BlastRadius_TestsOnly` — low
- `TestLayer8_BlastRadius_CoreSubsystem` — high
- `TestLayer9_Leverage_HighUnblocksMany` — > 0.5 → high_value
- `TestLayer10_Ownership_ActiveAuthor` — has owner
- `TestLayer10_Ownership_InactiveAuthor` — no owner
- `TestLayer14_AttentionCost_HighLargeDiff` — high
- `TestLayer15_Reversibility_TestsOnly` — true
- `TestLayer16_SignalQuality_Composite` — noise vs signal determination

3.2.T2 Integration tests
- `TestDeepJudgment_AllLayersProduceOutput` — every PR gets all layer scores
- `TestDeepJudgment_BucketRefinement` — layers correctly refine bucket assignment

**Acceptance:**
- [ ] All 11 new fields exist on AnalysisResponse
- [ ] Each layer computes correctly from available data
- [ ] Layers refine bucket assignment (merge_candidate vs needs_review with flags)
- [ ] All tests in 3.1.T1, 3.2.T1, 3.2.T2 pass

---

## Phase 4: Report Composition

The PDF reads like a decision map, not a dump. Depends on Phases 1-3 completing.

### 4.1 Restructure PDF sections

**Requirement:** Report section order matches GUIDELINE.md decision map.

**Implementation:**

4.1.1 Restructure `internal/report/`
- **File:** `internal/report/analyst_sections.go` — restructure section generation
- **File:** `internal/report/pdf.go` — update section ordering

4.1.2 Section order:
1. Executive summary (total PRs, breakdown by bucket)
2. Junk/noise section (garbage PRs with reason codes)
3. Duplicates section (canonicals + chains)
4. "Do this now" (high_value + merge_candidate, sorted by substance score)
5. "Needs review" (needs_review, sorted by risk flags)
6. "Defer to future" (low_value, future, blocked)
7. Full appendix (every PR with bucket + reason codes)

4.1.3 Executive summary format
- Must say something like: "6,646 PRs analyzed. 234 garbage, 187 duplicates. 142 high-value merge candidates. 6,083 need human review."
- Bucket counts must be accurate and auditable

**Tests:**

4.1.T1 Extend `internal/report/analyst_sections_test.go`
- `TestReportSectionOrder_MatchesGuideline` — section order matches GUIDELINE.md
- `TestExecutiveSummary_BucketCounts` — counts match actual bucket assignments
- `TestJunkSection_ReasonCodesPresent` — every garbage PR has a reason code
- `TestDuplicateSection_CanonicalsListed` — canonical PRs identified
- `TestNowQueue_SortedBySubstance` — sorted descending by substance score
- `TestAppendix_AllPRsPresent` — every PR appears in appendix

### 4.2 Actionable recommendations

**Requirement:** Report includes specific recommendations an operator can act on.

**Implementation:**

4.2.1 Add recommendations section
- **File:** `internal/report/analyst_sections.go`
- Recommendations:
  - "Close these N garbage PRs" (list)
  - "These N duplicates have a canonical — close the rest" (list)
  - "These N PRs are high-value and mergeable — merge them now" (list)
  - "These N PRs need focused review before merge" (list)

4.2.2 Each recommendation includes PR numbers and one-line reasons

**Tests:**

4.2.T1 Extend report tests
- `TestRecommendations_CloseGarbage` — lists garbage PRs with count
- `TestRecommendations_CloseDuplicates` — lists duplicate chains with canonicals
- `TestRecommendations_MergeNow` — lists high-value mergeable PRs
- `TestRecommendations_FocusedReview` — lists needs_review PRs

**Acceptance:**
- [ ] Report section order matches GUIDELINE.md decision map
- [ ] Executive summary has accurate bucket counts
- [ ] Every PR appears in report (none vanish)
- [ ] Recommendations are specific and actionable (include PR numbers)
- [ ] All tests in 4.1.T1, 4.2.T1 pass

---

## Phase 5: Performance

Full analysis < 15 minutes for 6,000+ PRs. Depends on Phases 1-4 completing.

### 5.1 Parallelize duplicate detection

**Requirement:** Duplicate detection completes in < 30s for 6,000+ PRs.

**Implementation:**

5.1.1 Parallelize outer loop
- **File:** `internal/app/service.go` (or wherever duplicate detection runs)
- **Pattern:** Goroutine workers for outer loop iterations
- Each worker handles a chunk of outer iterations
- Results merged at the end (conflict-free since each worker writes to its own slice)

5.1.2 Bounded concurrency
- Use `runtime.NumCPU()` workers (or configurable)
- Avoids overwhelming the system while maximizing throughput

**Tests:**

5.1.T1 Create `internal/app/duplicate_perf_test.go`
- `BenchmarkDuplicateDetection_Sequential` — baseline
- `BenchmarkDuplicateDetection_Parallel` — must be faster than sequential
- `TestDuplicateDetection_ParallelMatchesSequential` — same results regardless of parallelism

### 5.2 Cache intermediate results

**Requirement:** Subsequent runs reuse computed results instead of recomputing.

**Implementation:**

5.2.1 Store intermediate results in SQLite
- **File:** `internal/cache/sqlite.go` — add tables for duplicate_groups, conflict_graph, substance_scores
- **Schema:** tied to corpus hash (PR set fingerprint) so stale cache is detected

5.2.2 Cache invalidation
- **Rule:** Recompute only when the PR set changes (different PR count or different PR numbers)
- **File:** `internal/app/service.go` — check cache before recomputing

5.2.3 Cache hit path
- Load cached duplicate groups, conflict graph, substance scores
- Skip expensive recomputation
- Still run garbage classifier (cheap) to catch new junk

**Tests:**

5.2.T1 Create `internal/cache/intermediate_cache_test.go`
- `TestCacheStore_DuplicateGroups` — store and retrieve
- `TestCacheStore_ConflictGraph` — store and retrieve
- `TestCacheStore_SubstanceScores` — store and retrieve
- `TestCacheInvalidation_PRSetChanged` — different PR set triggers recomputation
- `TestCacheInvalidation_PRSetUnchanged` — same PR set uses cache
- `TestCacheHitPath_Faster` — cache hit completes faster than recomputation

**Acceptance:**
- [ ] Duplicate detection < 30s for 6,000 PRs
- [ ] Parallel results match sequential results
- [ ] Intermediate results cached in SQLite
- [ ] Cache invalidation works correctly
- [ ] All tests in 5.1.T1, 5.2.T1 pass

---

## Phase 6: Operational Guardrails (carried from v1.4.2)

Ongoing operational requirements. Work on these alongside feature phases.

### 6.1 Managed-service hardening

- [ ] Workflow/service path explicit about current sync state and resume metadata
- [ ] Sync resumes cleanly after rate-limit pauses and transient interruptions
- [ ] Health, status, and run-manifest output aligned for background supervision
- [ ] Last checkpoint preserved when session dies and worker restarts

### 6.2 Corpus-coverage regression guardrails

- [ ] Keep a 6,000+ PR smoke/benchmark in place
- [ ] Corpus caps explicit and configurable only where intentional
- [ ] Guard against hidden truncation reappearing in CLI or planning paths

### 6.3 Doc synchronization

- [ ] Keep README.md, ROADMAP.md, version1.4.2.md, v1.5-triage-engine-plan.md, and CHANGELOG.md aligned
- [ ] Keep this TODO.md updated as items complete
- [ ] Archive completed items instead of deleting them

---

## Release checklist

Before tagging v1.5:

- [ ] All pre-work test debt resolved (PW-1, PW-2, PW-3)
- [ ] All Phase 1 items complete and tested
- [ ] All Phase 2 items complete and tested
- [ ] All Phase 3 items complete and tested
- [ ] All Phase 4 items complete and tested
- [ ] All Phase 5 items complete and tested
- [ ] `go test ./...` passes with zero failures
- [ ] `go vet ./...` passes
- [ ] Production run against openclaw/openclaw meets all success criteria
- [ ] ROADMAP.md updated with v1.5 status
- [ ] CHANGELOG.md updated with v1.5 changes
- [ ] version1.5.md milestone summary created
- [ ] Git tag created
