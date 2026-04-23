#!/usr/bin/env python3
"""Tests for generating autonomous GAP_LIST.md from audit output."""
import json
import sys
from pathlib import Path

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
