from __future__ import annotations

import pytest

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
