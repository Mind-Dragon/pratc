# prATC TODO — v1.6 Pipeline-First Reset

## Goal

Turn prATC into a pipeline-first system with three primary surfaces only:
- CLI for humans
- API for AI systems
- PDF report for human decision-makers

The web dashboard is no longer part of the product direction. v1.6 is about making the pipeline sharper, cheaper, more explainable, and more durable for a future 24/7 daemon that continuously ingests and processes PRs.

## Source of truth

- `GUIDELINE.md` — non-negotiable product rules and the mandatory 16-gate funnel contract
- `ARCHITECTURE.md` — system shape, surfaces, and long-running pipeline model
- `ROADMAP.md` — release sequencing
- `CHANGELOG.md` — what has actually shipped
- `README.md` — current public release surface
- `internal/app/` — pipeline orchestration
- `internal/review/` — gate logic, analyzers, synthesis, and decision outputs
- `internal/cmd/serve.go` — AI-facing API surface
- `internal/report/` — PDF output contract

## v1.6 contract

v1.6 is done when all of these are true:

- [x] The product surface is CLI + API + PDF only; the web dashboard is removed or explicitly stubbed as non-product
- [x] All primary docs describe prATC as an AI-centric pipeline with human PDF output, not a dashboard product
- [x] Every non-garbage PR passes through the same 16 gates in order, with gate outputs recorded explicitly
- [x] The 16 gates are ordered by cheap outer peel → more expensive inner judgment, and the code matches the guideline
- [x] Diff-grounded evidence exists in the funnel for at least the first high-value analysis slice
- [x] Duplicate handling advances from detection into synthesis planning: nominate canonical PRs and define how to derive a merged candidate from related PR sets
- [ ] API responses are optimized for machine consumption: explicit fields, stable schemas, self-describing reasoning, no dashboard-shaped assumptions
- [x] PDF output remains first-class and reflects the strengthened funnel truthfully

## Workstream 1 — Product surface reset

### 1. Remove dashboard as product surface
- [x] Identify all code, docs, tests, and build paths that treat `web/` as a first-class product surface
- [x] Decide final v1.6 treatment for `web/`: remove entirely, quarantine, or leave a clearly marked stub
- [x] Remove dashboard-first language from README / ARCHITECTURE / CONTRIBUTING / AGENTS / docs
- [x] Remove or rewrite dashboard docs (`docs/dashboard-user-guide.md`, `docs/ui-wiring.md`, monitor docs as needed)
- [x] Keep `serve` as an AI-facing API server, not a backing server for a browser dashboard
- [x] Verify the repo can be understood and operated without any `web/` assumptions

### 2. Make the surfaces explicit
- [x] CLI surface = concise commands for humans
- [x] API surface = explicit machine-facing JSON for agents
- [x] PDF surface = final human handoff artifact
- [x] Audit all docs and contracts to ensure these three surfaces are the only promoted interfaces

## Workstream 2 — Strengthen the 16-gate funnel

### 3. Make the funnel contract mandatory
- [x] Rewrite the guideline/architecture language so every PR follows the same funnel journey
- [x] Define each gate as a mandatory stage, not a loose conceptual ladder
- [x] Record for every PR: gate entered, gate outcome, reason, cost tier, and whether the PR continues inward or exits at that gate
- [x] Ensure the outer peel removes broad junk fast and the inner gates spend more CPU only on survivors

### 4. Gate ordering and semantics
- [x] Reconcile current code with the intended onion model
- [x] Separate elimination gates from scoring/judgment gates
- [x] Make early gates cheap and deterministic
- [x] Make later gates richer and more expensive only when justified by surviving signal
- [x] Ensure duplicates, junk, spam, and obvious badness exit early but remain fully visible in output

### 5. Funnel truthfulness in output
- [x] Expose gate-by-gate journey in API output for every PR
- [ ] Reflect gate journey clearly in the PDF report
- [x] Ensure no PR “teleports” to a final state without an explicit gate trail

## Workstream 3 — Diff-grounded evidence in v1.6

### 6. Bring real diff evidence into the funnel
- [x] Add a first diff-aware evidence slice instead of relying only on metadata/path heuristics
- [x] Define which gates consume diff hunks and which must remain metadata-only
- [x] Start with high-value patterns: auth, secrets, dangerous config, risky query/perf patterns, test-gap evidence
- [x] Attach code location / diff evidence to findings so AI consumers and the PDF can explain judgments concretely

### 7. Keep cost discipline
- [x] Add explicit policy for which PRs get deeper diff analysis and when
- [x] Keep the outer funnel cheap enough for large corpora
- [x] Ensure inner diff analysis only runs on PRs that survive outer layers

## Workstream 4 — Duplicate synthesis beyond detection

### 8. Canonicalization and synthesis planning
- [x] Move duplicate handling from “same idea detected” to “best candidate nominated”
- [x] For each duplicate/near-duplicate group, identify canonical, alternates, and synthesis candidates
- [x] Define a bot-ready output contract for a future merge/synthesis agent
- [x] Specify what “best of N PRs” means: quality, completeness, freshness, conflict footprint, evidence quality, tests, mergeability

### 9. Candidate merge-by-bot plan
- [x] Design the artifact that a future bot can consume to create a synthetic best-of-group PR
- [x] Include inputs, ranking, exclusions, and explicit human-readable reasons
- [x] Keep v1.6 advisory-only: nominate and describe, do not mutate GitHub

## Workstream 5 — API and report tightening

### 10. AI-centric API
- [ ] Audit `serve` endpoints for machine readability and consistency
- [ ] Remove browser/dashboard assumptions from endpoint naming and docs
- [ ] Promote stable machine-facing fields for gate journey, evidence, canonicalization, and report artifacts
- [ ] Decide whether a first-class `/review` style endpoint should exist in v1.6 or whether `analyze` remains the canonical machine entrypoint

### 11. PDF as final human artifact
- [ ] Keep PDF mandatory in the main workflow
- [x] Ensure the report explains gate journey, canonical duplicate groups, and high-signal evidence clearly
- [x] Make the report read like a decision packet, not a dashboard export

## Immediate execution order

1. Product surface reset: remove/stub dashboard and clean docs
2. Funnel contract rewrite: every PR passes the same 16 gates in order
3. Diff-grounded evidence slice: bring real patch evidence into v1.6
4. Duplicate synthesis planning: canonical + future merge-by-bot contract

## Non-goals for v1.6

- [ ] No web dashboard rebuild
- [ ] No GitHub mutations or auto-merge behavior
- [ ] No live bot that comments/closes/bans users yet
- [ ] No attempt to build the full 24/7 daemon in the same slice as the funnel reset

## Exit note

v1.6 should leave prATC in a simpler state than it started:
- fewer surfaces
- clearer funnel semantics
- stronger machine-readable output
- better PDF packet
- a real path to 2.0 continuous operation
