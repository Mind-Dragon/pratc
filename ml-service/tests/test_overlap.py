from __future__ import annotations

import pytest

from pratc_ml.overlap import calculate_overlap
from pratc_ml.providers import ProviderConfig


@pytest.mark.unit
def test_calculate_overlap_returns_empty_conflicts() -> None:
    response = calculate_overlap({"repo": "owner/repo", "pullRequests": []})

    assert response["status"] == "not_implemented"
    assert response["conflicts"] == []


@pytest.mark.unit
def test_provider_config_reads_backend_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "openrouter")
    monkeypatch.setenv("OPENROUTER_API_KEY", "test-key")
    monkeypatch.setenv("OPENROUTER_EMBED_MODEL", "openrouter/embed")
    monkeypatch.setenv("OPENROUTER_REASON_MODEL", "openrouter/reason")

    config = ProviderConfig.from_env()

    assert config.backend == "openrouter"
    assert config.openrouter_api_key == "test-key"
    assert config.openrouter_embed_model == "openrouter/embed"
    assert config.openrouter_reason_model == "openrouter/reason"
