from __future__ import annotations

import pytest

from pratc_ml.clustering import cluster_pull_requests


@pytest.mark.unit
def test_cluster_pull_requests_returns_stubbed_payload() -> None:
    response = cluster_pull_requests({"repo": "owner/repo", "pullRequests": []})

    assert response["status"] == "not_implemented"
    assert response["action"] == "cluster"
    assert response["clusters"] == []

