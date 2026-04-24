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
# Small smoke runs are too shallow to require duplicate/junk presence.
MIN_PRS_FOR_CONTENT_PRESENCE_CHECKS = 150

ACTION_LANES = {
    'fast_merge',
    'fix_and_merge',
    'duplicate_close',
    'reject_or_close',
    'focused_review',
    'future_or_reengage',
    'human_escalate',
}

MUTATING_ACTIONS = {'merge', 'close', 'comment', 'label', 'request_changes', 'apply_fix'}


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


def analyzed_pr_count(analyze: dict[str, Any], prs: list[dict[str, Any]] | None = None) -> int:
    counts = analyze.get('counts', {}) or {}
    total = counts.get('total_prs')
    if isinstance(total, int) and total > 0:
        return total
    if prs is not None:
        return len(prs)
    listed = analyze.get('prs')
    if isinstance(listed, list):
        return len(listed)
    return 0


def sample_too_small_for_presence_checks(analyze: dict[str, Any], prs: list[dict[str, Any]] | None = None) -> bool:
    return analyzed_pr_count(analyze, prs) < MIN_PRS_FOR_CONTENT_PRESENCE_CHECKS


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


def pr_disposal_buckets(pr: dict[str, Any]) -> list[str]:
    """Return disposal buckets observed on the PR surface or terminal decision layers."""
    buckets = []
    bucket = pr_bucket(pr)
    if bucket in DISPOSAL_BUCKETS:
        buckets.append(bucket)
    layers = pr.get('decision_layers')
    if isinstance(layers, list):
        for layer in layers:
            if not isinstance(layer, dict):
                continue
            layer_bucket = layer.get('bucket')
            if layer_bucket in DISPOSAL_BUCKETS and layer.get('terminal') is True:
                buckets.append(layer_bucket)
    return sorted(set(buckets))


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
    sample_size = analyzed_pr_count(analyze)
    if dup_groups == 0 and sample_too_small_for_presence_checks(analyze):
        return {
            'id': 'duplicate_presence',
            'label': 'duplicate groups detected',
            'status': 'manual',
            'expected': f'> 0 once sample reaches >= {MIN_PRS_FOR_CONTENT_PRESENCE_CHECKS} PRs',
            'actual': (
                f'sample too small to require duplicates '
                f'({sample_size} < {MIN_PRS_FOR_CONTENT_PRESENCE_CHECKS})'
            ),
        }
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
    sample_size = analyzed_pr_count(analyze, prs)
    if total_garbage == 0 and sample_too_small_for_presence_checks(analyze, prs):
        return {
            'id': 'garbage_detected',
            'label': 'garbage detection has run',
            'status': 'manual',
            'expected': (
                f'> 0 garbage or junk PRs detected once sample reaches >= '
                f'{MIN_PRS_FOR_CONTENT_PRESENCE_CHECKS} PRs'
            ),
            'actual': (
                f'sample too small to require garbage/junk presence '
                f'({sample_size} < {MIN_PRS_FOR_CONTENT_PRESENCE_CHECKS}); '
                f'garbage_prs={garbage_count}, junk_buckets={junk_bucket_count}'
            ),
        }
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
    """Disposal bucket assignments must be backed by a terminal decision layer."""
    disposal_prs = [(pr, pr_disposal_buckets(pr)) for pr in prs if pr_disposal_buckets(pr)]
    violations = []
    for pr, buckets in disposal_prs:
        layers = pr.get('decision_layers')
        if not isinstance(layers, list) or not layers:
            violations.append(pr.get('number') or pr.get('id', 'unknown'))
            continue
        for bucket in buckets:
            terminal_match = False
            for layer in layers:
                if not isinstance(layer, dict):
                    continue
                if layer.get('bucket') == bucket and layer.get('terminal') is True and layer.get('continued') is False:
                    terminal_match = True
                    break
            if not terminal_match:
                violations.append(pr.get('number') or pr.get('id', 'unknown'))
                break

    if violations:
        return {
            'id': 'disposal_bucket_persistence',
            'label': 'disposal buckets are terminal (junk stays junk)',
            'status': 'fail',
            'expected': 'every disposal PR has a matching terminal decision layer',
            'actual': f'{len(disposal_prs)} disposal PRs missing terminal layer: {violations[:10]}',
        }
    return {
        'id': 'disposal_bucket_persistence',
        'label': 'disposal buckets are terminal (junk stays junk)',
        'status': 'pass',
        'expected': 'every disposal PR has a matching terminal decision layer',
        'actual': f'{len(disposal_prs)} disposal PRs verified',
    }


def check_deeper_judgment_layers(prs: list[dict[str, Any]]) -> dict[str, Any]:
    """Decision-layer artifacts must prove cheap obvious layers precede deeper judgment."""
    tier_rank = {'cheap': 0, 'medium': 1, 'expensive': 2}
    required_prefix = ['Garbage', 'Duplicates', 'Obvious badness']
    violations = []
    for pr in prs:
        layers = pr.get('decision_layers')
        pr_id = pr.get('number') or pr.get('id', 'unknown')
        if not isinstance(layers, list) or len(layers) < len(required_prefix):
            violations.append((pr_id, 'missing decision_layers'))
            continue
        layer_nums = [layer.get('layer') for layer in layers if isinstance(layer, dict)]
        if layer_nums != sorted(layer_nums) or len(layer_nums) != len(set(layer_nums)):
            violations.append((pr_id, 'decision layers not strictly ordered'))
            continue
        names = [str(layer.get('name', '')) for layer in layers[:len(required_prefix)]]
        if names != required_prefix:
            violations.append((pr_id, f'prefix {names} != {required_prefix}'))
            continue
        ranks = []
        unknown_tier = False
        for layer in layers:
            tier = layer.get('cost_tier')
            if tier not in tier_rank:
                unknown_tier = True
                break
            ranks.append(tier_rank[tier])
        if unknown_tier:
            violations.append((pr_id, 'unknown cost_tier'))
            continue
        if ranks != sorted(ranks):
            violations.append((pr_id, 'deeper cost tier appears before cheaper layer'))
            continue
        if not any(rank == tier_rank['expensive'] for rank in ranks):
            violations.append((pr_id, 'no deeper expensive layer observed'))
            continue

    if violations:
        return {
            'id': 'deeper_judgment_layers',
            'label': 'deeper judgment applied after obvious layers',
            'status': 'fail',
            'expected': 'ordered decision_layers: cheap obvious layers before medium/expensive deeper layers',
            'actual': f'{len(violations)} PRs violate layer ordering: {violations[:10]}',
        }
    return {
        'id': 'deeper_judgment_layers',
        'label': 'deeper judgment applied after obvious layers',
        'status': 'pass',
        'expected': 'ordered decision_layers: cheap obvious layers before medium/expensive deeper layers',
        'actual': f'{len(prs)} PRs have ordered decision-layer artifacts',
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


def check_action_plan_presence(action_plan: dict[str, Any]) -> dict[str, Any]:
    """v2 action audit requires a generated action-plan.json artifact."""
    present = bool(action_plan)
    return {
        'id': 'action_plan_presence',
        'label': 'v2 action plan artifact present',
        'status': 'pass' if present else 'fail',
        'expected': 'action-plan.json with ActionPlan payload',
        'actual': 'present' if present else 'missing action-plan.json',
    }


def check_action_lane_coverage(action_plan: dict[str, Any]) -> dict[str, Any]:
    """Every ActionPlan work item must have exactly one valid primary lane and summaries must match."""
    if not action_plan:
        return {
            'id': 'action_lane_coverage',
            'label': 'every action work item has one valid primary lane',
            'status': 'fail',
            'expected': 'work_items each carry one valid lane',
            'actual': 'missing action-plan.json',
        }

    work_items = action_plan.get('work_items') if isinstance(action_plan.get('work_items'), list) else []
    lanes = action_plan.get('lanes') if isinstance(action_plan.get('lanes'), list) else []
    snapshot = action_plan.get('corpus_snapshot') if isinstance(action_plan.get('corpus_snapshot'), dict) else {}
    expected_total = snapshot.get('total_prs')
    issues: list[str] = []

    lane_counts: dict[str, int] = {}
    seen_prs: set[Any] = set()
    duplicate_prs: list[Any] = []
    for item in work_items:
        if not isinstance(item, dict):
            issues.append('non_object_work_item')
            continue
        lane = item.get('lane')
        if not isinstance(lane, str) or not lane:
            issues.append(f"missing_lane:{item.get('id', 'unknown')}")
            continue
        if lane not in ACTION_LANES:
            issues.append(f"invalid_lane:{item.get('id', 'unknown')}:{lane}")
            continue
        lane_counts[lane] = lane_counts.get(lane, 0) + 1
        pr_number = item.get('pr_number')
        if pr_number in seen_prs:
            duplicate_prs.append(pr_number)
        seen_prs.add(pr_number)

    if duplicate_prs:
        issues.append(f'duplicate_pr_work_items:{duplicate_prs[:10]}')
    if isinstance(expected_total, int) and expected_total > 0 and len(work_items) != expected_total:
        issues.append(f'work_item_total:{len(work_items)} != corpus_total:{expected_total}')

    summary_counts: dict[str, int] = {}
    for summary in lanes:
        if not isinstance(summary, dict):
            issues.append('non_object_lane_summary')
            continue
        lane = summary.get('lane')
        count = summary.get('count')
        if lane not in ACTION_LANES:
            issues.append(f'invalid_summary_lane:{lane}')
            continue
        if not isinstance(count, int):
            issues.append(f'invalid_summary_count:{lane}')
            continue
        summary_counts[lane] = count

    if summary_counts != lane_counts:
        issues.append(f'summary_mismatch:{summary_counts} != {lane_counts}')

    return {
        'id': 'action_lane_coverage',
        'label': 'every action work item has one valid primary lane',
        'status': 'pass' if work_items and not issues else 'fail',
        'expected': 'one valid lane per work item; lane summaries match work items',
        'actual': 'ok' if work_items and not issues else '; '.join(issues) or 'no work_items',
    }


def check_advisory_zero_write(action_plan: dict[str, Any]) -> dict[str, Any]:
    """Advisory policy must not contain live mutation intents."""
    if not action_plan:
        return {
            'id': 'advisory_zero_write',
            'label': 'advisory action plan contains zero live writes',
            'status': 'fail',
            'expected': 'advisory action plan with only dry-run intents',
            'actual': 'missing action-plan.json',
        }
    if action_plan.get('policy_profile') != 'advisory':
        return {
            'id': 'advisory_zero_write',
            'label': 'advisory action plan contains zero live writes',
            'status': 'manual',
            'expected': 'checked when policy_profile=advisory',
            'actual': f"policy_profile={action_plan.get('policy_profile')}",
        }
    intents = action_plan.get('action_intents') if isinstance(action_plan.get('action_intents'), list) else []
    live = []
    for intent in intents:
        if not isinstance(intent, dict):
            continue
        if intent.get('action') in MUTATING_ACTIONS and intent.get('dry_run') is not True:
            live.append(intent.get('id') or intent.get('pr_number') or 'unknown')
    return {
        'id': 'advisory_zero_write',
        'label': 'advisory action plan contains zero live writes',
        'status': 'pass' if not live else 'fail',
        'expected': '0 non-dry-run mutating intents',
        'actual': 'all mutating intents are dry-run' if not live else f'live mutation intents: {live[:10]}',
    }


def audit(run_dir: Path) -> dict[str, Any]:
    """Run all deterministic checks against a run directory."""
    analyze = load_json(run_dir / 'analyze.json') if (run_dir / 'analyze.json').exists() else {}
    graph = load_json(run_dir / 'step-4-graph.json') if (run_dir / 'step-4-graph.json').exists() else {}
    plan = load_json(run_dir / 'step-5-plan.json') if (run_dir / 'step-5-plan.json').exists() else {}
    action_plan = load_json(run_dir / 'action-plan.json') if (run_dir / 'action-plan.json').exists() else {}

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
        check_action_plan_presence(action_plan),
        check_action_lane_coverage(action_plan),
        check_advisory_zero_write(action_plan),
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
