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
- [x] Wave D complete: circuit breaker, merge/close hardening, retry/backoff, ledger/queue hardening, fake dry-run/live E2E

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

## Completed queue — Wave D live mutation hardening

### D0 — Status/doc sync and clean binary proof

- [x] Archive stale root `TODO.md` before replacing it with the active queue.
- [x] Update `PLANS.md` to mark Wave C complete and Wave D active.
- [x] Replace duplicated Wave C TODO sections with this active Wave D queue.
- [x] Run verification bundle.
- [x] Commit doc/status sync (`b6925e6`).
- [x] Rebuild `bin/pratc` on clean doc-sync HEAD and verify `dirty=false` before Wave D coding.

### D1 — Safety circuit breaker

Goal: live worker cannot execute unbounded GitHub mutations.

- [x] Add failing tests for per-repo and global mutation limits.
- [x] Implement fail-closed circuit breaker in `internal/executor`/worker config.
- [x] Ensure dry-run/advisory paths are not blocked as live mutations.
- [x] Record circuit-breaker denials in ledger/state transitions.
- [x] Surface breaker status through `MutationCircuitBreaker.Status()`; API/TUI endpoint wiring remains Wave E.

Verification:

```bash
go test ./internal/executor ./internal/workqueue ./internal/cmd
```

### D2 — Merge action hardening

Goal: live merge uses intent payload, exact preconditions, and verified post-state.

- [x] Add tests for merge method selection/payload routing and unsupported method rejection.
- [x] Carry commit title/message and expected head SHA from intent payload.
- [x] Preserve idempotency across retry or already-merged responses.
- [x] Verify merged state after execution.
- [x] Ensure failed execution/verification returns queue item to safe failed state with cleared lease.

Verification:

```bash
go test ./internal/github ./internal/executor
```

### D3 — Close action hardening

Goal: duplicate/reject closure writes a reasoned comment and verifies closed state.

- [x] Add tests for comment-before-close ordering.
- [x] Require closure reason or comment text from intent evidence/reasons.
- [x] Verify closed state after execution.
- [x] Verify expected comment presence when comment text is configured.
- [x] Preserve dry-run zero-write behavior.

Verification:

```bash
go test ./internal/github ./internal/executor
```

### D4 — Retry/backoff for mutation calls

Goal: transient GitHub failures do not cause duplicate writes or unsafe retries.

- [x] Add HTTP-backed tests for 5xx retry behavior.
- [x] Reuse existing rate-limit retry/denial machinery; mutation-specific non-retryable 4xx covered.
- [x] Reuse existing GitHub retry/backoff helpers where possible.
- [x] Keep non-retryable 4xx errors fail-fast.
- [x] Keep ledger entries clear about final outcome; per-attempt retry telemetry remains log-level.

Verification:

```bash
go test ./internal/github ./internal/executor
```

### D5 — Ledger and queue transition hardening

Goal: every live attempt has an explainable transition trail.

- [x] Audit current `executor_ledger` and workqueue transition coverage.
- [x] Add missing tests for circuit denied, execution failed, verification failed, verified success, and preflight failure.
- [x] Ensure partial failure never leaves an item invisible or permanently claimed.
- [x] Existing queue stats/ledger readers retained; circuit status API/TUI wiring deferred to Wave E.

Verification:

```bash
go test ./internal/cache ./internal/executor ./internal/workqueue
```

### D6 — Sandbox E2E preparation

Goal: prove the live path without touching production repos.

- [x] Choose fake/FakeGitHub path first, sandbox repo second.
- [x] Add E2E scenarios for dry-run zero-write and explicit live fake write.
- [x] Include comment live/dry-run E2E; merge/close covered by focused hardening tests, label remains Wave F matrix.
- [x] Capture ledger/queue proof for each scenario.
- [x] Keep real PAT setup out of committed files.

Verification:

```bash
go test ./internal/e2e ./internal/github ./internal/executor
```

### D7 — Runbook and operator docs

Goal: operator commands match the actual `serve --live` worker path.

- [x] Update `autonomous/RUNBOOK.md` Wave C/D commands away from stale `execute --live` examples.
- [x] Document required env vars and token source behavior without exposing secrets.
- [x] Document circuit-breaker recovery and hold/resume behavior.
- [x] Update `VERSION2.0.md`/`PLANS.md` with Wave D completion status.

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

- Wave E: circuit breaker API/TUI status, queue stats API, TUI real mutation status, operator hold/resume controls.
- Wave F: sandbox repository lifecycle and real GitHub E2E gates.
- Wave G: runbook/API reference/release documentation.
- Wave H: metrics, profiling, concurrency tuning, dependency audit, final proof boundary.

## Not this iteration

- Browser dashboard revival.
- Multi-repo orchestration.
- Direct swarm-worker GitHub access.
- Pushing local `main` to origin without explicit authorization.
