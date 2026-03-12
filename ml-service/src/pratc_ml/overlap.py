from __future__ import annotations

from typing import Any


def calculate_overlap(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "action": "overlap",
        "status": "not_implemented",
        "repo": payload.get("repo"),
        "conflicts": [],
    }
