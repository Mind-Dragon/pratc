# prATC Analyst PDF — Audit TODO

Generated: 2026-04-17 from visual/text audit of openclaw/openclaw report (441 pages, 1.4MB)

## Status: Report produces a data dump, not an analyst packet

---

## P0 — Classification & Data Integrity

These break the GUIDELINE.md non-negotiables.

- [ ] **Fix temporal bucketing.** All 6,646 PRs show as `blocked inspect_now` in the Decision Trail. Review Buckets say "now: 5142, future: 664, duplicate: 0, junk: 0, blocked: 0" — but the trail contradicts this. Temporal buckets (now/future/blocked) must be mutually exclusive and visible per PR.
- [ ] **Populate junk/spam section.** Junk PRs page shows 0 rows across 6,646 PRs. This is impossible. Classifier should catch spam bots, dependency bumps, promotional PRs, malformed entries. The `ClassifyProblematicPR` helper exists in `internal/review/` — wire it into the report dataset.
- [ ] **Populate duplicate section.** Duplicate Detail page is empty. Analyze step detected 68 overlap groups but 0 duplicate groups. The duplicate threshold (0.90) or the ML bridge output may not be feeding into the report pipeline.
- [ ] **Surface confidence scores.** GUIDELINE requires 0.0–1.0 confidence per PR, with 0.79 cap for high-risk. No confidence visible anywhere in the report. Add `confidence` field to decision trail rows.

## P1 — Decision Trail Depth

The GUIDELINE defines a 16-layer decision ladder. Only 3 layers are visible.

- [ ] **Expand decision layers shown.** Current: "L1 Garbage | L2 Duplicates: ..." — need to surface layers 3–16: substance score, confidence, dependency, blast radius, leverage, ownership, stability, mergeability, strategic weight, attention cost, reversibility, signal quality.
- [ ] **Differentiate reasons per PR.** The boilerplate "majority agreement among 2 analyzers; CI passing/failing; not draft" is identical for 90%+ of PRs. Reasons should reflect the actual classification path — why this PR is `merge_candidate` vs `needs_review` vs `blocked`.
- [ ] **Add next-action labels.** GUIDELINE skill recommends mapping: merge_now→merge, duplicate_superseded→duplicate, problematic_quarantine+spam→close, problematic_quarantine→quarantine, unknown_escalate→escalate. Not visible in current output.

## P2 — Report Structure & Length

441 pages is not an analyst packet.

- [ ] **Cap Decision Trail at ~50 high-signal rows.** Move the full 6,646-row table to a condensed appendix. Top of report should show actionable PRs grouped by category, not a flat list.
- [ ] **Group PRs by classification in the trail.** Don't dump all 6,646 in PR-number order. Group by: merge_candidate → needs_review → blocked → future → stale → junk → duplicate. Each group gets a header and count.
- [ ] **Add per-category summary cards.** Before each group, show: count, top 5 examples (PR#, title, score), aggregate risk flags. Like a "decision map" not a "dump."
- [ ] **Remove empty pages.** Pages 5 (Junk PRs) and 6 (Duplicate Detail) render empty table headers with 0 rows. Either populate them or skip them.
- [ ] **Fix cover page timestamp mismatch.** Cover page shows `Fri, 17 Apr 2026 17:55:50 CEST` (report generation time). All other pages show `Thu, 16 Apr 2026 21:59:55 UTC` (analyze time). Use consistent timestamp.

## P3 — Visual Design

Current state: Arial only, dark blue bars, colored metric boxes on page 3. Everything else is plain text tables.

- [ ] **Upgrade typography.** Add a distinctive font stack: display font for section headers (e.g., Space Grotesk or DM Sans), monospace for data/PR numbers (JetBrains Mono), body font for prose (Inter). Currently all Arial.
- [ ] **Add category badges.** Each PR row should have a colored badge: green for merge_candidate, yellow for needs_review, red for blocked/junk, blue for future. Visual scanning beats reading text labels.
- [ ] **Add bucket distribution bar chart.** Show a horizontal stacked bar of now/future/blocked/duplicate/junk/stale with percentages. Makes the corpus shape obvious at a glance.
- [ ] **Add risk flag icons.** Security (shield), reliability (warning), performance (gauge) — visual indicators on risky PRs instead of text-only risk buckets.
- [ ] **Improve metrics dashboard spacing.** Page 3 is the best page but boxes are cramped. Increase gap between metric boxes, add subtle shadows, round corners.
- [ ] **Add section divider pages.** Between major sections (exec summary → junk → duplicates → action items → trail → appendix), add a colored divider with section title.
- [ ] **Generate actual charts.** Current ChartsSection renders placeholder rectangles. Implement at least: PR volume over time, cluster size distribution, merge vs review vs block ratio.

## P4 — Recommendations Section

- [ ] **Wire AnalystRecommendationsSection.** The code exists in `analyst_sections.go:642` and `report.go:122` loads it, but the last page of the PDF is mid-PR-list, not a recommendations page. Debug why recommendations aren't rendering.
- [ ] **Generate actionable recommendations.** Should include: "Merge these 5 PRs now," "Close these 10 spam PRs," "Re-engage these 3 authors," "Block these 2 security risks." Currently nothing.

## P5 — Dataset Scope Transparency

- [ ] **State cache staleness prominently.** Report covers 100 PRs from stale cache, not 6,646 live PRs. The analyze.json has 6,646 entries but cluster/graph/plan used `--force-cache` with only 100 PRs. This mismatch should be called out on page 2.
- [ ] **Show analysis coverage percentage.** "Analyzed 6,646 of 6,646 PRs (100%)" is correct for analyze, but cluster/graph/plan only cover 100 PRs. Show both numbers.

## P6 — Code Fixes Required

- [ ] `internal/report/analyst_sections.go` — `LoadAnalystDataset` needs to pull confidence, problem_type, decision_layers from analyze.json review_payload.results
- [ ] `internal/report/analyst_sections.go` — `DecisionTrailSection.Render()` needs pagination cap, grouping by classification, color-coded rows
- [ ] `internal/report/analyst_sections.go` — `FullPRTableSection` needs grouping or moved to appendix with condensed format
- [ ] `internal/report/pdf.go` — `CoverSection.Render()` timestamp should use analyze.json generatedAt, not time.Now()
- [ ] `internal/report/pdf.go` — `ChartsSection` needs real chart rendering (bar/pie via fpdf chart primitives or go-chart)
- [ ] `internal/cmd/report.go` — Debug why AnalystRecommendationsSection loads but doesn't appear in output
- [ ] `internal/review/orchestrator.go` — Ensure ClassifyProblematicPR results are written to analyze.json review_payload

## Verification Checklist

After fixes, regenerate and verify:
- [ ] PDF is ≤50 pages for the main report (appendix may be longer)
- [ ] Junk PRs section has >0 rows
- [ ] Duplicate section has >0 rows or is cleanly omitted with note
- [ ] Every PR in Decision Trail has a differentiated reason
- [ ] Confidence scores visible on high-risk PRs
- [ ] Temporal buckets (now/future/blocked) are correctly assigned
- [ ] No empty pages
- [ ] Recommendations page exists with actionable items
- [ ] Visual scan: can a human operator read the report in <10 min and know what to do next
