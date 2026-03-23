from __future__ import annotations

import json
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


class VoyageError(RuntimeError):
    pass


def embed_texts(
    *,
    api_key: str,
    model: str,
    base_url: str,
    texts: list[str],
    input_type: str = "document",
    timeout_seconds: float = 30.0,
) -> list[list[float]]:
    endpoint = f"{base_url.rstrip('/')}/embeddings"
    payload = {
        "input": texts,
        "model": model,
        "input_type": input_type,
        "truncation": True,
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
        raise VoyageError(f"voyage http error {exc.code}: {details}") from exc
    except URLError as exc:
        raise VoyageError(f"voyage network error: {exc.reason}") from exc

    decoded = json.loads(raw)
    data = decoded.get("data", [])
    embeddings: list[list[float]] = []
    for item in data:
        vector = item.get("embedding", [])
        if isinstance(vector, list):
            embeddings.append([float(value) for value in vector])

    if len(embeddings) != len(texts):
        raise VoyageError(
            f"voyage response size mismatch: expected {len(texts)} embeddings, got {len(embeddings)}"
        )

    return embeddings
