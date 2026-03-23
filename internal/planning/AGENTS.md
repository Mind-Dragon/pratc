# AGENTS.md — internal/planning/

Advanced planning algorithms (4984 LOC, 8 files). **Mostly NOT wired into production yet.**

## Architecture

```
PoolSelector ──► HierarchicalPlanner ──► PairwiseExecutor
     │                   │                      │
     ▼                   ▼                      ▼
TimeDecayWindow    Level 1: Select          Shard-based
(exponential       clusters by score        parallel conflict
 decay scoring)    Level 2: Rank PRs         detection
                   within clusters
                   Level 3: Topo sort
```

## Components

### PoolSelector (`pool.go`)
Weighted priority scoring across 5 components. Weights must sum to 1.0.

| Component | Default | Purpose |
|-----------|---------|---------|
| Staleness | 0.30 | Anti-starvation for old PRs |
| CI Status | 0.25 | Prefer green CI |
| Security Labels | 0.20 | Elevate security PRs |
| Cluster Coherence | 0.15 | Batch similar PRs |
| Time Decay | 0.10 | Recency weighting |

**Key types:** `PriorityWeights`, `PoolCandidate`, `PoolResult`, `ComponentScores`

**Settings integration:** `PriorityWeights.ToSettings()`, `PriorityWeightsFromSettings()`

### HierarchicalPlanner (`hierarchy.go`)
3-level planning reducing complexity from O(C(n,k)) to O(C(clusters,c) × C(avg,s)).

**Key types:** `HierarchicalConfig`, `HierarchyResult`, `ClusterSelection`

**Status:** NOT wired in production. `useDependencyOrdering()` always returns true (field exists, not configurable).

### PairwiseExecutor (`pairwise.go`)
Sharded parallel conflict detection with worker pool (sem channel) and early exit.

**Key types:** `ShardConfig`, `ShardMetrics`, `PairwiseResult`

**Status:** NOT wired in production.

### TimeDecayWindow (`time_decay.go`)
Exponential decay: `score = e^(-ln(2) × ageHours / halfLifeHours)`

Protected lane for security/urgent PRs prevents starvation. Old critical PRs get `MinScore` floor.

**Key types:** `TimeDecayConfig`, `TimeDecayStats`

**Settings integration:** `TimeDecayConfigFromSettings()`, `TimeDecayConfigToSettings()`

## Error Types

All implement `error` interface:

- `PoolError`: `ErrInvalidWeights`, `ErrInvalidWeightRange`, `ErrNilPoolResult`, `ErrPoolCountMismatch`, `ErrPoolNotDeterministic`
- `HierarchyError`: `ErrInvalidClusterCount`, `ErrInvalidPerClusterCount`, `ErrInvalidTargetTotal`, `ErrInsufficientCandidatePool`
- `TimeDecayError`: `ErrInvalidHalfLife`, `ErrInvalidWindowHours`, `ErrInvalidProtectedHours`, `ErrInvalidMinScore`

## Conventions

- All scoring functions produce reason codes for explainability
- Deterministic sorting: PR number tiebreaker everywhere
- Settings round-trip via `ToSettings()` / `FromSettings()` pattern
- Validation methods on config types return typed errors

## Gotchas

1. **Production gap:** PoolSelector, HierarchicalPlanner, PairwiseExecutor are implemented but NOT called. Production uses `internal/filter` + `internal/planner` instead.

2. **HierarchicalPlanner hardcoded:** `UseDependencyOrdering` field exists but `useDependencyOrdering()` method ignores it (always returns true).

3. **Weight validation:** `PriorityWeights.Validate()` requires sum within 0.001 of 1.0.

4. **Time decay protected lane:** Security/urgent labels bypass decay after `ProtectedHours`. Check `isProtectedPR()` for label matching logic.
