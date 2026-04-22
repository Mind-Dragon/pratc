# prATC v1.6.1 Multi-Model Audit Synthesis Report

**Repository:** `/home/agent/pratc` (branch: `main`)
**Generated:** 2026-04-22
**Auditors:** MiniMax 2.7, Kimi 2.6, GPT 5.4 (2 audit waves)
**Scope:** Backend core, HTTP/CLI, and infrastructure layers

---

## Executive Summary

This report synthesizes findings from two audit waves covering all major subsystems. After deduplication, we identified **22 critical issues**, **19 major issues**, and **7 minor issues** requiring remediation before the next release.

**Highest-risk areas:**
1. Budget/billing logic (inverted boundary conditions, unvalidated negative values)
2. Concurrency bugs (non-atomic DB operations, mutex hold during I/O)
3. Silent error suppression (review failures, API contract violations)
4. HTTP API contract drift (ignored parameters, incorrect response types)

---

## 1. CRITICAL Issues (22 issues — Fix Immediately)

### 1.1 Backend Core (`internal/app/service.go`, `internal/planning/`)

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| C1 | `service.go` | 1689 | Wrong emit argument: `(total,total)` instead of `(i+1,total)` | Progress events report incorrect position |
| C2 | `service.go` | 623–636 | Review failure silently swallowed; API contract violation | Invalid review states accepted without error |
| C3 | `service.go` | 258–269 | `resolveDynamicTargetConfig` mutates caller's argument | Shared mutable state causes hidden cross-request coupling |
| C4 | `service.go` | 278–280 | `DynamicTarget` mishandles negative `viablePool` (ignores fallback) | Crash-prone under certain pool drain scenarios |
| C5 | `service.go` | 809–812 | Per-PR review errors silently dropped (no logging) | Errors vanish without trace; debugging impossible |
| C6 | `service.go` | 808–812 | Context may not be honored by review pipeline | Cancellation signals ignored; resource leaks |
| C7 | `pool.go` | 304–311 | `scoreClusterCoherence` stub always returns 0.5 (wrong function called) | Clustering quality metric is non-functional |
| C8 | `engine.go` | 79, 86 | Empty `TierResult` with nil `Best.Selected` appended to `result.Tiers` | Nil pointer risk; malformed tier results |
| C9 | `planner.go` | 293–295 | `orderSelection` silently injects zero-value PRs for unknown nodes | Ghost PRs appear in plans; violates API contract |
| C10 | `scoring.go` | 86–100 | Incomplete `ReviewStatus` enum coverage | Unknown review states cause panics |

### 1.2 HTTP/CLI Layer (`internal/cmd/`, `internal/github/`)

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| C11 | `serve.go` | 778 | `handleSyncEvents` missing nil check on `syncAPI.Stream` | Nil pointer dereference under reconnect scenarios |
| C12 | `serve.go` | 541–562 | HTTP Plan API ignores documented parameters: `exclude_conflicts`, `stale_score_threshold`, `candidate_pool_cap`, `score_min` | API contract drift; clients receive unexpected results |
| C13 | `serve.go` | 40 (AGENTS.md) | `dry_run` parameter documented but not implemented in HTTP handler | Documentation lie; dry-run not functional via API |
| C14 | `serve.go` | 1063–1071 | `authMiddleware` missing `Content-Type` on 401 error responses | API consumers cannot parse error bodies |
| C15 | `serve.go` | 186–189 | `sanitizedError` substring match could leak internal details | Information disclosure; security risk |
| C16 | `serve.go` | 249 | `handleSettings` doesn't validate `scope` | Invalid scope values accepted; data corruption risk |
| C17 | `analyze.go` | 540 | I/O while holding `analyzeSyncMu` lock | Lock contention; potential deadlock under load |
| C18 | `serve.go` | 1177 | Middleware ordering: rate limit responses lack CORS headers | Browser clients receive opaque errors |

### 1.3 Infrastructure (`internal/billing/`, `internal/auth/`, `internal/cache/`)

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| C19 | `budget.go` | 206 | `CanAfford` boundary condition is inverted | Users charged when they shouldn't be; credit exhaustion |
| C20 | `auth.go` | 488–500 | `AttemptWithTokenFallback` continues after success (no `break`) | Unnecessary API calls; token rotation races |
| C21 | `sqlite.go` | 856–857 | Legacy paused state completely orphaned in scheduler | Paused jobs never resume; zombie job accumulation |
| C22 | `budget.go` | 212–218 | `RecordResponse` accepts negative `remaining` without validation | Negative balance allowed; billing arithmetic breaks |
| C23 | `sqlite.go` | 474–518 | `UpdateSyncJobProgress` is not atomic | Concurrent updates lose progress; sync state corruption |
| C24 | `sqlite.go` | 814–846 | `PauseSyncJob` is not atomic | Pause race conditions; job state inconsistency |

---

## 2. MAJOR Issues (19 issues — Fix Before Release)

### 2.1 HTTP API Contract Violations

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| M1 | `serve.go` | 541–569 | `cluster_id` query parameter silently ignored in plan handler | Filter-by-cluster doesn't work via API |
| M2 | `serve.go` | 310 | `handleImportSettings` `MaxBytesReader` first two arguments identical (type mismatch) | Import settings may truncate or reject valid payloads |
| M3 | `serve.go` | 251–262, 273–276, 316–318 | Invalid `scope` values return 500 instead of 400 | Client cannot distinguish bad input from server errors |
| M4 | `serve.go` | 230–238 | Settings GET handler ignores `scope` query parameter | Scope filtering doesn't work; all settings returned |
| M5 | `serve.go` | 239–264 | Empty `scope` accepted silently in POST handler | Ambiguous default behavior; potential data loss |
| M6 | `serve.go` | 265–278 | DELETE handler ignores `scope`, deletes across all scopes | Catastrophic data loss if client forgets scope |
| M7 | `serve.go` | 1104 vs `analyze.go:286` | HTTP handler doesn't set `IncludeReview`, CLI always does | API and CLI produce different results for same query |
| M8 | `analyze.go` | 213–224, 241–249 | `force` flag in analyze CLI creates race with active sync | Concurrent force-refresh corrupts cache state |

### 2.2 Reliability & Error Handling

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| M9 | `client.go` | 564–576 | `transientBackoff + addJitter` can exceed stated cap of 30 seconds | Client-side rate limit hammering; IP ban risk |
| M10 | `auth.go` | 462–476 | `IsRetryableError` string-matching is fragile and inconsistent | False negatives skip retry; false positives cause unnecessary load |
| M11 | `client.go` | 188 | REST fallback in `FetchPullRequests` bypasses token rotation | Stale tokens used; 401 storms on token expiry |
| M12 | `auth.go` | 211–214 | `ResolveTokenForLogin` ignores the `login` parameter | Token lookup ignores context; wrong token selected |
| M13 | `sqlite.go` | 785–806 | `resumeSyncJob` double-checks `RowsAffected` on wrong result set | Resume appears to succeed but job remains stuck |
| M14 | `store.go` | 199–211 | `Settings.ImportYAML` silently skips individual key failures | Partial import succeeds with no warning; configuration gaps |

### 2.3 Scoring & Planning Logic

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| M15 | `pool.go` | 233, 304–311 | `scoreClusterCoherence` is a stub/placeholder | Cluster quality not actually scored |
| M16 | `engine.go` | 97–121 | `Engine.Search` loop bound could exceed pool size with custom config | Out-of-bounds array access; index panic |
| M17 | `tiers.go` | 98 | `conflictCounts` skips cross-branch conflict detection | Conflicts across branches silently ignored |
| M18 | `planner.go` | 74 | `ValidatePlanInput(nil)` needs nil guard verification | Nil pointer dereference if called with nil |
| M19 | `planner.go` | 126 | Shared mutable state without locks (`formulaConfig.Mode`) | Concurrent plan requests corrupt shared state |

### 2.4 Observability

| # | File | Line(s) | Issue | Impact |
|---|------|---------|-------|--------|
| M20 | `serve.go` | 1032–1045 | Rate limit header shows wrong value for `rate.Inf` | Clients see nonsensical rate values; incorrect backoff |

---

## 3. MINOR Issues (7 issues — Fix When Touching Relevant Code)

### 3.1 Budget Boundary Inconsistency

| # | File | Line(s) | Issue | Theme |
|---|------|---------|-------|-------|
| m1 | `budget.go` | 111, 125, 144–147 | `ShouldPause/WouldPause/Reserve` boundary inconsistency | Budget logic |
| m2 | `budget.go` | 144–147 | `Reserve` allows exact-buffer reservation | Budget logic |

### 3.2 Database Anomalies

| # | File | Line(s) | Issue | Theme |
|---|------|---------|-------|-------|
| m3 | `sqlite.go` | 855 | `ListPausedSyncJobs` JOINs on `p.repo = j.repo` instead of `p.job_id = j.id` | DB queries |
| m4 | `sqlite.go` | 367–377 | `UpsertPRFiles` issues N inserts instead of batch | DB performance |

### 3.3 Concurrency Safety

| # | File | Line(s) | Issue | Theme |
|---|------|---------|-------|-------|
| m5 | `client.go` | 565 | `addJitter` uses package-global `rand` (not concurrent-safe) | Concurrency |

### 3.4 Legacy State Handling

| # | File | Line(s) | Issue | Theme |
|---|------|---------|-------|-------|
| m6 | `models.go` | 71 | `IsPaused()` includes legacy paused but all DB queries use `paused_rate_limit` | Legacy state |

### 3.5 Reporting

| # | File | Line(s) | Issue | Theme |
|---|------|---------|-------|-------|
| m7 | `report/analyst_sections.go` | 63–71 | `LoadAnalystDataset` silently uses `time.Now()` on missing artifact | Reporting |

---

## 4. Fix Priority Order

### P0 — Emergency (Fix in Current Sprint)

These bugs cause data corruption, security breaches, or system crashes:

1. **C19** (`budget.go:206`) — Inverted `CanAfford` boundary
2. **C22** (`budget.go:212-218`) — `RecordResponse` accepts negative remaining
3. **C21** (`sqlite.go:856-857`) — Legacy paused state orphaned
4. **C23** (`sqlite.go:474-518`) — `UpdateSyncJobProgress` not atomic
5. **C24** (`sqlite.go:814-846`) — `PauseSyncJob` not atomic
6. **C20** (`auth.go:488-500`) — Token fallback continues after success
7. **C14** (`serve.go:1063-1071`) — Missing `Content-Type` on 401
8. **C15** (`serve.go:186-189`) — Error message substring leak

### P1 — High (Fix Before Release)

These bugs cause incorrect behavior, API contract violations, or reliability issues:

1. **C1** (`service.go:1689`) — Wrong emit argument
2. **C2** (`service.go:623-636`) — Review failure silently swallowed
3. **C3** (`service.go:258-269`) — Mutates caller's argument
4. **C5** (`service.go:809-812`) — Review errors silently dropped
5. **C6** (`service.go:808-812`) — Context not honored
6. **C7** (`pool.go:304-311`) — `scoreClusterCoherence` stub
7. **C8** (`engine.go:79,86`) — Empty `TierResult` with nil `Best.Selected`
8. **C9** (`planner.go:293-295`) — Zero-value PR injection
9. **C11** (`serve.go:778`) — Nil check missing on `syncAPI.Stream`
10. **C12** (`serve.go:541-562`) — HTTP Plan API ignores parameters
11. **C13** (`serve.go:40`) — `dry_run` not implemented
12. **C16** (`serve.go:249`) — Scope not validated
13. **C17** (`analyze.go:540`) — I/O while holding lock
14. **C18** (`serve.go:1177`) — Rate limit responses lack CORS
15. **M1-M8** — All HTTP API contract issues
16. **M9** (`client.go:564-576`) — Backoff exceeds cap
17. **M15** (`pool.go:233,304-311`) — Stub scoring function
18. **M16** (`engine.go:97-121`) — Out-of-bounds loop

### P2 — Medium (Next Release)

These bugs affect correctness under edge cases or cause maintenance burden:

1. **C4** (`service.go:278-280`) — Negative viablePool mishandled
2. **C10** (`scoring.go:86-100`) — Incomplete ReviewStatus enum
3. **M10-M13** — Error handling and token issues
4. **M14** (`store.go:199-211`) — Silent YAML import failures
5. **M17** (`tiers.go:98`) — Cross-branch conflicts skipped
6. **M18-M19** — Nil guards and shared mutable state
7. **M20** (`serve.go:1032-1045`) — Wrong rate limit header

### P3 — Low (Backlog)

These are code smells, dead code, or minor inefficiencies:

1. **m1-m2** — Budget boundary inconsistencies
2. **m3-m4** — DB query anomalies
3. **m5** — Non-thread-safe random
4. **m6** — Legacy paused state confusion
5. **m7** — Silent time.Now() fallback

---

## 5. Estimated Effort Per Fix

| Priority | Issue Type | Estimated Fix Time | Notes |
|----------|------------|---------------------|-------|
| P0 | Atomic transaction fix | 2–4 hours | Requires DB transaction wrapping |
| P0 | Boundary condition | 15–30 min | Simple comparison flip |
| P0 | Missing nil check | 15 min | Single guard clause |
| P1 | Silent error suppression | 1–2 hours | Requires error propagation audit |
| P1 | API parameter handling | 2–4 hours | Handler parameter wiring |
| P1 | Non-atomic operations | 3–5 hours | Transaction + locking design |
| P2 | Context propagation | 2–3 hours | Call chain audit |
| P2 | Enum coverage | 1–2 hours | Add missing cases |
| P3 | Dead code removal | 1 hour | Straight deletion |

**Total estimated effort for P0+P1: 15–25 hours**
**Total estimated effort for all issues: 30–45 hours**

---

## 6. Deduplication Notes

The following items appeared in multiple audit waves but are actually distinct issues:

- `serve.go:541-562` (Wave 1) and `serve.go:513-539` (Wave 2) — Overlapping but Wave 2 has additional `cluster_id` detail
- `pool.go:304-311` (Wave 1) and `pool.go:233,304-311` (Wave 2) — Same issue with added context
- `analyze.go:540` appeared in both waves with identical description — merged
- `serve.go:1063-1071` appeared in both waves with identical description — merged
- `client.go:564-576` appeared in both waves with identical description — merged

Items marked "correct" in source findings (e.g., `service.go:1726-1727` non-idiomatic copy pattern) were excluded as they represent intentional patterns rather than bugs.

---

*Report generated by Hermes Agent. Verify all line numbers against current `main` branch before commencing fixes.*
