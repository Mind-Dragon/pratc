# prATC v1.6.1 Multi-Model Multi-Section Synthesis Report

**Generated:** 2026-04-22  
**Commit:** e4e2762  
**Scope:** Full codebase (5 architectural sections)  
**Models Analyzed:** MiniMax 2.7, Kimi 2.6, Z.AI 5.1 (multiple runs per section)  
**Synthesized Sources:** 
- 2 dedicated Review Engine analysis reports
- 2 prior synthesis documents covering all 5 sections
- Direct codebase analysis

---

## Executive Summary

This report synthesizes multi-model code review findings across **5 architectural sections** of prATC v1.6.1:

| Section | Files | Status |
|---------|-------|--------|
| 1. Review Engine | `internal/review/*.go` | Fully analyzed (9 model runs) |
| 2. App Service Layer | `internal/app/service.go` | Prior synthesis available |
| 3. Report Generation | `internal/report/*.go` | Prior synthesis available |
| 4. CLI and API Layer | `internal/cmd/`, `web/src/` | Prior synthesis available |
| 5. Python ML Bridge | `ml-service/src/pratc_ml/` | Prior synthesis available |

**Critical Findings:** 4 HIGH severity issues requiring immediate action  
**Major Findings:** 8 MEDIUM severity issues  
**Minor/Low:** 12 issues

---

## SECTION 1: Review Engine (internal/review/)

### CRITICAL Issues

#### C1: `hasQualityFindings` Uses Prefix Matching That Never Matches Real Analyzer Output

**File:** `internal/review/quickwin.go:200-209`  
**Confidence:** 9/9 models (consensus)  
**Severity:** CRITICAL

```go
func hasQualityFindings(findings []types.AnalyzerFinding) bool {
    for _, f := range findings {
        finding := strings.ToLower(f.Finding)
        if strings.HasPrefix(finding, "security_") ||
            strings.HasPrefix(finding, "reliability_") ||
            strings.HasPrefix(finding, "performance_") {
            return true
        }
    }
    return false
}
```

**Problem:** This function checks if a finding starts with `security_`, `reliability_`, or `performance_`. However, the actual analyzers produce findings with **entirely different formats**:

| Analyzer | Real Finding Format | Matches Prefix? |
|----------|---------------------|-----------------|
| SecurityAnalyzer | `"risky file path detected: " + file` | NO |
| SecurityAnalyzer | `"auth/permission surface change detected: " + file` | NO |
| SecurityAnalyzer | `"dependency change with security implications: " + file` | NO |
| ReliabilityAnalyzer | `"CI failure detected: build failed"` | NO |
| ReliabilityAnalyzer | `"Merge conflict detected: PR is not mergeable"` | NO |
| PerformanceAnalyzer | `"large diff detected: ..."` | NO |
| PerformanceAnalyzer | `"many files changed: ..."` | NO |
| PerformanceAnalyzer | `"performance-sensitive file change detected: " + file` | NO |

**Impact:** QuickWin **Rule 3** ("Has security/reliability/performance findings") will **never trigger** based on real analyzer output. The rule only works in tests because tests use synthetic findings like `security_sql_injection` which conform to the expected prefix convention.

**Fix:** Update `hasQualityFindings` to check `AnalyzerName` instead of prefix:
```go
func hasQualityFindings(findings []types.AnalyzerFinding) bool {
    for _, f := range findings {
        analyzerName := strings.ToLower(f.AnalyzerName)
        if analyzerName == "security" || analyzerName == "reliability" || analyzerName == "performance" {
            return true
        }
    }
    return false
}
```

---

#### C2: Category Taxonomy Mismatch — Spec vs Implementation

**Severity:** HIGH  
**Confidence:** 9/9 consensus  
**Files:** `internal/review/recovery.go`, `internal/review/quickwin.go`, `internal/types/models.go`

TODO.md uses category names (`needs_review`, `high_value`, `merge_candidate`, `junk`, `low_value`) that do not exist in `ReviewCategory`. The code uses `merge_after_focused_review` for all promoted PRs and `problematic_quarantine` for demoted PRs. This means:
- Recovered blocked PRs and promoted low-value PRs get the *same* category
- The spec's intent to distinguish "needs review" from "high value" is lost

**Fix:** Either add `needs_review`, `high_value`, `merge_candidate`, `low_value` to `ReviewCategory` enum, or update TODO.md to use actual type constants.

---

#### C3: `isLowValueCandidate` Does Not Verify Original Category

**Severity:** HIGH  
**Confidence:** 9/9 consensus  
**File:** `internal/review/quickwin.go:103-123`

`isLowValueCandidate` only checks `TemporalBucket != "future"`, `SubstanceScore < 50`, and `Category != duplicate_superseded/problematic_quarantine`. It does NOT verify the PR was actually `low_value` before the pass. This means:
- PRs reclassified from `blocked` → `merge_after_focused_review` (future) by `RunSecondPass` can then enter `RunQuickWinPass`
- A PR can be double-promoted: blocked → future → now

**Fix:** Add `result.Category == types.ReviewCategoryUnknownEscalate || result.TemporalBucket == "blocked"` to `isLowValueCandidate`, or gate `RunQuickWinPass` to only process PRs whose original category was `low_value`.

---

### MAJOR Issues

#### M1: QuickWin Rule 3 Lacks `TemporalBucket` Guard

**File:** `internal/review/quickwin.go:60-66`  
**Severity:** MAJOR  
**Confidence:** 9/9 consensus

```go
// Rule 3: Has security/reliability/performance findings
if !reclassified && hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
```

**Problem:** Rule 3 checks `!reclassified` and `SubstanceScore > 40` but does **NOT** verify `TemporalBucket == "future"`. Rules 1 and 2 both implicitly depend on `TemporalBucket == "future"` via `isLowValueCandidate`, but Rule 3 does not.

**Fix:**
```go
if !reclassified && result.TemporalBucket == "future" && 
   hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
```

---

#### M2: Quickwin Rule 4 Uses `temporalBucket = "blocked"` Instead of `"junk"`

**Severity:** MEDIUM  
**Confidence:** 7/9 majority  
**File:** `internal/review/quickwin.go:68-76`

TODO.md says abandoned PRs → `junk`, but the code sets `TemporalBucket = "blocked"` for Rule 4. This is inconsistent with recovery.go Rules 4-6 which use `temporalBucket = "junk"`.

**Status:** Fixed in commit 74cdc35 — line 75 now correctly sets `result.TemporalBucket = "junk"`.

---

#### M3: Missing Catchall Rule in Quickwin Pass

**Severity:** MEDIUM  
**Confidence:** 6/9 majority  
**File:** `internal/review/quickwin.go`

TODO.md says "All others → low_value (genuine low_value)" but the code does nothing for unmatched PRs. There's no way to distinguish "genuine low_value" from "was low_value but didn't match any rule."

**Fix:** Add a catchall that sets `ReclassificationReason = "genuine low_value"` or a `GenuineLowValue` flag for unmatched PRs.

---

### MINOR Issues

#### m1: Catchall Rule Doesn't Set `BatchTag` — Inconsistent Behavior

**File:** `internal/review/quickwin.go:82-88`  
**Severity:** MINOR

When the catchall matches, `BatchTag` is not set (empty string). Rules 1-3 all set appropriate batch tags. This creates inconsistent behavior.

**Fix:** Either set a consistent batch tag (e.g., `"genuine-low-value"`) or explicitly document that catchall cases don't receive batch tags.

---

#### m2: Test Naming Inconsistency — Misleading Test Names

**File:** `internal/review/quickwin_test.go`  
**Severity:** MINOR

Many test names say "gets reclassified to merge_candidate" but the assertions verify changes to `TemporalBucket`, not the category itself.

---

#### m3: Test Count Assertion Silently Ignores Mismatch When `checkFn` Exists

**File:** `internal/review/quickwin_test.go:710-712`  
**Severity:** MINOR

```go
if got != tt.wantReClass && tt.checkFn != nil {
    // checkFn will validate; we just log the count difference for info
}
```

The count mismatch is silently ignored when `checkFn` exists.

---

#### m4: Missing Integration Tests for Cross-Pass Interaction

**File:** N/A (test gap)  
**Severity:** MINOR

No tests verify behavior when `RunSecondPass` and `RunQuickWinPass` are called in sequence on the same PR set.

---

### Verified as Correct (Not Bugs)

| ID | Description | Verdict |
|----|-------------|---------|
| v1 | gate17 Struct Aliasing | NOT a bug — Go append copies struct values |
| v2 | Integer Division for Day Boundaries | Correct — 91+ days = abandoned |
| v3 | Rule 4 TemporalBucket = "junk" | FIXED in commit 74cdc35 |
| v4 | C2 Double-Promotion Fix | Working as intended |

---

## SECTION 2: App Service Layer (internal/app/)

### CONSENSUS Findings (All Models Agree)

#### C4: `ComputeDynamicTarget` Clamping Logic Is Correct

**Severity:** NONE (confirmed correct)  
**File:** `internal/app/service.go:277-292`  
**Confidence:** 9/9

All models verified the clamping formula `max(MinTarget, min(MaxTarget, Ratio*pool))` is correctly implemented. Test coverage in `dynamic_target_test.go` is comprehensive (233 lines, 12 test cases).

---

#### C5: `Plan()` Nil-Pointer Safety Is Correct

**Severity:** NONE (confirmed correct)  
**File:** `internal/app/service.go:1058-1154`  
**Confidence:** 9/9

- `collapsedCorpus` is nil-checked before map access at L1137
- `collapsedCorpus != nil` guard at L1152 before pool-size cap
- No race conditions (single-threaded execution)
- No nil-pointer panic risk

---

#### C6: `classifyDuplicates` Indentation Is Now Correct

**Severity:** NONE (confirmed correct)  
**File:** `internal/app/service.go:1814-1891`  
**Confidence:** 9/9

The `openOpenCandidates` post-processing block is correctly outside the `for _, pair := range candidatePairs` loop.

---

#### C7: `buildCollapsedCorpus` DFS and Canonical Selection Are Correct

**Severity:** NONE (confirmed correct)  
**File:** `internal/app/service.go:2642-2766`  
**Confidence:** 9/9

- DFS connected-component logic is standard and correct
- Only components with >1 PR are collapsed (L2708)
- Canonical selection uses synthesis score + PR number tiebreaker
- No PR can be both canonical and superseded

---

### MAJORITY Findings (6-8 of 9 Models Agree)

#### M4: `resolveDynamicTargetConfig` Does Not Default `Enabled=true`

**Severity:** WARNING  
**File:** `internal/app/service.go:258-272`  
**Confidence:** 8/9

The docstring says "enabled by default in v1.6.1" but the code does not set `Enabled = true`. Zero-valued `DynamicTargetConfig{}` has `Enabled=false`. CLI usage is fine (plan.go:82 sets `Enabled: true`), but programmatic callers and the API server path use zero-value (disabled).

**Fix:** Add `cfg.Enabled = true` in `resolveDynamicTargetConfig` when the struct is zero-valued, OR update the docstring.

---

#### M5: `Plan()` Path Passes `nil` Review/Conflict Data to `buildDuplicateSynthesis`

**Severity:** WARNING  
**File:** `internal/app/service.go:1091`  
**Confidence:** 7/9

In the `Plan()` path, `buildDuplicateSynthesis` is called with `nil` for `reviewPayload` and `conflicts`. This degrades canonical nomination to structural signals only (no substance score, confidence, or analyzer findings). The `Analyze()` path passes actual data.

**Fix:** Either pass empty (non-nil) structs for consistency, or document that `Plan()` collapse uses structural signals only.

---

#### M6: `bestScore[canonical] == 0` No-Op Assignment

**Severity:** LOW  
**File:** `internal/app/service.go:2684-2686`  
**Confidence:** 1/9 outlier

```go
if bestScore[canonical] == 0 {
    bestScore[canonical] = 0
}
```

This is a no-op. Harmless but indicates incomplete logic.

**Fix:** Remove or replace with meaningful default.

---

## SECTION 3: Report Generation (internal/report/)

### MAJORITY Findings

#### M7: `ReclassificationSection` Renders Awkward "None" Page When Empty

**Severity:** MEDIUM  
**File:** `internal/report/analyst_sections.go:903-923`  
**Confidence:** 7/9

When no reclassifications occurred, the section still creates a new page and prints "Recovered from blocked: 0 | Re-ranked from low value: 0 | Batch-tagged quick wins: 0" followed by three "None" lists.

**Fix:** Add early return when all three slices are empty:
```go
if len(s.FromBlocked) == 0 && len(s.FromLowValue) == 0 && len(s.BatchTagged) == 0 {
    return
}
```

---

#### M8: `CollapseImpactSection` Silently Fails on Empty Data

**Severity:** MEDIUM  
**File:** `internal/report/analyst_sections.go:1183-1220`  
**Confidence:** 6/9

`LoadCollapseImpactSection` returns an error `"no collapsed corpus data"` when `CollapsedGroupCount == 0`. The caller in `report.go` silently skips the section. A legitimate zero-collapse state produces no section in the PDF with no explanation.

**Fix:** Return a section with `CollapsedGroups: 0` and render a "No duplicate collapse impact" message instead of erroring.

---

#### M9: Duplicate Group Chain Flattening Not Fully Implemented

**Severity:** MEDIUM  
**Files:** `internal/report/analyst_sections.go:90-101`, `internal/types/models.go:39-43`  
**Confidence:** 3/3 consensus (from code review)

The backlog surgery plan specifies "recursive duplicate collapse" with chain detection (A→B→C becomes A→[B,C]), but the implementation may not handle chains completely.

**Fix:** Implement chain detection in duplicate synthesis pass. Verify no PR is both canonical and superseded.

---

#### M10: PDF Report Generation Timeout Not Bounded

**Severity:** MEDIUM  
**File:** `internal/report/pdf.go`  
**Confidence:** 3/3 consensus (from code review)

Report generation has no explicit timeout. On very large corpora (6,000+ PRs), PDF generation could take minutes without cancellation capability.

**Fix:** Add context-aware cancellation with timeout (backlog surgery plan specifies 30 seconds max).

---

### Verified as Correct

#### C8: `expansionSummaryLines()` Has No Overflow Risk

**Severity:** NONE (confirmed correct)  
**File:** `internal/report/plan_section.go:250-269`  
**Confidence:** 9/9

Maximum 4 lines, fixed-format strings, no user-controlled input, no truncation risk.

---

## SECTION 4: CLI and API Layer (internal/cmd/, web/src/)

### MAJORITY Findings

#### M11: Settings API Path Mismatch

**Severity:** MEDIUM  
**Confidence:** 3/3 consensus  
**Files:** `web/src/lib/api.ts`, `internal/cmd/serve.go`

- Web client uses RESTful paths: `/api/settings`
- Server uses query params: `/api/settings?repo=`

This mismatch can cause settings to not load properly in the web dashboard.

---

#### M12: CORS Defaults Empty

**Severity:** LOW  
**File:** `internal/cmd/serve.go`  
**Confidence:** 3/3 consensus

No dashboard origin is assumed by default. `PRATC_CORS_ALLOWED_ORIGINS` must be set explicitly to enable CORS.

---

#### M13: Audit DB Per-Call Open/Close

**Severity:** LOW  
**File:** `internal/cmd/audit.go`  
**Confidence:** 3/3 consensus

`logAuditEntry()` opens/closes SQLite each time. Pool if throughput increases.

---

#### M14: Import Limit 1MB

**Severity:** LOW  
**File:** `internal/cmd/serve.go`  
**Confidence:** 3/3 consensus

`http.MaxBytesReader(w, r.Body, 1<<20)` — 1MB max for settings import.

---

### OUTLIER Findings

#### O1: TypeScript `AnalysisResponse` Declared Twice

**Severity:** MEDIUM  
**File:** `web/src/types/api.ts`  
**Confidence:** 1/9 outlier

`AnalysisResponse` is declared twice (lines 198-209 and 325-336). The second definition is incomplete (missing `garbagePRs`, `stalenessSignals`, etc.). TypeScript uses the last declaration.

**Fix:** Remove the duplicate incomplete declaration.

---

## SECTION 5: Python ML Bridge (ml-service/)

### MAJORITY Findings

#### M15: `duplicate_synthesis` Missing from Python `AnalysisResponse`

**Severity:** HIGH  
**File:** `ml-service/src/pratc_ml/models.py`  
**Confidence:** 1/9 outlier (but HIGH severity if it occurs)

`duplicate_synthesis` field exists in Go `AnalysisResponse` but is completely absent from Python's Pydantic and Bootstrap `AnalysisResponse` models. Any Python code receiving this field would drop it.

**Fix:** Add `duplicate_synthesis` to Python models.

---

#### M16: Python Bootstrap `KeyError` on Missing Map Fields

**Severity:** MEDIUM  
**File:** `ml-service/src/pratc_ml/models.py`  
**Confidence:** 1/9 outlier

Python bootstrap `_coerce_dataclass` at line 306 does `value[item.name]` without checking field existence. If Go's `omitempty` omits a map field, bootstrap crashes with `KeyError`. Pydantic path handles this correctly.

**Fix:** Fix `_coerce_dataclass` to use `.get()` with defaults.

---

#### M17: Heuristic Similarity Weights Must Match Go

**Severity:** LOW  
**File:** `ml-service/src/pratc_ml/similarity.py`  
**Confidence:** 3/3 consensus

`heuristic_similarity` weights must match Go `internal/app/service.go` exactly. Thresholds are soft; Go backend may apply additional filtering.

---

## Summary Table

| ID | Finding | Severity | Confidence | Section | Status |
|----|---------|----------|------------|---------|--------|
| C1 | hasQualityFindings prefix mismatch | CRITICAL | 9/9 | Review Engine | Needs Fix |
| C2 | Category taxonomy mismatch | HIGH | 9/9 | Review Engine | Needs Fix |
| C3 | isLowValueCandidate no original cat check | HIGH | 9/9 | Review Engine | Needs Fix |
| C4 | ComputeDynamicTarget correct | NONE | 9/9 | App Service | Verified |
| C5 | Plan() nil-safety correct | NONE | 9/9 | App Service | Verified |
| C6 | classifyDuplicates indent correct | NONE | 9/9 | App Service | Verified |
| C7 | buildCollapsedCorpus correct | NONE | 9/9 | App Service | Verified |
| C8 | expansionSummaryLines safe | NONE | 9/9 | Report | Verified |
| M1 | Rule 3 lacks TemporalBucket guard | MAJOR | 9/9 | Review Engine | Needs Fix |
| M2 | Rule 4 bucket mismatch | MEDIUM | 7/9 | Review Engine | Fixed |
| M3 | Missing quickwin catchall | MEDIUM | 6/9 | Review Engine | Needs Fix |
| M4 | Enabled not defaulted true | WARNING | 8/9 | App Service | Needs Fix |
| M5 | Plan() passes nil review data | WARNING | 7/9 | App Service | Needs Doc |
| M6 | bestScore no-op assignment | LOW | 1/9 | App Service | Code Smell |
| M7 | ReclassificationSection awkward empty | MEDIUM | 7/9 | Report | Needs Fix |
| M8 | CollapseImpactSection silent fail | MEDIUM | 6/9 | Report | Needs Fix |
| M9 | Chain flattening incomplete | MEDIUM | 3/3 | Report | Needs Fix |
| M10 | PDF timeout unbounded | MEDIUM | 3/3 | Report | Needs Fix |
| M11 | Settings API path mismatch | MEDIUM | 3/3 | CLI/API | Needs Fix |
| M12 | CORS defaults empty | LOW | 3/3 | CLI/API | Documented |
| M13 | Audit DB per-call open/close | LOW | 3/3 | CLI/API | Perf Note |
| M14 | Import limit 1MB | LOW | 3/3 | CLI/API | Documented |
| M15 | duplicate_synthesis missing in Python | HIGH | 1/9 | ML Bridge | Needs Fix |
| M16 | Bootstrap KeyError | MEDIUM | 1/9 | ML Bridge | Needs Fix |
| M17 | Heuristic weights must match Go | LOW | 3/3 | ML Bridge | Documented |
| m1 | Catchall doesn't set BatchTag | MINOR | - | Review Engine | Cleanup |
| m2 | Test naming misleading | MINOR | - | Review Engine | Cleanup |
| m3 | Test count assertion silent | MINOR | - | Review Engine | Cleanup |
| m4 | Missing cross-pass integration tests | MINOR | - | Review Engine | Test Gap |
| O1 | TS AnalysisResponse duplicate | MEDIUM | 1/9 | CLI/API | Needs Fix |

---

## Recommended Action Items

### Must Fix Before Release (CRITICAL/HIGH)

1. **C1** — Fix `hasQualityFindings` to check `AnalyzerName` instead of prefix matching
2. **C2** — Align category taxonomy (add constants or update spec)
3. **C3** — Fix cross-pass double-promotion bug in `isLowValueCandidate`
4. **M1** — Add `TemporalBucket == "future"` guard to QuickWin Rule 3
5. **M15** — Add `duplicate_synthesis` to Python ML models

### Should Fix (MEDIUM)

6. **M3** — Add quickwin catchall rule for genuine low_value
7. **M4** — Default `Enabled=true` or fix docstring in `resolveDynamicTargetConfig`
8. **M5** — Document that `Plan()` duplicate collapse uses structural signals only
9. **M7** — Add empty-state guard to ReclassificationSection
10. **M8** — Handle zero-collapse gracefully in CollapseImpactSection
11. **M9** — Implement chain detection for duplicate group flattening
12. **M10** — Add timeout to PDF report generation
13. **M11** — Fix Settings API path mismatch between client and server
14. **M16** — Fix bootstrap `_coerce_dataclass` KeyError

### Nice to Have (LOW/Minor)

15. **M6** — Remove no-op bestScore assignment
16. **m1** — Set consistent BatchTag in catchall rule
17. **m2** — Fix misleading test names
18. **m3** — Remove silent test count assertion ignore
19. **m4** — Add cross-pass integration tests
20. **O1** — Remove duplicate TypeScript AnalysisResponse declaration

---

## Confidence Rating Key

| Rating | Definition |
|--------|------------|
| **Consensus** | All or nearly all models agree (8-9/9) |
| **Strong** | Strong evidence from majority (6-7/9) |
| **Majority** | Most models agree (5-6/9) |
| **Single** | Only one model flagged (1/9) |
| **Disputed** | Models contradict each other |

---

*Report synthesized from multi-model analysis of prATC v1.6.1 codebase.*
