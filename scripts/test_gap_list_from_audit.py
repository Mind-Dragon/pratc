#!/usr/bin/env python3
"""Tests for generating autonomous GAP_LIST.md from audit output."""
import json
import sys
from pathlib import Path

import yaml

sys.path.insert(0, str(Path(__file__).parent))

import gap_list_from_audit as gapgen


def test_gap_list_preserves_manual_checks_when_no_failures(tmp_path):
    audit = tmp_path / 'AUDIT_RESULTS.json'
    gap = tmp_path / 'GAP_LIST.md'
    state = tmp_path / 'STATE.yaml'
    state.write_text('open_gaps: []\nupdated_at: old\n')
    audit.write_text(json.dumps({
        'checks': [
            {'id': 'bucket_coverage', 'label': 'bucket coverage', 'status': 'pass', 'expected': 'ok', 'actual': 'ok'},
            {'id': 'deeper_judgment_layers', 'label': 'deeper judgment', 'status': 'manual', 'expected': 'observable gate order', 'actual': 'pipeline ordering not directly observable'},
        ]
    }))

    gapgen.generate_gap_list(audit, gap, state)

    text = gap.read_text()
    assert 'No open gaps. Latest audit passed.' in text
    assert '## Manual/unverifiable checks' in text
    assert '`deeper_judgment_layers`' in text
    assert 'pipeline ordering not directly observable' in text


def test_gap_list_preserves_fixed_and_blocked_history(tmp_path):
    audit = tmp_path / 'AUDIT_RESULTS.json'
    gap = tmp_path / 'GAP_LIST.md'
    state = tmp_path / 'STATE.yaml'
    state.write_text('open_gaps:\n  - G-001\nblocked_gaps:\n  - G-099\ncompleted_gaps: []\nupdated_at: old\n')
    gap.write_text('''# Autonomous Gap List

Updated from audit: `old/AUDIT_RESULTS.json`

## Open gaps

### G-001 — bucket coverage missing
- Audit check: `bucket_coverage`
- Severity: P0
- Expected: all PRs bucketed
- Actual: 10 missing
- Status: open
- Notes: generated from latest audit failure

## Gap history

### G-099 — blocked external dependency
- Audit check: `external_dependency`
- Severity: P2
- Expected: dependency available
- Actual: unavailable
- Status: blocked
- Notes: waiting on upstream

## Update protocol

This file is generated from audit output. Preserve stable gap IDs where possible.
''')
    audit.write_text(json.dumps({
        'checks': [
            {'id': 'bucket_coverage', 'label': 'bucket coverage', 'status': 'pass', 'expected': 'ok', 'actual': 'ok'},
        ]
    }))

    gapgen.generate_gap_list(audit, gap, state)

    text = gap.read_text()
    assert 'No open gaps. Latest audit passed.' in text
    assert '## Gap history' in text
    assert '### G-001 — bucket coverage missing' in text
    assert '- Status: fixed' in text
    assert '- Notes: fixed by latest audit' in text
    assert '### G-099 — blocked external dependency' in text
    assert '- Status: blocked' in text
    state_data = yaml.safe_load(state.read_text())
    assert state_data['open_gaps'] == []
    assert state_data['blocked_gaps'] == ['G-099']
    assert state_data['completed_gaps'] == ['G-001']


def test_gap_list_preserves_deferred_history_without_reopening(tmp_path):
    audit = tmp_path / 'AUDIT_RESULTS.json'
    gap = tmp_path / 'GAP_LIST.md'
    gap.write_text('''# Autonomous Gap List

## Open gaps

No open gaps. Latest audit passed.

## Gap history

### G-123 — future enhancement
- Audit check: `future_check`
- Severity: P3
- Expected: implemented later
- Actual: intentionally deferred
- Status: deferred
- Notes: out of current release scope
''')
    audit.write_text(json.dumps({
        'checks': [
            {'id': 'reason_coverage', 'label': 'reason coverage', 'status': 'pass', 'expected': 'ok', 'actual': 'ok'},
        ]
    }))

    gapgen.generate_gap_list(audit, gap)

    text = gap.read_text()
    assert '### G-123 — future enhancement' in text
    assert '- Status: deferred' in text
    assert 'out of current release scope' in text
