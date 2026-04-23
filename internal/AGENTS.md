# AGENTS.md — internal/

Go backend packages. Parent AGENTS.md covers top-level contracts; this covers package internals.

## Package Index

| Package | LOC | Purpose | Status |
|---------|-----|---------|--------|
| `app/` | 3284 | Service facade: Analyze, Cluster, Graph, Plan | Production |
| `planner/` | 489 | Core planner with functional options | Production |
| `planning/` | 3191 | Pool selector, hierarchy, pairwise, time decay | **UNWIRED** |
| `formula/` | 714 | Combinatorial engine: P/C/n^k via math/big | Production |
| `filter/` | 230 | Pre-filter pipeline + scoring | Production |
| `cmd/` | 5191 | HTTP server, API routes, CORS | Production |
| `cache/` | 1787 | SQLite + forward-only migrations | Production |
| `graph/` | 823 | Dependency/conflict graph + DOT + incremental | Production |
| `github/` | 1823 | GraphQL client, rate limiting | Production |
| `types/` | 856 | All domain types in single file | Production |
| `ml/` | 343 | Go→Python bridge (exec.CommandContext) | Production |
| `settings/` | 751 | Global/repo scope, YAML import/export | Production |
| `sync/` | 2050 | Background sync worker + git mirror | Production |
| `analysis/` | 52 | Bot detection (author + title patterns) | Production |
| `audit/` | 76 | Memory + SQLite audit stores | Production |
| `repo/` | 800 | Git mirror management (bare repos) | Production |
| `report/` | 3809 | PDF via fpdf | Production |
| `testutil/` | 85 | Fixture loader for fixtures/*.json | Test only |
| `util/` | 210 | Shared utilities (strings, tokenization, Jaccard) | Production |
| `telemetry/ratelimit/` | 538 | Rate limiting infrastructure (used by sync/, github/, cmd/) | Production |
| `monitor/` | 2305 | WebSocket server, SSE sync events | Production |
| `review/` | 4637 | PR review analyzers (quality, reliability, security) | Production |
| `version/` | 64 | Build info, version constants | Production |
| `models/` | — | **Does not exist** | N/A |
| `mq/` | — | **Does not exist** | N/A |
| `search/` | — | **Does not exist** | N/A |
| `config/` | — | **Does not exist** | N/A |

## Cross-Package Call Graph

```
cmd/pratc/          # CLI entrypoints
    ↓
internal/cmd/       # HTTP handlers
    ↓
internal/app/       # Service facade
    ↓               ↓               ↓               ↓
internal/filter/  internal/graph/  internal/planner/  internal/github/
    ↓               ↓               ↓
internal/cache/   internal/formula/  internal/ml/
    ↓
internal/sync/ → internal/repo/
```

## Known Code Smells

### planning/ is intentional future architecture
`internal/planning/` (3191 LOC) contains sophisticated planning algorithms with full test coverage:
- `PoolSelector`: Weighted multi-component priority scoring
- `HierarchicalPlanner`: 3-level planning reducing O(C(n,k)) to O(C(clusters,c) × C(avg,s))
- `PairwiseExecutor`: Sharded parallel conflict detection
- `TimeDecayWindow`: Exponential decay with protected lanes

**Status**: NOT wired to production. Production uses `internal/filter/` + `internal/planner/` instead.
**Decision point**: Wire up in v1.4+ OR delete before release. Do NOT import planning types into app/ until decided.
See `internal/planning/AGENTS.md` for full details.

### filter/scorer.go bubble sort — FIXED
`rankByConflictScore()` was O(n²) — FIXED in Phase 2, now uses `sort.Slice`

## Deviation from Standard Go

### Error wrapping: context first, operation second
Standard: `fmt.Errorf("failed to fetch: %w", err)`
This codebase: `fmt.Errorf("github client: %w", err)` — component prefix, not verb prefix.

### Functional options pattern
```go
// planner/planner.go
p := planner.New(prs,
    planner.WithNow(customTime),
    planner.WithIncludeBots(true),
)
```
All configurable types use `WithX()` options. No config structs passed to constructors.

### Sorting: stable + deterministic tiebreaker
Every sort must use PR number as final tiebreaker to ensure deterministic output for tests:
```go
sort.SliceStable(prs, func(i, j int) bool {
    if prs[i].Score != prs[j].Score {
        return prs[i].Score > prs[j].Score
    }
    return prs[i].Number < prs[j].Number // tiebreaker
})
```

## Package-Specific Gotchas

### formula/ — math/big for combinatorics
`Count()` returns `*big.Int` because P(5000,20) overflows uint64. `GenerateByIndex()` takes `*big.Int` index. Convert with `idx.Int64()` only after bounds check.

### graph/ — fingerprint-based incremental updates
`BuildIncremental()` requires prior fingerprint map. First call must use `Build()`. Changing detection logic invalidates all stored fingerprints.

### github/ — rate limit retry with jitter
Exponential backoff exists (2s→60s) with jitter via `addJitter()`. Secondary rate limits (403) retry 8x; 5xx retry 6x. No REST fallback yet.

### cache/ — forward-only migrations
`schema_migrations` table tracks applied versions. `user_version` pragma must equal latest migration or binary refuses to start. No down-migrations; fix forward with new migration.

### ml/ — JSON IPC timeouts
Default 30s timeout for ML calls. `cluster` action can exceed this on 5k+ PRs; caller in `app/` handles timeout with partial results fallback.

### settings/ — repo scope keys
Global settings: `scope="global", repo=""`. Repo settings: `scope="repo", repo="owner/repo"`. Empty repo field on repo scope = validation error.

### sync/ — cursor persistence
Sync jobs store `cursor` in `sync_progress` table. Resume uses cursor; empty cursor = full sync. Cursors are opaque strings from GitHub GraphQL.

## Testing Conventions

- Table-driven with `t.Run(name, func(t *testing.T){...})`
- No external deps: no testify/assert, no gomock
- Fixtures: `testutil.LoadFixture(t, "filename.json")` loads from `../../fixtures/`
- Golden files: `.golden` extension, update with `-update` flag (where implemented)

## Empty Packages (Do Not Use)

`telemetry/`, `models/`, `mq/`, `search/`, `config/` — reserved for future use. Do not add code here without coordinator approval.
