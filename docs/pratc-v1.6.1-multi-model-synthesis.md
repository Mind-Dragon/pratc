# prATC v1.6.1 Multi-Model Code Review Synthesis

**Commit:** e4e2762  
**Scope:** 18 files changed, 3374 insertions(+), 111 deletions(-)  
**Models:** MiniMax 2.7 (3 runs), Kimi 2.6 (3 runs), Z.AI 5.1 (3 runs) — 9 total analyses  
**Synthesized by:** GPT-5.4

---

## Consensus Findings (All 9 models agree)

### C1. Category Taxonomy Mismatch — Spec vs Implementation
**Severity: HIGH** | Files: `internal/review/recovery.go`, `internal/review/quickwin.go`, `internal/types/models.go`

TODO.md uses category names (`needs_review`, `high_value`, `merge_candidate`, `junk`, `low_value`) that do not exist in `ReviewCategory`. The code uses `merge_after_focused_review` for all promoted PRs and `problematic_quarantine` for demoted PRs. This means:
- Recovered blocked PRs and promoted low-value PRs get the *same* category
- The spec's intent to distinguish "needs review" from "high value" is lost
- All 9 models flagged this; 3 called it HIGH severity

**Fix:** Either add `needs_review`, `high_value`, `merge_candidate`, `low_value` to `ReviewCategory` enum, or update TODO.md to use actual type constants.

---

### C2. `isLowValueCandidate` Does Not Verify Original Category
**Severity: HIGH** | File: `internal/review/quickwin.go:103-123`

`isLowValueCandidate` only checks `TemporalBucket != "future"`, `SubstanceScore < 50`, and `Category != duplicate_superseded/problematic_quarantine`. It does NOT verify the PR was actually `low_value` before the pass. This means:
- PRs reclassified from `blocked` → `merge_after_focused_review` (future) by `RunSecondPass` can then enter `RunQuickWinPass`
- A PR can be double-promoted: blocked → future → now
- All 9 models identified this cross-pass interaction bug

**Fix:** Add `result.Category == types.ReviewCategoryUnknownEscalate || result.TemporalBucket == "blocked"` to `isLowValueCandidate`, or gate `RunQuickWinPass` to only process PRs whose original category was `low_value`.

---

### C3. `ComputeDynamicTarget` Clamping Logic Is Correct
**Severity: NONE (confirmed correct)** | File: `internal/app/service.go:277-292`

All 9 models verified the clamping formula `max(MinTarget, min(MaxTarget, Ratio*pool))` is correctly implemented. Test coverage in `dynamic_target_test.go` is comprehensive (233 lines, 12 test cases covering disabled, zero/negative pool, min/max boundaries, custom ratios).

---

### C4. `Plan()` Nil-Pointer Safety Is Correct
**Severity: NONE (confirmed correct)** | File: `internal/app/service.go:1058-1154`

All 9 models confirmed:
- `collapsedCorpus` is nil-checked before map access at L1137
- `collapsedCorpus != nil` guard at L1152 before pool-size cap
- No race conditions (single-threaded execution)
- No nil-pointer panic risk

---

### C5. `classifyDuplicates` Indentation Is Now Correct
**Severity: NONE (confirmed correct)** | File: `internal/app/service.go:1814-1891`

All 9 models verified the indentation fix is complete. The `openOpenCandidates` post-processing block is correctly outside the `for _, pair := range candidatePairs` loop.

---

### C6. `buildCollapsedCorpus` DFS and Canonical Selection Are Correct
**Severity: NONE (confirmed correct)** | File: `internal/app/service.go:2642-2766`

All 9 models verified:
- DFS connected-component logic is standard and correct
- Only components with >1 PR are collapsed (L2708)
- Canonical selection uses synthesis score + PR number tiebreaker
- No PR can be both canonical and superseded

---

### C7. `expansionSummaryLines()` Has No Overflow Risk
**Severity: NONE (confirmed correct)** | File: `internal/report/plan_section.go:250-269`

All 9 models confirmed: maximum 4 lines, fixed-format strings, no user-controlled input, no truncation risk.

---

## Majority Findings (6-8 of 9 models agree)

### M1. `resolveDynamicTargetConfig` Does Not Default `Enabled=true`
**Severity: WARNING** | File: `internal/app/service.go:258-272`

The docstring says "enabled by default in v1.6.1" but the code does not set `Enabled = true`. Zero-valued `DynamicTargetConfig{}` has `Enabled=false`. CLI usage is fine (plan.go:82 sets `Enabled: true`), but programmatic callers and the API server path use zero-value (disabled). 8 of 9 models flagged this; 1 considered it a docstring issue only.

**Fix:** Add `cfg.Enabled = true` in `resolveDynamicTargetConfig` when the struct is zero-valued, OR update the docstring to say "disabled by default, must be explicitly enabled."

---

### M2. `Plan()` Path Passes `nil` Review/Conflict Data to `buildDuplicateSynthesis`
**Severity: WARNING** | File: `internal/app/service.go:1091`

In the `Plan()` path, `buildDuplicateSynthesis` is called with `nil` for `reviewPayload` and `conflicts`. This degrades canonical nomination to structural signals only (no substance score, confidence, or analyzer findings). 7 of 9 models flagged this. The `Analyze()` path passes actual data. This is by design but means `Plan()` duplicate collapse is less accurate than `Analyze()` collapse.

**Fix:** Either pass empty (non-nil) structs for consistency, or document that `Plan()` collapse uses structural signals only.

---

### M3. `ReclassificationSection` Renders Awkward "None" Page When Empty
**Severity: MEDIUM** | File: `internal/report/analyst_sections.go:903-923`

When no reclassifications occurred, the section still creates a new page and prints "Recovered from blocked: 0 | Re-ranked from low value: 0 | Batch-tagged quick wins: 0" followed by three "None" lists. 7 of 9 models found this awkward. `NearDuplicateDetailSection` handles this correctly with an early `return` guard.

**Fix:** Add early return when all three slices are empty:
```go
if len(s.FromBlocked) == 0 && len(s.FromLowValue) == 0 && len(s.BatchTagged) == 0 {
    return
}
```

---

### M4. `CollapseImpactSection` Silently Fails on Empty Data
**Severity: MEDIUM** | File: `internal/report/analyst_sections.go:1183-1220`

`LoadCollapseImpactSection` returns an error `"no collapsed corpus data"` when `CollapsedGroupCount == 0`. The caller in `report.go` silently skips the section. 6 of 9 models flagged this. A legitimate zero-collapse state produces no section in the PDF with no explanation.

**Fix:** Return a section with `CollapsedGroups: 0` and render a "No duplicate collapse impact" message instead of erroring.

---

### M5. Recovery Rules Use "First Match Wins" Without Priority Validation
**Severity: MEDIUM** | File: `internal/review/recovery.go:36-199`

The 6 recovery rules are evaluated sequentially. If a PR matches multiple rules, only the first fires. 6 of 9 models noted this but disagreed on severity — some consider it correct (deterministic), others worry about rule ordering bugs. No test covers multi-rule matching.

**Fix:** Add a test case where a PR matches both Rule 1 (recoverable CI) and Rule 2 (active draft) to verify first-match behavior is intentional.

---

### M6. Quickwin Rule 4 Uses `temporalBucket = "blocked"` Instead of `"junk"`
**Severity: MEDIUM** | File: `internal/review/quickwin.go:68-76`

TODO.md says abandoned PRs → `junk`, but the code sets `TemporalBucket = "blocked"` for Rule 4. This is inconsistent with recovery.go Rules 4-6 which use `temporalBucket = "junk"`. 7 of 9 models flagged the inconsistency.

**Fix:** Change `TemporalBucket = "blocked"` to `TemporalBucket = "junk"` in quickwin.go Rule 4.

---

### M7. Missing Catchall Rule 5 in Quickwin Pass
**Severity: MEDIUM** | File: `internal/review/quickwin.go`

TODO.md says "All others → low_value (genuine low_value)" but the code does nothing for unmatched PRs. There's no way to distinguish "genuine low_value" from "was low_value but didn't match any rule." 6 of 9 models flagged this.

**Fix:** Add a catchall that sets `ReclassificationReason = "genuine low_value"` or a `GenuineLowValue` flag for unmatched PRs.

---

### M8. Test Coverage Gaps — Boundary Values
**Severity: LOW** | Files: `recovery_test.go`, `quickwin_test.go`

6 of 9 models identified missing boundary tests:
- Exactly 30 days / exactly 90 days for `isRecentPush`
- Exactly 10 conflict pairs for Rule 5
- Exactly 3 conflict pairs for Rule 1
- SubstanceScore boundaries: 20, 25, 30, 40

**Fix:** Add boundary-value test cases.

---

## Outlier Findings (1-3 of 9 models — needs verification)

### O1. `gate17` Struct Aliasing in `RunQuickWinPass`
**Severity: LOW** | File: `internal/review/quickwin.go:85-94`

1 model noted that `gate17` is allocated once outside the loop and appended in each iteration. If `append` doesn't trigger reallocation, all `DecisionLayers` entries could share the same struct instance. This is technically true in Go but practically harmless since the struct is never mutated after append.

**Verdict:** Not a bug. Go slices copy values on append.

---

### O2. `bestScore[canonical] == 0` No-Op Assignment
**Severity: LOW** | File: `internal/app/service.go:2684-2686`

1 model flagged:
```go
if bestScore[canonical] == 0 {
    bestScore[canonical] = 0
}
```
This is a no-op. Harmless but indicates incomplete logic — the intent may have been to distinguish "never set" from "set to 0.0".

**Verdict:** Code smell. Remove or replace with meaningful default.

---

### O3. Python Bootstrap `KeyError` on Missing Map Fields
**Severity: MEDIUM** | File: `ml-service/src/pratc_ml/models.py`

1 model noted that Python bootstrap `_coerce_dataclass` at line 306 does `value[item.name]` without checking field existence. If Go's `omitempty` omits a map field, bootstrap crashes with `KeyError`. Pydantic path handles this correctly.

**Verdict:** Real bug in bootstrap fallback path. Fix `_coerce_dataclass` to use `.get()` with defaults.

---

### O4. TypeScript `AnalysisResponse` Declared Twice
**Severity: MEDIUM** | File: `web/src/types/api.ts`

1 model noted `AnalysisResponse` is declared twice (lines 198-209 and 325-336). The second definition is incomplete (missing `garbagePRs`, `stalenessSignals`, etc.). TypeScript uses the last declaration.

**Verdict:** Real bug. Remove the duplicate incomplete declaration.

---

### O5. `duplicate_synthesis` Missing from Python `AnalysisResponse`
**Severity: HIGH** | File: `ml-service/src/pratc_ml/models.py`

1 model noted `duplicate_synthesis` field exists in Go `AnalysisResponse` but is completely absent from Python's Pydantic and Bootstrap `AnalysisResponse` models. Any Python code receiving this field would drop it.

**Verdict:** Real bug if Python ML service ever needs to process duplicate synthesis data.

---

## Summary Table

| ID | Finding | Severity | Confidence | Files |
|----|---------|----------|------------|-------|
| C1 | Category taxonomy mismatch | HIGH | 9/9 consensus | recovery.go, quickwin.go, models.go |
| C2 | `isLowValueCandidate` no original category check | HIGH | 9/9 consensus | quickwin.go |
| C3 | `ComputeDynamicTarget` correct | NONE | 9/9 consensus | service.go |
| C4 | `Plan()` nil-safety correct | NONE | 9/9 consensus | service.go |
| C5 | `classifyDuplicates` indent correct | NONE | 9/9 consensus | service.go |
| C6 | `buildCollapsedCorpus` correct | NONE | 9/9 consensus | service.go |
| C7 | `expansionSummaryLines` safe | NONE | 9/9 consensus | plan_section.go |
| M1 | `Enabled` not defaulted true | WARNING | 8/9 majority | service.go |
| M2 | `Plan()` passes nil review data | WARNING | 7/9 majority | service.go |
| M3 | ReclassificationSection awkward empty | MEDIUM | 7/9 majority | analyst_sections.go |
| M4 | CollapseImpactSection silent fail | MEDIUM | 6/9 majority | analyst_sections.go |
| M5 | First-match-wins untested | MEDIUM | 6/9 majority | recovery.go |
| M6 | Quickwin Rule 4 bucket mismatch | MEDIUM | 7/9 majority | quickwin.go |
| M7 | Missing quickwin catchall | MEDIUM | 6/9 majority | quickwin.go |
| M8 | Boundary test gaps | LOW | 6/9 majority | recovery_test.go, quickwin_test.go |
| O1 | gate17 struct aliasing | LOW | 1/9 outlier | quickwin.go |
| O2 | bestScore no-op | LOW | 1/9 outlier | service.go |
| O3 | Bootstrap KeyError | MEDIUM | 1/9 outlier | models.py |
| O4 | TS AnalysisResponse duplicate | MEDIUM | 1/9 outlier | api.ts |
| O5 | duplicate_synthesis missing in Python | HIGH | 1/9 outlier | models.py |

---

## Recommended Action Items

### Must Fix Before Release
1. **C1** — Align category taxonomy (add constants or update spec)
2. **C2** — Fix cross-pass double-promotion bug
3. **M6** — Fix quickwin Rule 4 bucket (`"blocked"` → `"junk"`)

### Should Fix
4. **M1** — Default `Enabled=true` or fix docstring
5. **M3** — Add empty-state guard to ReclassificationSection
6. **M4** — Handle zero-collapse gracefully in CollapseImpactSection
7. **M7** — Add quickwin catchall rule
8. **O3** — Fix bootstrap `_coerce_dataclass` KeyError
9. **O4** — Remove duplicate TS AnalysisResponse
10. **O5** — Add `duplicate_synthesis` to Python models

### Nice to Have
11. **M2** — Document or fix Plan() nil review data
12. **M5** — Add multi-rule match test
13. **M8** — Add boundary value tests
14. **O2** — Remove no-op bestScore assignment
