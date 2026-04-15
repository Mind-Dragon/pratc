# prATC v1.4 TODO

## Goal

Finish the v1.4 full-corpus triage engine so the codebase, docs, tests, and outputs all describe the same system.

## Definition of done

- Every PR in the corpus is represented somewhere in the system.
- No hidden cap silently drops PRs from analysis or reporting.
- The 16-layer decision ladder is defined and applied.
- Duplicates are canonicalized and linked.
- Obvious garbage is separated from meaningful work.
- Current and future priorities are split explicitly.
- The report shows reasons, confidence, and bucket placement.
- GUIDELINE.md, ARCHITECTURE.md, ROADMAP.md, version1.4.md, and TODO.md agree with the code.
- `git status` is clean on `main` except for intentionally ignored generated artifacts.
- Relevant tests on `main` are green.

## Bucket vocabulary

Locked by GUIDELINE.md:
- Temporal: `now`, `future`, `blocked`
- Quality: `high_value`, `merge_candidate`, `needs_review`, `re_engage`, `low_value`
- Disposal: `duplicate`, `junk`, `stale`
- Risk: `security_risk`, `reliability_risk`, `performance_risk`

## Completed foundation

- [x] Synchronize the doc suite around the v1.4 full-corpus triage model.
- [x] Remove the hidden `maxPRs=1000` truncation from the primary analysis path.
- [x] Add caller-visible paging/streaming for cache PR listing and wire the streaming consumer.
- [x] Merge the paged PR listing slice back into `main`.
- [x] Ignore generated media artifacts (`scorecard.png`, `videos/`) so the repo can stay clean.

## Next sprint backlog

### 1. Review vocabulary migration to the v1.4 top-level review buckets

**Objective:** Replace the legacy review-category surface with the v1.4 top-level bucket vocabulary everywhere the user sees or consumes review output.

**Files:**
- Modify: `internal/types/models.go`
- Modify: `internal/app/service.go`
- Modify: `internal/cmd/analyze.go`
- Modify: `internal/report/review_section.go`
- Modify: `web/src/types/api.ts`
- Modify: `web/src/pages/index.tsx`
- Modify: `web/src/components/TriageView.tsx`
- Update tests: `internal/app/review_vocabulary_test.go`, `internal/app/service_review_test.go`, `internal/app/review_boundary_test.go`, `internal/app/review_regression_test.go`, `internal/cmd/analyze_vocabulary_test.go`, `internal/cmd/analyze_command_test.go`, `internal/report/analyst_sections_test.go`, `web/src/components/TriageView.test.tsx`

**TDD steps:**
1. Add failing tests that assert the visible review summary emits `now`, `future`, `blocked`, `duplicate`, and `junk` instead of the legacy labels.
2. Verify the tests fail against the current legacy labels.
3. Implement the smallest compatibility layer needed in `internal/app/service.go`, `internal/cmd/analyze.go`, and the downstream renderers.
4. Re-run the tests until the new vocabulary is present in CLI, report, and web output.

**Verification:**
- `go test ./internal/app ./internal/cmd ./internal/report -run 'Review|Bucket|Vocabulary|Priority' -v`
- `bun run test`
- Manual scan of rendered output to confirm legacy labels no longer appear on the primary review summary path.

**Done when:**
- The review payload and UI/report surfaces expose the v1.4 top-level bucket labels.
- Any legacy strings survive only in clearly documented compatibility shims or tests.

### 2. Formalize the outer-peel ladder

**Objective:** Make garbage, duplicates, and obvious badness explicit first-class layers with readable reason trails.

**Files:**
- Modify: `internal/analysis/`
- Modify: `internal/filter/pipeline.go`
- Modify: `internal/review/orchestrator.go`
- Modify: `internal/app/service.go`
- Modify: `internal/report/analyst_sections.go`
- Update tests: `internal/filter/pipeline_test.go`, `internal/review/classifier_test.go`, `internal/app/review_boundary_test.go`, `internal/report/analyst_sections_test.go`

**TDD steps:**
1. Write a test that proves a PR can be classified as garbage, duplicate, or obvious badness and still keep a reason trail.
2. Verify the test fails with the current implicit layering.
3. Add the smallest layer objects / helpers needed to express the peel order.
4. Ensure each rejected or peeled item still appears in the corpus report with a reason code.

**Verification:**
- `go test ./internal/analysis ./internal/filter ./internal/review ./internal/report -run 'Peel|Reason|Duplicate|Junk|Stale' -v`
- Inspect report output for non-disappearing PRs.

**Done when:**
- Layer 1–3 are explicit in code and docs.
- Every peeled item keeps a reason trail.
- No PR silently vanishes from the report.

### 3. Implement the remaining deep judgment layers

**Objective:** Finish the v1.4 judgment stack for confidence, dependency, blast radius, leverage, ownership, stability, mergeability, strategic weight, attention cost, reversibility, and signal quality.

**Files:**
- Modify: `internal/review/`
- Modify: `internal/planning/`
- Modify: `internal/app/service.go`
- Modify: `internal/types/models.go`
- Update tests: `internal/app/planning_integration_test.go`, `internal/app/service_review_test.go`, `internal/review/*_test.go`, `internal/planning/*_test.go`

**TDD steps:**
1. Add tests for each layer’s scoring/reason behavior in the smallest unit that already exists.
2. Verify the tests fail before implementation.
3. Implement the layer logic one layer at a time, starting with the ones that already have data sources in the repo.
4. Wire the layer order explicitly so the sequence is readable and stable.

**Verification:**
- `go test ./internal/review ./internal/planning ./internal/app -run 'Confidence|Dependency|Blast|Leverage|Ownership|Stability|Mergeability|Strategic|Attention|Reversibility|Signal' -v`
- Confirm telemetry and rejection output include the new layer reasons.

**Done when:**
- The layer order matches GUIDELINE.md and ARCHITECTURE.md.
- The new reasons show up in the response payloads and report surfaces.

### 4. Add a 6,000+ PR corpus proof

**Objective:** Prove the system handles the intended corpus scale without hidden truncation or accidental narrowing.

**Files:**
- Create: `internal/app/v1_4_scale_benchmark_test.go` or `internal/app/plan_benchmark_test.go`
- Modify: `internal/testutil/` fixture helpers if a synthetic generator is needed
- Modify: `scripts/` only if a repeatable benchmark runner is needed

**TDD steps:**
1. Add a synthetic 6,000+ PR benchmark or reproducible fixture generator.
2. Verify the benchmark or test fails or is missing before implementation.
3. Implement the minimal generator / benchmark wiring.
4. Run the benchmark and capture the results in a local artifact or report.

**Verification:**
- `go test ./internal/app -run 'Test.*6000|Benchmark.*6000' -v`
- `go test ./internal/app -bench 'Benchmark.*6000' -run '^$'`
- Record the benchmark output in the sprint evidence if it is used for signoff.

**Done when:**
- A 6,000+ PR corpus run completes with the expected outputs.
- The benchmark proves the corpus path is still complete after the new layers are added.

### 5. Finish report composition and validation

**Objective:** Make the PDF / report path reflect the full decision model, not just a summary of legacy buckets.

**Files:**
- Modify: `internal/report/pdf.go`
- Modify: `internal/report/analyst_sections.go`
- Modify: `internal/report/review_section.go`
- Modify: `internal/report/validator.go`
- Modify: `internal/cmd/report.go`
- Update tests: `internal/report/pdf_test.go`, `internal/report/analyst_sections_test.go`, `internal/report/validator_test.go`

**TDD steps:**
1. Add tests that assert the report contains the new bucket sections and reason trails.
2. Verify the tests fail against the current report structure.
3. Update the report composer so the sections line up with the v1.4 vocabulary.
4. Verify the report validator rejects missing or truncated artifacts.

**Verification:**
- `go test ./internal/report ./internal/cmd -run 'Report|Validator|Analyst|ReviewSection' -v`
- Generate a sample report and inspect the produced sections.

**Done when:**
- The report reads like a decision map.
- Every PR has a visible reason or a visible place in the appendix.

## Suggested execution order

1. Review vocabulary migration to the v1.4 model.
2. Formalize the outer-peel ladder.
3. Implement the remaining deep judgment layers.
4. Add the 6,000+ PR corpus proof.
5. Finish report composition and validation.

## 16-layer crosswalk

- Layers 1–3 belong to `outer-peel-ladder`:
  - 1 Garbage
  - 2 Duplicates
  - 3 Obvious badness
- Layers 4–5 are already embodied in the existing scoring/bucket routing path:
  - 4 Substance score
  - 5 Now vs future
- Layers 6–16 belong to `deep-judgment-layers`:
  - 6 Confidence
  - 7 Dependency
  - 8 Blast radius
  - 9 Leverage
  - 10 Ownership
  - 11 Stability
  - 12 Mergeability
  - 13 Strategic weight
  - 14 Attention cost
  - 15 Reversibility
  - 16 Signal quality
- `report-composition-validation` owns the report surfaces that expose the ladder to humans.
- `review-vocabulary-migration` owns the surface labels that keep the buckets aligned with GUIDELINE.md.

## Notes

- The worktree branch is now redundant once its changes are represented on `main`.
- Do not treat a nice-looking label swap as complete unless the downstream report/UI surfaces and tests also changed.
- Keep `TODO.md`, `GUIDELINE.md`, `ARCHITECTURE.md`, and `version1.4.md` in lockstep.
