# prATC Review Engine Analysis Report

**Commit Analyzed:** 74cdc35  
**Files Reviewed:** `quickwin.go`, `recovery.go`, `analyzer.go`, and their test files  
**Date:** 2026-04-22  
**Analysis Scope:** Logic correctness, edge cases, concurrency safety, test coverage gaps, bugs

---

## Summary

The review engine consists of three main components:
- `analyzer.go` — Defines the `Analyzer` interface and `PRData` structure (141 lines)
- `quickwin.go` — Second-pass reclassification for low-value PRs (247 lines)
- `recovery.go` — Second-pass recovery rules for blocked PRs (264 lines)

All tests pass. This analysis finds **1 critical issue**, **1 major issue**, and **4 minor issues**.

---

## Severity Ratings & Findings

### CRITICAL

#### C1: `hasQualityFindings` Uses Prefix Matching That Never Matches Real Analyzer Output

**File:** `internal/review/quickwin.go:200-209`  
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

**Rule 3 Code (line 60-66):**
```go
// Rule 3: Has security/reliability/performance findings
if !reclassified && hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
    reason = "hidden value: addresses quality concerns"
    batchTag = ""
    result.Category = types.ReviewCategoryMergeAfterFocusedReview
    result.TemporalBucket = "future"
    reclassified = true
}
```

**Consequence:** PRs with genuine security/reliability/performance findings from the analyzers will NOT be reclassified by Rule 3, even though the rule was designed to capture this "hidden value."

**Fix:** Either:
1. Update `hasQualityFindings` to check if `AnalyzerName` is `"security"`, `"reliability"`, or `"performance"` (since findings don't have consistent prefixes)
2. OR update all analyzers to use consistent prefixes like `security_`, `reliability_`, `performance_`

Recommended fix:
```go
func hasQualityFindings(findings []types.AnalyzerFinding) bool {
    for _, f := range findings {
        // Check by analyzer name since findings don't have consistent prefixes
        analyzerName := strings.ToLower(f.AnalyzerName)
        if analyzerName == "security" || analyzerName == "reliability" || analyzerName == "performance" {
            return true
        }
    }
    return false
}
```

---

### MAJOR

#### M1: QuickWin Rule 3 Lacks `TemporalBucket` Guard — Incorrect Reclassification Risk

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
- `hasQualityFindings == true` (would be true if C1 is fixed)
- `!reclassified == true` (no prior rule matched)

Rule 3 would incorrectly reclassify this PR, changing `TemporalBucket` back to `"future"` and resetting `Category` to `merge_after_focused_review`. This could undo a legitimate "now" promotion.

**Current Mitigant:** The `!reclassified` check and Rules 1/2 matching first provide some protection. However, if such a PR exists (e.g., a large PR with security findings but CI still passing), Rule 3 would incorrectly reclassify it.

**Fix:**
```go
if !reclassified && result.TemporalBucket == "future" && 
   hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
```

---

### MINOR

#### m1: Catchall Rule Doesn't Set `BatchTag` — Inconsistent Behavior

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

**Problem:** When the catchall matches, `BatchTag` is not set (empty string). Rules 1-3 all set appropriate batch tags (`deriveBatchTag` for Rules 1-2, `"docs-batch"` for Rule 2). This creates inconsistent behavior.

**Fix:** Either set a consistent batch tag (e.g., `"genuine-low-value"`) or explicitly document that catchall cases don't receive batch tags.

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

#### m3: Test Count Assertion Silently Ignores Mismatch When `checkFn` Exists

**File:** `internal/review/quickwin_test.go:710-712`  
**Severity:** MINOR

```go
if got != tt.wantReClass && tt.checkFn != nil {
    // checkFn will validate; we just log the count difference for info
}
```

**Problem:** When `got != tt.wantReClass` but `checkFn != nil`, the count mismatch is silently ignored. This means a test could pass even though the reclassification count is incorrect, as long as `checkFn` doesn't fail.

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

### v1: gate17 Struct Aliasing — O1 in synthesis (Verified NOT a Bug)

**File:** `internal/review/quickwin.go:101-111`

The comment notes gate17 is allocated once outside the loop and appended each iteration. In Go, `append` copies struct values into the slice, so each `DecisionLayers` entry is independent. **Not a bug.**

### v2: Integer Division for Day Boundaries — Correct

**Files:** `quickwin.go:225`, `recovery.go:231`

Both `isAbandoned` and `isRecentPush` use `int(time.Since(t).Hours()/24)` for boundary calculations. This correctly:
- Treats 91+ days as abandoned (> 90)
- Treats 90+ days as not recent (>= withinDays)

### v3: Rule 4 TemporalBucket = "junk" — Fixed in Commit

**File:** `quickwin.go:75`

The M6 issue (Rule 4 using `"blocked"` instead of `"junk"`) was fixed in commit 74cdc35. Line 75 now correctly sets `result.TemporalBucket = "junk"`.

### v4: C2 Double-Promotion Fix — Working as Intended

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
| Rule 3 with real analyzer findings | CRITICAL | Would catch C1 bug — test uses synthetic findings |
| Rule 3 with TemporalBucket="now" | MAJOR | Would catch M1 bug |
| Exactly 30/90 day boundaries | MINOR | Boundary tests exist but could be more comprehensive |
| Multi-rule matching | MINOR | No test verifies first-match-wins when PR matches multiple rules |

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
| Critical | 1 (C1: hasQualityFindings prefix mismatch) |
| Major | 1 (M1: Rule 3 temporal bucket guard missing) |
| Minor | 4 (m1-m4: batch tag, test naming, test assertion, integration tests) |
| Verified Correct | 4 (v1-v4) |

The most urgent fix is **C1** — `hasQualityFindings` checks for prefixes that real analyzer findings never use. This renders QuickWin Rule 3 completely non-functional in production, even though tests pass.

---

*Report generated by Hermes Agent analysis of prATC v1.6.1 review engine.*