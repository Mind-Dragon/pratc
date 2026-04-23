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

### WS4 — P1 reliability + API contract repair (substantially completed)
- 401 auth responses now use structured machine-readable error output
- settings scope is validated uniformly across GET/POST/DELETE/export/import
- `ResolveTokenForLogin` no longer silently returns the active account token for a different requested login
- transient backoff is now capped within the documented 30s ceiling
- sync/cache/scheduler paths were revalidated against the green suite at the time of that pass
- PlanOptions widened: exclude_conflicts, candidate_pool_cap, score_min, stale_score_threshold,
  dry_run, collapse_duplicates now honored end-to-end from HTTP/CLI through service layer
- Review failure semantics made explicit: per-PR errors captured in FailedPRs, top-level
  failures return degraded response with Partial=true and error messages
- TokenSource interface unifies GraphQL and REST token acquisition
- IsRetryableError tightened: generic 403 without rate-limit context is no longer retryable

### WS5 — hardening
- hardening work landed during the original remediation pass
- this note no longer claims the full suite is currently revalidated from the present repo state
- TODO and roadmap state were reconciled in that earlier pass

## Residual deferred items

This note should not claim that all P1 work is closed without re-checking the current tree.
At the time this note was corrected, additional follow-up verification/work was still required
before making a release-ready or ship-ready statement for v1.7.

## Current truth

The remediation work above landed, but this document must not overclaim current release status.
Use live test results from the current branch state as the source of truth.

Examples of claims this note intentionally does not make anymore:
- that `go test ./...` is green right now
- that `go test ./... -race` is green right now
- that all audited P1 ambitions are definitively closed in the current tree
- that v1.7 is ready to ship without additional verification

## Recommended next move

Re-run the current verification suite and reconcile any remaining RED tests or contract gaps
before declaring v1.7 release-ready.
