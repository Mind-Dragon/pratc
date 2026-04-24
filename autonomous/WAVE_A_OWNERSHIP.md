# Wave A Ownership Map — 1.7.1 to 2.0 Iteration A

## Controller contract

- Controller owns docs, TODO state, integration, verification, tmux/runtime proof, and commits.
- Implementation workers do not commit.
- All subagents run through Hermes CLI pinned to Nacrof `kimi-k2.6` using `HERMES_HOME=/tmp/hermes-nacrof-test` unless the controller refreshes that temp home from `~/.hermes`.
- Canary proof required before worker launch: session model `kimi-k2.6`, base URL `https://crof.ai/v1`, repo access checked (`KIMI_NACROF_K26_REPO_CANARY_OK`). `kimi-k2.6-FCED` was not present in `/models` and returned provider 500/empty assistant responses in `/home/agent/pratc`.
- Advisory/dry-run only. No GitHub writes, no `gh pr comment`, no labels, no close/merge/push.
- TDD required: write failing tests first, run and capture RED, implement, run GREEN, then targeted package tests.
- Worker logs and prompts live under `.swarm/wave-a/` and are advisory until controller verification passes.

## First-wave cap

Maximum 4 implementation workers. Do not start Wave B until controller runs the Wave A barrier bundle green.

## Worker A1 — ActionIntent completeness + audit

Owned files:
- `internal/types/models.go`
- `internal/types/*_test.go`
- `internal/actions/*`
- `internal/app/actions.go`
- `internal/app/actions_test.go`
- `scripts/audit_guideline.py`
- `tests/test_audit_guideline.py`
- `fixtures/action-plan.json` only if the contract changes
- `ml-service/src/pratc_ml/models.py` and `web/src/types/api.ts` only if JSON parity changes

Do not edit:
- `internal/workqueue/*`
- `internal/cache/*`
- `internal/cmd/*` except read-only inspection
- `internal/monitor/*`

Acceptance:
- Every generated ActionIntent has reasons, evidence refs, confidence, policy profile, preconditions, idempotency key, and dry-run/write classification.
- Audit fails on missing intent completeness fields.
- Advisory mode remains zero-write.

Verification:
```bash
go test ./internal/types ./internal/actions ./internal/app -count=1
python -m pytest -q tests/test_audit_guideline.py
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > /tmp/pratc-wave-a1-action-plan.json
```

## Worker A2 — Swarm queue API foundation

Owned files:
- `internal/workqueue/*`
- `internal/workqueue/*_test.go`
- `internal/cache/*` only for queue persistence/migrations required by this slice
- `internal/cmd/actions_api_test.go`
- `internal/cmd/serve.go` or route files if endpoints live there

Do not edit:
- `internal/actions/*` classifier/policy logic
- `internal/app/actions.go` except the smallest adapter needed by HTTP routes, coordinated with controller
- `internal/executor/*`
- `internal/monitor/*`

Acceptance:
- Authenticated APIs can claim, release, heartbeat, and inspect queue status.
- Filters: lane, priority, state, expired leases.
- Race-safe tests prove disjoint claims and expired lease return.

Verification:
```bash
go test ./internal/workqueue ./internal/cache ./internal/cmd -count=1
```

## Worker A3 — Proof bundle attach path

Owned files:
- `internal/workqueue/*` proof association helpers not owned by A2 at the same time
- `internal/executor/*` proof validation/dry-run interfaces
- `internal/cache/*` only for proof bundle persistence/migrations required by this slice
- `internal/cmd/actions_api_test.go`
- `internal/cmd/serve.go` or route files if endpoints live there

Coordination rule:
- If A2 is active in `internal/workqueue/*` or `internal/cache/*`, A3 must start after A2 lands or must restrict itself to tests/spec review only.

Acceptance:
- Proof attach validates work item id, worker id, live lease ownership, artifact refs, command result, and proof status.
- Wrong owner, stale lease, invalid item, and duplicate/idempotent attach are tested.

Verification:
```bash
go test ./internal/workqueue ./internal/cache ./internal/cmd ./internal/executor -count=1
```

## Worker A4 — TUI PR detail inspector

Owned files:
- `internal/monitor/data/*`
- `internal/monitor/tui/*`
- `internal/monitor/**/*_test.go`

Do not edit:
- `internal/actions/*`
- `internal/workqueue/*`
- `internal/cache/*`
- `internal/cmd/*` except read-only inspection of `monitor`

Acceptance:
- Monitor has a PR detail data model with title, author, age, status, lane, bucket, confidence, reasons, decision layers, evidence refs, duplicate/synthesis refs, risk flags, and allowed actions.
- There is a testable render/snapshot path independent of the interactive terminal.

Verification:
```bash
go test ./internal/monitor/... -count=1
./bin/pratc monitor --once || true
```

## Barrier bundle after Wave A

Controller runs:
```bash
git diff --check
make build
GOCACHE=$PWD/.cache/go-build GOMODCACHE=$PWD/.cache/go-mod go test ./internal/types ./internal/actions ./internal/app ./internal/cache ./internal/workqueue ./internal/cmd ./internal/executor ./internal/monitor/... -count=1
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
RUN_DIR=autonomous/runs/wave-a-$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$RUN_DIR"
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > "$RUN_DIR/action-plan.json"
python3 scripts/audit_guideline.py "$RUN_DIR"
```

If any barrier fails, controller opens a targeted fix task and does not advance to Wave B.
