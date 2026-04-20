# Autonomous Gap List

Updated from audit: `projects/OpenClaw_OpenClaw/runs/20260419-065654/AUDIT_RESULTS.json`

## Open gaps

### G-001 — bucket coverage missing
- Audit check: `bucket_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open
- Notes: generated from latest audit failure

### G-002 — reason trail missing
- Audit check: `reason_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open
- Notes: generated from latest audit failure

### G-003 — confidence coverage missing
- Audit check: `confidence_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open
- Notes: generated from latest audit failure

### G-008 — future work visibility missing
- Audit check: `future_work_visible`
- Severity: P1
- Expected: > 0 future bucket PRs when non-disposal PRs exist
- Actual: 0 future PRs out of 4992 non-disposal
- Status: open
- Notes: generated from latest audit failure

### G-006 — temporal routing not visible
- Audit check: `temporal_routing`
- Severity: P1
- Expected: > 0 temporal buckets
- Actual: 0
- Status: open
- Notes: generated from latest audit failure

### G-007 — report surface not self-describing enough
- Audit check: `report_self_describing_prs`
- Severity: P1
- Expected: 4992
- Actual: 0
- Status: open
- Notes: generated from latest audit failure

### G-005 — conflict noise still too high
- Audit check: `conflict_pairs_threshold`
- Severity: P1
- Expected: < 5000
- Actual: 380716
- Status: open
- Notes: generated from latest audit failure

### G-004 — trivial dependency edge explosion
- Audit check: `dependency_edge_quality`
- Severity: P1
- Expected: <= 50% trivial depends_on edges
- Actual: {"depends_on_edges": 804678, "trivial_dep_edges": 804678, "ratio": 1.0}
- Status: open
- Notes: generated from latest audit failure

## Update protocol

This file is generated from audit output. Preserve stable gap IDs where possible.
