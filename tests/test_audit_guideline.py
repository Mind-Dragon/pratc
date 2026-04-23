#!/usr/bin/env python3
"""
Tests for scripts/audit_guideline.py

Run with: python -m pytest tests/test_audit_guideline.py -v
"""
import json
import tempfile
from pathlib import Path

import pytest

# Import the audit functions directly for unit testing
import sys
sys.path.insert(0, str(Path(__file__).parent.parent / 'scripts'))

from audit_guideline import (
    check_artifact_presence,
    check_truncation_explicit,
    check_bucket_coverage,
    check_reason_coverage,
    check_confidence_coverage,
    check_temporal_mutually_exclusive,
    check_low_confidence_high_value,
    check_high_confidence_cap_respected,
    check_duplicate_presence,
    check_garbage_detected,
    check_future_work_visible,
    check_temporal_routing,
    check_report_self_describing_prs,
    check_selected_reason_coverage,
    check_conflict_pairs_threshold,
    check_dependency_edge_quality,
    check_disposal_bucket_persistence,
    check_deeper_judgment_layers,
    check_bucket_reason_required,
    audit,
)


@pytest.fixture
def minimal_pass_analyze():
    """Minimal passing analyze.json fixture."""
    path = Path(__file__).parent / 'fixtures' / 'audit' / 'minimal_pass.json'
    return json.loads(path.read_text())


@pytest.fixture
def truncated_silent_analyze():
    """Truncated without reason - should fail."""
    path = Path(__file__).parent / 'fixtures' / 'audit' / 'truncated_silent.json'
    return json.loads(path.read_text())


@pytest.fixture
def low_conf_high_value_analyze():
    """Low confidence with high value buckets - should fail."""
    path = Path(__file__).parent / 'fixtures' / 'audit' / 'low_confidence_high_value.json'
    return json.loads(path.read_text())


class TestTruncationExplicit:
    def test_not_truncated_passes(self, minimal_pass_analyze):
        result = check_truncation_explicit(minimal_pass_analyze)
        assert result['status'] == 'pass'

    def test_truncated_with_reason_passes(self):
        analyze = {
            'analysis_truncated': True,
            'truncation_reason': 'max_prs_cap',
            'max_prs_applied': 5000,
            'counts': {'total_prs': 10000},
        }
        result = check_truncation_explicit(analyze)
        assert result['status'] == 'pass'
        assert 'max_prs_cap' in result['actual']

    def test_truncated_without_reason_fails(self):
        analyze = {
            'analysis_truncated': True,
            'counts': {'total_prs': 10000},
        }
        result = check_truncation_explicit(analyze)
        assert result['status'] == 'fail'


class TestBucketCoverage:
    def test_all_prs_have_bucket(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_bucket_coverage(prs)
        assert result['status'] == 'pass'
        assert result['actual'] == len(prs)

    def test_some_prs_missing_bucket(self):
        prs = [
            {'id': '1', 'bucket': 'now'},
            {'id': '2', 'bucket': None},
            {'id': '3', 'bucket': 'future'},
        ]
        result = check_bucket_coverage(prs)
        assert result['status'] == 'fail'
        assert result['actual'] == 2


class TestReasonCoverage:
    def test_all_prs_have_reason(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_reason_coverage(prs)
        assert result['status'] == 'pass'

    def test_prs_missing_reason(self):
        prs = [
            {'id': '1', 'reasons': ['a']},
            {'id': '2'},  # No reasons
            {'id': '3', 'review': {'reasons': ['b']}},
        ]
        result = check_reason_coverage(prs)
        assert result['status'] == 'fail'
        assert result['actual'] == 2


class TestConfidenceCoverage:
    def test_all_prs_have_confidence(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_confidence_coverage(prs)
        assert result['status'] == 'pass'

    def test_some_prs_missing_confidence(self):
        prs = [
            {'id': '1', 'confidence': 0.8},
            {'id': '2', 'review': {}},  # No confidence
            {'id': '3', 'review': {'confidence': 0.6}},
        ]
        result = check_confidence_coverage(prs)
        assert result['status'] == 'fail'
        assert result['actual'] == 2


class TestTemporalMutualExclusivity:
    def test_single_temporal_bucket_passes(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_temporal_mutually_exclusive(prs)
        assert result['status'] == 'pass'

    def test_multiple_temporal_buckets_fails(self):
        prs = [
            {'id': '1', 'temporal_bucket': ['now', 'future']},  # Multiple!
        ]
        result = check_temporal_mutually_exclusive(prs)
        assert result['status'] == 'fail'


class TestLowConfidenceHighValue:
    def test_no_violations_passes(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_low_confidence_high_value(prs)
        assert result['status'] == 'pass'

    def test_low_confidence_high_value_fails(self, low_conf_high_value_analyze):
        prs = low_conf_high_value_analyze['prs']
        result = check_low_confidence_high_value(prs)
        assert result['status'] == 'fail'
        assert 'violations' in result['actual']
        assert len(result['details']) == 2  # PR 1 and PR 2


class TestHighConfidenceCap:
    def test_high_confidence_risk_bucket_fails(self):
        prs = [
            {'id': '1', 'bucket': 'security_risk', 'confidence': 0.85},
        ]
        result = check_high_confidence_cap_respected(prs)
        assert result['status'] == 'fail'

    def test_normal_confidence_risk_bucket_passes(self):
        prs = [
            {'id': '1', 'bucket': 'security_risk', 'confidence': 0.7},
        ]
        result = check_high_confidence_cap_respected(prs)
        assert result['status'] == 'pass'


class TestDuplicatePresence:
    def test_duplicates_detected(self, minimal_pass_analyze):
        result = check_duplicate_presence(minimal_pass_analyze)
        assert result['status'] == 'pass'

    def test_no_duplicates_fails_on_large_corpus(self):
        analyze = {
            'counts': {'duplicate_groups': 0, 'total_prs': 200},
            'prs': [{'id': str(i)} for i in range(200)],
        }
        result = check_duplicate_presence(analyze)
        assert result['status'] == 'fail'

    def test_no_duplicates_is_manual_on_small_corpus(self):
        analyze = {
            'analysis_truncated': True,
            'counts': {'duplicate_groups': 0, 'total_prs': 50},
            'prs': [{'id': str(i)} for i in range(50)],
        }
        result = check_duplicate_presence(analyze)
        assert result['status'] == 'manual'
        assert 'sample too small' in result['actual']


class TestGarbageDetected:
    def test_garbage_detected(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_garbage_detected(minimal_pass_analyze, prs)
        assert result['status'] == 'pass'

    def test_no_garbage_fails_on_large_corpus(self):
        analyze = {'counts': {'garbage_prs': 0, 'total_prs': 200}}
        prs = [{'id': str(i), 'bucket': 'now'} for i in range(200)]
        result = check_garbage_detected(analyze, prs)
        assert result['status'] == 'fail'

    def test_no_garbage_is_manual_on_small_corpus(self):
        analyze = {
            'analysis_truncated': True,
            'counts': {'garbage_prs': 0, 'total_prs': 50},
        }
        prs = [{'id': str(i), 'bucket': 'now'} for i in range(50)]
        result = check_garbage_detected(analyze, prs)
        assert result['status'] == 'manual'
        assert 'sample too small' in result['actual']


class TestFutureWorkVisible:
    def test_future_work_visible(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_future_work_visible(prs)
        assert result['status'] == 'pass'

    def test_all_disposal_no_future_needed(self):
        prs = [
            {'id': '1', 'bucket': 'junk'},
            {'id': '2', 'bucket': 'duplicate'},
        ]
        result = check_future_work_visible(prs)
        assert result['status'] == 'pass'

    def test_all_now_no_future_fails(self):
        prs = [
            {'id': '1', 'bucket': 'now', 'temporal_bucket': 'now'},
            {'id': '2', 'bucket': 'high_value', 'temporal_bucket': 'now'},
        ]
        result = check_future_work_visible(prs)
        assert result['status'] == 'fail'


class TestTemporalRouting:
    def test_temporal_buckets_visible(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_temporal_routing(prs)
        assert result['status'] == 'pass'
        assert result['actual'] == 10


class TestReportSelfDescribingPRs:
    def test_current_pr_shape_passes(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_report_self_describing_prs(prs)
        assert result['status'] == 'pass'
        assert result['actual'] == len(prs)

    def test_missing_priority_tier_fails(self):
        prs = [
            {
                'id': '1',
                'category': 'merge_now',
                'temporal_bucket': 'now',
                'reasons': ['looks good'],
                'confidence': 0.9,
            }
        ]
        result = check_report_self_describing_prs(prs)
        assert result['status'] == 'fail'
        assert result['actual'] == 0


class TestSelectedReasonCoverage:
    def test_selected_with_reasons_passes(self):
        plan = {
            'selected': [
                {'pr_number': 1, 'reasons': ['a']},
                {'pr_number': 2, 'rationale': 'because'},
            ]
        }
        result = check_selected_reason_coverage(plan)
        assert result['status'] == 'pass'

    def test_selected_without_reasons_fails(self):
        plan = {
            'selected': [
                {'pr_number': 1},
                {'pr_number': 2},
            ]
        }
        result = check_selected_reason_coverage(plan)
        assert result['status'] == 'fail'

    def test_empty_selected_passes(self):
        plan = {'selected': []}
        result = check_selected_reason_coverage(plan)
        assert result['status'] == 'pass'


class TestConflictPairsThreshold:
    def test_low_conflict_pairs_passes(self):
        graph = {'edges': [{'edge_type': 'conflicts_with'}] * 100}
        result = check_conflict_pairs_threshold(graph)
        assert result['status'] == 'pass'

    def test_high_conflict_pairs_fails(self):
        graph = {'edges': [{'edge_type': 'conflicts_with'}] * 6000}
        result = check_conflict_pairs_threshold(graph)
        assert result['status'] == 'fail'


class TestDependencyEdgeQuality:
    def test_no_trivial_depends_passes(self):
        graph = {
            'edges': [
                {'edge_type': 'depends_on', 'reason': 'API change requires downstream update'},
                {'edge_type': 'depends_on', 'reason': 'Shared utility modified'},
            ]
        }
        result = check_dependency_edge_quality(graph)
        assert result['status'] == 'pass'

    def test_trivial_depends_dominate_fails(self):
        graph = {
            'edges': [
                {'edge_type': 'depends_on', 'reason': 'base branch head branch same-repo'},
                {'edge_type': 'depends_on', 'reason': 'base branch main head branch feat same'},
            ]
        }
        result = check_dependency_edge_quality(graph)
        assert result['status'] == 'fail'


class TestDisposalBucketPersistence:
    def test_disposal_terminal_layer_passes(self):
        prs = [
            {
                'number': 1,
                'bucket': 'duplicate',
                'decision_layers': [
                    {'layer': 1, 'name': 'Garbage', 'bucket': 'low_value', 'continued': True, 'terminal': False},
                    {'layer': 2, 'name': 'Duplicates', 'bucket': 'duplicate', 'continued': False, 'terminal': True},
                    {'layer': 3, 'name': 'Obvious badness', 'bucket': 'duplicate', 'continued': False, 'terminal': True},
                ],
            }
        ]
        result = check_disposal_bucket_persistence(prs)
        assert result['status'] == 'pass'

    def test_disposal_without_terminal_layer_fails(self):
        prs = [
            {
                'number': 2,
                'bucket': 'duplicate',
                'decision_layers': [
                    {'layer': 1, 'name': 'Garbage', 'bucket': 'low_value', 'continued': True, 'terminal': False},
                    {'layer': 2, 'name': 'Duplicates', 'bucket': 'duplicate', 'continued': True, 'terminal': False},
                ],
            }
        ]
        result = check_disposal_bucket_persistence(prs)
        assert result['status'] == 'fail'
    def test_terminal_decision_layer_disposal_is_detected(self):
        prs = [
            {
                'number': 3,
                'category': 'merge_after_focused_review',
                'decision_layers': [
                    {'layer': 1, 'name': 'Garbage', 'bucket': 'junk', 'continued': False, 'terminal': True},
                    {'layer': 2, 'name': 'Duplicates', 'bucket': 'junk', 'continued': False, 'terminal': True},
                ],
            }
        ]
        result = check_disposal_bucket_persistence(prs)
        assert result['status'] == 'pass'
        assert '1 disposal PRs verified' in result['actual']


class TestDeeperJudgmentLayers:
    def test_ordered_decision_layers_pass(self):
        prs = [
            {
                'number': 1,
                'decision_layers': [
                    {'layer': 1, 'name': 'Garbage', 'cost_tier': 'cheap', 'status': 'clear'},
                    {'layer': 2, 'name': 'Duplicates', 'cost_tier': 'cheap', 'status': 'clear'},
                    {'layer': 3, 'name': 'Obvious badness', 'cost_tier': 'cheap', 'status': 'clear'},
                    {'layer': 4, 'name': 'Substance score', 'cost_tier': 'medium', 'status': 'observed'},
                    {'layer': 5, 'name': 'Now vs future', 'cost_tier': 'medium', 'status': 'observed'},
                    {'layer': 6, 'name': 'Confidence', 'cost_tier': 'expensive', 'status': 'observed'},
                ],
            }
        ]
        result = check_deeper_judgment_layers(prs)
        assert result['status'] == 'pass'

    def test_missing_decision_layers_fails(self):
        result = check_deeper_judgment_layers([{'number': 1}])
        assert result['status'] == 'fail'

    def test_expensive_layer_before_obvious_layers_fails(self):
        prs = [
            {
                'number': 1,
                'decision_layers': [
                    {'layer': 1, 'name': 'Confidence', 'cost_tier': 'expensive', 'status': 'observed'},
                    {'layer': 2, 'name': 'Garbage', 'cost_tier': 'cheap', 'status': 'clear'},
                ],
            }
        ]
        result = check_deeper_judgment_layers(prs)
        assert result['status'] == 'fail'


class TestBucketReasonRequired:
    def test_all_buckets_have_reasons(self, minimal_pass_analyze):
        prs = minimal_pass_analyze['prs']
        result = check_bucket_reason_required(prs)
        assert result['status'] == 'pass'

    def test_bucket_without_reason_fails(self):
        prs = [
            {'id': '1', 'category': 'merge_now', 'reasons': ['a']},
            {'id': '2', 'category': 'merge_after_focused_review'},
        ]
        result = check_bucket_reason_required(prs)
        assert result['status'] == 'fail'


class TestAuditIntegration:
    """Integration tests running audit against temp directories."""

    def test_audit_minimal_pass_fixture(self, minimal_pass_analyze):
        with tempfile.TemporaryDirectory() as tmpdir:
            run_dir = Path(tmpdir)
            # Create required files
            (run_dir / 'analyze.json').write_text(json.dumps(minimal_pass_analyze))
            (run_dir / 'step-3-cluster.json').write_text('{}')
            (run_dir / 'step-4-graph.json').write_text('{"edges": []}')
            (run_dir / 'step-5-plan.json').write_text('{"selected": []}')
            (run_dir / 'report.pdf').write_text('%PDF-1.4 fake')

            result = audit(run_dir)

            # Should have failures because graph has no edges and some checks
            # expect conflicts_with edges for conflict_pairs check
            assert 'checks' in result
            assert result['summary']['total_prs'] == 10

    def test_audit_creates_output(self, minimal_pass_analyze):
        with tempfile.TemporaryDirectory() as tmpdir:
            run_dir = Path(tmpdir)
            (run_dir / 'analyze.json').write_text(json.dumps(minimal_pass_analyze))
            (run_dir / 'step-3-cluster.json').write_text('{}')
            (run_dir / 'step-4-graph.json').write_text('{"edges": []}')
            (run_dir / 'step-5-plan.json').write_text('{"selected": []}')
            (run_dir / 'report.pdf').write_text('%PDF-1.4 fake')

            result = audit(run_dir)

            output = run_dir / 'AUDIT_RESULTS.json'
            assert not output.exists()  # audit() doesn't write, main() does

    def test_audit_counts_summary(self):
        analyze = {
            'prs': [
                {
                    'id': '1',
                    'category': 'merge_now',
                    'priority_tier': 'fast_merge',
                    'temporal_bucket': 'now',
                    'confidence': 0.8,
                    'reasons': ['a'],
                },
            ],
            'counts': {'total_prs': 1},
        }
        with tempfile.TemporaryDirectory() as tmpdir:
            run_dir = Path(tmpdir)
            (run_dir / 'analyze.json').write_text(json.dumps(analyze))
            (run_dir / 'step-3-cluster.json').write_text('{}')
            (run_dir / 'step-4-graph.json').write_text('{"edges": []}')
            (run_dir / 'step-5-plan.json').write_text('{"selected": []}')
            (run_dir / 'report.pdf').write_text('%PDF-1.4 fake')

            result = audit(run_dir)

            assert result['summary']['total_prs'] == 1
            assert 'checks_passed' in result['summary']
            assert 'checks_failed' in result['summary']
            assert 'checks_manual' in result['summary']


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
