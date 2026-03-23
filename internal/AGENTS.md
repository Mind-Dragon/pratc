# AGENTS.md — internal/

Go backend packages. Parent AGENTS.md covers top-level contracts; this covers package internals.

## Package Index

| Package | LOC | Purpose | Status |
|---------|-----|---------|--------|
| `app/` | 836 | Service facade: Analyze, Cluster, Graph, Plan | Production |
| `planner/` | 892 | Core planner with functional options | Production |
| `planning/` | 4984 | Pool selector, hierarchy, pairwise, time decay | **UNWIRED** |
| `formula/` | 1004 | Combinatorial engine: P/C/n^k via math/big | Production |
| `filter/` | 980 | Pre-filter pipeline + scoring | Production |
| `cmd/` | 1608 | HTTP server, API routes, CORS | Production |
| `cache/` | 1404 | SQLite + forward-only migrations | Production |
| `graph/` | 1269 | Dependency/conflict graph + DOT + incremental | Production |
| `github/` | 1163 | GraphQL client, rate limiting | Production |
| `types/` | 516 | All domain types in single file | Production |
| `ml/` | ~200 | Go→Python bridge (exec.CommandContext) | Production |
| `settings/` | 440 | Global/repo scope, YAML import/export | Production |
| `sync/` | 374 | Background sync worker + git mirror | Production |
| `analysis/` | 196 | Bot detection (author + title patterns) | Production |
| `audit/` | ~150 | Memory + SQLite audit stores | Production |
| `repo/` | ~300 | Git mirror management (bare repos) | Production |
| `report/` | ~200 | PDF via fpdf | Production |
| `testutil/` | ~100 | Fixture loader for fixtures/*.json | Test only |
| `telemetry/` | 0 | **Empty placeholder** | Not implemented |
| `models/` | 0 | **Empty placeholder** | Not implemented |
| `mq/` | 0 | **Empty placeholder** | Not implemented |
| `search/` | 0 | **Empty placeholder** | Not implemented |
| `config/` | 0 | **Empty placeholder** | Not implemented |

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

### app/ duplicates planner/ (~70% overlap)
Both implement: `jaccard()`, `tokenize()`, `buildClusters()`, `clusterKey()`, `sanitizeClusterID()`.
**Rule:** New clustering logic goes in `planner/`; `app/service.go` calls `planner.New()` instead of duplicating.

### planning/ is mostly dead code
`HierarchicalPlanner`, `PoolSelector` have **zero production callers**. Only `planning/pool.go` utilities are wired. Do not import planning types into app/ until explicitly requested.

### filter/scorer.go uses bubble sort
`rankByConflictScore()` is O(n²). Acceptable for n<500 (current scale) but replace with `sort.Slice` if pool caps increase.

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

### github/ — rate limit retry missing jitter
Exponential backoff exists (2s→60s) but jitter is TODO. Secondary rate limits (403) retry 8x; 5xx retry 6x. No REST fallback yet.

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
