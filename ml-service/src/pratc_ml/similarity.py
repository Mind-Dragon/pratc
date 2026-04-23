from __future__ import annotations

import hashlib
import math
import re
from collections.abc import Iterable
from functools import lru_cache
from typing import Any, Callable


EMBEDDING_TEXT_MAX_FILES = 64
EMBEDDING_TEXT_MAX_CHARS = 8000
HEURISTIC_TITLE_WEIGHT = 0.6
HEURISTIC_FILES_WEIGHT = 0.3
HEURISTIC_BODY_WEIGHT = 0.1
HEURISTIC_WEIGHTS = {
    "title": HEURISTIC_TITLE_WEIGHT,
    "files": HEURISTIC_FILES_WEIGHT,
    "body": HEURISTIC_BODY_WEIGHT,
}
EMBEDDING_CACHE_NORMALIZE_WHITESPACE_RE = re.compile(r"\s+")

_EMBEDDING_CACHE: dict[tuple[str, str, str], list[float]] = {}
_EMBEDDING_CACHE_HITS = 0
_EMBEDDING_CACHE_MISSES = 0


@lru_cache(maxsize=1024)
def tokenize(value: str) -> tuple[str, ...]:
    return tuple(part for part in re.split(r"[^a-z0-9]+", value.lower()) if part)


def normalize_embedding_text(text: str) -> str:
    normalized_lines = [EMBEDDING_CACHE_NORMALIZE_WHITESPACE_RE.sub(" ", line).strip() for line in text.splitlines()]
    return "\n".join(line for line in normalized_lines if line)


def clear_embedding_cache() -> None:
    global _EMBEDDING_CACHE_HITS, _EMBEDDING_CACHE_MISSES
    _EMBEDDING_CACHE.clear()
    _EMBEDDING_CACHE_HITS = 0
    _EMBEDDING_CACHE_MISSES = 0


def get_embedding_cache_stats() -> dict[str, int]:
    return {
        "hits": _EMBEDDING_CACHE_HITS,
        "misses": _EMBEDDING_CACHE_MISSES,
        "entries": len(_EMBEDDING_CACHE),
    }


def jaccard(left: Iterable[str], right: Iterable[str]) -> float:
    left_set = {item.strip().lower() for item in left if item and item.strip()}
    right_set = {item.strip().lower() for item in right if item and item.strip()}
    if not left_set and not right_set:
        return 1.0
    union = left_set | right_set
    if not union:
        return 0.0
    return len(left_set & right_set) / len(union)


def cosine_similarity(left: list[float], right: list[float]) -> float:
    if not left or not right or len(left) != len(right):
        return 0.0

    dot = sum(l * r for l, r in zip(left, right, strict=True))
    left_norm = math.sqrt(sum(v * v for v in left))
    right_norm = math.sqrt(sum(v * v for v in right))
    if left_norm == 0 or right_norm == 0:
        return 0.0

    score = dot / (left_norm * right_norm)
    return max(-1.0, min(1.0, score))


def heuristic_similarity(left: dict, right: dict) -> float:
    title_score = jaccard(tokenize(left.get("title", "")), tokenize(right.get("title", "")))
    body_score = jaccard(tokenize(left.get("body", "")), tokenize(right.get("body", "")))
    files_score = jaccard(left.get("files_changed", []), right.get("files_changed", []))
    if not left.get("files_changed") and not right.get("files_changed"):
        files_score = 0.5

    return round(
        (HEURISTIC_TITLE_WEIGHT * title_score)
        + (HEURISTIC_FILES_WEIGHT * files_score)
        + (HEURISTIC_BODY_WEIGHT * body_score),
        4,
    )


def _summarize_files(files: list[str], *, max_files: int) -> str:
    visible_files = files[:max_files]
    parts = list(visible_files)
    if len(files) > max_files:
        remainder = len(files) - max_files
        label = "file" if remainder == 1 else "files"
        digest = hashlib.sha256("\n".join(files).encode("utf-8")).hexdigest()[:12]
        parts.append(f"... and {remainder} more {label} ({len(files)} files total, digest {digest})")
    return " ".join(parts)


def embedding_text(pr: dict[str, Any], *, max_files: int = EMBEDDING_TEXT_MAX_FILES) -> str:
    title = str(pr.get("title", "")).strip()
    body = str(pr.get("body", "")).strip()
    files = sorted({str(path).strip() for path in pr.get("files_changed", []) if path and str(path).strip()})
    file_text = _summarize_files(files, max_files=max_files)
    text = normalize_embedding_text("\n".join(part for part in (title, body, file_text) if part))
    if len(text) <= EMBEDDING_TEXT_MAX_CHARS:
        return text
    digest = hashlib.sha256(text.encode("utf-8")).hexdigest()[:12]
    return normalize_embedding_text(f"{text[:EMBEDDING_TEXT_MAX_CHARS]}\n... truncated embedding text digest {digest}")


def get_cached_embeddings(
    *,
    backend: str,
    model: str,
    texts: list[str],
    embedder: Callable[[], list[list[float]]],
) -> list[list[float]]:
    global _EMBEDDING_CACHE_HITS, _EMBEDDING_CACHE_MISSES

    normalized_texts = [normalize_embedding_text(text) for text in texts]
    vectors: list[list[float] | None] = []
    missing = False
    for text in normalized_texts:
        cache_key = (backend, model, text)
        cached = _EMBEDDING_CACHE.get(cache_key)
        if cached is None:
            _EMBEDDING_CACHE_MISSES += 1
            missing = True
            vectors.append(None)
        else:
            _EMBEDDING_CACHE_HITS += 1
            vectors.append(cached)

    if not missing:
        return [vector for vector in vectors if vector is not None]

    embeddings = embedder()
    for text, vector in zip(normalized_texts, embeddings, strict=True):
        _EMBEDDING_CACHE[(backend, model, text)] = vector

    return embeddings
