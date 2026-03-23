from __future__ import annotations

import json
import sys
from io import TextIOBase
from typing import Any

from pratc_ml.clustering import cluster_pull_requests
from pratc_ml.duplicates import detect_duplicates
from pratc_ml.overlap import calculate_overlap
from pratc_ml.providers import BackendConfigError


def _handle_action(payload: dict[str, Any]) -> dict[str, Any]:
    action = payload.get("action")
    if action == "health":
        return {"status": "ok"}
    if action == "cluster":
        return cluster_pull_requests(payload)
    if action == "duplicates":
        return detect_duplicates(payload)
    if action == "overlap":
        return calculate_overlap(payload)
    return {"error": "unknown action", "action": action}


def _handle_config_error(error: BackendConfigError) -> dict[str, Any]:
    """Handle backend configuration errors and return a structured error response."""
    return {
        "error": "configuration_error",
        "message": str(error),
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
        response = _handle_config_error(exc)

    json.dump(response, output_stream)
    output_stream.write("\n")

    if "error" in response:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
