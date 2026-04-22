# prATC Review Engine Analysis Report

**Commit Analyzed:** 74cdc35  
**Files Reviewed:** `quickwin.go`, `recovery.go`, `analyzer.go`, and their test files  
**Date:** 2026-04-22

---

## Summary

The review engine consists of three main components:
- `analyzer.go` — Defines the `Analyzer` interface and `PRData` structure (141 lines)
- `quickwin.go` — Second-pass reclassification for low-value PRs (247 lines)  
- `recovery.go` — Second-pass recovery rules for blocked PRs (264 lines)

All tests pass (0.010s for the review package). The previous multi-model synthesis (v1.6.1) identified 7 consensus issues; commit 74cdc35 addressed 6 of them. This analysis finds **1 remaining major issue** and **4 minor issues**.

---

## Severity Ratings & Findings

### MAJOR

#### M1: QuickWin Rule 3 Lacks TemporalBucket Guard — Incorrect Reclassification Risk

**File:** `internal/review/quickwin.go:60-66`  
**Severity:** MAJOR  

```go
// Rule 3: Has security/reliability/performance findings
if !reclassified && hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
```

**Problem:** Rule 3 checks `!reclassified` and `SubstanceScore > 40` but does **NOT** verify `TemporalBucket == "future"`. Rules 1 and 2 both implicitly depend on `TemporalBucket == "future"` via `isLowValueCandidate`, but Rule 3 does not.

**Scenario:** If a PR somehow enters `RunQuickWinPass` with:
- `TemporalBucket == "now"` (already promoted by SecondPass or a previous QuickWin pass)
- `SubstanceScore > 40`
- `hasQualityFindings == true`
- `!reclassified == true` (no prior rule matched)

Rule 3 would incorrectly reclassify this PR, changing `TemporalBucket` back to `"future"` and resetting `Category` to `merge_after_focused_review`. This could undo a legitimate "now" promotion.

**Current Mitigant:** The `!reclassified` check and Rules 1/2 matching first provide protection. A PR with `TemporalBucket == "now"` would need to not match Rules 1 or 2 AND have quality findings AND have `SubstanceScore > 40`. However, if such a PR exists (e.g., a large PR with security findings but CI still passing), Rule 3 would incorrectly reclassify it.

**Fix:**
```go
if !reclassified && result.TemporalBucket == "future" && 
   hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
```

---

### MINOR

#### m1: Catchall Rule Doesn't Set BatchTag — Inconsistent Behavior

**File:** `internal/review/quickwin.go:82-88`  
**Severity:** MINOR  

```go
// Catchall: PR matched no rule but is a genuine low_value candidate
if !reclassified {
    reason = "genuine low_value"
    result.ReclassificationReason = reason
    result.ReclassifiedFrom = "low_value"
    reclassifiedCount++
    continue
}
```

**Problem:** When the catchall matches, `BatchTag` is not set (empty string). Rules 1-3 all set appropriate batch tags (`deriveBatchTag` for Rules 1-2, `"docs-batch"` for Rule 2). This creates inconsistent behavior where genuinely low-value PRs don't receive batch tags but successfully reclassified PRs do.

**Fix:** Either set a consistent batch tag (e.g., `"genuine-low-value"`) or explicitly leave it empty to indicate no batch assignment.

---

#### m2: Test Naming Inconsistency — Misleading Test Names

**File:** `internal/review/quickwin_test.go`  
**Severity:** MINOR  

**Problem:** Many test names say "gets reclassified to merge_candidate" or "reclassified to needs_review" but the assertions verify changes to `TemporalBucket`, `ReclassifiedFrom`, `ReclassificationReason`, and `DecisionLayers` — not the category itself.

Example (line 23):
```go
name: "small PR with passing CI gets reclassified to merge_candidate",
```
The test verifies `Category` remains `merge_after_focused_review` (unchanged), while `TemporalBucket` changes to `"now"`.

**Impact:** Test names are misleading and make the test suite harder to understand. The logic is correct; only the naming is wrong.

---

#### m3: Test Count Assertion Silently Ignores Mismatch When checkFn Exists

**File:** `internal/review/quickwin_test.go:710-712`  
**Severity:** MINOR  

```go
if got != tt.wantReClass && tt.checkFn != nil {
    // checkFn will validate; we just log the count difference for info
}
```

**Problem:** When `got != tt.wantReClass` but `checkFn != nil`, the count mismatch is silently ignored. This means a test could pass even though the reclassification count is wrong, as long as `checkFn` doesn't fail.

**Impact:** Could mask bugs where the reclassification count is incorrect but the assertions inside `checkFn` don't catch it.

---

#### m4: Missing Integration Tests for Cross-Pass Interaction

**File:** N/A (test gap)  
**Severity:** MINOR  

**Problem:** There are no tests that verify behavior when `RunSecondPass` and `RunQuickWinPass` are called in sequence on the same PR set. Each function is tested in isolation, but the interaction between them (especially the C2 fix for blocking `"blocked"` reclassified PRs) is not tested in an integration context.

**Example missing test scenario:**
1. Call `RunSecondPass` on blocked PRs
2. Verify some get reclassified to `merge_after_focused_review` with `TemporalBucket = "future"`
3. Call `RunQuickWinPass` on the result
4. Verify those PRs are NOT reclassified again (due to `ReclassifiedFrom == "blocked"` check)

---

## Verified as Correct (Not Bugs)

### v1: gate17 Struct Aliasing — O1 in synthesis (Verified NOT a bug)

**File:** `internal/review/quickwin.go:101-111`

The comment notes gate17 is allocated once outside the loop and appended each iteration. In Go, `append` copies struct values into the slice, so each `DecisionLayers` entry is independent. **Not a bug.**

### v2: Integer Division for Day Boundaries — Correct

**Files:** `quickwin.go:225`, `recovery.go:231`

Both `isAbandoned` and `isRecentPush` use `int(time.Since(t).Hours()/24)` for boundary calculations. This correctly:
- Treats 91+ days as abandoned (> 90)
- Treats 90+ days as not recent (>= withinDays)

### v3: Rule 4 TemporalBucket = "junk" — Fixed in commit

**File:** `quickwin.go:75`

The M6 issue (Rule 4 using `"blocked"` instead of `"junk"`) was fixed in commit 74cdc35. Line 75 now correctly sets `result.TemporalBucket = "junk"`.

### v4: C2 Double-Promotion Fix — Working as intended

**File:** `quickwin.go:139-141`

```go
if result.ReclassifiedFrom == "blocked" {
    return false
}
```

This correctly prevents PRs reclassified by `RunSecondPass` from entering `RunQuickWinPass`. The `ReclassifiedFrom` is set to the original category (e.g., `"unknown_escalate"`) by SecondPass, so the `"blocked"` check works correctly.

---

## Test Coverage Gaps

| Gap | Severity | Description |
|-----|----------|-------------|
| Cross-pass integration | MINOR | No test for SecondPass → QuickWin sequence |
| Exactly 30/90 day boundaries | MINOR | Boundary tests exist (`TestIsRecentPushBoundary`, `TestIsAbandonedBoundary`) but could be more comprehensive |
| Multi-rule matching | MINOR | No test verifies first-match-wins when PR matches multiple rules (e.g., Rule 1 AND Rule 2) |
| Rule 3 with TemporalBucket="now" | MAJOR | Would catch the M1 bug |

---

## Concurrency Safety

**Assessment:** SAFE (for single-pass use)

Both `RunQuickWinPass` and `RunSecondPass` are pure functions that:
1. Iterate over results with index access (`for i := range results { result := &results[i] }`)
2. Mutate results in-place via pointer
3. Do not spawn goroutines

They are **not** concurrency-safe if called concurrently on the same `results` slice. However, the review pipeline is single-threaded, so this is not currently a problem.

If these functions are to be used in a concurrent context, the caller must ensure proper synchronization (e.g., process each PR independently with separate result slices).

---

## Conclusion

| Category | Count |
|----------|-------|
| Critical | 0 |
| Major | 1 (M1: Rule 3 temporal bucket guard missing) |
| Minor | 4 (m1-m4: batch tag, test naming, test assertion, integration tests) |
| Verified Correct | 4 (v1-v4) |

The most urgent fix is **M1** — adding a `TemporalBucket == "future"` check to Rule 3 to ensure consistent behavior across all QuickWin rules.

---

*Report generated by Hermes Agent analysis of prATC v1.6.1 review engine.*