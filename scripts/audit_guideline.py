#!/usr/bin/env python3
"""
Audit script for GUIDELINE.md compliance.

Checks deterministic rules from the GUIDELINE matrix.
Rules that are not machine-checkable are reported as "manual" truthfully.
"""
import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

REQUIRED_FILES = {
    'analyze': 'analyze.json',
    'cluster': 'step-3-cluster.json',
    'graph': 'step-4-graph.json',
    'plan': 'step-5-plan.json',
    'report': 'report.pdf',
}

# High-risk bucket types that should not exceed HighRiskConfidenceCap.
HIGH_RISK_BUCKETS = {'security_risk', 'reliability_risk', 'performance_risk'}
# Buckets that represent high-confidence decisions.
HIGH_VALUE_BUCKETS = {'high_value', 'merge_candidate'}
# Temporal buckets - should be mutually exclusive.
TEMPORAL_BUCKETS = {'now', 'future', 'blocked'}
# Disposal buckets - terminal states.
DISPOSAL_BUCKETS = {'junk', 'duplicate', 'stale'}


def load_json(path: Path) -> dict[str, Any]:
    with path.open() as f:
        return json.load(f)


def pr_list(analyze: dict[str, Any]) -> list[dict[str, Any]]:
    prs = analyze.get('prs')
    if isinstance(prs, list):
        return prs
    return []


def count_duplicate_groups(analyze: dict[str, Any]) -> int:
    counts = analyze.get('counts', {}) or {}
    if isinstance(counts.get('duplicate_groups'), int):
        return counts['duplicate_groups']
    duplicates = analyze.get('duplicates')
    if isinstance(duplicates, list):
        return len(duplicates)
    return 0


def pr_bucket(pr: dict[str, Any]) -> str:
    """Return the operator-facing bucket or the nearest current equivalent."""
    bucket = pr.get('bucket')
    if isinstance(bucket, str) and bucket.strip():
        return bucket.strip()

    category = pr.get('category')
    if category == 'duplicate_superseded':
        return 'duplicate'
    if category == 'problematic_quarantine':
        return 'blocked'
    if category == 'merge_now':
        return 'merge_candidate'
    if category == 'merge_after_focused_review':
        return 'needs_review'
    return ''


def pr_reasons(pr: dict[str, Any]) -> list[str]:
    """Return reasons from the current PR surface, falling back to the legacy nested shape."""
    reasons = pr.get('reasons')
    if isinstance(reasons, list):
        return [str(reason) for reason in reasons if str(reason).strip()]

    review = pr.get('review') or {}
    legacy = review.get('reasons') or review.get('bucket_reason') or review.get('reason')
    if isinstance(legacy, list):
        return [str(reason) for reason in legacy if str(reason).strip()]
    if legacy:
        return [str(legacy)]
    return []


def pr_confidence(pr: dict[str, Any]) -> float | int | None:
    """Return confidence from the current PR surface, falling back to the legacy nested shape."""
    conf = pr.get('confidence')
    if isinstance(conf, (float, int)):
        return conf

    review = pr.get('review') or {}
    legacy = review.get('confidence')
    if isinstance(legacy, (float, int)):
        return legacy
    return None


def pr_temporal_bucket(pr: dict[str, Any]) -> str:
    temporal = pr.get('temporal_bucket')
    if isinstance(temporal, str):
        return temporal
    return ''


def pr_priority_tier(pr: dict[str, Any]) -> str:
    tier = pr.get('priority_tier')
    if isinstance(tier, str):
        return tier
    return ''


def pr_is_disposal(pr: dict[str, Any]) -> bool:
    bucket = pr_bucket(pr)
    if bucket in DISPOSAL_BUCKETS:
        return True
    category = pr.get('category')
    return category == 'duplicate_superseded'


def check_artifact_presence(run_dir: Path) -> dict[str, Any]:
    """Check all required artifacts are present."""
    missing = [name for name, rel in REQUIRED_FILES.items() if not (run_dir / rel).exists()]
    return {
        'id': 'artifact_presence',
        'label': 'required artifacts present',
        'status': 'pass' if not missing else 'fail',
        'expected': 'all required files present',
        'actual': 'missing: ' + ', '.join(missing) if missing else 'all present',
    }


def check_truncation_explicit(analyze: dict[str, Any]) -> dict[str, Any]:
    """
    Non-negotiable: no pretending a truncated view is the whole corpus.
    If analysis_truncated is true, truncation_reason must be present.
    """
    truncated = analyze.get('analysis_truncated', False)
    reason = analyze.get('truncation_reason')
    max_prs = analyze.get('max_prs_applied')
    total_prs = analyze.get('counts', {}).get('total_prs', 0)

    if truncated and not reason:
        return {
            'id': 'truncation_explicit',
            'label': 'truncation is explicit when corpus is limited',
            'status': 'fail',
            'expected': 'truncation_reason present when analysis_truncated=true',
            'actual': 'analysis_truncated=true but no truncation_reason',
        }
    if truncated and reason:
        return {
            'id': 'truncation_explicit',
            'label': 'truncation is explicit when corpus is limited',
            'status': 'pass',
            'expected': 'truncation_reason present when analysis_truncated=true',
            'actual': f'truncated: {reason} (max_prs: {max_prs}, total: {total_prs})',
        }
    return {
        'id': 'truncation_explicit',
        'label': 'truncation is explicit when corpus is limited',
        'status': 'pass',
        'expected': 'truncation_reason present when analysis_truncated=true',
        'actual': 'not truncated',
    }


def check_bucket_coverage(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """Every analyzed PR must have a bucket or current-equivalent category."""
    total = len(prs)
    bucket_count = sum(1 for pr in prs if pr_bucket(pr))
    return {
        'id': 'bucket_coverage',
        'label': 'every analyzed PR has bucket',
        'status': 'pass' if total > 0 and bucket_count == total else 'fail',
        'expected': total,
        'actual': bucket_count,
    }


def check_reason_coverage(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """Every analyzed PR must have a reason trail."""
    total = len(prs)
    reason_count = sum(1 for pr in prs if pr_reasons(pr))
    return {
        'id': 'reason_coverage',
        'label': 'every analyzed PR has reason trail',
        'status': 'pass' if total > 0 and reason_count == total else 'fail',
        'expected': total,
        'actual': reason_count,
    }


def check_confidence_coverage(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """Every analyzed PR must have a confidence score."""
    total = len(prs)
    confidence_count = sum(1 for pr in prs if isinstance(pr_confidence(pr), (float, int)))
    return {
        'id': 'confidence_coverage',
        'label': 'every analyzed PR has confidence',
        'status': 'pass' if total > 0 and confidence_count == total else 'fail',
        'expected': total,
        'actual': confidence_count,
    }


def check_temporal_mutually_exclusive(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    Temporal buckets (now/future/blocked) are mutually exclusive.
    A PR cannot have more than one temporal bucket.
    """
    violations = []
    for pr in prs:
        temporal = pr.get('temporal_bucket', '')
        if isinstance(temporal, list) and len(temporal) > 1:
            violations.append(pr.get('number') or pr.get('id', 'unknown'))
    return {
        'id': 'temporal_mutually_exclusive',
        'label': 'temporal buckets now/future/blocked are mutually exclusive',
        'status': 'pass' if not violations else 'fail',
        'expected': '0 PRs with multiple temporal buckets',
        'actual': f'{len(violations)} PRs violate mutual exclusivity' if violations else 'none',
    }


def check_low_confidence_high_value(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    Non-negotiable: no claiming certainty when confidence is low.
    Confidence < 0.5 should not be paired with high_value or merge_candidate buckets.
    """
    violations = []
    for pr in prs:
        conf = pr_confidence(pr)
        bucket = pr_bucket(pr)
        if isinstance(conf, (float, int)) and conf < 0.5 and bucket in HIGH_VALUE_BUCKETS:
            violations.append({
                'pr': pr.get('number') or pr.get('id', 'unknown'),
                'bucket': bucket,
                'confidence': conf,
            })
    return {
        'id': 'low_confidence_high_value',
        'label': 'low confidence not paired with high-value buckets',
        'status': 'pass' if not violations else 'fail',
        'expected': 'no high_value/merge_candidate with confidence < 0.5',
        'actual': f'{len(violations)} violations' if violations else 'none',
        'details': violations[:5] if violations else None,
    }


def check_high_confidence_cap_respected(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    HighRiskConfidenceCap = 0.79: high-risk findings should not claim
    confidence above 0.79 without strong evidence.
    """
    violations = []
    for pr in prs:
        conf = pr_confidence(pr)
        bucket = pr_bucket(pr)
        if isinstance(conf, (float, int)) and conf > 0.79 and bucket in HIGH_RISK_BUCKETS:
            violations.append({
                'pr': pr.get('number') or pr.get('id', 'unknown'),
                'bucket': bucket,
                'confidence': conf,
            })
    return {
        'id': 'high_confidence_cap_respected',
        'label': 'high-risk buckets respect confidence cap of 0.79',
        'status': 'pass' if not violations else 'fail',
        'expected': 'no risk buckets with confidence > 0.79',
        'actual': f'{len(violations)} violations' if violations else 'none',
        'details': violations[:5] if violations else None,
    }


def check_duplicate_presence(analyze: dict[str, Any]) -> dict[str, Any]:
    """Duplicates are not separate problems - duplicate groups should be detected."""
    dup_groups = count_duplicate_groups(analyze)
    return {
        'id': 'duplicate_presence',
        'label': 'duplicate groups detected',
        'status': 'pass' if dup_groups > 0 else 'fail',
        'expected': '> 0',
        'actual': dup_groups,
    }


def check_garbage_detected(analyze: dict[str, Any], prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    Garbage gets removed early.
    Either counts.garbage_prs > 0 or there are PRs with junk bucket.
    """
    counts = analyze.get('counts', {}) or {}
    garbage_count = counts.get('garbage_prs', 0)
    junk_bucket_count = sum(1 for pr in prs if pr_bucket(pr) == 'junk')
    total_garbage = max(garbage_count, junk_bucket_count)
    return {
        'id': 'garbage_detected',
        'label': 'garbage detection has run',
        'status': 'pass' if total_garbage > 0 else 'fail',
        'expected': '> 0 garbage or junk PRs detected',
        'actual': f'garbage_prs={garbage_count}, junk_buckets={junk_bucket_count}',
    }


def check_future_work_visible(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    Future work stays visible. If there are non-disposal PRs, some should be in future bucket.
    """
    non_disposal = [pr for pr in prs if not pr_is_disposal(pr)]
    future_count = sum(1 for pr in non_disposal if pr_temporal_bucket(pr) == 'future')
    return {
        'id': 'future_work_visible',
        'label': 'future work stays visible',
        'status': 'pass' if len(non_disposal) == 0 or future_count > 0 else 'fail',
        'expected': '> 0 future bucket PRs when non-disposal PRs exist',
        'actual': f'{future_count} future PRs out of {len(non_disposal)} non-disposal',
    }


def check_temporal_routing(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """Temporal buckets (now/future/blocked) should be visible."""
    total = len(prs)
    temporal_count = sum(1 for pr in prs if pr_temporal_bucket(pr) in TEMPORAL_BUCKETS)
    return {
        'id': 'temporal_routing',
        'label': 'temporal buckets visible',
        'status': 'pass' if total > 0 and temporal_count > 0 else 'fail',
        'expected': '> 0 temporal buckets',
        'actual': temporal_count,
    }


def check_report_self_describing_prs(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    The report should not need to reconstruct the decision surface from side channels.
    Each PR row in analyze.json must be self-describing enough for appendix/report use.
    """
    total = len(prs)
    complete = 0
    for pr in prs:
        if (
            pr_bucket(pr)
            and pr_temporal_bucket(pr) in TEMPORAL_BUCKETS
            and pr_reasons(pr)
            and isinstance(pr_confidence(pr), (float, int))
            and pr_priority_tier(pr)
        ):
            complete += 1
    return {
        'id': 'report_self_describing_prs',
        'label': 'analyze PR rows are self-describing for report/appendix use',
        'status': 'pass' if total > 0 and complete == total else 'fail',
        'expected': total,
        'actual': complete,
    }


def check_selected_reason_coverage(plan: dict[str, Any]) -> dict[str, Any]:
    """Selected plan items must have reasons (no opaque ranking)."""
    selected = plan.get('selected') if isinstance(plan.get('selected'), list) else []
    selected_reason_count = 0
    for item in selected:
        if any(item.get(k) for k in ('reason', 'reasons', 'bucket_reason', 'why', 'rationale')):
            selected_reason_count += 1
    return {
        'id': 'selected_reason_coverage',
        'label': 'selected plan items have reasons',
        'status': 'pass' if len(selected) == 0 or selected_reason_count == len(selected) else 'fail',
        'expected': len(selected),
        'actual': selected_reason_count,
    }


def check_conflict_pairs_threshold(graph: dict[str, Any]) -> dict[str, Any]:
    """Conflict pairs should be below threshold to ensure graph quality."""
    edges = graph.get('edges') if isinstance(graph.get('edges'), list) else []
    conflict_pairs = sum(1 for e in edges if e.get('edge_type') == 'conflicts_with')
    return {
        'id': 'conflict_pairs_threshold',
        'label': 'conflict pairs below threshold',
        'status': 'pass' if conflict_pairs < 5000 else 'fail',
        'expected': '< 5000',
        'actual': conflict_pairs,
    }


def check_dependency_edge_quality(graph: dict[str, Any]) -> dict[str, Any]:
    """Dependency edges should not be dominated by trivial same-branch reasons."""
    edges = graph.get('edges') if isinstance(graph.get('edges'), list) else []
    depends_edges = [e for e in edges if e.get('edge_type') == 'depends_on']
    trivial_dep = sum(
        1 for e in depends_edges
        if 'base branch' in str(e.get('reason', '')).lower() and 'head branch' in str(e.get('reason', '')).lower()
    )
    ratio = trivial_dep / max(len(depends_edges), 1)
    return {
        'id': 'dependency_edge_quality',
        'label': 'dependency edges not dominated by trivial same-branch reasons',
        'status': 'pass' if len(depends_edges) == 0 or ratio <= 0.5 else 'fail',
        'expected': '<= 50% trivial depends_on edges',
        'actual': {
            'depends_on_edges': len(depends_edges),
            'trivial_dep_edges': trivial_dep,
            'ratio': round(ratio, 3),
        },
    }


def check_disposal_bucket_persistence(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    Disposal buckets are terminal: junk stays junk.
    This requires longitudinal data (same PR reviewed at multiple timepoints),
    so it's manual/unverifiable in a single snapshot.
    """
    return {
        'id': 'disposal_bucket_persistence',
        'label': 'disposal buckets are terminal (junk stays junk)',
        'status': 'manual',
        'expected': 'requires longitudinal data to verify PRs dont exit disposal state',
        'actual': 'uncheckable in single-run audit',
    }


def check_deeper_judgment_layers(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """
    Deeper judgment after obvious layers (16-layer decision ladder).
    Verifying proper ordering of layers requires examining the review pipeline,
    which is not directly observable in the output artifacts.
    """
    return {
        'id': 'deeper_judgment_layers',
        'label': 'deeper judgment applied after obvious layers',
        'status': 'manual',
        'expected': 'garbage->duplicate->obvious badness->substance->temporal->deeper layers',
        'actual': 'pipeline ordering not directly observable in artifacts',
    }


def check_bucket_reason_required(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """Every bucket assignment must carry a reason code."""
    missing_reasons = []
    for pr in prs:
        bucket = pr_bucket(pr)
        if not bucket:
            continue
        if not pr_reasons(pr):
            missing_reasons.append(pr.get('number') or pr.get('id', 'unknown'))
    return {
        'id': 'bucket_reason_required',
        'label': 'every bucket assignment has reason code',
        'status': 'pass' if not missing_reasons else 'fail',
        'expected': '0 PRs with bucket but no reason',
        'actual': f'{len(missing_reasons)} PRs missing reasons' if missing_reasons else 'all have reasons',
    }


def audit(run_dir: Path) -> dict[str, Any]:
    """Run all deterministic checks against a run directory."""
    analyze = load_json(run_dir / 'analyze.json') if (run_dir / 'analyze.json').exists() else {}
    graph = load_json(run_dir / 'step-4-graph.json') if (run_dir / 'step-4-graph.json').exists() else {}
    plan = load_json(run_dir / 'step-5-plan.json') if (run_dir / 'step-5-plan.json').exists() else {}

    prs = pr_list(analyze)

    checks = [
        check_artifact_presence(run_dir),
        check_truncation_explicit(analyze),
        check_bucket_coverage(prs),
        check_reason_coverage(prs),
        check_confidence_coverage(prs),
        check_temporal_mutually_exclusive(prs),
        check_low_confidence_high_value(prs),
        check_high_confidence_cap_respected(prs),
        check_duplicate_presence(analyze),
        check_garbage_detected(analyze, prs),
        check_future_work_visible(prs),
        check_temporal_routing(prs),
        check_report_self_describing_prs(prs),
        check_selected_reason_coverage(plan),
        check_conflict_pairs_threshold(graph),
        check_dependency_edge_quality(graph),
        check_disposal_bucket_persistence(prs),
        check_deeper_judgment_layers(prs),
        check_bucket_reason_required(prs),
    ]

    passed = sum(1 for c in checks if c['status'] == 'pass')
    failed = sum(1 for c in checks if c['status'] == 'fail')
    manual = sum(1 for c in checks if c['status'] == 'manual')

    result = {
        'generated_at': datetime.now(timezone.utc).isoformat(),
        'run_dir': str(run_dir),
        'summary': {
            'total_prs': len(prs),
            'checks_passed': passed,
            'checks_failed': failed,
            'checks_manual': manual,
        },
        'checks': checks,
        'passed': failed == 0,
    }
    return result


def main():
    parser = argparse.ArgumentParser(
        description='Audit a run directory for GUIDELINE.md compliance'
    )
    parser.add_argument('run_dir', help='Path to run directory containing artifacts')
    parser.add_argument('--output', default=None, help='Output path for AUDIT_RESULTS.json')
    args = parser.parse_args()

    run_dir = Path(args.run_dir)
    result = audit(run_dir)
    output = Path(args.output) if args.output else run_dir / 'AUDIT_RESULTS.json'
    output.write_text(json.dumps(result, indent=2) + '\n')

    print(f"\nAudit Results for: {run_dir}")
    print('=' * 60)
    for check in result['checks']:
        status = check['status'].upper()
        print(f"[{status:5}] {check['id']}: {check['label']}")
        if check['status'] == 'fail':
            print(f"       expected={check['expected']} actual={check['actual']}")
    print('=' * 60)
    print(
        f"Summary: {result['summary']['checks_passed']} passed, "
        f"{result['summary']['checks_failed']} failed, "
        f"{result['summary']['checks_manual']} manual/unverifiable"
    )
    print(f"\nAudit written to {output}")
    sys.exit(0 if result['passed'] else 1)


if __name__ == '__main__':
    main()
