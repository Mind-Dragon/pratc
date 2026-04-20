#!/usr/bin/env python3
import argparse
import json
from pathlib import Path
from datetime import datetime, timezone

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


def load_json(path: Path):
    with path.open() as f:
        return json.load(f)


def render_gap(check):
    gap_id, title, sev = GAP_MAP.get(check['id'], (f"X-{check['id']}", check['label'], 'P2'))
    actual = check['actual']
    if isinstance(actual, dict):
        actual = json.dumps(actual)
    return f"### {gap_id} — {title}\n- Audit check: `{check['id']}`\n- Severity: {sev}\n- Expected: {check['expected']}\n- Actual: {actual}\n- Status: open\n- Notes: generated from latest audit failure\n"


def is_failure(check: dict) -> bool:
    """Check if a audit result is a failure.
    Supports both old format (pass: bool) and new format (status: str).
    """
    status = check.get('status')
    if status is not None:
        return status == 'fail'
    # Legacy format
    return not check.get('pass', True)


def update_state(path: Path, open_ids):
    if not path.exists():
        return
    lines = path.read_text().splitlines()
    out = []
    in_open = False
    for line in lines:
        if line.startswith('open_gaps:'):
            out.append('open_gaps:')
            for gid in open_ids:
                out.append(f'  - {gid}')
            in_open = True
            continue
        if in_open:
            if line.lstrip().startswith('- '):
                continue
            in_open = False
        if line.startswith('updated_at:'):
            out.append(f"updated_at: {datetime.now(timezone.utc).isoformat()}")
        else:
            out.append(line)
    path.write_text('\n'.join(out) + '\n')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--audit', required=True)
    parser.add_argument('--gap-list', required=True)
    parser.add_argument('--state', required=False)
    args = parser.parse_args()

    audit = load_json(Path(args.audit))
    failures = [c for c in audit.get('checks', []) if is_failure(c)]
    open_ids = []
    body = [
        '# Autonomous Gap List',
        '',
        f'Updated from audit: `{args.audit}`',
        '',
        '## Open gaps',
        '',
    ]
    if failures:
        for check in failures:
            gap_id = GAP_MAP.get(check['id'], (f"X-{check['id']}", '', ''))[0]
            open_ids.append(gap_id)
            body.append(render_gap(check))
    else:
        body.append('No open gaps. Latest audit passed.')
        body.append('')

    body.extend([
        '## Update protocol',
        '',
        'This file is generated from audit output. Preserve stable gap IDs where possible.',
        ''
    ])
    Path(args.gap_list).write_text('\n'.join(body))
    if args.state:
        update_state(Path(args.state), open_ids)
    print(f"Wrote {args.gap_list} with {len(open_ids)} open gaps")


if __name__ == '__main__':
    main()
