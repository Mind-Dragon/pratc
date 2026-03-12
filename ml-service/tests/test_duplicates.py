from __future__ import annotations

import pytest

from pratc_ml.duplicates import detect_duplicates


@pytest.mark.unit
def test_detect_duplicates_returns_empty_duplicate_sets() -> None:
    response = detect_duplicates({"repo": "owner/repo", "pullRequests": []})

    assert response["status"] == "not_implemented"
    assert response["duplicates"] == []
    assert response["overlaps"] == []

