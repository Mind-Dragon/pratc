#!/usr/bin/env python3
"""Tests for shared autonomous gap catalog."""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))

import autonomous_controller as ctrl
import gap_list_from_audit as gap_list
from gap_catalog import GAP_MAP, gap_metadata


def test_controller_and_gap_generator_share_gap_catalog():
    assert ctrl.GAP_MAP is GAP_MAP
    assert gap_list.GAP_MAP is GAP_MAP


def test_gap_metadata_preserves_stable_fallback_for_unknown_check():
    assert gap_metadata('bucket_coverage', 'Bucket coverage') == ('G-001', 'bucket coverage missing', 'P0')
    assert gap_metadata('new_check', 'New Check') == ('X-new_check', 'New Check', 'P2')
