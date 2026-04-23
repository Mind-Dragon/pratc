#!/usr/bin/env python3
import argparse
import json
import re
from pathlib import Path
from datetime import datetime, timezone

import yaml

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


_GAP_ENTRY_RE = re.compile(
    r'^###\s+(?P<id>[A-Z]+-\d+)\s+[—–]\s+(?P<title>[^\n]+)\n'
    r'(?P<body>(?:(?:- ).+\n)*)',
    re.MULTILINE,
)
_STATUS_RE = re.compile(r'^- Status:\s*([^\n]+)', re.MULTILINE)


def _status_for_entry(entry: dict) -> str:
    match = _STATUS_RE.search(entry.get('body', ''))
    return match.group(1).strip() if match else 'unknown'


def parse_existing_gap_entries(path: Path) -> dict[str, dict]:
    """Return existing GAP_LIST entries keyed by stable gap ID."""
    if not path.exists():
        return {}
    entries = {}
    for match in _GAP_ENTRY_RE.finditer(path.read_text()):
        entry = {
            'id': match.group('id').strip(),
            'title': match.group('title').strip(),
            'body': match.group('body') or '',
        }
        entry['status'] = _status_for_entry(entry)
        entries[entry['id']] = entry
    return entries


def render_history_gap(entry: dict, status: str | None = None, notes: str | None = None) -> str:
    """Render a historical gap entry, preserving fields but normalizing status/notes."""
    status = status or entry.get('status', 'unknown')
    preserved = []
    existing_notes = None
    for line in entry.get('body', '').splitlines():
        if line.startswith('- Status:'):
            continue
        if line.startswith('- Notes:'):
            existing_notes = line.removeprefix('- Notes:').strip()
            continue
        preserved.append(line)
    preserved.append(f'- Status: {status}')
    preserved.append(f'- Notes: {notes or existing_notes or "preserved from previous gap list"}')
    body = '\n'.join(preserved)
    return f"### {entry['id']} — {entry['title']}\n{body}\n"


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


def _stable_unique(values):
    out = []
    seen = set()
    for value in values:
        if value and value not in seen:
            seen.add(value)
            out.append(value)
    return out


def update_state(path: Path, open_ids, blocked_ids=None, completed_ids=None):
    if not path.exists():
        return
    state = yaml.safe_load(path.read_text()) or {}
    blocked_ids = blocked_ids or []
    completed_ids = completed_ids or []
    open_set = set(open_ids)
    blocked_set = set(blocked_ids)
    state['open_gaps'] = list(open_ids)
    state['blocked_gaps'] = _stable_unique(blocked_ids)
    prior_completed = state.get('completed_gaps') or []
    state['completed_gaps'] = _stable_unique(
        [gid for gid in prior_completed if gid not in open_set and gid not in blocked_set]
        + [gid for gid in completed_ids if gid not in open_set and gid not in blocked_set]
    )
    state['updated_at'] = datetime.now(timezone.utc).isoformat()
    path.write_text(yaml.dump(state, default_flow_style=False, sort_keys=False))


def generate_gap_list(audit_path: Path, gap_list_path: Path, state_path: Path | None = None):
    audit = load_json(audit_path)
    checks = audit.get('checks', [])
    failures = [c for c in checks if is_failure(c)]
    manuals = [c for c in checks if is_manual(c)]
    existing_entries = parse_existing_gap_entries(gap_list_path)
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

    open_id_set = set(open_ids)
    history_entries = []
    blocked_ids = []
    completed_ids = []
    for gid in sorted(existing_entries):
        if gid in open_id_set:
            continue
        entry = existing_entries[gid]
        status = entry.get('status', 'unknown')
        if status == 'open':
            history_entries.append(render_history_gap(entry, 'fixed', 'fixed by latest audit'))
            completed_ids.append(gid)
        elif status in {'fixed', 'completed'}:
            history_entries.append(render_history_gap(entry, status))
            completed_ids.append(gid)
        elif status == 'blocked':
            history_entries.append(render_history_gap(entry, status))
            blocked_ids.append(gid)
        elif status == 'deferred':
            history_entries.append(render_history_gap(entry, status))
        else:
            history_entries.append(render_history_gap(entry, status))

    if history_entries:
        body.extend([
            '## Gap history',
            '',
            'Resolved, deferred, and blocked gaps are preserved across generated updates so autonomous state is auditable over time.',
            '',
        ])
        for entry_text in history_entries:
            body.append(entry_text)

    body.extend([
        '## Update protocol',
        '',
        'This file is generated from audit output. Preserve stable gap IDs where possible.',
        ''
    ])
    gap_list_path.write_text('\n'.join(body))
    if state_path:
        update_state(state_path, open_ids, blocked_ids=blocked_ids, completed_ids=completed_ids)
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
