from __future__ import annotations

from typing import Any

from pratc_ml.providers import ProviderConfig
from pratc_ml.providers.minimax import MinimaxError, embed_texts as minimax_embed_texts
from pratc_ml.providers.voyage import VoyageError, embed_texts as voyage_embed_texts
from pratc_ml.similarity import cosine_similarity, heuristic_similarity


def _embedding_text(pr: dict[str, Any]) -> str:
    files = " ".join(pr.get("files_changed", [])[:5])
    return f"{pr.get('title', '')}\n{pr.get('body', '')}\n{files}".strip()


def detect_duplicates(payload: dict[str, Any]) -> dict[str, Any]:
    prs = payload.get("pullRequests") or payload.get("prs") or []
    if not isinstance(prs, list):
        prs = []

    duplicate_threshold = float(payload.get("duplicateThreshold", 0.9))
    overlap_threshold = float(payload.get("overlapThreshold", 0.7))

    config = ProviderConfig.from_env()
    config.validate()  # Raises BackendConfigError if required config is missing
    embeddings: list[list[float]] | None = None
    if config.backend == "minimax" and config.minimax_api_key and prs:
        texts = [_embedding_text(pr) for pr in prs]
        try:
            embeddings = minimax_embed_texts(
                api_key=config.minimax_api_key,
                model=config.minimax_embed_model,
                texts=texts,
            )
        except MinimaxError:
            embeddings = None
    elif config.backend == "voyage" and config.voyage_api_key and prs:
        texts = [_embedding_text(pr) for pr in prs]
        try:
            embeddings = voyage_embed_texts(
                api_key=config.voyage_api_key,
                model=config.voyage_model,
                base_url=config.voyage_base_url,
                texts=texts,
            )
        except VoyageError:
            embeddings = None

    duplicates: dict[int, dict[str, Any]] = {}
    overlaps: dict[int, dict[str, Any]] = {}

    for i in range(len(prs)):
        for j in range(i + 1, len(prs)):
            left = prs[i]
            right = prs[j]

            if embeddings is not None:
                score = round((cosine_similarity(embeddings[i], embeddings[j]) + 1.0) / 2.0, 4)
            else:
                score = heuristic_similarity(left, right)

            if score < overlap_threshold:
                continue

            canonical = min(int(left.get("number", 0)), int(right.get("number", 0)))
            duplicate = max(int(left.get("number", 0)), int(right.get("number", 0)))

            target = duplicates if score > duplicate_threshold else overlaps
            if canonical not in target:
                target[canonical] = {
                    "canonical_pr_number": canonical,
                    "duplicate_pr_numbers": [],
                    "similarity": score,
                    "reason": "similarity above duplicate threshold"
                    if target is duplicates
                    else "similarity in overlap threshold range",
                }

            target[canonical]["similarity"] = max(target[canonical]["similarity"], score)
            if duplicate not in target[canonical]["duplicate_pr_numbers"]:
                target[canonical]["duplicate_pr_numbers"].append(duplicate)

    duplicate_groups = [duplicates[key] for key in sorted(duplicates)]
    overlap_groups = [overlaps[key] for key in sorted(overlaps)]

    for group in duplicate_groups + overlap_groups:
        group["duplicate_pr_numbers"].sort()

    return {
        "action": "duplicates",
        "status": "ok",
        "repo": payload.get("repo"),
        "duplicates": duplicate_groups,
        "overlaps": overlap_groups,
    }
