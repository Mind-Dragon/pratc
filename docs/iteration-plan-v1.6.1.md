# prATC v1.6.1 Precision Iteration Plan

## 1. Philosophy

**No batch changes. Every change is a single-file, single-purpose commit with a contract test that fails before the fix and passes after.**

This prevents the cascading breakage that killed v1.6.0. Each PR is reviewable in under 5 minutes. Each PR has exactly one reason to exist.

---

## 2. Dependency Graph

```
Phase A: Safety Net (no code changes, only tests)
  |
  v
Phase B: Data Integrity (SQLite, cache, settings)
  |
  v
Phase C: Error Propagation (stop swallowing errors)
  |
  v
Phase D: API Contract (HTTP handler fixes)
  |
  v
Phase E: Scoring Logic (pool, engine, planner)
  |
  v
Phase F: Telemetry & Observability (budget, rate limit, progress)
  |
  v
Phase G: Polish & Merge (final verification, benchmark)
```

**Rule:** You cannot start Phase N+1 until Phase N is fully merged to main and the full suite is green.

---

## 3. Phase Breakdown

### Phase A: Safety Net (Week 1, Days 1-2)

**Goal:** Every bug we fix gets a test that catches it. No test = no fix.

**PR A1:** `test(audit): contract tests for P0 critical issues`
- Files: `internal/telemetry/ratelimit/budget_test.go`, `internal/github/auth_test.go`, `internal/cache/sqlite_test.go`
- Add 8 contract tests, each failing with current code:
  - `TestCanAfford_ZeroRequests` — documents inverted boundary
  - `TestAttemptWithTokenFallback_StopsAfterSuccess` — documents no-break bug
  - `TestLegacyPausedJobs_ResumedByScheduler` — documents orphaned state
  - `TestRecordResponse_NegativeRemaining` — documents unvalidated input
  - `TestUpdateSyncJobProgress_Atomic` — documents non-atomic update
  - `TestPauseSyncJob_Atomic` — documents non-atomic pause
  - `TestTierResult_EmptyNotAppended` — documents nil Best.Selected
  - `TestScoreClusterCoherence_NotPlaceholder` — documents stub function
- **Merge gate:** All 8 tests fail. CI is red. This is correct.

**PR A2:** `test(audit): contract tests for P1 major issues`
- Files: `internal/cmd/serve_test.go` (new), `internal/planning/pool_test.go`, `internal/formula/engine_test.go`
- Add 6 contract tests:
  - `TestPlanHandler_RespectsExcludeConflicts` — documents ignored param
  - `TestPlanHandler_RespectsDryRun` — documents missing dry_run
  - `TestAuthMiddleware_ContentTypeOn401` — documents missing header
  - `TestRateLimitHeader_Unlimited` — documents wrong Inf display
  - `TestResolveTokenForLogin_UsesLoginParam` — documents ignored login
  - `TestTransientBackoff_WithinDocumentedCap` — documents jitter overflow
- **Merge gate:** All 6 tests fail. CI is red.

---

### Phase B: Data Integrity (Week 1, Days 3-5)

**Goal:** Fix the SQLite and cache issues that can corrupt state or lose jobs.

**PR B1:** `fix(budget): CanAfford boundary condition`
- File: `internal/telemetry/ratelimit/budget.go` line 206
- Change: `(available - requests) > b.reserveBuffer` → `(available - requests) >= b.reserveBuffer`
- **Merge gate:** `TestCanAfford_ZeroRequests` passes. Full suite green.

**PR B2:** `fix(github): AttemptWithTokenFallback stops after success`
- File: `internal/github/auth.go` line 488-500
- Change: Add `break` after `lastErr == nil`
- **Merge gate:** `TestAttemptWithTokenFallback_StopsAfterSuccess` passes.

**PR B3:** `fix(cache): legacy paused jobs resumed by scheduler`
- File: `internal/cache/sqlite.go` line 856-857
- Change: `ListPausedSyncJobs` queries both `paused` and `paused_rate_limit`
- **Merge gate:** `TestLegacyPausedJobs_ResumedByScheduler` passes.

**PR B4:** `fix(budget): RecordResponse validates negative remaining`
- File: `internal/telemetry/ratelimit/budget.go` line 212-218
- Change: Clamp `remaining` to `>= 0` before assignment
- **Merge gate:** `TestRecordResponse_NegativeRemaining` passes.

**PR B5:** `fix(cache): UpdateSyncJobProgress is atomic`
- File: `internal/cache/sqlite.go` line 474-518
- Change: Wrap both UPDATEs in a single transaction
- **Merge gate:** `TestUpdateSyncJobProgress_Atomic` passes.

**PR B6:** `fix(cache): PauseSyncJob is atomic`
- File: `internal/cache/sqlite.go` line 814-846
- Change: Wrap both UPDATEs in a single transaction with rollback on error
- **Merge gate:** `TestPauseSyncJob_Atomic` passes.

---

### Phase C: Error Propagation (Week 2, Days 1-2)

**Goal:** Stop swallowing errors. Every error must be visible to the caller or logged.

**PR C1:** `fix(service): per-PR review errors are logged and returned`
- File: `internal/app/service.go` line 808-812
- Change: Log error + accumulate into `errs []error`, return `multierr.Combine(errs)` if any
- **Merge gate:** Existing `TestAnalyze` passes. New contract test `TestAnalyze_PerPRErrorVisible` passes.

**PR C2:** `fix(service): review failure returns error instead of empty payload`
- File: `internal/app/service.go` line 623-636
- Change: Return error from `orchestrator.Review` instead of continuing with zeroed result
- **Merge gate:** `TestReview_ReturnsError` passes. Existing analyze tests updated if needed.

**PR C3:** `fix(service): context cancellation honored by review pipeline`
- File: `internal/app/service.go` line 808-812
- Change: Check `ctx.Err()` before each `orchestrator.Review` call, propagate `context.Canceled`
- **Merge gate:** `TestAnalyze_ContextCancellation` passes.

---

### Phase D: API Contract (Week 2, Days 3-5)

**Goal:** HTTP handlers must match documented behavior.

**PR D1:** `fix(serve): plan handler respects exclude_conflicts, stale_score_threshold, candidate_pool_cap, score_min`
- File: `internal/cmd/serve.go` line 541-562
- Change: Parse params, pass to `service.Plan` via new `PlanOptions` struct (backward-compatible)
- **Merge gate:** `TestPlanHandler_RespectsExcludeConflicts` passes.

**PR D2:** `fix(serve): plan handler implements dry_run`
- File: `internal/cmd/serve.go` line 40 (AGENTS.md), handler code
- Change: When `dry_run=true`, run plan but don't write to cache/store, return plan without side effects
- **Merge gate:** `TestPlanHandler_RespectsDryRun` passes.

**PR D3:** `fix(serve): authMiddleware sets Content-Type on 401`
- File: `internal/cmd/serve.go` line 1063-1071
- Change: Use `writeHTTPError` helper instead of direct `json.NewEncoder`
- **Merge gate:** `TestAuthMiddleware_ContentTypeOn401` passes.

**PR D4:** `fix(serve): rate limit header shows "unlimited" for Inf`
- File: `internal/cmd/serve.go` line 1032-1045
- Change: Check `math.IsInf(critical, 1)` before `int()` conversion
- **Merge gate:** `TestRateLimitHeader_Unlimited` passes.

**PR D5:** `fix(github): ResolveTokenForLogin uses login parameter`
- File: `internal/github/auth.go` line 211-214
- Change: Pass `--user` flag to `gh auth token` or iterate accounts
- **Merge gate:** `TestResolveTokenForLogin_UsesLoginParam` passes.

---

### Phase E: Scoring Logic (Week 3, Days 1-3)

**Goal:** Fix the stub functions and incorrect algorithms.

**PR E1:** `fix(pool): scoreClusterCoherence uses context-aware implementation`
- File: `internal/planning/pool.go` line 304-311
- Change: Call `scoreClusterCoherenceWithContext` instead of stub, or remove stub and rename
- **Merge gate:** `TestScoreClusterCoherence_NotPlaceholder` passes.

**PR E2:** `fix(engine): empty TierResult not appended to result.Tiers`
- File: `internal/formula/engine.go` line 79, 86
- Change: Skip append when `tierResult.Best.Selected == nil`
- **Merge gate:** `TestTierResult_EmptyNotAppended` passes.

**PR E3:** `fix(planner): orderSelection validates all nodes exist in selected map`
- File: `internal/planner/planner.go` line 293-295
- Change: Return error if any `orderedNodes` PRNumber not in `byNumber` map
- **Merge gate:** `TestOrderSelection_UnknownNodeError` passes.

**PR E4:** `fix(scoring): complete ReviewStatus enum coverage`
- File: `internal/formula/scoring.go` line 86-100
- Change: Add explicit cases for `"pending"`, `"in_progress"`, `"dismissed"` with documented scores
- **Merge gate:** `TestAverageReviewStatusScore_CompleteCoverage` passes.

**PR E5:** `fix(engine): Search loop bound respects pool size`
- File: `internal/formula/engine.go` line 97-121
- Change: `min(tierResult.CandidateCount, len(pool))` as loop bound
- **Merge gate:** `TestEngineSearch_LoopBoundRespectsPoolSize` passes.

**PR E6:** `fix(tiers): conflictCounts detects cross-branch conflicts`
- File: `internal/formula/tiers.go` line 98
- Change: Remove `BaseBranch` check, or add `CrossBranchConflictWeight` config
- **Merge gate:** `TestConflictCounts_CrossBranch` passes.

---

### Phase F: Telemetry & Observability (Week 3, Days 4-5)

**PR F1:** `fix(service): emit argument uses processed count not total`
- File: `internal/app/service.go` line 1689
- Change: `s.emit("enrich_files_done", i+1, total)` (already correct in loop, fix final emit)
- **Merge gate:** `TestEmit_ProgressAccuracy` passes.

**PR F2:** `fix(github): transient backoff within documented 30s cap`
- File: `internal/github/client.go` line 564-576
- Change: Clamp total (base + jitter) to 30s, not just base
- **Merge gate:** `TestTransientBackoff_WithinDocumentedCap` passes.

**PR F3:** `fix(serve): settings handlers validate scope`
- File: `internal/cmd/serve.go` line 249, 265-278
- Change: Reject empty/invalid scope with 400 before calling store
- **Merge gate:** Settings API tests pass.

---

### Phase G: Polish & Merge (Week 4, Day 1)

**PR G1:** `chore: extract constants and naming cleanup`
- Files: `internal/app/service.go`, `internal/cmd/serve.go`
- No behavior changes. Extract magic numbers to `const`. Rename `scoreClusterCoherenceWithContext` to `scoreClusterCoherence` (remove stub).
- **Merge gate:** Full suite green. Benchmark delta < 5%.

**PR G2:** `docs: update AGENTS.md API contract`
- File: `AGENTS.md`
- Document actual behavior of settings API, plan API params, rate limit headers.
- **Merge gate:** No code changes. Docs review only.

---

## 4. Branch Strategy

```
main (protected)
  |
  +-- fix/v1.6.1-p0-budget-boundary      (PR B1)
  +-- fix/v1.6.1-p0-token-fallback       (PR B2)
  +-- fix/v1.6.1-p0-legacy-paused        (PR B3)
  ... etc
```

**Rule:** Each PR branches from `main`, not from another feature branch. No stacked PRs. This prevents cascade failures.

---

## 5. Verification Checklist Per PR

- [ ] Contract test fails before fix, passes after
- [ ] `go test ./...` green
- [ ] `go test ./... -race` green (if touching concurrency)
- [ ] `go build ./cmd/pratc` succeeds
- [ ] Python tests green (if touching models)
- [ ] No new lint warnings (`golangci-lint run`)
- [ ] PR description includes: "Prompt to Recreate" section
- [ ] Reviewed by at least one human or subagent

---

## 6. Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Fix introduces regression | Every fix has a contract test. No test = no merge. |
| Phase N+1 blocked by Phase N delay | Each phase is independent. Delays don't cascade. |
| SQLite transaction changes deadlock | PR B5/B6 include timeout and rollback tests. |
| HTTP API param changes break web client | D1/D2 add optional params; existing calls unchanged. |
| Scoring changes alter plan output | E1-E6 include golden-file tests for plan output stability. |

---

## 7. Timeline

| Week | Days | Phase | Deliverable |
|------|------|-------|-------------|
| 1 | 1-2 | A | 14 contract tests, all failing |
| 1 | 3-5 | B | 6 P0 data integrity fixes |
| 2 | 1-2 | C | 3 error propagation fixes |
| 2 | 3-5 | D | 5 API contract fixes |
| 3 | 1-3 | E | 6 scoring logic fixes |
| 3 | 4-5 | F | 3 telemetry fixes |
| 4 | 1 | G | Constants extraction, docs update |
| 4 | 2 | — | Buffer for unexpected issues |

**Total: 4 weeks, 23 PRs, 0 batch changes.**
