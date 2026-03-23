from __future__ import annotations

import pytest

from pratc_ml.overlap import calculate_overlap
from pratc_ml.providers import BackendConfigError, ProviderConfig


@pytest.mark.unit
def test_calculate_overlap_returns_conflicts_for_shared_files() -> None:
    response = calculate_overlap(
        {
            "repo": "owner/repo",
            "pullRequests": [
                {
                    "number": 20,
                    "mergeable": "mergeable",
                    "base_branch": "main",
                    "files_changed": ["internal/planner/plan.go", "internal/planner/order.go"],
                },
                {
                    "number": 21,
                    "mergeable": "mergeable",
                    "base_branch": "main",
                    "files_changed": ["internal/planner/plan.go"],
                },
            ],
        }
    )

    assert response["status"] == "ok"
    assert len(response["conflicts"]) >= 1
    assert response["conflicts"][0]["source_pr"] == 20
    assert response["conflicts"][0]["target_pr"] == 21


@pytest.mark.unit
def test_provider_config_reads_backend_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "voyage")
    monkeypatch.setenv("VOYAGE_API_KEY", "voyage-key")
    monkeypatch.setenv("VOYAGE_MODEL", "voyage-code-3")

    config = ProviderConfig.from_env()

    assert config.backend == "voyage"
    assert config.voyage_api_key == "voyage-key"
    assert config.voyage_model == "voyage-code-3"


@pytest.mark.unit
def test_provider_config_validate_minimax_missing_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "minimax")
    monkeypatch.delenv("MINIMAX_API_KEY", raising=False)

    config = ProviderConfig.from_env()

    with pytest.raises(BackendConfigError) as exc_info:
        config.validate()

    assert "MINIMAX_API_KEY" in str(exc_info.value)
    assert "ML_BACKEND=minimax" in str(exc_info.value)


@pytest.mark.unit
def test_provider_config_validate_voyage_missing_key(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "voyage")
    monkeypatch.delenv("VOYAGE_API_KEY", raising=False)

    config = ProviderConfig.from_env()

    with pytest.raises(BackendConfigError) as exc_info:
        config.validate()

    assert "VOYAGE_API_KEY" in str(exc_info.value)
    assert "ML_BACKEND=voyage" in str(exc_info.value)


@pytest.mark.unit
def test_provider_config_validate_local_no_key_required(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("ML_BACKEND", "local")
    monkeypatch.delenv("MINIMAX_API_KEY", raising=False)
    monkeypatch.delenv("VOYAGE_API_KEY", raising=False)

    config = ProviderConfig.from_env()

    # Should not raise - local backend requires no API key
    config.validate()
    assert config.backend == "local"
