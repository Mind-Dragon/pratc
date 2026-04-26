# prATC TODO — Active Queue Toward 2.0

## Focus

Current iteration: finish the safe live-mutation path after Wave C wired `serve --live` to the central executor worker.

Active wave: **Wave D — live mutation hardening**.

The full roadmap lives in `PLANS.md` and `VERSION2.0.md`. Historical Wave A/B/C backlog detail was archived to `docs/archive/TODO-pre-wave-d-2026-04-26.md`.

## Source of truth

- `GUIDELINE.md` — action policy, lanes, bucket rules, non-negotiables
- `ARCHITECTURE.md` — system shape and component ownership
- `VERSION2.0.md` — product/release plan
- `PLANS.md` — current wave roadmap and status boundaries
- `TODO.md` — active implementation queue
- `AUTONOMOUS.md` and `autonomous/RUNBOOK.md` — autonomous controller mechanics
- `scripts/audit_guideline.py` — deterministic artifact audit gate

## Live status

- [x] Branch: `main`
- [x] Current implementation boundary: `2d2a36d4a897` — `feat: wire live executor worker`
- [x] Local branch is ahead of `origin/main`; do not push without explicit authorization
- [x] Wave A complete: ActionPlan, queue, proof bundle, TUI PR detail foundation
- [x] Wave B complete: guarded executor, preflight, ledger, verification, fake backend, sandbox, E2E harness
- [x] Wave C complete: `serve --live` flag, persisted executable intents, central `executor.Worker`
- [ ] Wave D active: circuit breaker, merge/close hardening, retry/backoff, sandbox E2E

## Guardrails

- Dry-run/advisory remains the default.
- Live GitHub writes require explicit `--live` and policy-approved `ActionIntent` records.
- No direct swarm-worker-to-GitHub mutation path.
- No merge/close without live preflight, idempotency key, ledger transition, and post-action verification.
- No runtime/proof success claim from stale or dirty `bin/pratc`; rebuild on clean HEAD first.
- No committed credentials, tokens, or token-derived logs.

## Completed boundary — Wave C worker-pool slice

- [x] Add `--live` flag to `serve` command.
- [x] Extend `runServer` signature to accept `live` parameter.
- [x] Link `ActionIntent.WorkItemID` to persisted queue work items.
- [x] Persist executable `ActionIntent` records in `internal/workqueue`.
- [x] Add `GetIntentsForWorkItem()`.
- [x] Make `LiveGitHubMutator` satisfy `executor.GitHubMutator` with dry-run-aware methods.
- [x] Add `executor.Worker` to claim work items and execute persisted intents through `Executor.ExecuteIntent`.
- [x] Wire `serve --live` to start the executor worker.
- [x] Verify focused tests, build, help output, and diff hygiene.

Verification used:

```bash
git diff --check
go test ./internal/cmd ./internal/executor ./internal/workqueue ./internal/app ./internal/types/...
make build
./bin/pratc serve --help | grep -- --live
```

## Active queue — Wave D live mutation hardening

### D0 — Status/doc sync and clean binary proof

- [x] Archive stale root `TODO.md` before replacing it with the active queue.
- [x] Update `PLANS.md` to mark Wave C complete and Wave D active.
- [x] Replace duplicated Wave C TODO sections with this active Wave D queue.
- [ ] Run verification bundle.
- [ ] Commit doc/status sync.
- [ ] After this doc-sync commit, rebuild `bin/pratc` on clean HEAD and verify `dirty=false`.

### D1 — Safety circuit breaker

Goal: live worker cannot execute unbounded GitHub mutations.

- [ ] Add failing tests for per-repo and global mutation limits.
- [ ] Implement fail-closed circuit breaker in `internal/executor` or worker config.
- [ ] Ensure dry-run/advisory paths are not blocked as live mutations.
- [ ] Record circuit-breaker denials in ledger/state transitions.
- [ ] Surface breaker status for API/TUI consumption.

Verification:

```bash
go test ./internal/executor ./internal/workqueue ./internal/cmd
```

### D2 — Merge action hardening

Goal: live merge uses intent payload, exact preconditions, and verified post-state.

- [ ] Add tests for merge method selection: `merge`, `squash`, `rebase`.
- [ ] Carry commit title/message and expected head SHA from intent payload.
- [ ] Preserve idempotency across retry or already-merged responses.
- [ ] Verify merged state after execution.
- [ ] Ensure failed verification returns queue item to safe failure/escalation state.

Verification:

```bash
go test ./internal/github ./internal/executor
```

### D3 — Close action hardening

Goal: duplicate/reject closure writes a reasoned comment and verifies closed state.

- [ ] Add tests for comment-before-close ordering.
- [ ] Require closure reason text from intent evidence/reasons.
- [ ] Verify closed state after execution.
- [ ] Verify expected comment presence when comment text is configured.
- [ ] Preserve dry-run zero-write behavior.

Verification:

```bash
go test ./internal/github ./internal/executor
```

### D4 — Retry/backoff for mutation calls

Goal: transient GitHub failures do not cause duplicate writes or unsafe retries.

- [ ] Add fake/HTTP-backed tests for 5xx retry behavior.
- [ ] Add rate-limit retry/denial tests for mutation endpoints.
- [ ] Reuse existing GitHub retry helpers where possible.
- [ ] Keep non-retryable 4xx errors fail-fast.
- [ ] Keep ledger entries clear about retry attempts and final outcome.

Verification:

```bash
go test ./internal/github ./internal/executor
```

### D5 — Ledger and queue transition hardening

Goal: every live attempt has an explainable transition trail.

- [ ] Audit current `executor_ledger` and workqueue transition coverage.
- [ ] Add missing tests for preflight denied, circuit denied, execution failed, verification failed, verified success.
- [ ] Ensure partial failure never leaves an item invisible or permanently claimed.
- [ ] Add queue stats needed by later TUI/API work.

Verification:

```bash
go test ./internal/cache ./internal/executor ./internal/workqueue
```

### D6 — Sandbox E2E preparation

Goal: prove the live path without touching production repos.

- [ ] Choose fake HTTP GitHub server first, sandbox repo second.
- [ ] Add E2E scenarios for dry-run zero-write and explicit live write.
- [ ] Include merge, close, comment, and label where feasible.
- [ ] Capture artifact/ledger proof for each scenario.
- [ ] Keep real PAT setup out of committed files.

Verification:

```bash
go test ./internal/e2e ./internal/github ./internal/executor
```

### D7 — Runbook and operator docs

Goal: operator commands match the actual `serve --live` worker path.

- [ ] Update `autonomous/RUNBOOK.md` Wave C/D commands away from stale `execute --live` examples unless that command exists.
- [ ] Document required env vars and token source behavior without exposing secrets.
- [ ] Document circuit-breaker recovery and hold/resume behavior.
- [ ] Update `VERSION2.0.md` with Wave C complete / Wave D active status.

Verification:

```bash
git diff --check
```

## Current verification bundle

Run before calling Wave D green:

```bash
git diff --check
gofmt -w <changed-go-files>
go test ./internal/github ./internal/executor ./internal/workqueue ./internal/cmd ./internal/app ./internal/types/...
make build
./bin/pratc version
./bin/pratc serve --help | grep -- --live
```

## Later waves

- Wave E: ledger API, queue stats API, TUI real mutation status, operator hold/resume controls.
- Wave F: sandbox repository lifecycle and real GitHub E2E gates.
- Wave G: runbook/API reference/release documentation.
- Wave H: metrics, profiling, concurrency tuning, dependency audit, final proof boundary.

## Not this iteration

- Browser dashboard revival.
- Multi-repo orchestration.
- Direct swarm-worker GitHub access.
- Pushing local `main` to origin without explicit authorization.
