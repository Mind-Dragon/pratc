from __future__ import annotations

import json
from io import StringIO

import pytest

from pratc_ml import cli


def invoke(payload: dict[str, object]) -> tuple[int, dict[str, object]]:
    stdin = StringIO(json.dumps(payload))
    stdout = StringIO()
    exit_code = cli.main(stdin=stdin, stdout=stdout)
    return exit_code, json.loads(stdout.getvalue())


@pytest.mark.unit
def test_health_action_returns_ok_status() -> None:
    exit_code, response = invoke({"action": "health"})

    assert exit_code == 0
    assert response == {"status": "ok"}


@pytest.mark.unit
def test_unknown_action_returns_structured_error() -> None:
    exit_code, response = invoke({"action": "nonexistent"})

    assert exit_code == 1
    assert response["error"] == "unknown action"
    assert response["action"] == "nonexistent"


@pytest.mark.unit
def test_invalid_json_returns_structured_error() -> None:
    stdout = StringIO()
    exit_code = cli.main(stdin=StringIO("{"), stdout=stdout)
    response = json.loads(stdout.getvalue())

    assert exit_code == 1
    assert response["error"] == "invalid json"

