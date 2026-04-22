# prATC v1.7 Release-Ready Remediation Note

## What landed in this autonomous run

### WS2 — Dependency Impact
- Added deterministic dependency-impact evidence over changed files
- Detects:
  - public API surface changes
  - shared module changes
  - schema / migration rollout impact
  - config rollout impact
- Wired through analyzer findings so the evidence reaches JSON/report surfaces

### WS3 — Test Evidence
- Added positive test-evidence signals when production code and tests move together
- Added partial-coverage heuristics when test movement appears too small relative to production changes
- Kept existing test-gap detection for missing tests

### WS4 — P1 reliability fixes landed
- 401 auth responses now use structured machine-readable error output
- settings scope is validated uniformly across GET/POST/DELETE/export/import
- `ResolveTokenForLogin` no longer silently returns the active account token for a different requested login
- transient backoff is now capped within the documented 30s ceiling
- sync/cache/scheduler paths were revalidated against the green suite

### WS5 — hardening
- full suite re-run successfully
- race run re-run successfully
- vet/build/Python all green
- TODO and roadmap state reconciled

## Residual deferred items

The remaining unchecked v1.7 items are not simple local fixes. They require contract-level design choices:

1. Plan API options are still not truly honored end-to-end
   - `exclude_conflicts`
   - `candidate_pool_cap`
   - `score_min`
   - `stale_score_threshold`
   - explicit `dry_run` semantics at the service layer

   Why deferred:
   - `app.Service.Plan(ctx, repo, target, mode)` does not accept option structs
   - the downstream planner path does not expose enough hooks to apply these safely from HTTP only
   - a truthful fix requires widening the service contract and associated tests

2. Review error propagation still degrades to partial success
   - `Analyze()` / `buildReviewPayload()` currently prefer partial output over hard failure

   Why deferred:
   - changing this now is a user-visible behavior change to CLI/API semantics
   - needs an explicit product decision: partial analysis vs fail-fast

3. REST fallback token rotation is not fully wired through multi-token client state
   - current client behavior is green, but the full fallback/token-source architecture is still narrower than the audit ideal

   Why deferred:
   - requires client-level token-source plumbing rather than a local patch

## Current truth

The repo is green and stable, but **not every audited P1 ambition is fully closed**.

Current machine-checkable state:
- `go test ./...` green
- `go test ./... -race` green
- `go vet ./...` green
- `go build ./cmd/pratc` green
- Python tests green

## Recommended next move

If v1.7 must ship immediately, it can ship as:
- WS1/WS2/WS3 complete
- WS4 partially complete with the above deferred items explicitly documented
- WS5 complete

If you want true 100% P1 closeout, the next work item should be a dedicated contract-widening pass for:
- `PlanOptions` flowing from HTTP/CLI into `app.Service.Plan`
- an explicit product decision on review failure semantics
- multi-token REST fallback architecture in `internal/github`
