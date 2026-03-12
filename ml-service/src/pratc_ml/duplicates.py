from __future__ import annotations

from typing import Any


def detect_duplicates(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "action": "duplicates",
        "status": "not_implemented",
        "repo": payload.get("repo"),
        "duplicates": [],
        "overlaps": [],
    }
