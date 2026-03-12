from __future__ import annotations

from typing import Any


def cluster_pull_requests(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "action": "cluster",
        "status": "not_implemented",
        "repo": payload.get("repo"),
        "clusters": [],
    }
