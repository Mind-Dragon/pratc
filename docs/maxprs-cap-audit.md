# maxPRs cap audit

Date: 2026-04-23

## Summary

There is no hidden hardcoded 5,000 PR cap in the current runtime.

The 5,000 cap seen in the overnight openclaw/openclaw run came from operator-facing command wiring and runbook usage, not from an unconditional constant in `internal/app/service.go` or `internal/sync/`.

## Where the cap is actually applied

### 1. Analysis/runtime truncation

File: `internal/app/service.go`

`Service.applyIntakeControls()` applies the cap after loading PRs:

- optional PR-number windowing runs first
- then `s.maxPRs` truncates the slice with `output = output[:effectiveMaxPRs]`
- metadata is emitted:
  - `analysis_truncated: true`
  - `truncation_reason: "max_prs_cap"`
  - `max_prs_applied: <value>`

This affects analyze/cluster/graph/plan code paths because they all load PRs through `loadPRs()`.

### 2. Initial sync fetch limiting

File: `internal/sync/default_runner.go`

`githubMetadataSource.SyncRepo()` derives an `effectiveMax` from the rate-limit chunk size and `g.maxPRs`, then sets:

- `opts.PerPage = effectiveMax`
- `opts.MaxPRs = effectiveMax`
- `opts.SnapshotCeiling = effectiveMax`

So sync can also be capped, but only when a non-zero sync max is explicitly passed into the runner.

### 3. GitHub client enforcement

Files:
- `internal/github/client.go`
- `internal/github/rest.go`

Both GraphQL and REST fetch paths honor `PullRequestListOptions.MaxPRs` and stop once the configured maximum is reached.

## Where the 5,000 value comes from

The concrete 5,000 value is documented and exemplified in operator tooling, especially:

- `autonomous/RUNBOOK.md`
  - `pratc analyze --repo openclaw/openclaw --force-cache --max-prs 5000`
  - `pratc graph --repo openclaw/openclaw --force-cache --max-prs 5000`
  - `pratc plan --repo openclaw/openclaw --force-cache --max-prs 5000`
- `internal/cmd/workflow.go`
  - workflow example uses `--max-prs=5000`

So the architecture note about a 5,000-cap run is consistent with the runbooked invocation pattern.

## Is it configurable today?

Yes, but unevenly.

### Configurable today via CLI flags

- `pratc analyze --max-prs` (default `0` = no cap)
- `pratc plan --max-prs` (default `0` = no cap)
- `pratc graph --max-prs` (default `0` = no cap)
- `pratc workflow --max-prs` (default `0` = no cap)
- `pratc sync --sync-max-prs` (default `0` = no cap)

### Not effectively configurable via settings/runtime policy

The settings layer already accepts `max_prs`:

- `internal/settings/validator.go` allows `max_prs`
- settings API tests cover reading/writing `max_prs`

But the app/CLI runtime does not currently load `max_prs` from settings into `app.Config` for analyze/graph/plan/workflow/serve. In practice:

- settings can store `max_prs`
- current runtime does not consume it for these paths
- HTTP/API server paths also do not expose a dedicated max-prs query parameter

## Current behavior to document

1. Default behavior is uncapped (`0` means no cap).
2. When a cap is set, truncation is explicit in the response payload.
3. Sync-time capping and analysis-time capping are separate controls.
4. A 5,000 cap is currently a caller choice/runbook convention, not a hidden system-wide default.

## Recommendation

Keep `maxPRs` configurable, but make the configuration surface consistent:

1. preserve CLI overrides
2. wire `max_prs` settings into `app.Config` for non-CLI/server flows
3. document the distinction between:
   - `--sync-max-prs` for ingestion
   - `--max-prs` for analysis/planning corpus size
4. remove or update any operational runbooks that still recommend 5,000 for corpora that should be processed fully

For the openclaw/openclaw corpus specifically, the simplest immediate fix is to stop passing `--max-prs 5000` unless the truncation is intentional.
