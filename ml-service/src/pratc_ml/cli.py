from __future__ import annotations

import json
import sys
from io import TextIOBase
from typing import Any

from pratc_ml.clustering import cluster_pull_requests
from pratc_ml.duplicates import detect_duplicates
from pratc_ml.logging import error, get_request_id, info
from pratc_ml.overlap import calculate_overlap
from pratc_ml.providers import BackendConfigError


def run_analyzer(payload: dict[str, Any]) -> dict[str, Any]:
    """Return explicit not-implemented status for optional Python analyzers."""
    return {
        "error": "not_implemented",
        "status": "not_implemented",
        "action": payload.get("action", "analyze"),
        "message": "Python analyze action is not implemented",
    }


def _handle_action(payload: dict[str, Any]) -> dict[str, Any]:
    action = payload.get("action")
    request_id = get_request_id(payload)
    if action == "health":
        info(request_id, "health check")
        return {"status": "ok"}
    if action == "cluster":
        info(request_id, "clustering started", repo=payload.get("repo"))
        try:
            result = cluster_pull_requests(payload)
            info(request_id, "clustering completed", repo=payload.get("repo"))
            return result
        except Exception as exc:
            error(request_id, "clustering failed", repo=payload.get("repo"), error=str(exc))
            raise
    if action == "duplicates":
        info(request_id, "duplicates detection started", repo=payload.get("repo"))
        try:
            result = detect_duplicates(payload)
            info(request_id, "duplicates detection completed", repo=payload.get("repo"))
            return result
        except Exception as exc:
            error(
                request_id, "duplicates detection failed", repo=payload.get("repo"), error=str(exc)
            )
            raise
    if action == "overlap":
        info(request_id, "overlap calculation started", repo=payload.get("repo"))
        try:
            result = calculate_overlap(payload)
            info(request_id, "overlap calculation completed", repo=payload.get("repo"))
            return result
        except Exception as exc:
            error(
                request_id, "overlap calculation failed", repo=payload.get("repo"), error=str(exc)
            )
            raise
    if action == "analyze":
        info(request_id, "analyzer action started", repo=payload.get("repo"))
        try:
            result = run_analyzer(payload)
            info(request_id, "analyzer action completed", repo=payload.get("repo"))
            return result
        except Exception as exc:
            error(request_id, "analyzer action failed", repo=payload.get("repo"), error=str(exc))
            raise
    return {"error": "unknown action", "action": action}


def _handle_config_error(exc: BackendConfigError, request_id: str | None = None) -> dict[str, Any]:
    error(request_id, "backend configuration error", error=str(exc))
    return {
        "error": "configuration_error",
        "message": str(exc),
        "status": "configuration_error",
    }


def main(stdin: TextIOBase | None = None, stdout: TextIOBase | None = None) -> int:
    input_stream = stdin or sys.stdin
    output_stream = stdout or sys.stdout

    try:
        payload = json.load(input_stream)
    except json.JSONDecodeError:
        json.dump({"error": "invalid json"}, output_stream)
        output_stream.write("\n")
        return 1

    try:
        response = _handle_action(payload)
    except BackendConfigError as exc:
        response = _handle_config_error(exc, get_request_id(payload))

    json.dump(response, output_stream)
    output_stream.write("\n")

    if "error" in response:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
