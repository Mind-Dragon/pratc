from __future__ import annotations

import pytest

import pratc_ml.duplicates as duplicates
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


@pytest.mark.unit
def test_duplicates_response_exposes_local_backend_degradation_metadata(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "local")

    response = detect_duplicates(
        {
            "repo": "owner/repo",
            "pullRequests": [
                {"number": 10, "title": "planner alpha", "body": "", "files_changed": ["a.py"]},
                {"number": 11, "title": "planner beta", "body": "", "files_changed": ["b.py"]},
            ],
        }
    )

    assert response["degradation"] == {
        "embeddings_used": False,
        "heuristic_fallback": True,
        "fallback_reason": "local_backend",
    }


@pytest.mark.unit
def test_duplicate_embedding_text_accounts_for_files_beyond_first_five() -> None:
    text = duplicates._embedding_text(
        {
            "title": "planner update",
            "body": "body",
            "files_changed": [f"src/file_{index}.py" for index in range(1, 7)],
        }
    )

    assert "src/file_6.py" in text or "1 more file" in text.lower() or "6 files" in text.lower()


@pytest.mark.unit
def test_duplicates_reuse_embedding_cache_for_identical_requests(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "minimax")
    monkeypatch.setenv("MINIMAX_API_KEY", "test-key")
    calls: list[list[str]] = []

    def fake_embed_texts(*, api_key: str, model: str, texts: list[str]) -> list[list[float]]:
        calls.append(list(texts))
        return [[1.0, 0.0] for _ in texts]

    monkeypatch.setattr(duplicates, "minimax_embed_texts", fake_embed_texts)

    payload = {
        "repo": "owner/repo",
        "pullRequests": [
            {"number": 10, "title": "planner alpha", "body": "", "files_changed": ["a.py"]},
            {"number": 11, "title": "planner beta", "body": "", "files_changed": ["b.py"]},
        ],
    }

    detect_duplicates(payload)
    detect_duplicates(payload)

    assert len(calls) == 1
