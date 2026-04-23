#!/usr/bin/env python3
"""Stable autonomous audit-check to gap metadata catalog."""

GAP_MAP = {
    'bucket_coverage': ('G-001', 'bucket coverage missing', 'P0'),
    'reason_coverage': ('G-002', 'reason trail missing', 'P0'),
    'confidence_coverage': ('G-003', 'confidence coverage missing', 'P0'),
    'dependency_edge_quality': ('G-004', 'trivial dependency edge explosion', 'P1'),
    'conflict_pairs_threshold': ('G-005', 'conflict noise still too high', 'P1'),
    'temporal_routing': ('G-006', 'temporal routing not visible', 'P1'),
    'report_self_describing_prs': ('G-007', 'report surface not self-describing enough', 'P1'),
    'future_work_visible': ('G-008', 'future work visibility missing', 'P1'),
    'duplicate_presence': ('G-009', 'duplicate presence missing on cache-backed rerun', 'P1'),
    'selected_reason_coverage': ('G-010', 'selected plan items lack reasons', 'P1'),
}


def gap_metadata(check_id: str, label: str = '') -> tuple[str, str, str]:
    """Return stable gap id, title, severity for an audit check."""
    return GAP_MAP.get(check_id, (f'X-{check_id}', label, 'P2'))
