from __future__ import annotations

import pytest

import pratc_ml.clustering as clustering
from pratc_ml.clustering import cluster_pull_requests


@pytest.mark.unit
def test_cluster_pull_requests_groups_related_titles() -> None:
    payload = {
        "repo": "owner/repo",
        "pullRequests": [
            {
                "number": 1,
                "title": "chore(deps): bump react",
                "body": "dependency refresh",
                "files_changed": ["web/package.json"],
            },
            {
                "number": 2,
                "title": "chore(deps): bump next",
                "body": "dependency refresh",
                "files_changed": ["web/package.json"],
            },
            {
                "number": 3,
                "title": "planner: optimize merge ordering",
                "body": "planner work",
                "files_changed": ["internal/planner/plan.go"],
            },
        ],
    }
    response = cluster_pull_requests(payload)

    assert response["status"] == "ok"
    assert response["action"] == "cluster"
    assert len(response["clusters"]) >= 2
    assert any(len(cluster["pr_ids"]) >= 2 for cluster in response["clusters"])


@pytest.mark.unit
def test_cluster_response_exposes_local_backend_degradation_metadata(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "local")

    response = cluster_pull_requests(
        {
            "repo": "owner/repo",
            "pullRequests": [
                {"number": 1, "title": "planner: one", "body": "", "files_changed": ["a.py"]},
                {"number": 2, "title": "planner: two", "body": "", "files_changed": ["b.py"]},
            ],
        }
    )

    assert response["degradation"] == {
        "embeddings_used": False,
        "heuristic_fallback": True,
        "fallback_reason": "local_backend",
    }


@pytest.mark.unit
def test_cluster_embedding_text_accounts_for_files_beyond_first_five() -> None:
    text = clustering._embedding_text(
        {
            "title": "planner update",
            "body": "body",
            "files_changed": [f"src/file_{index}.py" for index in range(1, 7)],
        }
    )

    assert "src/file_6.py" in text or "1 more file" in text.lower() or "6 files" in text.lower()


@pytest.mark.unit
@pytest.mark.parametrize(
    ("backend", "api_key_env", "embed_attr", "error", "expected_reason"),
    [
        ("minimax", "MINIMAX_API_KEY", "minimax_embed_texts", clustering.MinimaxError("boom"), "minimax_error"),
        ("voyage", "VOYAGE_API_KEY", "voyage_embed_texts", clustering.VoyageError("boom"), "voyage_error"),
    ],
)
def test_cluster_provider_error_fallback_reports_heuristic_model_honestly(
    monkeypatch: pytest.MonkeyPatch,
    backend: str,
    api_key_env: str,
    embed_attr: str,
    error: Exception,
    expected_reason: str,
) -> None:
    monkeypatch.setenv("ML_BACKEND", backend)
    monkeypatch.setenv(api_key_env, "test-key")

    def fake_embed_texts(**_: object) -> list[list[float]]:
        raise error

    monkeypatch.setattr(clustering, embed_attr, fake_embed_texts)

    response = cluster_pull_requests(
        {
            "repo": "owner/repo",
            "pullRequests": [
                {"number": 1, "title": "planner: alpha", "body": "", "files_changed": ["a.py"]},
                {"number": 2, "title": "planner: beta", "body": "", "files_changed": ["b.py"]},
            ],
        }
    )

    assert response["degradation"] == {
        "embeddings_used": False,
        "heuristic_fallback": True,
        "fallback_reason": expected_reason,
    }
    assert response["model"] == "heuristic-fallback"


@pytest.mark.unit
def test_cluster_reuses_embedding_cache_for_identical_requests(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "minimax")
    monkeypatch.setenv("MINIMAX_API_KEY", "test-key")
    calls: list[list[str]] = []

    def fake_embed_texts(*, api_key: str, model: str, texts: list[str]) -> list[list[float]]:
        calls.append(list(texts))
        return [[1.0, 0.0] for _ in texts]

    monkeypatch.setattr(clustering, "minimax_embed_texts", fake_embed_texts)

    payload = {
        "repo": "owner/repo",
        "pullRequests": [
            {"number": 1, "title": "planner: alpha", "body": "", "files_changed": ["a.py"]},
            {"number": 2, "title": "planner: beta", "body": "", "files_changed": ["b.py"]},
        ],
    }

    cluster_pull_requests(payload)
    cluster_pull_requests(payload)

    assert len(calls) == 1
