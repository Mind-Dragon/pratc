from __future__ import annotations

from typing import Any


def calculate_overlap(payload: dict[str, Any]) -> dict[str, Any]:
    prs = payload.get("pullRequests") or payload.get("prs") or []
    if not isinstance(prs, list):
        prs = []

    conflicts: list[dict[str, Any]] = []
    for left_index in range(len(prs)):
        for right_index in range(left_index + 1, len(prs)):
            left = prs[left_index]
            right = prs[right_index]

            if (
                left.get("base_branch")
                and right.get("base_branch")
                and left.get("base_branch") != right.get("base_branch")
            ):
                continue

            left_files = set(left.get("files_changed", []))
            right_files = set(right.get("files_changed", []))
            shared = sorted(left_files & right_files)

            if (
                not shared
                and left.get("mergeable") != "conflicting"
                and right.get("mergeable") != "conflicting"
            ):
                continue

            severity = "low"
            if len(shared) >= 3:
                severity = "high"
            elif (
                shared
                or left.get("mergeable") == "conflicting"
                or right.get("mergeable") == "conflicting"
            ):
                severity = "medium"

            conflicts.append(
                {
                    "source_pr": int(left.get("number", 0)),
                    "target_pr": int(right.get("number", 0)),
                    "conflict_type": "file_overlap",
                    "files_touched": shared if shared else ["mergeability_signal"],
                    "severity": severity,
                    "reason": "shared files detected"
                    if shared
                    else "mergeability indicates conflict",
                }
            )

    conflicts.sort(key=lambda item: (item["source_pr"], item["target_pr"]))

    return {
        "action": "overlap",
        "status": "ok",
        "repo": payload.get("repo"),
        "conflicts": conflicts,
    }
