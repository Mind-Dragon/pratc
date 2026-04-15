# prATC v1.4 — Full-Corpus Triage Engine

This is the working understanding of v1.4. It is a synthesis of the roadmap, guideline, architecture, and TODO docs so the milestone reads like one system instead of four overlapping notes.

If this file ever conflicts with GUIDELINE.md on bucket names, layer order, or non-negotiables, GUIDELINE.md wins.

## What v1.4 means

- Every open PR in the repo is accounted for.
- The system peels away noise in layers instead of flattening everything into one score.
- Every decision carries a reason trail.
- The system distinguishes now, future, and blocked work.
- No PR disappears silently.
- No auto-merge, auto-approve, or write-back behavior is allowed.
- The report should read like a decision map, not a flat list.

## Locked vocabulary

Temporal buckets:
- now
- future
- blocked

Quality buckets:
- high_value
- merge_candidate
- needs_review
- re_engage
- low_value

Disposal buckets:
- duplicate
- junk
- stale

Risk buckets:
- security_risk
- reliability_risk
- performance_risk

The bucket system is intentionally larger than the old v1.3 review categories. Historical v1.3 names may still appear in ROADMAP.md as Phase 0 context, but the active v1.4 operating model is the one above.

## What is already in place

- Planning integration is wired through the main app path.
- The review output sorts queue work by the current priority model.
- Cache listing now supports caller-visible paged/streaming access.
- Bootstrap sync can stream directly into the cache store.
- The docs now have a dedicated milestone summary and a single architecture document.

These are foundation pieces. They are not the remaining v1.4 finish line.

## What still needs work

- Remove or make explicit the remaining corpus caps.
- Make candidate pool limits configurable or eliminate the hard gate.
- Prove ingest and sync behavior at 6,000+ PR scale.
- Formalize the outer-peel layers so garbage, duplicates, and obvious badness are visibly reasoned outcomes.
- Complete the deeper judgment layers and their reasons.
- Finish the corpus-scale report and validation pass.

## Next development pass order

1. Corpus coverage and cap removal
2. Large-corpus proof and scale validation
3. Outer peel formalization
4. Substance scoring and routing
5. Deep judgment layers
6. Report composition and validation

## Relationship to other docs

- ROADMAP.md says what v1.4 is and the order of work.
- GUIDELINE.md says what the system is allowed to do.
- ARCHITECTURE.md says how the system is shaped.
- TODO.md says what is still left to build.

This file exists so the milestone can be read quickly without losing the underlying contracts.
