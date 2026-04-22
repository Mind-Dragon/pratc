# prATC v1.6.1 Precision Improvement Plan

**Branch:** `fix/v1.6.1-post-review`  
**Base Commit:** `74cdc35`  
**Goal:** Fix all CRITICAL/HIGH/MEDIUM findings from multi-model synthesis without breaking existing functionality  
**Method:** 6-phase iterative pipeline with contract tests as safety net

---

## Phase 0: Preflight (COMPLETE)

[CHECK] `go test ./...` — all green  
[CHECK] `go build ./...` — clean  
[CHECK] Working tree clean except synthesis docs (untracked analysis files quarantined)  
[ACK] Baseline established

---

## Phase 1: Safety Net — Contract Tests + Model Alignment

**Goal:** Before touching any production code, write tests that prove the bugs exist. These tests become the regression suite.

**P1.1** `internal/review/quickwin_test.go` — Add `TestHasQualityFindings_RealAnalyzerOutput`  
- Create findings with `AnalyzerName: "security"` but `Finding: "risky file path detected: main.go"`  
- Assert `hasQualityFindings` returns `true` (currently returns `false` — test should FAIL)  
- This test documents the CRITICAL bug and will pass after the fix

**P1.2** `internal/review/quickwin_test.go` — Add `TestRule3_TemporalBucketGuard`  
- PR with `TemporalBucket: "now"`, `SubstanceScore: 45`, quality findings  
- Assert Rule 3 does NOT reclassify (currently would reclassify — test should FAIL)

**P1.3** `internal/app/dynamic_target_test.go` — Add `TestResolveDynamicTargetConfig_ZeroValueEnabledDefault`  
- Pass `DynamicTargetConfig{}` to `resolveDynamicTargetConfig`  
- Assert `Enabled == true` (currently returns `false` — test should FAIL)

**P1.4** `ml-service/tests/test_models.py` — Add `TestCoerceDataclass_MissingKey`  
- Pass dict missing a field to `_coerce_dataclass`  
- Assert no KeyError (currently panics — test should FAIL)

**P1.5** `ml-service/tests/test_duplicate_synthesis_models.py` — Fix imports  
- Remove import of non-existent `DuplicateSynthesisCandidate`/`DuplicateSynthesisPlan`  
- Mark tests as `pytest.skip("models not yet implemented")`  
- Prevents ImportError blocking the suite

**P1.6** `internal/cmd/plan_command_test.go` — Add `TestValidateTargetRatio_Negative`  
- Pass `-0.5` to `validateTargetRatio`  
- Assert error returned (currently passes silently — test should FAIL)

**Verification:** Run all test suites. Expect NEW failures only on P1.1–P1.6 tests. Pre-existing failures must be documented.

---

## Phase 2: Critical Fixes — Core Logic Bugs

**Goal:** Fix the 3 CRITICAL/HIGH consensus bugs that affect production behavior. Each fix is paired with its Phase 1 test.

**P2.1** `internal/review/quickwin.go:200-209` — Fix `hasQualityFindings`  
```go
func hasQualityFindings(findings []types.AnalyzerFinding) bool {
    for _, f := range findings {
        name := strings.ToLower(f.AnalyzerName)
        if name == "security" || name == "reliability" || name == "performance" {
            return true
        }
    }
    return false
}
```
- Verification: P1.1 test now passes
- Regression: All existing quickwin tests still pass

**P2.2** `internal/review/quickwin.go:60-66` — Add Rule 3 temporal guard  
```go
if !reclassified && result.TemporalBucket == "future" &&
   hasQualityFindings(result.AnalyzerFindings) && result.SubstanceScore > 40 {
```
- Verification: P1.2 test now passes
- Regression: Run `go test ./internal/review/...`

**P2.3** `internal/app/service.go:258-272` — Fix `resolveDynamicTargetConfig` Enabled default  
```go
func resolveDynamicTargetConfig(cfg DynamicTargetConfig) DynamicTargetConfig {
    if cfg.Ratio <= 0 { cfg.Ratio = 0.05 }
    if cfg.MinTarget <= 0 { cfg.MinTarget = 20 }
    if cfg.MaxTarget <= 0 { cfg.MaxTarget = 100 }
    if !cfg.Enabled { cfg.Enabled = true }  // ADD THIS
    return cfg
}
```
- Verification: P1.3 test now passes
- Update existing test expectation if it asserts `Enabled: false`

**P2.4** `internal/cmd/plan.go:146-153` — Add lower bound to `validateTargetRatio`  
```go
if ratio < 0 || ratio > 1.0 {
    return fmt.Errorf("target-ratio %.2f must be between 0 and 1.0", ratio)
}
```
- Verification: P1.6 test now passes

**P2.5** `internal/review/quickwin.go:103-123` — Harden `isLowValueCandidate`  
- Add check that original category was actually `low_value` or `unknown`  
- Prevents blocked PRs that were promoted by SecondPass from entering QuickWin

**Commit checkpoint:** `fix(v1.6.1-p2): critical logic bugs — hasQualityFindings, Rule3 guard, DynamicTarget default`

---

## Phase 3: High-Impact Fixes — Model Alignment + Serialization

**Goal:** Fix the Python/Go contract drift and missing models. These are structural changes that must be done together.

**P3.1** `ml-service/src/pratc_ml/models.py` — Add `DuplicateSynthesisCandidate` (Pydantic + Bootstrap)  
- Fields: `pr_number`, `title`, `author`, `role`, `synthesis_score`, `confidence`, `substance_score`, `mergeable`, `has_test_evidence`, `conflict_footprint`, `is_draft`, `signal_quality`, `scoring_factors`, `rationale`
- Match Go `internal/types/models.go:65-88` exactly

**P3.2** `ml-service/src/pratc_ml/models.py` — Add `DuplicateSynthesisPlan` (Pydantic + Bootstrap)  
- Fields: `group_id`, `group_type`, `original_canonical_pr`, `nominated_canonical_pr`, `similarity`, `reason`, `candidates`, `synthesis_notes`
- Match Go `internal/types/models.go:90-101` exactly

**P3.3** `ml-service/src/pratc_ml/models.py` — Add `duplicate_synthesis` to `AnalysisResponse`  
- Pydantic: `duplicate_synthesis: list[DuplicateSynthesisPlan] | None = None`
- Bootstrap: `duplicate_synthesis: list[DuplicateSynthesisPlan] = field(default_factory=list)`

**P3.4** `ml-service/src/pratc_ml/models.py` — Fix `_coerce_dataclass` KeyError  
```python
def _coerce_dataclass(cls: type[T], value: Any) -> T:
    kwargs = {}
    for item in fields(cls):
        if item.name in value:
            kwargs[item.name] = _coerce_value(item.type, value[item.name])
        else:
            if item.default_factory is not MISSING:
                kwargs[item.name] = item.default_factory()
            elif item.default is not MISSING:
                kwargs[item.name] = item.default
            else:
                kwargs[item.name] = None
    return cls(**kwargs)
```
- Verification: P1.4 test now passes

**P3.5** `ml-service/tests/test_duplicate_synthesis_models.py` — Re-enable tests  
- Remove `pytest.skip`
- Fix field names to match Go contract (`synthesis_score` not `score`)
- Add round-trip test: Go JSON → Python model → Python JSON

**P3.6** `web/src/types/api.ts` — Merge duplicate `AnalysisResponse` declarations  
- Keep one interface with ALL fields: `prs`, `clusters`, `duplicates`, `overlaps`, `conflicts`, `stalenessSignals`, `review_payload`, `collapsed_corpus`, `duplicate_synthesis`
- Use `?` for optional fields

**P3.7** `web/src/types/api.ts` — Add missing fields to `MergePlanCandidate`  
- `reasons: string[]` (Go has `Reasons []string`)

**P3.8** `web/src/types/api.ts` — Add `api_version` to `HealthResponse`

**Commit checkpoint:** `fix(v1.6.1-p3): Go/Python/TS model alignment — duplicate_synthesis, _coerce_dataclass, TS contracts`

---

## Phase 4: Medium Fixes — Empty-State + API Surface

**Goal:** Fix edge cases and API mismatches. These are lower risk but improve robustness.

**P4.1** `internal/report/analyst_sections.go:203-205,225-227` — Add canonical PR existence check  
```go
if canonical, ok := prByNumber[dup.CanonicalPRNumber]; ok {
    entry.CanonicalTitle = canonical.Title
} else {
    entry.CanonicalTitle = fmt.Sprintf("PR #%d (not in analysis)", dup.CanonicalPRNumber)
}
```

**P4.2** `internal/report/analyst_sections.go:1203-1204` — Return zero-section instead of error  
```go
if collapsed.CollapsedGroupCount == 0 {
    return &CollapseImpactSection{Repo: repo, GeneratedAt: time.Now()}, nil
}
```

**P4.3** `internal/cmd/serve.go:513-518` — Remove or implement `exclude_conflicts`  
- Option A: Remove parsing (dead code)
- Option B: Pass to `service.Plan()` (requires Plan signature change — higher risk, defer)
- **Decision:** Remove for now. Add back when planner supports it.

**P4.4** `internal/cmd/serve.go:551-559` — Document per-request service allocation  
- Add comment explaining why new service is needed (different CollapseDuplicates config)
- Add TODO for future optimization (service pool or config mutation)

**P4.5** `web/src/lib/api.ts` — Fix default port 8080 → 7400  
- Update all hardcoded `localhost:8080` to `localhost:7400`
- Update `.env.example`
- Update `web/AGENTS.md`

**P4.6** `web/src/lib/api.ts` vs `internal/cmd/serve.go` — Document settings API mismatch  
- Add comment in both files noting the known mismatch
- Do NOT change paths yet (requires coordinated handler + client change — Phase 5)

**P4.7** `internal/review/quickwin.go:82-88` — Add BatchTag to catchall rule  
- `result.BatchTag = "genuine-low-value"`

**Commit checkpoint:** `fix(v1.6.1-p4): empty-state guards, API cleanup, port alignment`

---

## Phase 5: Polish — Tests + Constants + Documentation

**Goal:** Fill test gaps, extract magic numbers, update docs.

**P5.1** `internal/review/quickwin_test.go` — Add cross-pass integration test  
- Sequence: `RunSecondPass` → verify blocked PRs promoted → `RunQuickWinPass` → verify promoted PRs NOT reclassified again

**P5.2** `internal/review/recovery_test.go` — Add boundary tests for substance_score 49/50, temporal thresholds at exactly 30/90 days

**P5.3** `internal/cmd/plans_api_test.go` — Add `handlePlanOmni` invalid selector test

**P5.4** `internal/report/` — Add empty-state tests for `ClusterSection`, `PoolCompositionSection`, `ReviewSection`

**P5.5** `internal/review/quickwin.go` + `recovery.go` — Extract temporal thresholds to constants  
```go
const (
    abandonedThresholdDays = 90
    recentPushThresholdDays = 30
)
```

**P5.6** Update `docs/pratc-v1.6.1-multi-model-synthesis.md` — Mark fixed items as resolved

**Commit checkpoint:** `fix(v1.6.1-p5): test coverage, constants extraction, docs`

---

## Phase 6: Final Verification

**V6.1** `go test ./...` — all green  
**V6.2** `go build ./...` — clean  
**V6.3** Python tests: `cd ml-service && uv run pytest` — all green  
**V6.4** TypeScript type-check: `cd web && npm run type-check` (or `tsc --noEmit`) — clean  
**V6.5** Stash/unstash verification: confirm no pre-existing failures masked by changes  
**V6.6** Merge `fix/v1.6.1-post-review` → `main`  
**V6.7** Tag: `v1.6.1-fix1` or continue to `v1.6.2`

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Model alignment changes break Go→Python IPC | P3.5 round-trip test validates serialization |
| TS type changes break frontend build | V6.4 type-check gate |
| DynamicTarget default change breaks API callers | P1.3 test + P2.3 fix are paired; explicit `Enabled=false` still respected |
| Rule 3 guard changes existing behavior | Only affects PRs with `TemporalBucket="now"` — these should NOT have been reclassified anyway |
| hasQualityFindings change breaks tests | Tests use synthetic data — update test data to use real analyzer names |
| Per-request service allocation becomes bottleneck | P4.4 documents it; optimization deferred to v1.7 |

## Rollback Plan

Each phase is a separate commit. If any phase fails verification:
1. `git revert HEAD` (revert single phase commit)
2. Fix the issue
3. Re-run verification
4. Continue to next phase

## Subagent Delegation

| Phase | Parallel Tasks | Budget |
|-------|---------------|--------|
| P1 | 6 contract tests | 100 turns each |
| P2 | 5 fixes (sequential within file) | 150 turns each |
| P3 | 8 model alignment tasks | 200 turns each |
| P4 | 7 medium fixes | 100 turns each |
| P5 | 6 polish tasks | 100 turns each |
| P6 | 1 verification task | 50 turns |

**Coordination:** Each phase waits for previous phase commit + verification before starting.
