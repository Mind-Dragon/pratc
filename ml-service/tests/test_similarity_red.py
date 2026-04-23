from __future__ import annotations

import pytest

import pratc_ml.duplicates as duplicates
import pratc_ml.similarity as similarity


@pytest.mark.unit
def test_embedding_text_is_deterministic_across_input_file_order() -> None:
    ordered = {
        "title": "planner update",
        "body": "keep embeddings stable",
        "files_changed": [
            "web/zeta.ts",
            "internal/planner/plan.go",
            "README.md",
            "docs/guide.md",
            "cmd/pratc/main.go",
            "internal/app/service.go",
        ],
    }
    reversed_order = {
        **ordered,
        "files_changed": list(reversed(ordered["files_changed"])),
    }

    assert similarity.embedding_text(ordered) == similarity.embedding_text(reversed_order)


@pytest.mark.unit
def test_duplicate_detection_knobs_are_exposed_via_centralized_constants() -> None:
    assert duplicates.EMBEDDING_TEXT_MAX_FILES == similarity.EMBEDDING_TEXT_MAX_FILES
    assert duplicates.LSH_NUM_PERM == 128
    assert duplicates.LSH_MIN_CANDIDATE_THRESHOLD == 0.5
    assert similarity.HEURISTIC_WEIGHTS == {
        "title": 0.6,
        "files": 0.3,
        "body": 0.1,
    }


@pytest.mark.unit
def test_embedding_text_stays_bounded_for_large_file_lists() -> None:
    text = similarity.embedding_text(
        {
            "title": "large change",
            "body": "body" * 2000,
            "files_changed": [f"src/generated/path_{index:04d}.py" for index in range(500)],
        }
    )

    assert len(text) <= similarity.EMBEDDING_TEXT_MAX_CHARS + 80
    assert "truncated embedding text digest" in text
    assert "digest" in text


@pytest.mark.unit
def test_embedding_cache_normalizes_text_and_exposes_hit_miss_stats() -> None:
    similarity.clear_embedding_cache()
    calls: list[list[str]] = []

    def fake_embedder() -> list[list[float]]:
        calls.append(["called"])
        return [[1.0, 0.0]]

    stats_before = similarity.get_embedding_cache_stats()

    similarity.get_cached_embeddings(
        backend="minimax",
        model="test-model",
        texts=["Planner update\nBody\ninternal/app/service.go"],
        embedder=fake_embedder,
    )
    similarity.get_cached_embeddings(
        backend="minimax",
        model="test-model",
        texts=["  Planner update\n\nBody\ninternal/app/service.go  "],
        embedder=fake_embedder,
    )

    stats_after = similarity.get_embedding_cache_stats()

    assert len(calls) == 1
    assert stats_after == {
        "hits": stats_before["hits"] + 1,
        "misses": stats_before["misses"] + 1,
        "entries": stats_before["entries"] + 1,
    }
