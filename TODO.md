# prATC TODO — v1.7 Stabilization + Evidence Enrichment Finish

## Goal

Finish v1.7 as a bugfix / stabilization release with evidence enrichment as the product-facing payload.

v1.6.1 backlog surgery is done enough to stop carrying it as the active TODO surface. The remaining active work is:
1. finish the last v1.7 feature blocks in roadmap order
2. land the queued P1 reliability / API-contract fixes inside v1.7
3. leave the repo in a state that can be called debugged: green, race-clean, contract-tested, and docs-aligned

## Source of truth

- `ROADMAP.md` — release sequencing and the canonical v1.7 scope
- `GUIDELINE.md` — product rules and non-negotiable surfaces
- `ARCHITECTURE.md` — system shape and contracts
- `CHANGELOG.md` — shipped facts only
- `docs/audit-v1.6.1-synthesis.md` — multi-model audit findings
- `docs/iteration-plan-v1.6.1.md` — precision fix workflow that proved out in v1.6.1
- `internal/app/` — orchestration and analyzer output assembly
- `internal/cmd/` — HTTP/API contract surface
- `internal/github/` — auth, retry, rate-limit behavior
- `internal/cache/`, `internal/sync/`, `internal/settings/` — persistence and control-plane correctness
- `internal/report/` — operator packet / analyst packet output

## v1.7 release contract

v1.7 is done when all of these are true:

- [ ] Diff analysis emits real subsystem / risky-change evidence, not just metadata-derived hints
- [ ] Dependency impact emits API / shared-library / schema-change signals that can influence review output
- [ ] Test evidence flags production-code changes without corresponding test movement
- [ ] P1 HTTP/API contract issues are fixed and covered by focused tests
- [ ] P1 error propagation gaps are fixed; failures are surfaced instead of silently dropped
- [ ] P1 GitHub auth / retry issues are fixed and documented
- [ ] P1 settings / sync / cache consistency issues are fixed atomically
- [ ] `go test ./...`, `go test ./... -race`, `go vet ./...`, `go build ./cmd/pratc`, and Python tests are all green
- [ ] `ROADMAP.md`, `TODO.md`, and release docs describe the same reality

---

## Workstream 1 — Diff Analysis (first active v1.7 feature block)

### 1. Subsystem detection from diffs
- [ ] Identify the current entry point where raw diff evidence is assembled (`internal/app/`, `internal/review/`, `internal/github/`)
- [ ] Add deterministic subsystem tagging for changed paths (`security/`, `auth/`, `api/`, config, infra, tests)
- [ ] Emit subsystem evidence into the analyzer / review payload
- [ ] Add tests proving path-based subsystem detection on realistic PR fixtures

### 2. Risky pattern detection
- [ ] Detect auth-sensitive edits (permission checks, token handling, session logic)
- [ ] Detect data-safety edits (SQL, migrations, schema-affecting code)
- [ ] Detect crypto / secrets / credential-touching changes
- [ ] Ensure detections are additive evidence, not silent auto-reclassification
- [ ] Add focused fixtures for each risky pattern class

### 3. Diff evidence surface
- [ ] Add reviewer-facing diff evidence summary fields to response types if needed
- [ ] Ensure `analyze` JSON and PDF/operator surfaces can render the evidence cleanly
- [ ] Keep output deterministic and bounded for large PRs

---

## Workstream 2 — Dependency Impact (second active v1.7 feature block)

### 4. API / shared surface change detection
- [ ] Detect public API signature or contract file changes
- [ ] Detect shared-library or shared-module edits that likely impact downstream consumers
- [ ] Detect migration / config / schema changes requiring coordinated rollout
- [ ] Attach dependency-impact evidence to review results and plan reasoning

### 5. Verification
- [ ] Add tests showing API surface changes are detected from representative fixtures
- [ ] Add tests showing schema/config changes produce explicit rollout signals
- [ ] Verify false-positive rate stays tolerable on existing fixtures

---

## Workstream 3 — Test Evidence (third active v1.7 feature block)

### 6. Test movement detection
- [ ] Detect whether PRs modify production code, test code, or both
- [ ] Emit a `test evidence` signal into review output
- [ ] Flag production-code changes with no matching test changes
- [ ] Distinguish harmless docs/config-only edits from real missing-test cases

### 7. Coverage-impact estimation
- [ ] Estimate whether changed code appears covered vs untouched by tests in the diff
- [ ] Keep the heuristic simple and auditable; no opaque scoring layer
- [ ] Add fixture-backed tests for covered, partially covered, and untested changes

---

## Workstream 4 — P1 Reliability + API Contract Repair (fourth active v1.7 block)

### 8. Plan API parameter correctness
- [ ] Honor documented params: `exclude_conflicts`, `stale_score_threshold`, `candidate_pool_cap`, `score_min`
- [ ] Make `dry_run` behavior explicit and consistent between CLI and HTTP
- [ ] Add contract tests that fail on ignored params and pass once wired through

### 9. Error propagation and response integrity
- [ ] Stop swallowing review / per-PR errors in the analyze path
- [ ] Preserve structured headers/body on auth and rate-limit failures
- [ ] Ensure HTTP responses remain machine-readable on 401/429/error paths
- [ ] Add handler tests covering these cases

### 10. Settings API correctness
- [ ] Validate `scope` uniformly across GET/POST/DELETE/import/export paths
- [ ] Make settings behavior match `internal/cmd/AGENTS.md` or update docs to match reality
- [ ] Ensure import/export failures are atomic and properly surfaced

### 11. GitHub auth / retry correctness
- [ ] Fix `ResolveTokenForLogin` semantics so named account selection is real
- [ ] Tighten `IsRetryableError` logic to avoid fragile string-only behavior
- [ ] Ensure REST fallback honors token rotation / retry policy
- [ ] Keep transient backoff inside documented caps

### 12. Sync / cache / scheduler consistency
- [ ] Re-audit sync job transitions after the atomic fixes already landed
- [ ] Verify paused/resume state bookkeeping stays consistent through scheduler paths
- [ ] Add regression tests for resume / pause / import edge cases that previously caused drift

---

## Workstream 5 — Release hardening

### 13. Contract reconciliation
- [ ] Re-read `ROADMAP.md`, `GUIDELINE.md`, `ARCHITECTURE.md`, `internal/cmd/AGENTS.md`, and active tests after each major merge
- [ ] Remove stale statements that still describe pre-fix behavior
- [ ] Keep `TODO.md` as the active execution ledger only

### 14. Final debug sweep
- [ ] Run full green suite: Go, Python, race, vet, build
- [ ] Repeat a fresh final codebase examination after the last v1.7 fix lands
- [ ] List any residual deferred work explicitly instead of leaving it implicit
- [ ] Write the final release-ready remediation note if anything remains out of scope

---

## Execution order

1. Diff Analysis
2. Dependency Impact
3. Test Evidence
4. P1 Reliability + API Contract Repair
5. Release hardening / final debug sweep

## Rules for execution

- [ ] Every bugfix gets a failing contract test before the code change
- [ ] Every merge keeps `main` green
- [ ] Prefer one-purpose commits; no grab-bag edits
- [ ] Use `go test ./... -race` whenever concurrency, sync, or cache code changes
- [ ] Do not reopen v1.6.1 scope unless a v1.7 task proves it was not actually complete

## Exit note

v1.7 should finish as the release that makes prATC trustworthy under pressure:
- richer evidence
- fewer silent failures
- tighter HTTP/API behavior
- cleaner auth/retry handling
- a codebase that can plausibly be called debugged rather than merely working on happy paths
