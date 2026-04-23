# prATC TODO — Post-v1.7 / v1.8 Preparation

## Goal

Clean up the v1.7 release artifacts and prepare the foundation for v1.8 (Multi-Repo + ML Feedback).

v1.7 is code-complete with all 5 workstreams done. The next batch addresses:
1. Release hygiene that was deferred during the v1.7 sprint
2. Scaling debt called out in ARCHITECTURE.md
3. Doc/codebase reconciliation across stale references
4. Foundation work for v1.8's multi-repo and GitHub App features

## Source of truth

- `ROADMAP.md` — v1.8 scope (Multi-Repo + ML Feedback + GitHub App)
- `GUIDELINE.md` — product rules (still references v1.6.0, needs update)
- `ARCHITECTURE.md` — system shape (service contract stale, scaling constraints documented)
- `CHANGELOG.md` — v1.7 not yet recorded
- `docs/pratc-v1.7-release-remediation-note.md` — v1.7 closeout status

---

## Wave 1 — Release Hygiene (do first)

### 1. CHANGELOG.md for v1.7

Record what actually shipped in v1.7:

- [x] Add v1.7 entry to CHANGELOG.md covering:
  - WS1: Diff analysis evidence (subsystem detection, risky patterns)
  - WS2: Dependency impact (API surface, shared module, schema changes)
  - WS3: Test evidence (test movement, coverage estimation)
  - WS4: PlanOptions contract widening, review failure semantics, token source unification
  - WS5: Release hardening
- [x] Include test counts and new contract surface

Files: `CHANGELOG.md`

### 2. Doc version references

Update stale version references across docs:

- [x] GUIDELINE.md line 5: "v1.6.0 full-corpus triage engine" → v1.7
- [x] ARCHITECTURE.md line 4: "v1.6.0 full-corpus triage engine" → v1.7
- [x] ARCHITECTURE.md line 214: `Plan(ctx, repo, target, mode)` → `PlanWithOptions(ctx, repo, PlanOptions)`
- [x] ARCHITECTURE.md line 214: add `PlanOptions` struct to service contract
- [x] internal/cmd/AGENTS.md: verify plan query params match what's now wired
- [x] internal/AGENTS.md: update "github/ — rate limit retry missing jitter" if jitter now exists

Files: `GUIDELINE.md`, `ARCHITECTURE.md`, `internal/cmd/AGENTS.md`, `internal/AGENTS.md`

### 3. AGENTS.md regeneration

The root AGENTS.md says "Generated: 2026-03-23" — over a month stale:

- [x] Regenerate root AGENTS.md with current codebase facts
- [x] Verify package LOC counts, command surface, and cross-cutting patterns match reality
- [x] Update the "Anti-Patterns" section if any v1.7 changes affect them

Files: `AGENTS.md`

---

## Wave 2 — Scaling Debt (ARCHITECTURE.md calls these out)

### 4. Legacy pool-cap constants

ARCHITECTURE.md line 166: "Legacy pool-cap constants still exist in `internal/types/`, but `BuildCandidatePool()` does not enforce them"

- [x] Audit `DefaultCandidatePoolCap` (100) and `DefaultPoolCap` (64) in `internal/types/`
- [x] Determine if anything still references these constants
- [x] Remove or deprecate with clear documentation if unused
- [x] Add a test proving the constants don't silently affect behavior

Files: `internal/types/models.go`, possibly `internal/app/service.go`

### 5. maxPRs cap audit

ARCHITECTURE.md line 163: "maxPRs cap of 5,000 applied to the overnight openclaw/openclaw run — full corpus is ~6,632 PRs"

- [x] Find where the 5,000 cap is applied (likely in sync or service config)
- [x] Determine if it should be configurable via CLI/API
- [x] Document the current behavior and recommended override
- [ ] Add a warning when the cap truncates the corpus

Audit notes: see `docs/maxprs-cap-audit.md`. Current finding: the cap is caller-supplied (`--max-prs` / `--sync-max-prs`), with truncation metadata emitted by `internal/app/service.go`; settings validation accepts `max_prs`, but the runtime does not yet wire that value into analyze/plan/graph/workflow/serve paths.

Files: `internal/sync/`, `internal/app/service.go`, `internal/cmd/`

### 6. Conflict pairs scaling

ARCHITECTURE.md line 164: "Conflict pairs at 38,884 after noise filtering — still above the 5,000 target"

- [x] Profile the conflict detection path on the full 6.6k PR corpus
- [x] Evaluate raising the shared-file minimum from 2 to 3
- [x] Evaluate expanding the noise file list further
- [x] Add a benchmark for `buildConflicts()` at corpus scale
- [ ] Target: conflict pairs below 5,000 on openclaw/openclaw

Files: `internal/app/service.go`, `internal/graph/`, `internal/types/noise_files.go`

---

## Wave 3 — v1.8 Foundation

### 7. Multi-repo data model design

ROADMAP.md v1.8 calls for multi-repo analysis. The current schema is single-repo:

- [x] Design a `repositories` table schema (id, owner, name, last_sync, status)
- [x] Design cross-repo dependency detection approach (shared files, import paths, API surface overlap)
- [x] Design unified merge plan output (per-repo sections + cross-repo conflict detection)
- [x] Write the design doc in `docs/plans/`
- [x] Do NOT implement yet — design only
  - Complete: `docs/plans/multi-repo-data-model.md` written and audited; merged recommendations added from the GPT 5.4 audit.

Files: `docs/plans/` (new), `internal/cache/schema.go` (read-only audit)

### 8. GitHub App integration prep

ROADMAP.md v1.8 calls for OAuth, webhooks, status checks:

- [x] Audit current auth flow: `ResolveToken`, `ResolveTokenForLogin`, `DiscoverTokens`
- [x] Design OAuth device-flow integration for GitHub App
- [x] Design webhook receiver endpoint (what events, what triggers)
- [x] Design status check API (what gets posted back to PRs)
- [x] Write the design doc in `docs/plans/`
- [x] Do NOT implement yet — design only
  - Complete: `docs/plans/github-app-integration.md` written and audited; merged recommendations added from the z.ai 5.1 audit.

Files: `docs/plans/` (new), `internal/github/auth.go` (read-only audit)

### 9. ML feedback loop design

ROADMAP.md v1.8 calls for operator decisions as training signals:

- [x] Design how operator overrides (bucket changes, rejections) get captured
- [x] Design the feedback format (JSONL? SQLite table?)
- [x] Design how feedback feeds back into scoring/duplicate detection
- [x] Design privacy boundaries (what gets sent to ML service, what stays local)
- [x] Write the design doc in `docs/plans/`
- [x] Do NOT implement yet — design only
  - Complete: `docs/plans/ml-feedback-loop.md` written; quick review added and the doc is ready for a deeper external audit.

Files: `docs/plans/` (new), `internal/review/` (read-only audit)

---

## Execution rules

- [ ] Every bugfix gets a failing contract test before the code change
- [ ] Every merge keeps `main` green
- [ ] Prefer one-purpose commits; no grab-bag edits
- [ ] Use `go test ./... -race` whenever concurrency, sync, or cache code changes
- [ ] Do not expand scope beyond what's listed here without explicit approval

## Exit criteria

- Wave 1: CHANGELOG updated, all doc version references current, AGENTS.md regenerated
- Wave 2: Legacy constants cleaned up, maxPRs behavior documented, conflict pairs profiled
- Wave 3: Three design docs written and reviewed (multi-repo, GitHub App, ML feedback)
- All tests green: `go test ./...`, `go test ./... -race`, `go vet ./...`, Python tests
