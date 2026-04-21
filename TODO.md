# prATC TODO — v1.6.1 Backlog Surgery

## Goal

Expand the merge plan surface beyond the current top-20 ceiling by recursively collapsing duplicate groups, automatically reclassifying blocked and low_value PRs through deterministic second passes, and scaling the planner target dynamically with corpus health.

v1.6.1 is a surgical release: it does not change the 16-gate funnel, the diff-grounded evidence layer, or the API contract. It adds three post-review passes that unlock hidden value in the two largest buckets (blocked: 50.3%, low_value: 42.6%).

## Source of truth

- `GUIDELINE.md` — non-negotiable product rules and the mandatory 16-gate funnel contract
- `ARCHITECTURE.md` — system shape, surfaces, and long-running pipeline model
- `ROADMAP.md` — release sequencing
- `CHANGELOG.md` — what has actually shipped
- `docs/plans/2026-04-21-pratc-v1.6.1-backlog-surgery-plan.md` — this release's detailed plan
- `internal/app/` — pipeline orchestration
- `internal/review/` — gate logic, second-pass reclassifiers, decision outputs
- `internal/planner/` — merge plan target calculation and candidate selection
- `internal/report/` — PDF output contract

## v1.6.1 contract

v1.6.1 is done when all of these are true:

- [ ] Recursive duplicate expansion collapses all duplicate/overlap groups to canonical representatives before planning
- [ ] Flattened chains (A→B→C) resolve to a single canonical with full superseded list
- [ ] Blocked PRs (3,334 in openclaw) receive an automatic second pass; recoverable PRs are reclassified into viable buckets
- [ ] Low_value PRs (2,824 in openclaw) receive an automatic second pass; quick wins and hidden high-value PRs are promoted
- [ ] Stale/abandoned PRs in blocked/low_value are downgraded to junk automatically
- [ ] Merge plan target is dynamic: `clamp(viable_pool * 0.05, min=20, max=100)`
- [ ] Report PDF shows collapse impact, reclassification arrows, recovery queue, and quick-win batch tags
- [ ] All changes are deterministic, auditable, and covered by focused tests

## Workstream 1 — Recursive Duplicate Expansion

### 1. Collapse duplicate groups before planning
- [ ] Implement `CollapsedCorpus` builder in `internal/app/duplicate_synthesis.go`
- [ ] Map each canonical PR to its full superseded list
- [ ] Detect and flatten chains (A→B→C becomes canonical A with [B, C] superseded)
- [ ] Ensure no PR is both canonical and superseded in the final collapsed corpus
- [ ] Replace superseded PRs in the planning pool with canonical representatives
- [ ] Mark canonical PRs with `IsCollapsedCanonical = true`

### 2. Planner integration
- [ ] Add `plan --collapse-duplicates` flag (default: true in v1.6.1)
- [ ] Planner runs on collapsed corpus: `len(PRs) - len(superseded)` candidates
- [ ] Target auto-adjusts when corpus shrinks

### 3. Report surface
- [ ] New PDF section: "Duplicate Collapse Impact"
- [ ] Show original PR count → collapsed PR count, groups collapsed, plan slots freed
- [ ] List top canonical PRs nominated

## Workstream 2 — Automatic Second Pass: Blocked PR Reclassification

### 4. Second pass architecture
- [ ] Add `RunSecondPass()` to `internal/review/orchestrator.go` (called after main review)
- [ ] Collect PRs where `Category == blocked`
- [ ] Implement `ReclassifyBlocked(prData, firstPassResult)` in `internal/review/recovery.go`
- [ ] Update `ReviewResult` fields on reclassification:
  - `Category` — needs_review | high_value | merge_candidate | junk | blocked
  - `DecisionLayers` — append Gate 17: Recovery Assessment
  - `ReclassifiedFrom` — original category string
  - `ReclassificationReason` — human-readable path forward
  - `NextAction` — updated action

### 5. Recovery rules (deterministic)
- [ ] CI failing + last push <30d + not draft + <3 conflicts → needs_review
- [ ] Draft + ≥2 reviews/activity + not stale → high_value
- [ ] Mergeable=unknown + small size + no risk flags → needs_review
- [ ] CI failing + last push >90d + no reviews → junk
- [ ] >10 conflict pairs + stale → junk
- [ ] Spam/junk markers → junk
- [ ] All others → blocked (permanent_blocker reason)

### 6. Report surface
- [ ] Decision Trail shows reclassified PRs: `#5649 [blocked → needs_review]`
- [ ] New PDF subsection: "Reclassified from Blocked" with count and top examples

## Workstream 3 — Automatic Second Pass: Low_Value PR Reclassification

### 7. Second pass architecture
- [ ] Extend `RunSecondPass()` to handle `Category == low_value`
- [ ] Implement `ReclassifyLowValue(prData, firstPassResult)` in `internal/review/quickwin.go`
- [ ] Update `ReviewResult` fields on reclassification (same pattern as blocked)
- [ ] Append Gate 17: Value Reassessment to `DecisionLayers`

### 8. Re-rank rules (deterministic)
- [ ] size_XS/S + CI passing + not draft + <3 conflicts + substance >30 → merge_candidate
- [ ] Docs cluster + CI passing + no conflicts + substance >25 → high_value
- [ ] Security/reliability/perf findings + substance >40 → needs_review
- [ ] No activity >90d + CI failing + no reviews + substance <20 → junk
- [ ] All others → low_value (genuine low_value)

### 9. Batch-merge tagging
- [ ] Add `BatchTag` field to `ReviewResult` for promoted PRs
- [ ] Derive tags from cluster or file patterns: `docs-batch`, `typo-batch`, `dependency-batch`
- [ ] Planner optionally batch-selects tagged PRs

### 10. Report surface
- [ ] Decision Trail shows reclassified PRs: `#10195 [low_value → merge_candidate]`
- [ ] New PDF subsection: "Reclassified from Low Value" with count and examples
- [ ] Batch tags rendered where applicable

## Workstream 4 — Expanded Merge Plan Target

### 11. Dynamic target calculation
- [ ] Replace hardcoded target-20 with dynamic formula
- [ ] `target = clamp(viable_pool * 0.05, min_target, max_target)`
- [ ] Default `min_target = 20`, `max_target = 100`
- [ ] `viable_pool` = non-junk, non-abandoned PRs after all passes

### 12. CLI flags
- [ ] `--target-ratio float` — % of viable pool (default: 0.05)
- [ ] `--min-target int` — minimum target (default: 20)
- [ ] `--max-target int` — maximum target (default: 100)

### 13. Planner behavior
- [ ] Calculate target before candidate selection
- [ ] Report shows: "Target: X PRs (Y% of viable pool Z)"
- [ ] Runtime does not degrade when target increases

## Workstream 5 — Operator Packet Report Enhancements

### 14. PDF sections
- [ ] "Duplicate Collapse Impact" — original vs collapsed counts, slots freed
- [ ] "Reclassified from Blocked" — count, arrows, reasons
- [ ] "Reclassified from Low Value" — count, arrows, batch tags
- [ ] "Expanded Plan Summary" — target formula, pool breakdown
- [ ] "Before / After" PR counts — original corpus → collapsed → viable pool

### 15. Report integrity
- [ ] All new sections fit within 30-page limit
- [ ] Report generation stays under 30 seconds
- [ ] No overflow or truncation on 6,632-PR corpus

## Immediate execution order

1. Workstream 1: Recursive duplicate expansion
2. Workstream 4: Dynamic target calculation (depends on 1 for corpus shrink)
3. Workstream 2: Blocked second pass (adds recoverable PRs to viable pool)
4. Workstream 3: Low_value second pass (adds quick wins to viable pool)
5. Workstream 5: Report enhancements (surfaces all passes in PDF)

## Non-goals for v1.6.1

- [ ] No changes to the 16-gate funnel semantics
- [ ] No changes to diff-grounded evidence layer
- [ ] No changes to API response schemas (only additive fields)
- [ ] No GitHub mutations or auto-merge behavior
- [ ] No ML models or probabilistic classification
- [ ] No dashboard or web surface work

## Exit note

v1.6.1 should leave prATC with a larger actionable merge queue:
- fewer duplicate PRs competing for plan slots
- blocked PRs with paths forward re-entering the planning pool
- low_value PRs with hidden value surfaced and batch-tagged
- a planner target that scales with corpus health
- a PDF that tells the full before/after story
