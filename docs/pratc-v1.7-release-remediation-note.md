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

### WS4 — P1 reliability + API contract repair (completed)
- 401 auth responses now use structured machine-readable error output
- settings scope is validated uniformly across GET/POST/DELETE/export/import
- `ResolveTokenForLogin` no longer silently returns the active account token for a different requested login
- transient backoff is now capped within the documented 30s ceiling
- sync/cache/scheduler paths were revalidated against the green suite
- PlanOptions widened: exclude_conflicts, candidate_pool_cap, score_min, stale_score_threshold,
  dry_run, collapse_duplicates now honored end-to-end from HTTP/CLI through service layer
- Review failure semantics made explicit: per-PR errors captured in FailedPRs, top-level
  failures return degraded response with Partial=true and error messages
- TokenSource interface unifies GraphQL and REST token acquisition
- IsRetryableError tightened: generic 403 without rate-limit context is no longer retryable

### WS5 — hardening
- full suite re-run successfully
- race run re-run successfully
- vet/build/Python all green
- TODO and roadmap state reconciled

## Residual deferred items

All previously deferred P1 items have been resolved in this pass:

1. ~~Plan API options~~ — RESOLVED: PlanOptions struct flows from HTTP/CLI through service layer
2. ~~Review error propagation~~ — RESOLVED: Option 2 (partial-success with explicit degradation) implemented
3. ~~REST fallback token rotation~~ — RESOLVED: TokenSource interface unifies GraphQL and REST

No P1 items remain deferred for v1.7.

## Current truth

The repo is green and stable. All audited P1 ambitions are closed.

Current machine-checkable state:
- `go test ./...` green
- `go test ./... -race` green
- `go vet ./...` green
- `go build ./cmd/pratc` green
- Python tests green

## Recommended next move

v1.7 is ready to ship:
- WS1/WS2/WS3 complete
- WS4 complete (all P1 contract issues resolved)
- WS5 complete

Next: see ROADMAP.md for post-v1.7 priorities.
