# AGENTS.md — internal/filter

## Pipeline Stages (BuildCandidatePool)

Order is fixed:
1. `FilterDraft` — reject `IsDraft == true`
2. `FilterMergeConflict` — reject `Mergeable == "conflicting"`
3. `FilterCIFailure` — reject `CIStatus == "failure"`
4. `FilterBot` — reject if `IsBot && !includeBots`
5. `AssignClusterIDs` — map cluster IDs from ML results
6. `ScoreAndSortPool` — priority desc, PR number asc tiebreak
7. `CapPool` — hard cap at 64 with rejection tracking

## Scoring (PlannerPriority)

| Factor | Value |
|--------|-------|
| CI success | +3 |
| CI pending/unknown | +1 |
| CI failure | -2 |
| Review approved | +2 |
| Review changes_requested | -2 |
| Mergeable | +1 |
| Age (days/15, max 2) | +[0-2] |
| Bot PR | +0.5 |

Sort: priority descending, then PR number ascending for determinism.

## Rejection Reasons

Returned in `PlanRejection.Reason`:
- `"draft"`
- `"merge conflict"`
- `"ci failure"`
- `"bot pr"` (when filtered)
- `"candidate pool cap"` (exceeded max)

## Gotchas

- `ScoreAndSortPool` was O(n²) bubble sort — FIXED in Phase 2, now uses `sort.Slice`
- Pool cap is `types.DefaultPoolCap` (64), configurable via constants
- `ApplyFilters` returns two slices; both must be preserved for `plan` output.
- `PlannerRationale` strings used in plan output for human-readable justification.
