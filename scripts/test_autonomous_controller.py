#!/usr/bin/env python3
"""Tests for autonomous_controller.py and wave synthesis."""
import argparse
import os
import sys
import tempfile
import yaml
from pathlib import Path

import pytest

# Import the module under test
sys.path.insert(0, str(Path(__file__).parent))
import autonomous_controller as ctrl

REPO_ROOT = Path('/home/agent/pratc')


class FakeArgs:
    """Simple namespace for fake parsed args."""
    def __init__(self, **kwargs):
        self.__dict__.update(kwargs)


@pytest.fixture
def tmp_paths(tmp_path):
    """Create temp state and gap list files, patch controller constants."""
    state = tmp_path / 'STATE.yaml'
    gap = tmp_path / 'GAP_LIST.md'
    ctrl.STATE_PATH = state
    ctrl.GAP_LIST_PATH = gap
    return state, gap


@pytest.fixture
def sample_state():
    """Minimal valid STATE.yaml structure."""
    return {
        'mode': 'active',
        'repo': 'openclaw/openclaw',
        'branch': 'main',
        'baseline_commit': '1b84d21',
        'current_run_id': '20260419-065654',
        'corpus_dir': 'projects/OpenClaw_OpenClaw/runs/20260419-065654',
        'phase': 'wave_1',
        'current_wave': '1',
        'open_gaps': ['G-001', 'G-002', 'G-003', 'G-006', 'G-005', 'G-004'],
        'blocked_gaps': [],
        'completed_gaps': [],
        'last_audit_path': 'projects/OpenClaw_OpenClaw/runs/20260419-065654/AUDIT_RESULTS.json',
        'last_green_commit': '1b84d21',
        'paused': False,
        'stop_reason': '',
        'resume_command': 'python3 scripts/autonomous_controller.py resume',
        'updated_at': '2026-04-19T16:43:23.946990+00:00',
        'notes': ['Initial scaffold.'],
    }


@pytest.fixture
def sample_gap_list():
    """Minimal GAP_LIST.md content."""
    return """# Autonomous Gap List

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

### G-006 — temporal routing not visible
- Audit check: `temporal_routing`
- Severity: P1
- Expected: > 0 temporal buckets
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
- Actual: {'depends_on_edges': 804678, 'trivial_dep_edges': 804678}
- Status: open
- Notes: generated from latest audit failure

## Update protocol

This file is generated from audit output. Preserve stable gap IDs where possible.
"""


# ---------------------------------------------------------------------------
# Tests: parse_yamlish / dump_yamlish round-trip
# ---------------------------------------------------------------------------

def test_parse_state_yaml(tmp_paths, sample_state):
    """PyYAML parse + round-trip matches original data."""
    state_path, _ = tmp_paths
    state_path.write_text(yaml.dump(sample_state))
    result = ctrl.load_state()
    assert result['mode'] == 'active'
    assert result['repo'] == 'openclaw/openclaw'
    assert result['open_gaps'] == ['G-001', 'G-002', 'G-003', 'G-006', 'G-005', 'G-004']
    assert result['paused'] is False


def test_save_and_load_state(tmp_paths, sample_state):
    """State written by save_state is readable by load_state."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)
    loaded = ctrl.load_state()
    assert loaded['mode'] == 'active'
    assert loaded['current_wave'] == '1'
    assert loaded['open_gaps'] == ['G-001', 'G-002', 'G-003', 'G-006', 'G-005', 'G-004']
    assert isinstance(loaded['updated_at'], str)
    assert len(loaded['updated_at']) > 0


def test_parse_missing_state_file(tmp_paths):
    """load_state raises SystemExit when file is absent."""
    state_path, _ = tmp_paths
    # STATE_PATH already set to tmp_paths[0] which doesn't exist
    with pytest.raises(SystemExit) as exc:
        ctrl.load_state()
    assert 'missing state file' in str(exc.value)


# ---------------------------------------------------------------------------
# Tests: pause / resume
# ---------------------------------------------------------------------------

def test_pause_sets_flags(tmp_paths, sample_state):
    """pause command sets paused=True, mode=paused, stop_reason."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)
    args = FakeArgs(reason='test hold')
    ctrl.cmd_pause(args)
    state = ctrl.load_state()
    assert state['paused'] is True
    assert state['mode'] == 'paused'
    assert state['stop_reason'] == 'test hold'


def test_resume_clears_paused(tmp_paths, sample_state):
    """resume command clears paused flag and restores mode=active."""
    state_path, _ = tmp_paths
    sample_state['paused'] = True
    sample_state['mode'] = 'paused'
    sample_state['stop_reason'] = 'prior hold'
    ctrl.save_state(sample_state)
    args = FakeArgs()
    ctrl.cmd_resume(args)
    state = ctrl.load_state()
    assert state['paused'] is False
    assert state['mode'] == 'active'
    assert state['stop_reason'] == ''


def test_resume_sets_phase_to_reconcile_when_bootstrap(tmp_paths, sample_state):
    """resume from bootstrap phase transitions to reconcile."""
    state_path, _ = tmp_paths
    sample_state['phase'] = 'bootstrap'
    ctrl.save_state(sample_state)
    args = FakeArgs()
    ctrl.cmd_resume(args)
    state = ctrl.load_state()
    assert state['phase'] == 'reconcile'


# ---------------------------------------------------------------------------
# Tests: reconcile
# ---------------------------------------------------------------------------

def test_reconcile_resets_mode_and_clears_verification_pause(tmp_paths, sample_state):
    """reconcile clears 'verification pause' stop_reason and sets mode=active."""
    state_path, _ = tmp_paths
    sample_state['mode'] = 'paused'
    sample_state['paused'] = True
    sample_state['stop_reason'] = 'verification pause'
    ctrl.save_state(sample_state)
    args = FakeArgs()
    ctrl.cmd_reconcile(args)
    state = ctrl.load_state()
    assert state['mode'] == 'active'
    assert state['paused'] is False
    assert state['stop_reason'] == ''


# ---------------------------------------------------------------------------
# Tests: next-wave
# ---------------------------------------------------------------------------

def test_next_wave_increments_counter(tmp_paths, sample_state):
    """next-wave increments current_wave and updates phase."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)
    args = FakeArgs()
    ctrl.cmd_next_wave(args)
    state = ctrl.load_state()
    assert state['current_wave'] == '2'
    assert state['phase'] == 'wave_2'


def test_next_wave_from_zero(tmp_paths, sample_state):
    """next-wave handles wave='0' correctly."""
    state_path, _ = tmp_paths
    sample_state['current_wave'] = '0'
    sample_state['phase'] = 'bootstrap'
    ctrl.save_state(sample_state)
    args = FakeArgs()
    ctrl.cmd_next_wave(args)
    state = ctrl.load_state()
    assert state['current_wave'] == '1'
    assert state['phase'] == 'wave_1'


# ---------------------------------------------------------------------------
# Tests: complete
# ---------------------------------------------------------------------------

def test_complete_sets_final_state(tmp_paths, sample_state):
    """complete marks mode=complete, phase=complete, stores reason."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)
    args = FakeArgs(reason='all required audit checks passed')
    ctrl.cmd_complete(args)
    state = ctrl.load_state()
    assert state['mode'] == 'complete'
    assert state['phase'] == 'complete'
    assert state['stop_reason'] == 'all required audit checks passed'
    assert state['paused'] is False


# ---------------------------------------------------------------------------
# Tests: init
# ---------------------------------------------------------------------------

def test_init_creates_new_state(tmp_paths):
    """init creates a fresh state from scratch."""
    state_path, _ = tmp_paths
    args = FakeArgs(repo='test/repo', corpus_dir='/tmp/corpus')
    ctrl.cmd_init(args)
    state = ctrl.load_state()
    assert state['mode'] == 'active'
    assert state['repo'] == 'test/repo'
    assert state['corpus_dir'] == '/tmp/corpus'
    assert state['phase'] == 'bootstrap'
    assert state['current_wave'] == '0'
    assert state['paused'] is False


def test_init_preserves_existing_base_fields(tmp_paths, sample_state):
    """init on existing state preserves baseline_commit, branch, etc."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)
    args = FakeArgs(repo='new/repo', corpus_dir='/tmp/new_corpus')
    ctrl.cmd_init(args)
    state = ctrl.load_state()
    # baseline_commit should be preserved from existing state
    assert state['baseline_commit'] == '1b84d21'
    assert state['repo'] == 'new/repo'


# ---------------------------------------------------------------------------
# Tests: gap-list parsing
# ---------------------------------------------------------------------------

def test_gap_list_parsing(tmp_paths, sample_gap_list):
    """parse_gap_list returns structured gap entries from GAP_LIST.md."""
    _, gap_path = tmp_paths
    gap_path.write_text(sample_gap_list)
    gaps = ctrl.parse_gap_list()
    assert len(gaps) == 6
    ids = [g['id'] for g in gaps]
    assert 'G-001' in ids
    assert 'G-004' in ids
    assert 'G-006' in ids


def test_gap_list_extracts_severity_and_status(tmp_paths, sample_gap_list):
    """Gap entries include severity and status."""
    _, gap_path = tmp_paths
    gap_path.write_text(sample_gap_list)
    gaps = ctrl.parse_gap_list()
    by_id = {g['id']: g for g in gaps}
    assert by_id['G-001']['severity'] == 'P0'
    assert by_id['G-001']['status'] == 'open'
    assert by_id['G-005']['severity'] == 'P1'


# ---------------------------------------------------------------------------
# Tests: wave synthesis
# ---------------------------------------------------------------------------

def test_synthesize_wave_produces_wave_tasks(tmp_paths, sample_state, sample_gap_list):
    """synthesize-wave maps open gaps to wave tasks with ordering and notes."""
    state_path, gap_path = tmp_paths
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    tasks = ctrl.synthesize_wave()

    assert len(tasks) > 0
    # Every task needs an id, gap_id, action, verification_note
    for t in tasks:
        assert 'gap_id' in t
        assert 'action' in t
        assert 'verification_note' in t
        assert 'wave_group' in t
    # Gap IDs from open gaps must be covered
    open_gap_ids = set(sample_state['open_gaps'])
    synthesized_ids = {t['gap_id'] for t in tasks}
    assert synthesized_ids == open_gap_ids


def test_synthesize_wave_ordering_respects_default_rules(tmp_paths, sample_state, sample_gap_list):
    """P0 gaps appear before P1 gaps in synthesis output."""
    state_path, gap_path = tmp_paths
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    tasks = ctrl.synthesize_wave()

    # Find first P1 in the synthesized sequence
    p0_gaps = {'G-001', 'G-002', 'G-003'}
    p1_gaps = {'G-004', 'G-005', 'G-006'}
    p0_positions = [i for i, t in enumerate(tasks) if t['gap_id'] in p0_gaps]
    p1_positions = [i for i, t in enumerate(tasks) if t['gap_id'] in p1_gaps]

    if p0_positions and p1_positions:
        assert max(p0_positions) < min(p1_positions), \
            "P0 gaps should precede P1 gaps per default wave ordering"


def test_synthesize_wave_deterministic(tmp_paths, sample_state, sample_gap_list):
    """Running synthesis twice produces identical output."""
    state_path, gap_path = tmp_paths
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    tasks_a = ctrl.synthesize_wave()
    tasks_b = ctrl.synthesize_wave()

    assert tasks_a == tasks_b, "Wave synthesis must be deterministic"


def test_synthesize_wave_includes_verification_notes(tmp_paths, sample_state, sample_gap_list):
    """Each synthesized task has a verification note that references audit check."""
    state_path, gap_path = tmp_paths
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    tasks = ctrl.synthesize_wave()
    for t in tasks:
        assert len(t['verification_note']) > 0
        # verification_note should mention the audit check or expected value
        assert t['gap_id'] in t['verification_note'] or t['action'] in t['verification_note']


def test_synthesize_wave_empty_when_no_open_gaps(tmp_paths, sample_state):
    """No open gaps means no tasks synthesized."""
    state_path, gap_path = tmp_paths
    sample_state['open_gaps'] = []
    ctrl.save_state(sample_state)
    gap_path.write_text("# Autonomous Gap List\n\nNo open gaps.\n")
    tasks = ctrl.synthesize_wave()
    assert tasks == []


def test_rebuild_session_todo_from_repo_local_state(tmp_paths, sample_state, sample_gap_list):
    """
    Prove /autonomous can rebuild the Hermes session todo entirely from
    repo-local state and latest audit output.

    This test verifies that given only:
    - autonomous/STATE.yaml (open_gaps list)
    - autonomous/GAP_LIST.md (gap details)

    The synthesize_wave() function produces the session todo with no
    hidden chat memory or external context required.

    This is the core proof that a new controller session can resume from
    repo-local state without hidden chat context (exit criteria item 1).
    """
    state_path, gap_path = tmp_paths
    # Write only repo-local files - no chat memory, no external state
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    # Rebuild session todo purely from repo-local files
    tasks = ctrl.synthesize_wave()

    # Verify we got tasks for every open gap
    assert len(tasks) == 6, f"Expected 6 tasks, got {len(tasks)}"
    synthesized_gap_ids = {t['gap_id'] for t in tasks}
    assert synthesized_gap_ids == set(sample_state['open_gaps'])

    # Verify each task has the required fields for the session todo
    for t in tasks:
        assert 'gap_id' in t
        assert 'title' in t
        assert 'wave_group' in t
        assert 'action' in t
        assert 'verification_note' in t
        # verification_note must reference the audit check
        assert 'audit' in t['verification_note'].lower() or 'audit' in t['action'].lower()

    # Verify ordering is deterministic (run twice, get same result)
    tasks2 = ctrl.synthesize_wave()
    assert tasks == tasks2, "Session todo rebuild must be deterministic"


def test_synthesize_wave_cmd_executable(tmp_paths, sample_state, sample_gap_list, capsys):
    """The synthesize-wave subcommand runs without error and prints tasks."""
    state_path, gap_path = tmp_paths
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    args = FakeArgs()  # no extra fields needed
    ctrl.cmd_synthesize_wave(args)

    out = capsys.readouterr().out
    assert 'G-001' in out
    assert 'wave_group' in out.lower() or 'wave' in out.lower()


def test_rebuild_session_todo_cli_output(tmp_paths, sample_state, sample_gap_list, capsys):
    """
    Verify the synthesize-wave CLI command produces machine-readable YAML
    that the /autonomous skill can consume for session todo reconstruction.

    This test proves the CLI is usable by external callers (the /autonomous
    skill) for repo-local session rebuild.
    """
    state_path, gap_path = tmp_paths
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    args = FakeArgs()
    ctrl.cmd_synthesize_wave(args)

    out = capsys.readouterr().out
    # Must have YAML marker indicating machine-readable output
    assert '--- YAML ---' in out
    # Must have wave_tasks key with list value
    assert 'wave_tasks:' in out
    # Must have the gap IDs
    assert 'G-001' in out
    assert 'G-002' in out


# ---------------------------------------------------------------------------
# Tests: null-list normalization across save/load
# ---------------------------------------------------------------------------

def test_null_gap_lists_normalized_to_empty_on_save(tmp_paths, sample_state):
    """blocked_gaps/completed_gaps: null must become [] after a save/load cycle."""
    state_path, _ = tmp_paths
    # Simulate the current STATE.yaml state: null lists instead of empty lists
    sample_state['blocked_gaps'] = None
    sample_state['completed_gaps'] = None
    ctrl.save_state(sample_state)

    # Read raw YAML to verify null is not written
    raw = yaml.safe_load(state_path.read_text())
    assert raw['blocked_gaps'] == [], f"expected [], got {raw['blocked_gaps']!r}"
    assert raw['completed_gaps'] == [], f"expected [], got {raw['completed_gaps']!r}"

    # Also verify load round-trips cleanly
    loaded = ctrl.load_state()
    assert loaded['blocked_gaps'] == []
    assert loaded['completed_gaps'] == []


def test_empty_open_gaps_serializes_as_list_not_null(tmp_paths, sample_state):
    """open_gaps: [] must serialize as a YAML list, not null or absent."""
    state_path, _ = tmp_paths
    sample_state['open_gaps'] = []
    ctrl.save_state(sample_state)

    raw = yaml.safe_load(state_path.read_text())
    assert 'open_gaps' in raw, "open_gaps key must be present in output"
    assert raw['open_gaps'] == [], f"expected [], got {raw['open_gaps']!r}"


# ---------------------------------------------------------------------------
# Tests: repeated reconcile/pause/resume/next-wave/complete cycles
# ---------------------------------------------------------------------------

def test_three_consecutive_pause_resume_cycles_preserve_all_fields(tmp_paths, sample_state):
    """Three pause→resume cycles must preserve every state field truthfully."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)

    # Capture baseline fields that must survive all cycles
    baseline_fields = {
        'mode', 'repo', 'branch', 'baseline_commit', 'current_run_id',
        'corpus_dir', 'phase', 'current_wave', 'open_gaps', 'blocked_gaps',
        'completed_gaps', 'last_audit_path', 'last_green_commit',
        'resume_command', 'notes',
    }

    for cycle in range(3):
        # Pause
        ctrl.cmd_pause(FakeArgs(reason=f'hold {cycle}'))
        state = ctrl.load_state()
        assert state['paused'] is True, f"cycle {cycle}: paused should be True"
        assert state['mode'] == 'paused', f"cycle {cycle}: mode should be paused"

        # Resume
        ctrl.cmd_resume(FakeArgs())
        state = ctrl.load_state()
        assert state['paused'] is False, f"cycle {cycle}: paused should be False"
        assert state['mode'] == 'active', f"cycle {cycle}: mode should be active"
        assert state['stop_reason'] == '', f"cycle {cycle}: stop_reason should be empty"

        # Verify stable fields unchanged across the full cycle
        for f in baseline_fields:
            if f == 'updated_at':
                continue  # updated_at changes on every save
            assert state.get(f) == sample_state.get(f), \
                f"cycle {cycle}: field {f} changed: {state.get(f)!r} vs {sample_state.get(f)!r}"


def test_reconcile_preserves_wave_and_phase(tmp_paths, sample_state):
    """reconcile must not clobber current_wave or phase."""
    state_path, _ = tmp_paths
    sample_state['current_wave'] = '2'
    sample_state['phase'] = 'wave_2'
    ctrl.save_state(sample_state)

    ctrl.cmd_reconcile(FakeArgs())

    state = ctrl.load_state()
    assert state['current_wave'] == '2', f"current_wave was clobbered: {state['current_wave']!r}"
    assert state['phase'] == 'wave_2', f"phase was clobbered: {state['phase']!r}"


def test_next_wave_then_pause_then_resume_full_cycle(tmp_paths, sample_state):
    """next-wave → pause → resume preserves the advanced wave number."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)

    ctrl.cmd_next_wave(FakeArgs())
    state = ctrl.load_state()
    assert state['current_wave'] == '2'
    assert state['phase'] == 'wave_2'

    ctrl.cmd_pause(FakeArgs(reason='mid-wave hold'))
    state = ctrl.load_state()
    assert state['paused'] is True
    assert state['current_wave'] == '2'

    ctrl.cmd_resume(FakeArgs())
    state = ctrl.load_state()
    assert state['paused'] is False
    assert state['current_wave'] == '2', "wave must be preserved across pause/resume"
    assert state['phase'] == 'wave_2'


def test_resume_from_active_mode_does_not_clobber_phase(tmp_paths, sample_state):
    """resume when not paused (active mode) must not reset phase to reconcile."""
    state_path, _ = tmp_paths
    sample_state['phase'] = 'wave_3'
    sample_state['current_wave'] = '3'
    sample_state['paused'] = False
    sample_state['mode'] = 'active'
    ctrl.save_state(sample_state)

    ctrl.cmd_resume(FakeArgs())

    state = ctrl.load_state()
    # phase should stay wave_3 since we were already active, not bootstrap
    assert state['phase'] == 'wave_3', f"phase clobbered: got {state['phase']!r}, want wave_3"


def test_complete_then_resume_is_idempotent(tmp_paths, sample_state):
    """complete sets mode=complete; subsequent resume must not resurrect active state."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)

    ctrl.cmd_complete(FakeArgs(reason='all done'))
    state = ctrl.load_state()
    assert state['mode'] == 'complete'
    assert state['phase'] == 'complete'

    # Trying to resume from complete is a no-op on mode (stays complete)
    ctrl.cmd_resume(FakeArgs())
    state = ctrl.load_state()
    assert state['mode'] == 'complete', "resume should not resurrect complete mode"
    assert state['phase'] == 'complete'


def test_updated_at_changes_on_every_save(tmp_paths, sample_state):
    """updated_at must be fresh after each save, proving the timestamp is not stale."""
    state_path, _ = tmp_paths
    ctrl.save_state(sample_state)
    state1 = ctrl.load_state()
    ts1 = state1['updated_at']

    import time
    time.sleep(0.01)  # ensure different timestamp

    ctrl.cmd_pause(FakeArgs(reason='test'))
    state2 = ctrl.load_state()
    ts2 = state2['updated_at']

    assert ts1 != ts2, f"updated_at did not change: {ts1!r} == {ts2!r}"


def test_init_does_not_reset_baseline_commit_if_already_set(tmp_paths, sample_state):
    """init must not overwrite baseline_commit when state already has one."""
    state_path, _ = tmp_paths
    sample_state['baseline_commit'] = 'abc1234'
    ctrl.save_state(sample_state)

    ctrl.cmd_init(FakeArgs(repo='same/repo', corpus_dir='/tmp/same'))

    state = ctrl.load_state()
    assert state['baseline_commit'] == 'abc1234'


def test_init_does_not_reset_branch_if_already_set(tmp_paths, sample_state):
    """init must not overwrite branch when state already has one."""
    state_path, _ = tmp_paths
    sample_state['branch'] = 'release-v1'
    ctrl.save_state(sample_state)

    ctrl.cmd_init(FakeArgs(repo='same/repo', corpus_dir='/tmp/same'))

    state = ctrl.load_state()
    assert state['branch'] == 'release-v1'


# ---------------------------------------------------------------------------
# Tests: closeout discipline
# ---------------------------------------------------------------------------

def test_closeout_moves_gaps_from_open_to_completed(tmp_paths, sample_state):
    """closeout moves specified gap IDs from open_gaps to completed_gaps."""
    state_path, gap_path = tmp_paths
    # Set up state with 3 open gaps
    sample_state['open_gaps'] = ['G-001', 'G-002', 'G-003']
    sample_state['completed_gaps'] = []
    ctrl.save_state(sample_state)

    # Closeout G-001 only
    args = FakeArgs(gaps='G-001', audit_path=None, todo_path=None)
    ctrl.cmd_closeout(args)

    state = ctrl.load_state()
    assert state['open_gaps'] == ['G-002', 'G-003'], f"open_gaps should only have remaining: {state['open_gaps']}"
    assert state['completed_gaps'] == ['G-001'], f"completed_gaps should have G-001: {state['completed_gaps']}"


def test_closeout_moves_multiple_gaps(tmp_paths, sample_state):
    """closeout can move multiple gap IDs at once."""
    state_path, gap_path = tmp_paths
    sample_state['open_gaps'] = ['G-001', 'G-002', 'G-003', 'G-004']
    sample_state['completed_gaps'] = []
    ctrl.save_state(sample_state)

    args = FakeArgs(gaps='G-001,G-003', audit_path=None, todo_path=None)
    ctrl.cmd_closeout(args)

    state = ctrl.load_state()
    assert set(state['open_gaps']) == {'G-002', 'G-004'}
    assert set(state['completed_gaps']) == {'G-001', 'G-003'}


def test_closeout_preserves_other_state_fields(tmp_paths, sample_state):
    """closeout must not clobber other STATE.yaml fields."""
    state_path, gap_path = tmp_paths
    sample_state['open_gaps'] = ['G-001']
    sample_state['current_wave'] = '2'
    sample_state['phase'] = 'wave_2'
    sample_state['notes'] = ['wave 2 in progress']
    ctrl.save_state(sample_state)

    args = FakeArgs(gaps='G-001', audit_path=None, todo_path=None)
    ctrl.cmd_closeout(args)

    state = ctrl.load_state()
    assert state['current_wave'] == '2'
    assert state['phase'] == 'wave_2'
    assert state['notes'] == ['wave 2 in progress']


def test_closeout_with_no_open_gaps_is_noop(tmp_paths, sample_state):
    """closeout on empty open_gaps does nothing (safe to call)."""
    state_path, gap_path = tmp_paths
    sample_state['open_gaps'] = []
    sample_state['completed_gaps'] = []
    ctrl.save_state(sample_state)

    args = FakeArgs(gaps='', audit_path=None, todo_path=None)
    ctrl.cmd_closeout(args)

    state = ctrl.load_state()
    assert state['open_gaps'] == []
    assert state['completed_gaps'] == []


def test_closeout_updates_gap_list_md(tmp_paths, sample_state, sample_gap_list):
    """closeout regenerates GAP_LIST.md from audit results."""
    import json as json_module
    state_path, gap_path = tmp_paths
    # Create a minimal audit results file in temp dir
    audit_dir = state_path.parent / 'audit'
    audit_dir.mkdir()
    audit_path = audit_dir / 'AUDIT_RESULTS.json'

    # Create an audit with only G-003 as failure (G-001, G-002 now passing)
    audit_data = {
        'repo': 'openclaw/openclaw',
        'generated_at': '2026-04-20T18:00:00Z',
        'checks': [
            {'id': 'bucket_coverage', 'label': 'bucket coverage', 'status': 'pass', 'expected': '4992', 'actual': '4992'},
            {'id': 'reason_coverage', 'label': 'reason trail', 'status': 'pass', 'expected': '4992', 'actual': '4992'},
            {'id': 'confidence_coverage', 'label': 'confidence coverage', 'status': 'fail', 'expected': '4992', 'actual': '0'},
        ]
    }
    audit_path.write_text(json_module.dumps(audit_data, indent=2))

    sample_state['open_gaps'] = ['G-001', 'G-002', 'G-003']
    sample_state['completed_gaps'] = []
    ctrl.save_state(sample_state)

    args = FakeArgs(gaps='G-001,G-002', audit_path=str(audit_path), todo_path=None)
    ctrl.cmd_closeout(args)

    # GAP_LIST.md should now only have G-003 as open
    gap_text = gap_path.read_text()
    assert 'G-003' in gap_text
    # G-001 and G-002 are not listed as open gaps
    assert '### G-001' not in gap_text
    assert '### G-002' not in gap_text


def test_closeout_updates_todo_md(tmp_paths, sample_state):
    """closeout appends a closeout note to TODO.md."""
    state_path, gap_path = tmp_paths
    todo_path = state_path.parent / 'TODO.md'
    todo_path.write_text("# prATC TODO\n\n## Current status\n\n- Work in progress\n")

    sample_state['open_gaps'] = ['G-001']
    sample_state['current_wave'] = '2'
    ctrl.save_state(sample_state)

    args = FakeArgs(gaps='G-001', audit_path=None, todo_path=str(todo_path))
    ctrl.cmd_closeout(args)

    todo_text = todo_path.read_text()
    assert 'G-001' in todo_text
    assert 'verified' in todo_text.lower() or 'closed' in todo_text.lower() or 'wave 2' in todo_text.lower()


def test_closeout_idempotent(tmp_paths, sample_state):
    """closing out the same gap twice is safe (no double-add)."""
    state_path, gap_path = tmp_paths
    sample_state['open_gaps'] = ['G-001', 'G-002']
    sample_state['completed_gaps'] = []
    ctrl.save_state(sample_state)

    args = FakeArgs(gaps='G-001', audit_path=None, todo_path=None)
    ctrl.cmd_closeout(args)
    ctrl.cmd_closeout(args)  # close G-001 again

    state = ctrl.load_state()
    assert state['completed_gaps'] == ['G-001'], f"no double-add: {state['completed_gaps']}"
    assert state['open_gaps'] == ['G-002']


# ---------------------------------------------------------------------------
# Tests: interruption recovery
# ---------------------------------------------------------------------------

def test_resume_from_mid_wave_interruption_preserves_state(tmp_paths, sample_state):
    """
    Simulate a mid-wave interruption: controller is active (not paused),
    phase=wave_2, current_wave='2', mode=active. The controller process
    is killed unexpectedly. On restart, resume is called.

    Proof: after resume, wave truth (current_wave, phase) and gap state
    (open_gaps, completed_gaps) are identical to pre-interruption values.
    This is the core interruption recovery guarantee.
    """
    state_path, _ = tmp_paths

    # Simulate mid-wave-2 state: subagent is fixing G-003, G-004 (G-001, G-002 already closed)
    sample_state['current_wave'] = '2'
    sample_state['phase'] = 'wave_2'
    sample_state['mode'] = 'active'
    sample_state['paused'] = False  # NOT a deliberate pause — genuine interruption
    sample_state['open_gaps'] = ['G-003', 'G-004']
    sample_state['completed_gaps'] = ['G-001', 'G-002']
    sample_state['stop_reason'] = ''
    ctrl.save_state(sample_state)

    # Simulate fresh controller process calling resume
    args = FakeArgs()
    ctrl.cmd_resume(args)

    state = ctrl.load_state()

    # Phase and wave must be preserved (not clobbered to bootstrap/reconcile)
    assert state['current_wave'] == '2', \
        f"current_wave clobbered: got {state['current_wave']!r}, want '2'"
    assert state['phase'] == 'wave_2', \
        f"phase clobbered: got {state['phase']!r}, want 'wave_2'"
    # Gap state must be preserved exactly
    assert state['open_gaps'] == ['G-003', 'G-004'], \
        f"open_gaps clobbered: {state['open_gaps']}"
    assert state['completed_gaps'] == ['G-001', 'G-002'], \
        f"completed_gaps clobbered: {state['completed_gaps']}"
    # Mode must be active (not reset to paused)
    assert state['mode'] == 'active'
    assert state['paused'] is False


def test_resume_from_mid_wave_synthesizes_same_tasks_after_recovery(tmp_paths, sample_state, sample_gap_list):
    """
    Prove that synthesize_wave() produces identical output before an
    interruption and after resume recovery.

    This is the strongest proof that mid-cycle state is fully recoverable:
    given the same STATE.yaml + GAP_LIST.md, the wave synthesis is identical.
    """
    state_path, gap_path = tmp_paths

    # Mid-wave-2 state with partial closeout
    sample_state['current_wave'] = '2'
    sample_state['phase'] = 'wave_2'
    sample_state['mode'] = 'active'
    sample_state['paused'] = False
    sample_state['open_gaps'] = ['G-003', 'G-004', 'G-006']  # G-001, G-002 closed
    sample_state['completed_gaps'] = ['G-001', 'G-002']
    ctrl.save_state(sample_state)

    # GAP_LIST.md only has G-003, G-004, G-006 as open
    partial_gap_list = """# Autonomous Gap List

## Open gaps

### G-003 — confidence coverage missing
- Audit check: `confidence_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open

### G-004 — trivial dependency edge explosion
- Audit check: `dependency_edge_quality`
- Severity: P1
- Expected: <= 50% trivial depends_on edges
- Actual: {'depends_on_edges': 804678, 'trivial_dep_edges': 804678}
- Status: open

### G-006 — temporal routing not visible
- Audit check: `temporal_routing`
- Severity: P1
- Expected: > 0 temporal buckets
- Actual: 0
- Status: open

## Update protocol

This file is generated from audit output.
"""
    gap_path.write_text(partial_gap_list)

    # Pre-interruption synthesis
    tasks_before = ctrl.synthesize_wave()
    gap_ids_before = sorted(t['gap_id'] for t in tasks_before)

    # Simulate fresh controller restart + resume
    args = FakeArgs()
    ctrl.cmd_resume(args)

    # Post-recovery synthesis must be identical
    tasks_after = ctrl.synthesize_wave()
    gap_ids_after = sorted(t['gap_id'] for t in tasks_after)

    assert gap_ids_before == gap_ids_after, \
        f"synthesis changed after resume: before={gap_ids_before}, after={gap_ids_after}"


def test_resume_from_active_mode_does_not_trigger_spurious_warning(tmp_paths, sample_state):
    """
    resume when mode=active and paused=False (mid-cycle) must not
    reset phase to reconcile or emit a stop_reason.

    This guards against resume accidentally acting like a pause/resume cycle
    when it should be a transparent mid-cycle recovery.
    """
    state_path, _ = tmp_paths

    sample_state['mode'] = 'active'
    sample_state['paused'] = False
    sample_state['phase'] = 'wave_3'
    sample_state['current_wave'] = '3'
    sample_state['stop_reason'] = ''
    ctrl.save_state(sample_state)

    args = FakeArgs()
    ctrl.cmd_resume(args)

    state = ctrl.load_state()
    assert state['phase'] == 'wave_3', \
        f"mid-cycle resume must not reset phase: got {state['phase']!r}"
    assert state['stop_reason'] == '', \
        f"mid-cycle resume must not set stop_reason: got {state['stop_reason']!r}"


def test_verify_state_consistency_detects_orphaned_gap_ids(tmp_paths, sample_state):
    """
    verify_state_consistency() must detect when open_gaps contains IDs
    that do not exist in GAP_LIST.md (orphaned gap references).
    This can happen if GAP_LIST.md was regenerated from a fresh audit
    that no longer lists those gaps, but STATE.yaml was not updated.
    """
    state_path, gap_path = tmp_paths

    # STATE.yaml says G-001 is still open
    sample_state['open_gaps'] = ['G-001', 'G-002']
    ctrl.save_state(sample_state)

    # But GAP_LIST.md only has G-002 (G-001 was closed by audit, gap list regenerated)
    gap_path.write_text("""# Autonomous Gap List

## Open gaps

### G-002 — reason trail missing
- Audit check: `reason_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open

## Update protocol
""")

    issues = ctrl.verify_state_consistency()

    orphaned = [i for i in issues if i['type'] == 'orphaned_gap_id']
    assert len(orphaned) > 0, "verify_state_consistency must detect orphaned G-001 in open_gaps"
    assert orphaned[0]['gap_id'] == 'G-001'
    assert 'not found in GAP_LIST.md' in orphaned[0]['message']


def test_verify_state_consistency_passes_when_state_is_clean(tmp_paths, sample_state, sample_gap_list):
    """
    verify_state_consistency() must return an empty issues list when
    STATE.yaml and GAP_LIST.md are in perfect agreement.
    """
    state_path, gap_path = tmp_paths

    sample_state['open_gaps'] = ['G-001', 'G-002', 'G-003']
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    issues = ctrl.verify_state_consistency()
    assert issues == [], f"expected no issues, got: {issues}"


def test_verify_state_consistency_detects_completed_gap_still_in_open(tmp_paths, sample_state):
    """
    If a gap ID appears in both completed_gaps and open_gaps, that is
    inconsistent and must be flagged.
    """
    state_path, gap_path = tmp_paths

    # G-001 is marked both completed AND still open — a data anomaly
    sample_state['open_gaps'] = ['G-001', 'G-002']
    sample_state['completed_gaps'] = ['G-001']  # G-001 appears in both
    ctrl.save_state(sample_state)

    gap_path.write_text("""# Autonomous Gap List

## Open gaps

### G-001 — bucket coverage missing
- Audit check: `bucket_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open

### G-002 — reason trail missing
- Audit check: `reason_coverage`
- Severity: P0
- Expected: 4992
- Actual: 0
- Status: open
""")

    issues = ctrl.verify_state_consistency()
    dupes = [i for i in issues if i['type'] == 'duplicate_gap_id']
    assert len(dupes) > 0, "must detect gap ID in both open_gaps and completed_gaps"


def test_closeout_mini_cycle_simulated(tmp_paths, sample_state, sample_gap_list):
    """
    Simulate a mini-cycle: wave starts with open gaps, subagent fixes G-001,
    closeout is called, then next wave synthesize only sees remaining gaps.

    This proves the closeout path updates repo-local state consistently.
    """
    state_path, gap_path = tmp_paths

    # Initial state: wave 1 with 3 open gaps
    sample_state['open_gaps'] = ['G-001', 'G-002', 'G-003']
    sample_state['completed_gaps'] = []
    sample_state['current_wave'] = '1'
    sample_state['phase'] = 'wave_1'
    ctrl.save_state(sample_state)
    gap_path.write_text(sample_gap_list)

    # Simulate: subagent fixed G-001, verified by audit
    # Closeout G-001
    args = FakeArgs(gaps='G-001', audit_path=None, todo_path=None)
    ctrl.cmd_closeout(args)

    # Verify STATE.yaml updated correctly
    state = ctrl.load_state()
    assert state['completed_gaps'] == ['G-001']
    assert set(state['open_gaps']) == {'G-002', 'G-003'}
    assert state['current_wave'] == '1'  # wave not advanced by closeout

    # Synthesize next wave - should only see remaining gaps
    tasks = ctrl.synthesize_wave()
    synthesized_ids = {t['gap_id'] for t in tasks}
    assert 'G-001' not in synthesized_ids, "G-001 should not appear in new wave synthesis"
    assert synthesized_ids == set(state['open_gaps']), f"synthesized: {synthesized_ids}, open: {state['open_gaps']}"


# ---------------------------------------------------------------------------
# Tests: per-run artifact discipline
# ---------------------------------------------------------------------------

def test_make_run_dir_creates_timestamped_directory(tmp_path, monkeypatch):
    """
    make_run_dir() creates autonomous/runs/<timestamp>/ with subagent-results/ subdir.
    """
    runs_dir = tmp_path / 'autonomous' / 'runs'
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)

    run_dir = ctrl.make_run_dir(timestamp='20260420-test')

    assert run_dir.exists(), f"run_dir should exist: {run_dir}"
    assert run_dir.is_dir(), f"run_dir should be a directory: {run_dir}"
    assert (run_dir / 'subagent-results').is_dir(), "subagent-results/ subdir should be created"
    assert run_dir.name == '20260420-test', f"run_dir name should be timestamp: {run_dir.name}"


def test_make_run_dir_generates_timestamp_when_not_provided(tmp_path, monkeypatch):
    """
    make_run_dir() without a timestamp argument generates one from current UTC time.
    """
    runs_dir = tmp_path / 'autonomous' / 'runs'
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)

    run_dir = ctrl.make_run_dir()

    assert run_dir.exists(), f"run_dir should exist: {run_dir}"
    # Timestamp should be YYYYMMDD-HHMMSS format
    assert len(run_dir.name) == 15, f"timestamp should be 15 chars (YYYYMMDD-HHMMSS): {run_dir.name}"
    assert run_dir.name[0:4].isdigit(), f"year should be digits: {run_dir.name}"


def test_make_run_dir_is_idempotent(tmp_path, monkeypatch):
    """
    Calling make_run_dir twice with the same timestamp returns the same path.
    """
    runs_dir = tmp_path / 'autonomous' / 'runs'
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)

    run_dir1 = ctrl.make_run_dir(timestamp='20260420-idempotent')
    run_dir2 = ctrl.make_run_dir(timestamp='20260420-idempotent')

    assert run_dir1 == run_dir2, "same timestamp should return same directory"


def test_write_controller_log_creates_file(tmp_path, monkeypatch):
    """
    write_controller_log() creates controller-log.md with the provided log lines.
    """
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    run_dir = tmp_path / 'autonomous' / 'runs' / '20260420-log-test'
    run_dir.mkdir(parents=True)

    log_lines = ['- line one', '- line two', '- line three']
    ctrl.write_controller_log(run_dir, log_lines)

    log_file = run_dir / 'controller-log.md'
    assert log_file.exists(), f"controller-log.md should exist: {log_file}"
    content = log_file.read_text()
    assert 'line one' in content
    assert 'line two' in content
    assert 'line three' in content


def test_write_wave_summary_creates_file(tmp_path, monkeypatch):
    """
    write_wave_summary() creates wave-summary.md with the provided summary lines.
    """
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    run_dir = tmp_path / 'autonomous' / 'runs' / '20260420-summary-test'
    run_dir.mkdir(parents=True)

    summary_lines = ['- source: test corpus', '- outcome: green', '- gaps closed: 3']
    ctrl.write_wave_summary(run_dir, summary_lines)

    summary_file = run_dir / 'wave-summary.md'
    assert summary_file.exists(), f"wave-summary.md should exist: {summary_file}"
    content = summary_file.read_text()
    assert 'source: test corpus' in content
    assert 'outcome: green' in content
    assert 'gaps closed: 3' in content


def test_link_or_copy_audit_copies_when_file_exists(tmp_path, monkeypatch):
    """
    link_or_copy_audit() copies AUDIT_RESULTS.json into the run directory when the source exists.
    """
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    run_dir = tmp_path / 'autonomous' / 'runs' / '20260420-audit-test'
    run_dir.mkdir(parents=True)

    # Create a fake audit source file
    audit_source = tmp_path / 'source' / 'AUDIT_RESULTS.json'
    audit_source.parent.mkdir()
    audit_source.write_text('{"checks": [{"id": "test", "status": "pass"}]}')

    ctrl.link_or_copy_audit(run_dir, str(audit_source))

    copied_audit = run_dir / 'AUDIT_RESULTS.json'
    assert copied_audit.exists(), f"AUDIT_RESULTS.json should be copied: {copied_audit}"
    content = copied_audit.read_text()
    assert 'test' in content
    assert '"status": "pass"' in content


def test_link_or_copy_audit_skips_silently_when_source_missing(tmp_path, monkeypatch):
    """
    link_or_copy_audit() does nothing when the audit source file does not exist.
    """
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    run_dir = tmp_path / 'autonomous' / 'runs' / '20260420-missing-audit'
    run_dir.mkdir(parents=True)

    # Should not raise - just skips
    ctrl.link_or_copy_audit(run_dir, str(tmp_path / 'nonexistent' / 'AUDIT_RESULTS.json'))

    # No file should be created
    assert not (run_dir / 'AUDIT_RESULTS.json').exists()


def test_new_run_command_creates_all_required_artifacts(tmp_path, monkeypatch):
    """
    The new-run command creates the full per-run artifact structure:
    autonomous/runs/<timestamp>/ with controller-log.md, wave-summary.md, subagent-results/,
    and optionally AUDIT_RESULTS.json.
    """
    runs_dir = tmp_path / 'autonomous' / 'runs'
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    monkeypatch.setattr(ctrl, 'RUNS_DIR', runs_dir)

    args = FakeArgs(
        timestamp='20260420-newrun',
        log_lines=['- step one', '- step two'],
        summary_lines=['- source: test', '- result: ok'],
        audit_path=None
    )
    ctrl.cmd_new_run(args)

    run_dir = tmp_path / 'autonomous' / 'runs' / '20260420-newrun'
    assert run_dir.exists(), f"run_dir should exist: {run_dir}"
    assert (run_dir / 'controller-log.md').exists(), "controller-log.md should exist"
    assert (run_dir / 'wave-summary.md').exists(), "wave-summary.md should exist"
    assert (run_dir / 'subagent-results').is_dir(), "subagent-results/ should exist"


def test_new_run_command_with_audit_copies_audit_file(tmp_path, monkeypatch):
    """
    new-run with --audit-path copies the audit file to the run directory.
    """
    runs_dir = tmp_path / 'autonomous' / 'runs'
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    monkeypatch.setattr(ctrl, 'RUNS_DIR', runs_dir)

    # Create source audit file
    audit_source = tmp_path / 'source' / 'AUDIT_RESULTS.json'
    audit_source.parent.mkdir()
    audit_source.write_text('{"checks": []}')

    args = FakeArgs(
        timestamp='20260420-audit',
        log_lines=['- audited'],
        summary_lines=['- audit passed'],
        audit_path=str(audit_source)
    )
    ctrl.cmd_new_run(args)

    run_dir = tmp_path / 'autonomous' / 'runs' / '20260420-audit'
    assert (run_dir / 'AUDIT_RESULTS.json').exists(), "AUDIT_RESULTS.json should be copied"


def test_run_dir_path_returns_correct_path(tmp_path, monkeypatch):
    """
    run_dir_path(timestamp) returns the correct path without creating anything.
    """
    runs_dir = tmp_path / 'autonomous' / 'runs'
    monkeypatch.setattr(ctrl, 'REPO_ROOT', tmp_path)
    monkeypatch.setattr(ctrl, 'RUNS_DIR', runs_dir)

    path = ctrl.run_dir_path('20260420-testpath')

    assert path == tmp_path / 'autonomous' / 'runs' / '20260420-testpath'
    assert not path.exists(), "run_dir_path should not create the directory"
