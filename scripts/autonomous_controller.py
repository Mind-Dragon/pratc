#!/usr/bin/env python3
"""
Autonomous controller — deterministic control-plane for prATC autonomous mode.

Reads AUTONOMOUS.md for policy, autonomous/STATE.yaml for checkpoint, and
autonomous/GAP_LIST.md for the current failure surface. All non-trivial
implementation work is delegated to subagents; this script only manages state.
"""
import argparse
from datetime import datetime, timezone
from pathlib import Path
import re
import sys

import yaml

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
REPO_ROOT = Path('/home/agent/pratc')
STATE_PATH = REPO_ROOT / 'autonomous' / 'STATE.yaml'
GAP_LIST_PATH = REPO_ROOT / 'autonomous' / 'GAP_LIST.md'

# ---------------------------------------------------------------------------
# YAML state handling — robust, replaces the hand-rolled yamlish parser
# ---------------------------------------------------------------------------

def _bool_like(value):
    """Convert YAML-ish string booleans to Python bool."""
    if isinstance(value, bool):
        return value
    if not isinstance(value, str):
        return value
    if value.lower() in ('true', 'yes', 'on'):
        return True
    if value.lower() in ('false', 'no', 'off'):
        return False
    return value


def load_state():
    """Load and validate STATE.yaml using PyYAML."""
    if not STATE_PATH.exists():
        raise SystemExit(f'missing state file: {STATE_PATH}')
    raw = yaml.safe_load(STATE_PATH.read_text())
    # Convert string booleans that PyYAML left as strings
    for key in ('paused',):
        if key in raw:
            raw[key] = _bool_like(raw[key])
    return raw


def save_state(state):
    """Write STATE.yaml using PyYAML with a stable field order."""
    state = dict(state)  # don't mutate caller
    state['updated_at'] = datetime.now(timezone.utc).isoformat()

    ordered_keys = [
        'mode', 'repo', 'branch', 'baseline_commit', 'current_run_id',
        'corpus_dir', 'phase', 'current_wave',
        'open_gaps', 'blocked_gaps', 'completed_gaps',
        'last_audit_path', 'last_green_commit',
        'paused', 'stop_reason', 'resume_command',
        'notes', 'updated_at',
    ]

    # Build output with explicit ordering, fill in missing keys with defaults
    output = {}
    for key in ordered_keys:
        if key in state and state[key] not in (None, ''):
            output[key] = state[key]
        elif key in ('open_gaps', 'blocked_gaps', 'completed_gaps'):
            val = state.get(key)
            # Normalize None/missing to []; preserve actual lists
            output[key] = val if isinstance(val, list) else []
        elif key == 'notes':
            output[key] = state.get(key, [])
        elif key == 'paused':
            output[key] = _bool_like(state.get(key, False))
        else:
            output[key] = state.get(key, '')

    STATE_PATH.write_text(yaml.dump(output, default_flow_style=False, sort_keys=False))


# ---------------------------------------------------------------------------
# Gap list parsing
# ---------------------------------------------------------------------------

_GAP_ENTRY_RE = re.compile(
    r'^###\s+(?P<id>[A-Z]+-\d+)\s+[—–]\s+(?P<title>[^\n]+)\n'
    r'(?P<body>(?:(?:  |- ).+\n)*)',
    re.MULTILINE,
)
_SEVERITY_RE = re.compile(r'^- Severity:\s*([^\s]+)', re.MULTILINE)
_STATUS_RE = re.compile(r'^- Status:\s*([^\s]+)', re.MULTILINE)
_AUDIT_CHECK_RE = re.compile(r'^- Audit check:\s*`([^`]+)`', re.MULTILINE)
_EXPECTED_RE = re.compile(r'^- Expected:\s*([^\n]+)', re.MULTILINE)
_ACTUAL_RE = re.compile(r'^- Actual:\s*([^\n]+)', re.MULTILINE)


def parse_gap_list():
    """
    Parse autonomous/GAP_LIST.md and return a list of gap dictionaries::

        {
            'id': 'G-001',
            'title': 'bucket coverage missing',
            'severity': 'P0',
            'status': 'open',
            'audit_check': 'bucket_coverage',
            'expected': '4992',
            'actual': '0',
        }
    """
    if not GAP_LIST_PATH.exists():
        return []
    text = GAP_LIST_PATH.read_text()
    gaps = []
    for m in _GAP_ENTRY_RE.finditer(text):
        gid = m.group('id').strip()
        title = m.group('title').strip()
        body = m.group('body') or ''

        sev_match = _SEVERITY_RE.search(body)
        status_match = _STATUS_RE.search(body)
        check_match = _AUDIT_CHECK_RE.search(body)
        exp_match = _EXPECTED_RE.search(body)
        act_match = _ACTUAL_RE.search(body)

        gaps.append({
            'id': gid,
            'title': title,
            'severity': sev_match.group(1).strip() if sev_match else 'P2',
            'status': status_match.group(1).strip() if status_match else 'unknown',
            'audit_check': check_match.group(1).strip() if check_match else '',
            'expected': exp_match.group(1).strip() if exp_match else '',
            'actual': act_match.group(1).strip() if act_match else '',
        })
    return gaps


# ---------------------------------------------------------------------------
# Wave synthesis
# ---------------------------------------------------------------------------

# Default wave ordering per AUTONOMOUS.md §Phase 4:
WAVE_ORDER = ['data_model', 'core_logic', 'wiring', 'verification']
# Owner area → wave group mapping (heuristic, based on audit check prefixes)
CHECK_TO_WAVE_GROUP = {
    'bucket_coverage': 'data_model',
    'reason_coverage': 'data_model',
    'confidence_coverage': 'data_model',
    'duplicate_presence': 'core_logic',
    'conflict_pairs_threshold': 'core_logic',
    'dependency_edge_quality': 'core_logic',
    'temporal_routing': 'wiring',
    'selected_reason_coverage': 'wiring',
    'artifact_presence': 'verification',
}
# Severity priority for tiebreaking
SEV_PRIORITY = {'P0': 0, 'P1': 1, 'P2': 2, 'P3': 3}


def synthesize_wave():
    """
    Synthesize wave tasks from open gaps in GAP_LIST.md and STATE.yaml.

    Returns a list of task dictionaries, each containing:
      - gap_id        : stable gap identifier
      - title         : human-readable title
      - wave_group    : data_model | core_logic | wiring | verification
      - action        : short imperative description
      - verification_note: how to verify this gap is fixed

    Ordering rules (per AUTONOMOUS.md):
      1. data_model → core_logic → wiring → verification
      2. Within same wave_group, P0 before P1 before P2
      3. Gap IDs are stable across runs (deterministic output)
    """
    state = load_state()
    open_gap_ids = set(state.get('open_gaps', []))
    if not open_gap_ids:
        return []

    gaps = parse_gap_list()
    by_id = {g['id']: g for g in gaps}

    tasks = []
    for gid in sorted(open_gap_ids):
        if gid not in by_id:
            # Gap is listed in STATE.yaml but not parseable from GAP_LIST.md
            tasks.append({
                'gap_id': gid,
                'title': '(unparseable gap)',
                'wave_group': 'verification',
                'action': f'Verify gap {gid} manually.',
                'verification_note': f'Gap {gid} is listed in STATE.yaml but has no entry in {GAP_LIST_PATH}. Manual verification required.',
            })
            continue

        gap = by_id[gid]
        check = gap['audit_check']
        wave_group = CHECK_TO_WAVE_GROUP.get(check, 'verification')
        severity = gap['severity']

        verification_note = (
            f"Gap {gid}: verify via `python3 scripts/audit_guideline.py <run_dir>`; "
            f"check '{check}' passes (expected={gap['expected']}, actual={gap['actual']})."
        )

        tasks.append({
            'gap_id': gid,
            'title': gap['title'],
            'wave_group': wave_group,
            'severity': severity,
            'audit_check': check,
            'action': _action_for_gap(gap),
            'verification_note': verification_note,
        })

    # Stable sort: wave_group order first, then severity, then gap ID
    def sort_key(t):
        wave_idx = WAVE_ORDER.index(t['wave_group']) if t['wave_group'] in WAVE_ORDER else len(WAVE_ORDER)
        sev = SEV_PRIORITY.get(t.get('severity', 'P2'), 3)
        return (wave_idx, sev, t['gap_id'])

    tasks.sort(key=sort_key)
    return tasks


def _action_for_gap(gap):
    """Produce a short imperative action string for a gap."""
    check = gap['audit_check']
    title = gap['title']
    actions = {
        'bucket_coverage': f'Ensure every PR has a bucket field populated. {title}',
        'reason_coverage': f'Ensure every PR has a reason trail in review metadata. {title}',
        'confidence_coverage': f'Ensure every PR has a numeric confidence score. {title}',
        'duplicate_presence': f'Duplicate detection must run and produce groups. {title}',
        'conflict_pairs_threshold': f'Conflict graph must have < 5000 conflict edges. {title}',
        'dependency_edge_quality': f'Dependency edges must not be dominated by trivial same-branch reasons. {title}',
        'temporal_routing': f'PRs must be routed into temporal buckets (now/future/blocked). {title}',
        'selected_reason_coverage': f'Every selected plan item must have a reason field. {title}',
        'artifact_presence': f'All required pipeline artifacts must be produced. {title}',
    }
    return actions.get(check, f'Fix gap {gap["id"]}: {title}')


def format_wave_synthesis(tasks):
    """Format synthesized wave tasks as a human-readable text block."""
    lines = ['# Wave Synthesis', '']
    if not tasks:
        lines.append('No open gaps. Wave synthesis complete.')
        return '\n'.join(lines)

    current_group = None
    for t in tasks:
        if t['wave_group'] != current_group:
            current_group = t['wave_group']
            lines.append(f'## {current_group.replace("_", " ").title()} Wave')
            lines.append('')
        lines.append(f"### {t['gap_id']} — {t['title']}")
        lines.append(f"- Action: {t['action']}")
        lines.append(f"- Verification: {t['verification_note']}")
        lines.append('')
    return '\n'.join(lines)


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------

def cmd_init(args):
    """Initialize or refresh STATE.yaml checkpoint."""
    state = load_state() if STATE_PATH.exists() else {}
    # Capture git truth at init time
    import subprocess
    try:
        branch = subprocess.check_output(
            ['git', '-C', str(REPO_ROOT), 'rev-parse', '--abbrev-ref', 'HEAD'],
            text=True).strip()
        commit = subprocess.check_output(
            ['git', '-C', str(REPO_ROOT), 'rev-parse', '--short', 'HEAD'],
            text=True).strip()
    except subprocess.CalledProcessError:
        branch = state.get('branch', 'unknown')
        commit = state.get('baseline_commit', 'unknown')

    state.update({
        'mode': 'active',
        'repo': args.repo,
        'branch': state.get('branch', branch),  # preserve existing branch
        'baseline_commit': state.get('baseline_commit', commit),  # preserve existing baseline
        'current_run_id': Path(args.corpus_dir).name,
        'corpus_dir': args.corpus_dir,
        'phase': state.get('phase', 'bootstrap'),
        'current_wave': state.get('current_wave', '0'),
        'paused': False,
        'stop_reason': '',
        'resume_command': 'python3 scripts/autonomous_controller.py resume',
    })
    save_state(state)
    print(f'initialized autonomous state at {STATE_PATH}')


def cmd_reconcile(_args):
    """Reconcile repo truth and clear any verification pause."""
    state = load_state()
    state['mode'] = 'active'
    state['paused'] = False
    if state.get('stop_reason') == 'verification pause':
        state['stop_reason'] = ''
    # Refresh git info
    import subprocess
    try:
        state['branch'] = subprocess.check_output(
            ['git', '-C', str(REPO_ROOT), 'rev-parse', '--abbrev-ref', 'HEAD'],
            text=True).strip()
        state['baseline_commit'] = subprocess.check_output(
            ['git', '-C', str(REPO_ROOT), 'rev-parse', '--short', 'HEAD'],
            text=True).strip()
    except subprocess.CalledProcessError:
        pass
    save_state(state)
    print('reconciled state')


def cmd_next_wave(_args):
    """Advance to the next wave number."""
    state = load_state()
    wave = int(str(state.get('current_wave', '0')))
    state['current_wave'] = str(wave + 1)
    state['phase'] = f'wave_{wave + 1}'
    save_state(state)
    print(f"advanced to {state['phase']}")


def cmd_pause(args):
    """Pause the autonomous loop."""
    state = load_state()
    state['paused'] = True
    state['stop_reason'] = args.reason
    state['mode'] = 'paused'
    save_state(state)
    print(f'paused: {args.reason}')


def verify_state_consistency():
    """
    Verify that STATE.yaml and GAP_LIST.md are internally consistent.

    Checks:
      - Every gap ID in open_gaps exists in GAP_LIST.md (no orphaned references)
      - No gap ID appears in both open_gaps and completed_gaps (duplicate entry)

    Returns a list of issue dictionaries, each with keys:
      - type: 'orphaned_gap_id' | 'duplicate_gap_id'
      - gap_id: the problematic gap ID
      - message: human-readable description

    An empty list means the state is consistent.
    """
    state = load_state()
    gaps = parse_gap_list()
    gap_ids_in_list = {g['id'] for g in gaps}

    issues = []

    open_gaps = state.get('open_gaps') or []
    completed_gaps = state.get('completed_gaps') or []

    # Check for orphaned gap IDs in open_gaps
    for gid in open_gaps:
        if gid not in gap_ids_in_list:
            issues.append({
                'type': 'orphaned_gap_id',
                'gap_id': gid,
                'message': f"gap {gid} is in open_gaps but not found in GAP_LIST.md; run 'closeout' or 'audit-state' to reconcile",
            })

    # Check for orphaned gap IDs in completed_gaps
    for gid in completed_gaps:
        if gid not in gap_ids_in_list:
            issues.append({
                'type': 'orphaned_gap_id',
                'gap_id': gid,
                'message': f"gap {gid} is in completed_gaps but not found in GAP_LIST.md; this is non-fatal but unusual",
            })

    # Check for gap IDs appearing in both open and completed
    open_set = set(open_gaps)
    completed_set = set(completed_gaps)
    for gid in open_set & completed_set:
        issues.append({
            'type': 'duplicate_gap_id',
            'gap_id': gid,
            'message': f"gap {gid} appears in both open_gaps and completed_gaps; call 'closeout' to reconcile",
        })

    return issues


def cmd_audit_state(_args):
    """Check STATE.yaml consistency against GAP_LIST.md and print issues."""
    issues = verify_state_consistency()
    if not issues:
        print('audit-state: STATE.yaml is consistent with GAP_LIST.md — no issues found')
    else:
        print(f'audit-state: found {len(issues)} issue(s):')
        for issue in issues:
            print(f"  [{issue['type']}] {issue['message']}")


def cmd_resume(_args):
    """Resume from a paused state or recover from an interruption."""
    state = load_state()
    # Guard: do not resurrect a completed session
    if state.get('mode') == 'complete':
        print(f"resume: session is complete, cannot resume from phase={state.get('phase')} wave={state.get('current_wave')}")
        return
    state['paused'] = False
    state['mode'] = 'active'
    state['stop_reason'] = ''
    if state.get('phase') in ('', 'bootstrap'):
        state['phase'] = 'reconcile'
    save_state(state)
    print(f"resume from phase={state.get('phase')} wave={state.get('current_wave')}")
    # Run post-resume consistency check to detect any state corruption
    issues = verify_state_consistency()
    if issues:
        for issue in issues:
            print(f"WARNING: {issue['message']}", file=sys.stderr)


def cmd_closeout(args):
    """
    Mark verified-gap IDs as closed after a verified wave.

    After the subagent verifies gaps are fixed (audit passes), closeout moves
    those gap IDs from open_gaps to completed_gaps in STATE.yaml, regenerates
    GAP_LIST.md from the latest audit, and appends a note to TODO.md.

    Args:
        gaps: comma-separated list of gap IDs that were verified (e.g. 'G-001,G-002')
        audit_path: path to AUDIT_RESULTS.json for GAP_LIST.md regeneration (optional)
        todo_path: path to TODO.md to append closeout note (optional)
    """
    state = load_state()
    gap_ids = [g.strip() for g in args.gaps.split(',') if g.strip()]

    if not gap_ids:
        print('closeout: no gaps specified, nothing to do')
        return

    open_gaps = list(state.get('open_gaps', []))
    completed_gaps = list(state.get('completed_gaps', []))

    # Move each gap from open to completed (idempotent: skip if not in open_gaps)
    for gid in gap_ids:
        if gid in open_gaps:
            open_gaps.remove(gid)
            if gid not in completed_gaps:
                completed_gaps.append(gid)

    state['open_gaps'] = open_gaps
    state['completed_gaps'] = completed_gaps
    save_state(state)

    # Regenerate GAP_LIST.md from audit if audit_path provided
    if args.audit_path:
        _regenerate_gap_list_from_audit(Path(args.audit_path))

    # Append closeout note to TODO.md if path provided
    if args.todo_path:
        _append_closeout_note(Path(args.todo_path), gap_ids, state.get('current_wave', '?'))

    wave = state.get('current_wave', '?')
    print(f'closeout: marked {len(gap_ids)} gap(s) as verified-closed (wave {wave})')


def _regenerate_gap_list_from_audit(audit_path: Path):
    """
    Regenerate GAP_LIST.md from a full audit results file.
    Only gaps that still fail remain as 'open'; verified gaps are excluded.
    """
    if not audit_path.exists():
        print(f'closeout: audit file not found: {audit_path}')
        return

    import json
    try:
        audit_data = json.loads(audit_path.read_text())
    except (json.JSONDecodeError, OSError) as e:
        print(f'closeout: could not read audit file: {e}')
        return

    checks = audit_data.get('checks', [])
    failures = [c for c in checks if c.get('status') == 'fail']

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
            check_id = check.get('id', '')
            gap_id, title, sev = GAP_MAP.get(check_id, (f'X-{check_id}', check.get('label', ''), 'P2'))
            actual = check.get('actual', '')
            if isinstance(actual, dict):
                actual = json.dumps(actual)
            body.append(f'### {gap_id} — {title}')
            body.append(f'- Audit check: `{check_id}`')
            body.append(f'- Severity: {sev}')
            body.append(f"- Expected: {check.get('expected', '')}")
            body.append(f'- Actual: {actual}')
            body.append('- Status: open')
            body.append('- Notes: generated from latest audit failure')
            body.append('')
    else:
        body.append('No open gaps. Latest audit passed.')
        body.append('')

    body.extend([
        '## Update protocol',
        '',
        'This file is generated from audit output. Preserve stable gap IDs where possible.',
        ''
    ])

    GAP_LIST_PATH.write_text('\n'.join(body))


def _append_closeout_note(todo_path: Path, gap_ids, wave):
    """Append a closeout note to TODO.md."""
    if not todo_path.exists():
        print(f'closeout: TODO.md not found at {todo_path}')
        return

    ts = datetime.now(timezone.utc).strftime('%Y-%m-%d')
    note = (
        f'\n## Wave {wave} closeout ({ts})\n'
        f'- Verified and closed: {", ".join(gap_ids)}\n'
    )

    try:
        existing = todo_path.read_text()
        todo_path.write_text(existing.rstrip() + note + '\n')
    except OSError as e:
        print(f'closeout: could not write TODO.md: {e}')


def cmd_complete(args):
    """Mark the autonomous session complete."""
    state = load_state()
    state['mode'] = 'complete'
    state['phase'] = 'complete'
    state['paused'] = False
    state['stop_reason'] = args.reason
    save_state(state)
    print(f'completed: {args.reason}')


def cmd_synthesize_wave(_args):
    """Synthesize wave tasks from open gaps; print to stdout."""
    tasks = synthesize_wave()
    print(format_wave_synthesis(tasks))
    # Also emit machine-readable YAML for downstream consumption
    print('--- YAML ---')
    print(yaml.dump({'wave_tasks': tasks}, default_flow_style=False, sort_keys=False))


# ---------------------------------------------------------------------------
# Per-run artifact discipline
# ---------------------------------------------------------------------------

RUNS_DIR = REPO_ROOT / 'autonomous' / 'runs'


def run_dir_path(timestamp):
    """Return the path to a run directory for the given timestamp (no side effects)."""
    return RUNS_DIR / timestamp


def make_run_dir(timestamp=None):
    """
    Create and return the per-run artifact directory for the given timestamp.

    Creates autonomous/runs/<timestamp>/ with a subagent-results/ subdirectory.
    If timestamp is None, generates one from current UTC time in YYYYMMDD-HHMMSS format.
    Idempotent: if the directory already exists, returns the existing path.

    Returns:
        Path to the created (or existing) run directory.
    """
    if timestamp is None:
        timestamp = datetime.now(timezone.utc).strftime('%Y%m%d-%H%M%S')

    run_dir = RUNS_DIR / timestamp
    subagent_results = run_dir / 'subagent-results'

    run_dir.mkdir(parents=True, exist_ok=True)
    subagent_results.mkdir(parents=True, exist_ok=True)

    return run_dir


def write_controller_log(run_dir, log_lines):
    """
    Write controller-log.md into the run directory.

    Args:
        run_dir: Path to the run directory (created by make_run_dir)
        log_lines: list of log entry strings to write
    """
    log_file = run_dir / 'controller-log.md'
    header = '# Controller log\n\n'
    entries = '\n'.join(log_lines) + '\n' if log_lines else ''
    log_file.write_text(header + entries)


def write_wave_summary(run_dir, summary_lines):
    """
    Write wave-summary.md into the run directory.

    Args:
        run_dir: Path to the run directory (created by make_run_dir)
        summary_lines: list of summary entry strings to write
    """
    summary_file = run_dir / 'wave-summary.md'
    header = '# Wave summary\n\n'
    entries = '\n'.join(summary_lines) + '\n' if summary_lines else ''
    summary_file.write_text(header + entries)


def link_or_copy_audit(run_dir, audit_path):
    """
    Copy AUDIT_RESULTS.json into the run directory if the source file exists.

    Uses copy (not symlink) for portability across filesystems and to ensure
    the artifact persists independently of the source.

    Args:
        run_dir: Path to the run directory (created by make_run_dir)
        audit_path: path to the source AUDIT_RESULTS.json file
    """
    import shutil

    src = Path(audit_path)
    if not src.exists():
        return  # silent no-op if source is missing

    dst = run_dir / 'AUDIT_RESULTS.json'
    shutil.copy2(src, dst)


def cmd_new_run(args):
    """
    Create a new per-run artifact directory with all required artifacts.

    Args:
        timestamp:   string timestamp for the run directory name
        log_lines:   list of strings for controller-log.md
        summary_lines: list of strings for wave-summary.md
        audit_path:  optional path to AUDIT_RESULTS.json to copy
    """
    run_dir = make_run_dir(timestamp=args.timestamp)

    if args.log_lines:
        write_controller_log(run_dir, args.log_lines)

    if args.summary_lines:
        write_wave_summary(run_dir, args.summary_lines)

    if args.audit_path:
        link_or_copy_audit(run_dir, args.audit_path)

    print(f'created run directory: {run_dir}')


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description='Autonomous controller for prATC — state management only.')
    sub = parser.add_subparsers(dest='cmd', required=True)

    p = sub.add_parser('init')
    p.add_argument('--repo', required=True)
    p.add_argument('--corpus-dir', required=True)
    p.set_defaults(func=cmd_init)

    p = sub.add_parser('reconcile')
    p.set_defaults(func=cmd_reconcile)

    p = sub.add_parser('next-wave')
    p.set_defaults(func=cmd_next_wave)

    p = sub.add_parser('pause')
    p.add_argument('--reason', required=True)
    p.set_defaults(func=cmd_pause)

    p = sub.add_parser('resume')
    p.set_defaults(func=cmd_resume)

    p = sub.add_parser('complete')
    p.add_argument('--reason', required=True)
    p.set_defaults(func=cmd_complete)

    p = sub.add_parser('closeout')
    p.add_argument('--gaps', required=True, help='comma-separated gap IDs verified in this wave (e.g. G-001,G-002)')
    p.add_argument('--audit-path', help='path to AUDIT_RESULTS.json for GAP_LIST.md regeneration')
    p.add_argument('--todo-path', help='path to TODO.md for closeout note')
    p.set_defaults(func=cmd_closeout)

    p = sub.add_parser('synthesize-wave')
    p.set_defaults(func=cmd_synthesize_wave)

    p = sub.add_parser('audit-state')
    p.set_defaults(func=cmd_audit_state)

    p = sub.add_parser('new-run')
    p.add_argument('--timestamp', default=None, help='timestamp for run directory name (default: current UTC time)')
    p.add_argument('--log-lines', nargs='*', default=[], help='log entry lines for controller-log.md')
    p.add_argument('--summary-lines', nargs='*', default=[], help='summary lines for wave-summary.md')
    p.add_argument('--audit-path', default=None, help='path to AUDIT_RESULTS.json to copy into run directory')
    p.set_defaults(func=cmd_new_run)

    args = parser.parse_args()
    args.func(args)


if __name__ == '__main__':
    main()
