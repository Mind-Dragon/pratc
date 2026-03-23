from __future__ import annotations

import pytest

from pratc_ml.duplicates import detect_duplicates


@pytest.mark.unit
def test_detect_duplicates_flags_similar_pull_requests() -> None:
    payload = {
        "repo": "owner/repo",
        "duplicateThreshold": 0.9,
        "overlapThreshold": 0.7,
        "pullRequests": [
            {
                "number": 10,
                "title": "planner: simplify candidate scoring",
                "body": "adjust planner scoring logic",
                "files_changed": ["internal/planner/scoring.go"],
            },
            {
                "number": 11,
                "title": "planner simplify candidate scoring",
                "body": "adjust planner scoring logic",
                "files_changed": ["internal/planner/scoring.go"],
            },
            {
                "number": 12,
                "title": "planner: improve ordering heuristics",
                "body": "related planner change",
                "files_changed": ["internal/planner/order.go"],
            },
        ],
    }
    response = detect_duplicates(payload)

    assert response["status"] == "ok"
    assert len(response["duplicates"]) >= 1
    assert any(group["canonical_pr_number"] == 10 for group in response["duplicates"])
