#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


ROOT = Path(__file__).resolve().parents[1]
FIXTURES_DIR = ROOT / "fixtures"
PRS_DIR = FIXTURES_DIR / "prs"
MANIFEST_PATH = FIXTURES_DIR / "manifest.json"
DEFAULT_REPO = "opencode-ai/opencode"
REQUEST_SLEEP_SECONDS = 0.25


@dataclass(frozen=True)
class FixtureManifest:
    repo: str
    fetched_at: str
    count: int
    pr_numbers: list[int]
    command: str
    sanitized: bool
    description: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Fetch and freeze PR fixtures for prATC.")
    parser.add_argument("--repo", default=DEFAULT_REPO, help="GitHub repository in owner/name form.")
    parser.add_argument("--limit", type=int, default=100, help="Maximum number of PRs to fetch.")
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Fetch and transform data without writing fixture files.",
    )
    return parser.parse_args()


def run_gh_list(repo: str, limit: int) -> list[dict[str, Any]]:
    command = [
        "gh",
        "pr",
        "list",
        "--repo",
        repo,
        "--state",
        "open",
        "--limit",
        str(limit),
        "--json",
        ",".join(
            [
                "number",
                "title",
                "body",
                "author",
                "labels",
                "files",
                "createdAt",
                "updatedAt",
                "baseRefName",
                "headRefName",
                "mergeable",
                "isDraft",
                "additions",
                "deletions",
                "reviewDecision",
                "statusCheckRollup",
                "url",
            ]
        ),
    ]
    completed = subprocess.run(command, check=True, capture_output=True, text=True)
    return json.loads(completed.stdout)


def summarize_ci_status(status_checks: list[dict[str, Any]]) -> str:
    if not status_checks:
        return "unknown"

    conclusions = []
    pending = False
    for check in status_checks:
        status = str(check.get("status") or "").upper()
        conclusion = str(check.get("conclusion") or "").upper()
        if status and status != "COMPLETED":
            pending = True
        if conclusion:
            conclusions.append(conclusion)

    if any(conclusion in {"FAILURE", "TIMED_OUT", "ACTION_REQUIRED", "CANCELLED"} for conclusion in conclusions):
        return "failed"
    if pending:
        return "pending"
    if conclusions and all(conclusion == "SUCCESS" for conclusion in conclusions):
        return "passed"
    return "unknown"


def normalize_review_status(review_decision: str | None) -> str:
    decision = (review_decision or "").strip().upper()
    mapping = {
        "APPROVED": "approved",
        "CHANGES_REQUESTED": "changes_requested",
        "REVIEW_REQUIRED": "review_required",
    }
    return mapping.get(decision, "unknown")


def transform_pr(repo: str, payload: dict[str, Any]) -> dict[str, Any]:
    author = payload.get("author") or {}
    files = payload.get("files") or []
    labels = payload.get("labels") or []
    login = str(author.get("login") or "unknown")

    return {
        "id": f"{repo}#{payload['number']}",
        "repo": repo,
        "number": payload["number"],
        "title": payload.get("title") or "",
        "body": payload.get("body") or "",
        "url": payload.get("url") or "",
        "author": login,
        "labels": [str(label.get("name") or "") for label in labels if label.get("name")],
        "files_changed": [str(file_info.get("path") or "") for file_info in files if file_info.get("path")],
        "review_status": normalize_review_status(payload.get("reviewDecision")),
        "ci_status": summarize_ci_status(payload.get("statusCheckRollup") or []),
        "mergeable": str(payload.get("mergeable") or "UNKNOWN").lower(),
        "base_branch": payload.get("baseRefName") or "",
        "head_branch": payload.get("headRefName") or "",
        "cluster_id": "",
        "created_at": payload.get("createdAt") or "",
        "updated_at": payload.get("updatedAt") or "",
        "is_draft": bool(payload.get("isDraft")),
        "is_bot": bool(author.get("is_bot")) or login.endswith("[bot]"),
        "additions": int(payload.get("additions") or 0),
        "deletions": int(payload.get("deletions") or 0),
        "changed_files_count": len(files),
    }


def write_fixtures(repo: str, fixtures: list[dict[str, Any]]) -> FixtureManifest:
    FIXTURES_DIR.mkdir(parents=True, exist_ok=True)
    PRS_DIR.mkdir(parents=True, exist_ok=True)

    for path in PRS_DIR.glob("pr-*.json"):
        path.unlink()

    for fixture in fixtures:
        target = PRS_DIR / f"pr-{fixture['number']}.json"
        target.write_text(json.dumps(fixture, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    manifest = FixtureManifest(
        repo=repo,
        fetched_at=datetime.now(timezone.utc).isoformat(),
        count=len(fixtures),
        pr_numbers=[fixture["number"] for fixture in fixtures],
        command=f"gh pr list --repo {repo} --state open --limit 100 --json …",
        sanitized=True,
        description="Public open PR fixtures normalized to prATC internal/types.PR shape.",
    )
    MANIFEST_PATH.write_text(
        json.dumps(manifest.__dict__, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return manifest


def main() -> int:
    args = parse_args()
    time.sleep(REQUEST_SLEEP_SECONDS)
    pulled = run_gh_list(args.repo, args.limit)
    fixtures = [transform_pr(args.repo, payload) for payload in pulled]
    fixtures.sort(key=lambda fixture: fixture["number"], reverse=True)

    if args.dry_run:
        print(
            json.dumps(
                {
                    "repo": args.repo,
                    "count": len(fixtures),
                    "numbers": [fixture["number"] for fixture in fixtures[:10]],
                    "dry_run": True,
                },
                indent=2,
                sort_keys=True,
            )
        )
        return 0

    manifest = write_fixtures(args.repo, fixtures)
    print(json.dumps(manifest.__dict__, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
