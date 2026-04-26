# prATC Development Roadmap — Version Plan

> Canonical forward plan for prATC (Pipeline for Automated Triaging & Corrections).
> Live file: `PLANS.md` at repo root.
> Last verified: Wave D closeout gate from local implementation boundary after `b6925e6a` doc sync.

---

## Current Status

**Product line:** prATC `1.7.1` → `2.0-dev` action engine
**Branch:** `main`, local-only work ahead of `origin/main`
**Current implementation boundary:** local Wave D closeout patch on top of `b6925e6a` — live mutation hardening
**Wave status:** Wave A complete, Wave B complete, Wave C worker-pool slice complete, Wave D implementation complete pending final commit
**Safety posture:** dry-run/advisory remains default; live GitHub writes require explicit `serve --live`, policy approval, preflight, idempotency, ledger, and verification
**Verified before final Wave D docs:** focused executor/github/workqueue tests pass; final full gate listed below
**Runtime note:** `bin/pratc` is ignored build output. Rebuild on a clean commit before using binary version as runtime proof.

---

## Document Hierarchy

1. `GUIDELINE.md` owns action policy, lanes, bucket rules, and non-negotiables.
2. `ARCHITECTURE.md` owns system shape, data flow, and component ownership.
3. `VERSION2.0.md` owns the v2.0 product/release plan.
4. `PLANS.md` owns the current wave roadmap and status boundaries.
5. `TODO.md` owns the active implementation queue.
6. `AUTONOMOUS.md` and `autonomous/RUNBOOK.md` own controller mechanics and operator commands.

If documents conflict on safety or mutation policy, `GUIDELINE.md` wins.

---

## Architecture Overview

```
GitHub corpus
    ↓
Sync/cache layer              internal/sync/, internal/github/, internal/cache/
    ↓
Analysis/review pipeline      internal/analysis/, internal/filter/, internal/review/, internal/planning/
    ↓
Action engine                 internal/actions/, internal/app/actions.go
    ↓
ActionPlan + work queue       internal/workqueue/, action_intents, proof bundles
    ↓
Central executor              internal/executor/
    ↓
GitHub mutations              only when explicit --live + policy + preflight + ledger + verification pass
```

Key components:

- **Analyzer**: reads cached/live PR corpus and produces explainable review data.
- **Planner/action engine**: assigns action lanes and emits typed `ActionIntent` records.
- **WorkQueue**: persists `ActionWorkItem`, executable `ActionIntent`, leases, proof bundles, and state transitions.
- **Executor**: centralizes all GitHub mutation logic behind preflight and verification.
- **LiveGitHubMutator**: wraps `github.Client` for real GitHub reads/writes while retaining dry-run controls.
- **Serve**: HTTP API and optional live worker host.
- **TUI/monitor**: operator view into lanes, queue, executor, proof, and audit state.

---

## Version History

### v0.1–v0.5 — Foundation

| Version | Features | Status |
|---------|----------|--------|
| 0.1 | Repo scaffolding, Go CLI skeleton | Complete |
| 0.2 | Work queue and state machine | Complete |
| 0.3 | Analyzer and doc/code reconciliation seed | Complete |
| 0.4 | Planner, diff generation, task breakdown | Complete |
| 0.5 | Executor scaffolding, provider routing, dry-run logging | Complete |

### v1.x — Advisory triage engine

| Feature | Description | Status |
|---------|-------------|--------|
| Serve command | `pratc serve` API server with health, metrics, repo routes | Complete |
| TUI | terminal dashboard for triage/control surfaces | Complete |
| Full-corpus workflow | sync → analyze → cluster → graph → plan → report | Complete |
| ActionPlan advisory surface | schema `2.0` action lanes, work items, intents | Complete |
| Runtime proof baseline | OpenClaw full-corpus run, audit-green, PDF artifact | Complete |

---

## v2.0 — Wave-Based Release Plan

Release strategy: incremental waves, each adding one safe layer. Mutations default to dry-run/advisory; live operations require explicit opt-in and gates.

### Wave A — ActionPlan + queue foundation — COMPLETE

Delivered:

- `ActionIntent` completeness fields: reasons, evidence refs, confidence, policy profile, preconditions, idempotency key, dry-run/write classification.
- Queue API: claim, release, heartbeat, status, filters.
- Proof bundle attach/store path.
- TUI PR detail inspector.
- OpenClaw ActionPlan proof remained audit-green.

### Wave B — Guarded executor foundation — COMPLETE

Delivered:

- TUI queue/proof/executor/audit panels.
- Live preflight checker gates.
- Guarded comment/label executor with fake backend.
- SQLite executor ledger persistence.
- Post-action verification helpers.
- Fix-and-merge sandbox proof path.
- E2E harness and audit expansion.
- Wave B docs/runbook sync.

### Wave C — Live flag + central worker — COMPLETE

Implementation boundary: `2d2a36d4a897`.

Delivered:

- Added `--live` flag to `serve`.
- Extended `runServer(..., live bool)` and logged live mode.
- Added `ActionIntent.WorkItemID` across Go/Python/TypeScript parity surfaces.
- Persisted executable `action_intents` in `internal/workqueue`.
- Added `GetIntentsForWorkItem()`.
- Made `LiveGitHubMutator` satisfy `executor.GitHubMutator` with dry-run-aware methods.
- Added `executor.Worker` to claim queue items, load persisted intents, and execute through `Executor.ExecuteIntent`.
- Wired `serve --live` to spawn the central executor worker.

Verified:

```bash
git diff --check
go test ./internal/cmd ./internal/executor ./internal/workqueue ./internal/app ./internal/types/...
make build
./bin/pratc serve --help | grep -- --live
```

### Wave D — Live mutation hardening — COMPLETE

Goal: make the first live mutation path safe enough for fake/sandbox E2E, without broadening direct swarm permissions.

Delivered:

1. **Safety circuit breaker**
   - enforce max concurrent live mutations per repo and globally
   - fail closed when limits are exceeded
   - expose in-process status for later operator/TUI surfaces
2. **Merge action hardening**
   - carry merge method, commit title/message, expected SHA, and idempotency key from intent payload
   - test squash/rebase/merge strategy routing against fake and HTTP-backed clients
   - verify merged GitHub state after live execution
3. **Close action hardening**
   - require reason/comment text for duplicate/reject closures
   - create comment before close when configured
   - verify closed GitHub state and comment presence where applicable
4. **Retry/backoff for live mutations**
   - reuse existing GitHub transient/rate-limit behavior where possible
   - add mutation-specific tests for 5xx/rate-limit retry boundaries
   - preserve idempotency across retries
5. **Ledger/state transition hardening**
   - record preflight, execution, verification, failure, and circuit-breaker denials
   - keep queue transitions safe on partial failure
6. **Fake/sandbox E2E preparation**
   - use FakeGitHub first; disposable repo matrix deferred to Wave F
   - prove dry-run remains zero-write
   - prove live mode writes only after explicit gate acceptance
7. **Operator/runbook sync**
   - document `serve --live` worker path
   - document `PRATC_LIVE_MAX_GLOBAL`, `PRATC_LIVE_MAX_PER_REPO`, `PRATC_QUEUE_DB_PATH`, and GitHub token source behavior
   - document hold/recovery through stopping `serve --live` and restarting with safer breaker limits

Exit gate:

```bash
git diff --check
gofmt -w <changed-go-files>
go test ./internal/github ./internal/executor ./internal/workqueue ./internal/cmd
make build
./bin/pratc serve --help | grep -- --live
```

### Wave E — Ledger, API, and UI integration — NEXT

- API endpoints for circuit-breaker status, ledger, and queue stats.
- TUI real mutation status and circuit-breaker state.
- Operator hold/resume controls.
- Notification hooks only after ledger semantics are stable.

### Wave F — Sandbox and E2E tests — NEXT

- Sandbox repo lifecycle.
- Dry-run → live mutation test path.
- Merge/close/comment/label scenarios.
- Chaos tests for GitHub 5xx, rate-limit, stale SHA, and verification failure.

### Wave G — Documentation and runbook — NEXT

- Update `autonomous/RUNBOOK.md` with actual `serve --live` worker commands.
- Document circuit breaker recovery.
- Update `VERSION2.0.md` release notes.
- Add/refresh API reference for queue/ledger endpoints.

### Wave H — Release hardening — NEXT

- Worker pool/concurrency tuning.
- Prometheus/pprof metrics.
- Dependency and credential audit.
- Final version/proof boundary.

---

## Provider Status

Hermes provider config was refreshed during the Wave C prep pass. Runtime provider health is external to this repo and should be checked live before relying on it for implementation swarms.

Known stable rule:

- Use env-sourced provider keys, not committed secrets.
- Do not reintroduce RunPod as an assumed provider while balance is zero.
- Keep provider availability probes separate from prATC source status.

---

## Change Log

| Date | Version | Change |
|------|---------|--------|
| 2026-04-26 | 2.0-dev Wave D | Added live mutation circuit breaker, merge/close/retry hardening, failure ledger transitions, and fake dry-run/live E2E proof |
| 2026-04-26 | 2.0-dev Wave C | Added `--live` flag, persisted executable intents, and wired `serve --live` to central `executor.Worker` |
| 2026-04-26 | 2.0-dev Wave B | Completed guarded executor foundation: preflight, fake guarded actions, ledger, verification, sandbox, E2E harness |
| 2026-04-26 | 2.0-dev Wave A | Completed ActionPlan, queue, proof, and TUI foundation |
| 2026-04-24 | 1.7.1 | Verified advisory baseline on OpenClaw full-corpus run |

---

## Contact

Maintained by Nous Research / Hermes Agent session.
Issues: `github.com/jeffersonnunn/pratc`
