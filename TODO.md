# prATC v1.4 TODO

## Goal
Build the full-corpus triage engine for large repositories.
Every PR is accounted for. Noise is peeled away in 16 layers. Duplicates are collapsed. Substance is scored. Current priorities are separated from future work. The final output is a decision map, not a flat list.

## Definition of done
- Every PR in the corpus is represented somewhere in the system.
- No hidden cap silently drops PRs from analysis.
- The 16-layer decision ladder is defined and applied.
- Duplicates are canonicalized and linked.
- Obvious garbage is separated from meaningful work.
- Current and future priorities are split explicitly.
- The report shows reasons, confidence, and bucket placement.
- The architecture, guideline, and roadmap documents match the behavior.
- All tests on main are green.

## Bucket vocabulary (locked — GUIDELINE.md is the authority)

Temporal (mutually exclusive): `now`, `future`, `blocked`
Quality: `high_value`, `merge_candidate`, `needs_review`, `re_engage`, `low_value`
Disposal: `duplicate`, `junk`, `stale`
Risk (additive): `security_risk`, `reliability_risk`, `performance_risk`

## Workstreams

### 0. Phase 0 — Baseline repair
These items are debt from prior work. They must be resolved before new v1.4 work begins.

- [ ] Fix TestHandleAnalyze x3 failures in internal/cmd/sync_api_test.go
- [ ] Fix TestCorsMiddleware x2 failures in internal/cmd/cors_test.go
- [ ] Verify `make test-go` is fully green on main after fixes
- [ ] Verify `make build` succeeds on main after fixes

### 1. Contract alignment
- [x] Rewrite ROADMAP.md to define v1.4 as the full-corpus triage engine with Phase 0 history.
- [x] Create GUIDELINE.md with the 16-layer decision ladder and full bucket vocabulary.
- [x] Create ARCHITECTURE.md with system shape, data flow, and codebase mapping.
- [x] Rename docs/architecture.md to docs/techref.md (technical reference).
- [x] Archive pratc14a.md to docs/archive/pratc14a-remediation.md.
- [ ] Audit all four docs for bucket vocabulary consistency (14 buckets, same names everywhere).
- [ ] Cross-reference ROADMAP phase labels with CHANGELOG phase labels (Phase 0 = old A+B, new A–F = new work).

### 2. Corpus coverage
- [ ] Remove the hidden maxPRs=1000 default in internal/cmd/analyze.go, internal/cmd/workflow.go, and any related help text; make no-cap behavior explicit and verify analyze/workflow no longer silently truncate unless the user opts in.
- [ ] Make DefaultPoolCap (internal/types/models.go + internal/filter/pipeline.go + internal/app/service.go + internal/formula/config.go) configurable via settings or remove it as a hard gate on analysis/planning coverage; verify analysis and plan keep the full active set unless a visible truncation reason is emitted.
- [ ] Make DefaultCandidatePoolCap (internal/types/constants.go + internal/app/service.go + internal/planning/hierarchy.go) configurable via settings or remove it as a hard gate on planning coverage; verify planning does not silently drop candidates.
- [ ] Add cursor-based pagination to ListPRs() in internal/cache/sqlite.go (pratc14a perf-07); verify large repos can stream through the cache API without loading the whole corpus at once.
- [ ] Add streaming insert to bootstrap sync in internal/sync/worker.go (pratc14a perf-09); verify sync inserts remain bounded in memory and preserve all rows.
- [ ] Confirm the ingest path can represent every PR in the repository end-to-end; verify with a corpus-size fixture or synthetic load.
- [ ] Preserve the full corpus in storage even when later layers narrow the active queue; verify active filtering never destroys the underlying data.
- [ ] Keep a reason trail for every item that leaves the active path; verify in report output and rejection metadata.
- [ ] Add a large-corpus fixture or benchmark case for 6,000+ PRs; verify with a reproducible test or benchmark command.

### 3. Outer peel layers
- [ ] Layer 1 (garbage): extend internal/analysis/ bot detection + new classifiers for abandoned, malformed, empty PRs. Emit reason codes.
- [ ] Layer 2 (duplicates): wire existing ML duplicate detection (internal/ml/ bridge → Python) into the layered pipeline. Choose canonical. Link duplicates.
- [ ] Layer 3 (obvious badness): extend existing spam classification (bot_generated, spam_pattern, malformed, promotional) into a formal layer. Add malware/junkware patterns.
- [ ] Verify each layer emits a readable reason per PR.
- [ ] Verify a PR can move out of the active queue without disappearing from the corpus report.

### 4. Substance scoring
- [ ] Layer 4 (substance): extend internal/review/ security analyzer to emit a layer-4 security score.
- [ ] Extend internal/review/ reliability analyzer to emit a layer-4 reliability score.
- [ ] Add performance scoring (new, or extend quality analyzer).
- [ ] Add roadmap alignment scoring (new — requires roadmap context as input).
- [ ] Define what "low score" means: threshold, where low-score PRs go (low_value bucket).
- [ ] Keep score explanations short but specific.

### 5. Now vs future routing
- [ ] Layer 5: define what counts as current priority (CI green + active author + aligns with current roadmap phase).
- [ ] Define what counts as future priority (aligns with v1.5+ roadmap, or touches areas not yet active).
- [ ] Make future work visible without mixing it into the now queue.
- [ ] Preserve future items with enough detail to revisit later.
- [ ] Ensure "good, but later" is a distinct bucket outcome.

### 6. Deep judgment layers
- [ ] Layer 6 (confidence): flag PRs where the system's judgment is below 0.5. Respect HighRiskConfidenceCap=0.79 for risk claims.
- [ ] Layer 7 (dependency): detect PRs blocked on other PRs using existing graph data (internal/graph/).
- [ ] Layer 8 (blast radius): estimate from changed_files count, subsystem detection, and additions/deletions.
- [ ] Layer 9 (leverage): detect PRs that unblock other blocked PRs using reverse dependency lookup.
- [ ] Layer 10 (ownership): check author activity recency and review assignment status.
- [ ] Layer 11 (stability): check recent commit churn on the PR (last updated vs created, number of force pushes if available).
- [ ] Layer 12 (mergeability): use existing mergeable field + CI status + conflict detection from graph.
- [ ] Layer 13 (strategic weight): score alignment with ROADMAP current and future phases.
- [ ] Layer 14 (attention cost): estimate from PR size (additions + deletions), file count, and body length.
- [ ] Layer 15 (reversibility): heuristic from changed file paths — config/migration/schema = low reversibility, docs/tests = high.
- [ ] Layer 16 (signal quality): compare title/body coherence, presence of description, test file changes alongside source changes.
- [ ] Make the layer order explicit in the pipeline so the sequence stays understandable.

### 7. Report composition
- [ ] Executive summary section.
- [ ] Now section (temporal bucket: now).
- [ ] Future section (temporal bucket: future).
- [ ] Blocked section (temporal bucket: blocked).
- [ ] Duplicates section (disposal bucket: duplicate, with canonical chains).
- [ ] Junk/noise section (disposal bucket: junk).
- [ ] Stale section (disposal bucket: stale).
- [ ] Risk section (risk buckets: security_risk, reliability_risk, performance_risk).
- [ ] Full appendix covering every PR with bucket and reason code.
- [ ] Make sure every PR has at least one visible reason code.
- [ ] Make the output read like a decision map.

### 8. Validation
- [ ] Test: all PRs are accounted for (no PR in corpus missing from report).
- [ ] Test: duplicates are canonicalized (canonical has links, duplicates point back).
- [ ] Test: junk is separated early (junk PRs do not appear in substance scoring).
- [ ] Test: now/future routing works (PRs land in correct temporal bucket).
- [ ] Test: low-score PRs are excluded from active queue but not lost (appear in low_value or appendix).
- [ ] Test: blocked PRs are identified and linked to their dependency.
- [ ] Benchmark: 6,000+ PRs end-to-end through the full pipeline.
- [ ] Cross-check: docs, tests, and implementation use the same bucket names.
- [ ] Cross-check: layer ordering in code matches GUIDELINE.md ladder.

## Suggested execution order

1. Phase 0: fix the 5 test failures, get main green.
2. Lock the bucket vocabulary (done — see GUIDELINE.md).
3. Lock the layer definitions (done — see GUIDELINE.md).
4. Corpus coverage: remove hidden caps, add pagination, add 6k+ fixture.
5. Outer peel: layers 1–3 in order.
6. Substance scoring: layer 4.
7. Now vs future: layer 5.
8. Deep judgment: layers 6–16.
9. Report composition.
10. Validation at 6,000+ scale.

## Absorbed from pratc14a-remediation.md

The following items from the archived remediation plan are incorporated into v1.4 workstreams:
- perf-07 (ListPRs pagination) → workstream 2
- perf-09 (bootstrap streaming) → workstream 2
- slop-07 (planning/ dead code) → resolved: planning/ is now wired in Phase 0
- slop-09 (DefaultPoolCap magic number) → workstream 2

Remaining pratc14a items (perf-01 through perf-06, perf-08, sec-01 through sec-05, slop-01 through slop-06, slop-08, slop-10 through slop-16) are tracked in docs/archive/pratc14a-remediation.md and may be addressed opportunistically or in future versions.

## Notes
- This version values completeness over cheap shortcuts.
- Cost is allowed to be high.
- Hidden truncation is not allowed.
- The work is done when the full corpus can be explained cleanly.
