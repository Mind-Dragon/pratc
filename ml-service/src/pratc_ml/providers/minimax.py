from __future__ import annotations

import json
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


class MinimaxError(RuntimeError):
    pass


def embed_texts(
    *,
    api_key: str,
    model: str,
    texts: list[str],
    timeout_seconds: float = 30.0,
) -> list[list[float]]:
    endpoint = "https://api.minimax.io/v1/embeddings"
    payload = {
        "model": model,
        "texts": texts,
        "type": "query",
    }

    request = Request(
        endpoint,
        data=json.dumps(payload).encode("utf-8"),
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )

    try:
        with urlopen(request, timeout=timeout_seconds) as response:
            raw = response.read().decode("utf-8")
    except HTTPError as exc:
        details = exc.read().decode("utf-8", errors="replace")
        raise MinimaxError(f"minimax http error {exc.code}: {details}") from exc
    except URLError as exc:
        raise MinimaxError(f"minimax network error: {exc.reason}") from exc

    decoded = json.loads(raw)
    data = decoded.get("vectors", [])
    embeddings: list[list[float]] = []
    for item in data:
        vector = item.get("embedding", [])
        if isinstance(vector, list):
            embeddings.append([float(value) for value in vector])

    if len(embeddings) != len(texts):
        raise MinimaxError(
            f"minimax response size mismatch: expected {len(texts)} embeddings, got {len(embeddings)}"
        )

    return embeddings
