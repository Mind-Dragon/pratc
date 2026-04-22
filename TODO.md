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

- [x] Diff analysis emits real subsystem / risky-change evidence, not just metadata-derived hints
- [x] Dependency impact emits API / shared-library / schema-change signals that can influence review output
- [x] Test evidence flags production-code changes without corresponding test movement
- [ ] P1 HTTP/API contract issues are fully fixed and covered by focused tests
- [ ] P1 error propagation gaps are fully fixed; failures are surfaced instead of silently dropped
- [x] P1 GitHub auth / retry issues are partially fixed and documented (`ResolveTokenForLogin`, backoff cap, 401 JSON integrity)
- [x] P1 settings / sync / cache consistency issues already landed or were revalidated in this pass
- [x] `go test ./...`, `go test ./... -race`, `go vet ./...`, `go build ./cmd/pratc`, and Python tests are all green
- [x] `ROADMAP.md`, `TODO.md`, and release docs describe the same reality

### Concrete blocker for full WS4 closeout

The remaining unchecked P1 items all require widening core service/client contracts rather than local additive fixes:
- `app.Service.Plan(ctx, repo, target, mode)` does not accept plan-behavior options, so `exclude_conflicts`, `candidate_pool_cap`, `score_min`, and `stale_score_threshold` cannot be truthfully honored from HTTP without changing the service contract and downstream planner path.
- `Analyze()` and `buildReviewPayload()` currently degrade to partial success on review failures. Converting that to hard error propagation is a user-visible contract change that needs an explicit product decision.
- REST fallback token rotation requires client-level multi-token plumbing in the GitHub client rather than a local handler fix.

---

## Workstream 1 — Diff Analysis (first active v1.7 feature block)

### 1. Current wired path confirmed
- [x] Verified live diff path: `internal/repo/mirror.go:GetDiffPatch()` → `parseDiffOutput()` → `internal/app/service.go` diff population → `types.PRAnalysisData`
- [x] Verified only `internal/review/analyzer_security.go` currently uses diff-grounded evidence in production
- [x] Verified `ReviewResult.AnalyzerFindings` is the lowest-blast-radius evidence surface
- [x] Verified report gap: analyst/report path does not strongly surface analyzer findings today
- [x] Detailed plan written: `docs/plans/2026-04-22-pratc-v1.7-ws1-diff-analysis-plan.md`

### 2. Subsystem detection from diffs
- [x] Add contract tests locking the current diff evidence path (`internal/repo/`, `internal/app/`)
- [x] Add additive subsystem evidence fields in `internal/types/models.go`
- [x] Implement deterministic path-based subsystem classifier in `internal/review/diff_subsystems.go`
- [x] Emit subsystem evidence into `ReviewResult.AnalyzerFindings`
- [x] Add tests proving subsystem detection on realistic PR fixtures

### 3. Risky pattern detection
- [x] Implement diff-content detectors in `internal/review/diff_patterns.go`
- [x] Detect auth-sensitive edits (permission checks, token handling, session logic)
- [x] Detect data-safety edits (SQL, migrations, schema-affecting code)
- [x] Detect crypto / secrets / credential-touching changes
- [x] Ensure detections are additive evidence, not silent auto-reclassification
- [x] Add focused fixtures for each risky pattern class

### 4. Diff evidence surface
- [x] Wire subsystem / risky-pattern findings through the non-security analyzers
- [x] Surface findings into analyst/report paths via `internal/report/analyst_sections.go`
- [x] Add reviewer-facing bounded diff evidence summaries to JSON/PDF outputs
- [x] Keep output deterministic and bounded for large PRs
- [x] Reuse analyzer findings inside duplicate-synthesis output if useful

---

## Workstream 2 — Dependency Impact (second active v1.7 feature block)

### 4. API / shared surface change detection
- [x] Detect public API signature or contract file changes
- [x] Detect shared-library or shared-module edits that likely impact downstream consumers
- [x] Detect migration / config / schema changes requiring coordinated rollout
- [x] Attach dependency-impact evidence to review results and plan reasoning

### 5. Verification
- [x] Add tests showing API surface changes are detected from representative fixtures
- [x] Add tests showing schema/config changes produce explicit rollout signals
- [x] Verify false-positive rate stays tolerable on existing fixtures

---

## Workstream 3 — Test Evidence (third active v1.7 feature block)

### 6. Test movement detection
- [x] Detect whether PRs modify production code, test code, or both
- [x] Emit a `test evidence` signal into review output
- [x] Flag production-code changes with no matching test changes
- [x] Distinguish harmless docs/config-only edits from real missing-test cases

### 7. Coverage-impact estimation
- [x] Estimate whether changed code appears covered vs untouched by tests in the diff
- [x] Keep the heuristic simple and auditable; no opaque scoring layer
- [x] Add fixture-backed tests for covered, partially covered, and untested changes

---

## Workstream 4 — P1 Reliability + API Contract Repair (fourth active v1.7 block)

### 8. Plan API parameter correctness
- [ ] Honor documented params: `exclude_conflicts`, `stale_score_threshold`, `candidate_pool_cap`, `score_min`
- [ ] Make `dry_run` behavior explicit and consistent between CLI and HTTP
- [ ] Add contract tests that fail on ignored params and pass once wired through

### 9. Error propagation and response integrity
- [ ] Stop swallowing review / per-PR errors in the analyze path
- [x] Preserve structured headers/body on auth and rate-limit failures
- [x] Ensure HTTP responses remain machine-readable on 401/429/error paths
- [x] Add handler tests covering these cases

### 10. Settings API correctness
- [x] Validate `scope` uniformly across GET/POST/DELETE/import/export paths
- [x] Make settings behavior match `internal/cmd/AGENTS.md` or update docs to match reality
- [x] Ensure import/export failures are atomic and properly surfaced

### 11. GitHub auth / retry correctness
- [x] Fix `ResolveTokenForLogin` semantics so named account selection is real
- [ ] Tighten `IsRetryableError` logic to avoid fragile string-only behavior
- [ ] Ensure REST fallback honors token rotation / retry policy
- [x] Keep transient backoff inside documented caps

### 12. Sync / cache / scheduler consistency
- [x] Re-audit sync job transitions after the atomic fixes already landed
- [x] Verify paused/resume state bookkeeping stays consistent through scheduler paths
- [x] Add regression tests for resume / pause / import edge cases that previously caused drift

---

## Workstream 5 — Release hardening

### 13. Contract reconciliation
- [x] Re-read `ROADMAP.md`, `GUIDELINE.md`, `ARCHITECTURE.md`, `internal/cmd/AGENTS.md`, and active tests after each major merge
- [x] Remove stale statements that still describe pre-fix behavior
- [x] Keep `TODO.md` as the active execution ledger only

### 14. Final debug sweep
- [x] Run full green suite: Go, Python, race, vet, build
- [x] Repeat a fresh final codebase examination after the last v1.7 fix lands
- [x] List any residual deferred work explicitly instead of leaving it implicit
- [x] Write the final release-ready remediation note if anything remains out of scope

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
