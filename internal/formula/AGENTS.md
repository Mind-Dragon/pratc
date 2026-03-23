# AGENTS.md — internal/formula/

## Usage

```go
engine := formula.NewEngine(formula.DefaultConfig())
result, err := engine.Search(formula.SearchInput{
    Pool:        filteredPRs,  // Must be pre-filtered
    Target:      10,           // PRs to select
    PreFiltered: true,         // Required by default
})
```

**Result**: `result.Best` contains highest-scored selection across all tiers.

## Mathematical Correctness

- Always `math/big.Int` — never float for combinatorics. `Count()` returns `*big.Int`.
- Multiplicative formula for combinations avoids intermediate overflow.
- `GenerateByIndex` uses lexicographic ordering. Index 0 = first combination/permutation.

## Complexity & Gotchas

| Function | Complexity | Warning |
|----------|------------|---------|
| `Count()` | O(k) | Trivial |
| `GenerateByIndex(combination)` | O(k) | Linear in selection size |
| `GenerateByIndex(permutation)` | O(n*k) | Allocates `available` slice |
| `conflictCounts()` | O(n²) | **Only use with filtered pools** |
| `Engine.Search()` | O(tiers × candidates × (n² + k)) | Caps at `MaxCandidates` per tier |

**Tier Filtering** (applied before scoring):
- `quick`: main branch + non-conflicting + CI success
- `thorough`: non-conflicting only
- `exhaustive`: no filtering

**Scoring Weights** (sum to 1.0): Age 0.20, Size 0.15, CI 0.20, Review 0.20, Conflict 0.15, Cluster 0.10

## Safety

- `ErrInputNotPreFiltered` if `RequirePreFiltered: true` and `PreFiltered: false`
- `ErrPoolTooLarge` if pool exceeds `MaxPoolSize` (default 64)
- `ErrNoCandidates` if all tiers produce empty results
- `Count()` returns 0 for invalid inputs (k > n for perm/comb, n=0 with k>0 for replacement)
