"""Structured logging for prATC ML service.

Logs to stderr only when PRATC_ML_DEBUG=1, matching Go logger JSON format.
Stdout is reserved for JSON IPC with the Go bridge.
"""

from __future__ import annotations

import json
import os
import sys
from datetime import datetime, timezone
from typing import Any


# Debug mode is enabled when PRATC_ML_DEBUG=1
_DEBUG = os.environ.get("PRATC_ML_DEBUG", "0") == "1"

_COMPONENT = "ml"


def _timestamp() -> str:
    """Return current UTC timestamp in RFC 3339 format."""
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%f")[:-3] + "Z"


def _log(level: str, request_id: str | None, msg: str, **kwargs: Any) -> None:
    """Emit a structured JSON log line to stderr.

    Args:
        level: Log level (INFO or ERROR)
        request_id: Request ID from payload (may be None)
        msg: Human-readable message
        **kwargs: Additional context fields
    """
    if not _DEBUG:
        return

    entry: dict[str, Any] = {
        "ts": _timestamp(),
        "level": level,
        "component": _COMPONENT,
        "msg": msg,
    }

    if request_id:
        entry["request_id"] = request_id

    # Sort kwargs keys alphabetically after core fields
    for key in sorted(kwargs.keys()):
        entry[key] = kwargs[key]

    sys.stderr.write(json.dumps(entry) + "\n")
    sys.stderr.flush()


def info(request_id: str | None, msg: str, **kwargs: Any) -> None:
    """Log an INFO-level message to stderr."""
    _log("INFO", request_id, msg, **kwargs)


def error(request_id: str | None, msg: str, **kwargs: Any) -> None:
    """Log an ERROR-level message to stderr."""
    _log("ERROR", request_id, msg, **kwargs)


def get_request_id(payload: dict[str, Any]) -> str | None:
    """Extract request_id from payload if present."""
    return payload.get("request_id") or None
