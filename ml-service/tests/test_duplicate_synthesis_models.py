"""Tests for DuplicateSynthesisPlan and DuplicateSynthesisCandidate models.

These tests verify that the duplicate_synthesis field can be serialized
and deserialized correctly in AnalysisResponse for both Pydantic and Bootstrap models.
"""

from __future__ import annotations

import pytest

try:
    from pratc_ml.models import (
        AnalysisResponse,
        Counts,
        DuplicateGroup,
        DuplicateSynthesisCandidate,
        DuplicateSynthesisPlan,
        PR,
        PRCluster,
        ConflictPair,
        StalenessReport,
        GarbagePR,
        CollapsedCorpus,
    )
except ImportError:
    # Models not yet implemented - skip all tests in this module
    pytest.skip("DuplicateSynthesisCandidate and/or DuplicateSynthesisPlan not yet implemented", allow_module_level=True)


@pytest.mark.unit
def test_duplicate_synthesis_plan_serialization() -> None:
    """Test that DuplicateSynthesisPlan serializes to JSON correctly."""
    plan = DuplicateSynthesisPlan(
        group_id="dup-10-0.95",
        group_type="duplicate",
        original_canonical_pr=10,
        nominated_canonical_pr=11,
        similarity=0.95,
        reason="title and body are nearly identical",
        candidates=[
            DuplicateSynthesisCandidate(
                pr_number=10,
                title="planner: simplify candidate scoring",
                author="alice",
                role="canonical",
                synthesis_score=0.85,
                rationale="higher review count",
            ),
            DuplicateSynthesisCandidate(
                pr_number=11,
                title="planner simplify candidate scoring",
                author="bob",
                role="alternate",
                synthesis_score=0.80,
                rationale="newer but fewer reviews",
            ),
        ],
        synthesis_notes=[
            "Consider combining test coverage from both PRs",
            "Resolve conflicting formatter changes",
        ],
    )

    # Verify it can be dumped to JSON
    json_str = plan.model_dump_json(by_alias=True)
    assert "dup-10-0.95" in json_str
    assert "planner: simplify candidate scoring" in json_str
    assert "alternate" in json_str


@pytest.mark.unit
def test_duplicate_synthesis_plan_deserialization() -> None:
    """Test that DuplicateSynthesisPlan can be deserialized from JSON."""
    data = {
        "group_id": "dup-10-0.95",
        "group_type": "duplicate",
        "original_canonical_pr": 10,
        "nominated_canonical_pr": 11,
        "similarity": 0.95,
        "reason": "title and body are nearly identical",
        "candidates": [
            {
                "pr_number": 10,
                "title": "planner: simplify candidate scoring",
                "author": "alice",
                "role": "canonical",
                "synthesis_score": 0.85,
                "rationale": "higher review count",
            },
        ],
        "synthesis_notes": ["Consider combining test coverage"],
    }

    plan = DuplicateSynthesisPlan.model_validate(data)

    assert plan.group_id == "dup-10-0.95"
    assert plan.group_type == "duplicate"
    assert plan.nominated_canonical_pr == 11
    assert len(plan.candidates) == 1
    assert plan.candidates[0].pr_number == 10
    assert plan.candidates[0].role == "canonical"


@pytest.mark.unit
def test_analysis_response_with_duplicate_synthesis() -> None:
    """Test that AnalysisResponse with duplicate_synthesis serializes/deserializes correctly."""
    pr = PR(
        id="1",
        repo="owner/repo",
        number=10,
        title="test pr",
        body="description",
        url="https://github.com/owner/repo/pull/10",
        author="alice",
        labels=[],
        files_changed=["internal/planner/scoring.go"],
        review_status="APPROVED",
        ci_status="success",
        mergeable="true",
        base_branch="main",
        head_branch="feature",
        cluster_id="cluster-1",
        created_at="2024-01-01T00:00:00Z",
        updated_at="2024-01-02T00:00:00Z",
        is_draft=False,
        is_bot=False,
        additions=50,
        deletions=10,
        changed_files_count=1,
        review_count=2,
        comment_count=5,
    )

    cluster = PRCluster(
        cluster_id="cluster-1",
        cluster_label="planner",
        summary="planner related changes",
        pr_ids=[10, 11],
        health_status="healthy",
        average_similarity=0.92,
        sample_titles=["planner: simplify", "planner simplify"],
    )

    duplicate_group = DuplicateGroup(
        canonical_pr_number=10,
        duplicate_pr_numbers=[11],
        similarity=0.95,
        reason="title and body are nearly identical",
    )

    synthesis_plan = DuplicateSynthesisPlan(
        group_id="dup-10-0.95",
        group_type="duplicate",
        original_canonical_pr=10,
        nominated_canonical_pr=11,
        similarity=0.95,
        reason="title and body are nearly identical",
        candidates=[
            DuplicateSynthesisCandidate(
                pr_number=10,
                title="planner: simplify candidate scoring",
                author="alice",
                role="canonical",
                synthesis_score=0.85,
                rationale="higher review count",
            ),
            DuplicateSynthesisCandidate(
                pr_number=11,
                title="planner simplify candidate scoring",
                author="bob",
                role="alternate",
                synthesis_score=0.80,
                rationale="newer but fewer reviews",
            ),
        ],
        synthesis_notes=["Consider combining test coverage"],
    )

    counts = Counts(
        total_prs=2,
        cluster_count=1,
        duplicate_groups=1,
        overlap_groups=0,
        conflict_pairs=0,
        stale_prs=0,
        garbage_prs=0,
        collapsed_duplicate_groups=1,
    )

    response = AnalysisResponse(
        repo="owner/repo",
        generatedAt="2024-01-02T00:00:00Z",
        counts=counts,
        prs=[pr],
        clusters=[cluster],
        duplicates=[duplicate_group],
        overlaps=[],
        conflicts=[],
        stalenessSignals=[],
        garbagePRs=[],
        collapsed_corpus=CollapsedCorpus(
            canonical_to_superseded={10: [11]},
            superseded_to_canonical={11: 10},
            collapsed_group_count=1,
            total_superseded=1,
        ),
        duplicate_synthesis=[synthesis_plan],
    )

    # Verify it serializes to JSON with duplicate_synthesis field
    json_str = response.model_dump_json(by_alias=True, exclude_none=True)
    assert "duplicate_synthesis" in json_str
    assert "dup-10-0.95" in json_str
    assert "nominated_canonical_pr" in json_str

    # Verify it can be deserialized back
    restored = AnalysisResponse.model_validate_json(json_str)
    assert len(restored.duplicate_synthesis) == 1
    assert restored.duplicate_synthesis[0].group_id == "dup-10-0.95"
    assert restored.duplicate_synthesis[0].nominated_canonical_pr == 11
    assert len(restored.duplicate_synthesis[0].candidates) == 2
    assert restored.duplicate_synthesis[0].candidates[0].role == "canonical"


@pytest.mark.unit
def test_analysis_response_without_duplicate_synthesis() -> None:
    """Test that AnalysisResponse without duplicate_synthesis still works."""
    counts = Counts(
        total_prs=1,
        cluster_count=0,
        duplicate_groups=0,
        overlap_groups=0,
        conflict_pairs=0,
        stale_prs=0,
        garbage_prs=0,
    )

    response = AnalysisResponse(
        repo="owner/repo",
        generatedAt="2024-01-02T00:00:00Z",
        counts=counts,
        prs=[],
        clusters=[],
        duplicates=[],
        overlaps=[],
        conflicts=[],
        stalenessSignals=[],
    )

    # Should serialize without duplicate_synthesis field
    json_str = response.model_dump_json(by_alias=True, exclude_none=True)
    assert "duplicate_synthesis" not in json_str

    # Should deserialize fine
    restored = AnalysisResponse.model_validate_json(json_str)
    assert restored.duplicate_synthesis is None


@pytest.mark.unit
def test_duplicate_synthesis_candidate_roundrip() -> None:
    """Test Candidate with all fields roundtrips through JSON."""
    candidate_data = {
        "pr_number": 42,
        "title": "feat: add new feature",
        "author": "carol",
        "role": "contributor",
                "synthesis_score": 0.60,
        "rationale": "partial implementation, needs more tests",
    }

    candidate = DuplicateSynthesisCandidate.model_validate(candidate_data)
    json_str = candidate.model_dump_json(by_alias=True)

    restored = DuplicateSynthesisCandidate.model_validate_json(json_str)
    assert restored.pr_number == 42
    assert restored.role == "contributor"
    assert restored.rationale == "partial implementation, needs more tests"
