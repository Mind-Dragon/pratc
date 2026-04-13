from __future__ import annotations

import math
import re
from collections.abc import Iterable
from functools import lru_cache


@lru_cache(maxsize=1024)
def tokenize(value: str) -> tuple[str, ...]:
    return tuple(part for part in re.split(r"[^a-z0-9]+", value.lower()) if part)


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

    return round((0.6 * title_score) + (0.3 * files_score) + (0.1 * body_score), 4)
