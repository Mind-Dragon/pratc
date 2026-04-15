# prATC v1.4 Active Work Backlog

## Goal

Finish the v1.4 full-corpus triage engine so every PR is accounted for, the decision path is layered and explainable, and the report tells a human what matters now versus later.

## Source of truth

- `ROADMAP.md` defines the remaining v1.4 phases.
- `GUIDELINE.md` defines the allowed bucket vocabulary and non-negotiables.
- `ARCHITECTURE.md` defines the system shape and data flow.
- `version1.4.md` is the short milestone summary.

## What is already shipped

- Planning integration is in the codebase.
- Full-corpus analysis no longer depends on a hidden `maxPRs=1000` default.
- The review/report path already carries reasons, confidence, duplicates, and staleness.
- The 16-layer ladder exists in code and tests.
- The 6,000+ PR proof test/benchmark exists.

These are foundations. They stay out of the active backlog.

## Remaining v1.4 work

### 1. Corpus coverage and baseline repair

**Goal:** Make the corpus path honest under large repos and keep all coverage limits explicit.

**Work items:**
- Verify there is no remaining hidden corpus cap in the primary analysis path.
- Make any remaining candidate-pool limit explicit and configurable, or remove it as a hard gate.
- Keep every PR visible through storage, analysis, and reporting.
- Keep the 6,000+ corpus proof as a first-class regression test or benchmark.

**Files to inspect:**
- `internal/app/service.go`
- `internal/cmd/analyze.go`
- `internal/filter/pool.go`
- `internal/github/client.go`
- `internal/app/v1_4_scale_benchmark_test.go`
- `internal/app/service_test.go`

**Verification:**
- `go test ./internal/app ./internal/cmd -run 'Analyze|MaxPRs|Corpus|6000' -v`
- `go test ./internal/app -bench 'Benchmark.*6000' -run '^$'`

**Done when:**
- No hidden cap can silently shrink the corpus.
- A 6,000+ PR run completes without truncation unless explicitly requested.

### 2. Outer peel layers

**Goal:** Keep garbage, duplicates, and obvious badness as visible first-class outcomes.

**Work items:**
- Preserve layer 1–3 ordering and reason trails.
- Make peeled/rejected items visible in the report rather than disappearing from view.
- Keep duplicate canonicals and chains readable.
- Keep junk and stale classifications auditable.

**Files to inspect:**
- `internal/review/orchestrator.go`
- `internal/app/service.go`
- `internal/report/analyst_sections.go`
- `internal/report/review_section.go`
- `internal/review/layers_test.go`

**Verification:**
- `go test ./internal/analysis ./internal/filter ./internal/review ./internal/report -run 'Peel|Reason|Duplicate|Junk|Stale' -v`

**Done when:**
- Every peeled PR keeps a reason trail.
- Nothing falls out of the corpus without an explanation.

### 3. Substance scoring and now/future routing

**Goal:** Make layer 4 and layer 5 route real work, not just label it.

**Work items:**
- Keep substance scoring aligned with security, reliability, performance, and roadmap fit.
- Keep now/future routing explicit in both the API and report surfaces.
- Keep low-value work from crowding out active queue items.
- Preserve bucket mappings in the types, CLI, and web surfaces.

**Files to inspect:**
- `internal/review/orchestrator.go`
- `internal/review/analyzer_security.go`
- `internal/review/analyzer_reliability.go`
- `internal/review/analyzer_quality.go`
- `internal/types/models.go`
- `internal/cmd/analyze.go`
- `web/src/types/api.ts`
- `web/src/components/TriageView.tsx`

**Verification:**
- `go test ./internal/app ./internal/cmd ./internal/report -run 'Review|Bucket|Vocabulary|Priority' -v`
- `bun run test`

**Done when:**
- `now`, `future`, and `blocked` are consistent across code and output.
- The review payload and UI/report surfaces tell the same story.

### 4. Deep judgment layers

**Goal:** Finish layers 6–16 as a readable, stable decision ladder.

**Work items:**
- Keep confidence, dependency, blast radius, leverage, ownership, stability, mergeability, strategic weight, attention cost, reversibility, and signal quality wired into final review output.
- Keep each layer’s reasons visible in the decision trail.
- Keep bucket routing stable as the layer order evolves.

**Files to inspect:**
- `internal/review/orchestrator.go`
- `internal/review/layers_test.go`
- `internal/app/service.go`
- `internal/types/models.go`
- `internal/report/analyst_sections.go`

**Verification:**
- `go test ./internal/review ./internal/planning ./internal/app -run 'Confidence|Dependency|Blast|Leverage|Ownership|Stability|Mergeability|Strategic|Attention|Reversibility|Signal' -v`

**Done when:**
- Layers 6–16 show up in payloads and report output with reason trails.
- The layer order matches `GUIDELINE.md` and `ARCHITECTURE.md`.

### 5. Report and output composition

**Goal:** Make the report read like a decision map for the full corpus.

**Work items:**
- Keep the analyst section aligned with the current review vocabulary.
- Keep the PDF composer and validator in sync with the decision ladder.
- Keep duplicates, junk, blocked, and risk items visible in dedicated sections.
- Keep the appendix covering the full corpus.

**Files to inspect:**
- `internal/report/pdf.go`
- `internal/report/analyst_sections.go`
- `internal/report/review_section.go`
- `internal/report/validator.go`
- `internal/cmd/report.go`

**Verification:**
- `go test ./internal/report ./internal/cmd -run 'Report|Validator|Analyst|ReviewSection' -v`
- Generate a sample report and inspect the sections.

**Done when:**
- The report exposes reasons, confidence, and bucket placement without hiding corpus coverage.
- No PR vanishes from the final artifact.

## Non-goals for v1.4

- No auto-merge or auto-approve behavior.
- No GitHub App or webhook expansion.
- No ML feedback loop.
- No gRPC control plane.
- No silent corpus truncation.

## Suggested execution order

1. Corpus coverage and baseline repair.
2. Outer peel layers.
3. Substance scoring and now/future routing.
4. Deep judgment layers.
5. Report and output composition.
