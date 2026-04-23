#!/usr/bin/env python3
import argparse
import json
from pathlib import Path
from datetime import datetime, timezone

from gap_catalog import GAP_MAP, gap_metadata


def load_json(path: Path):
    with path.open() as f:
        return json.load(f)


def render_gap(check):
    gap_id, title, sev = gap_metadata(check['id'], check['label'])
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


def is_manual(check: dict) -> bool:
    return check.get('status') == 'manual'


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


def generate_gap_list(audit_path: Path, gap_list_path: Path, state_path: Path | None = None):
    audit = load_json(audit_path)
    checks = audit.get('checks', [])
    failures = [c for c in checks if is_failure(c)]
    manuals = [c for c in checks if is_manual(c)]
    open_ids = []
    body = [
        '# Autonomous Gap List',
        '',
        f'Updated from audit: `{audit_path}`',
        '',
        '## Open gaps',
        '',
    ]
    if failures:
        for check in failures:
            gap_id = gap_metadata(check['id'])[0]
            open_ids.append(gap_id)
            body.append(render_gap(check))
    else:
        body.append('No open gaps. Latest audit passed.')
        body.append('')

    if manuals:
        body.extend([
            '## Manual/unverifiable checks',
            '',
            'These are not open required failures, but they remain autonomy gaps until converted to machine checks or explicitly accepted by an operator in `wave-summary.md`.',
            '',
        ])
        for check in manuals:
            actual = check.get('actual', '')
            if isinstance(actual, dict):
                actual = json.dumps(actual)
            body.append(f"- `{check.get('id', '')}` — {actual}")
        body.append('')

    body.extend([
        '## Update protocol',
        '',
        'This file is generated from audit output. Preserve stable gap IDs where possible.',
        ''
    ])
    gap_list_path.write_text('\n'.join(body))
    if state_path:
        update_state(state_path, open_ids)
    return open_ids


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--audit', required=True)
    parser.add_argument('--gap-list', required=True)
    parser.add_argument('--state', required=False)
    args = parser.parse_args()

    open_ids = generate_gap_list(Path(args.audit), Path(args.gap_list), Path(args.state) if args.state else None)
    print(f"Wrote {args.gap_list} with {len(open_ids)} open gaps")


if __name__ == '__main__':
    main()
