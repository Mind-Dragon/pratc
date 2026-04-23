from __future__ import annotations

from typing import Any

from datasketch import MinHash, MinHashLSH

from pratc_ml.providers import ProviderConfig
from pratc_ml.providers.minimax import MinimaxError, embed_texts as minimax_embed_texts
from pratc_ml.providers.voyage import VoyageError, embed_texts as voyage_embed_texts
from pratc_ml.similarity import (
    EMBEDDING_TEXT_MAX_FILES,
    cosine_similarity,
    embedding_text,
    get_cached_embeddings,
    heuristic_similarity,
)


LSH_NUM_PERM = 128
LSH_MIN_CANDIDATE_THRESHOLD = 0.5


def _embedding_text(pr: dict[str, Any]) -> str:
    return embedding_text(pr)


def _minhash_signature(pr: dict[str, Any], num_perm: int = LSH_NUM_PERM) -> MinHash:
    """Create a MinHash signature for a PR using its title, body, and files."""
    tokens: set[str] = set()
    title = pr.get("title", "")
    body = pr.get("body", "")
    files = sorted({str(path).strip() for path in pr.get("files_changed", []) if path and str(path).strip()})[
        :EMBEDDING_TEXT_MAX_FILES
    ]

    for part in title.lower().split():
        if part:
            tokens.add(part)
    for part in body.lower().split():
        if part:
            tokens.add(part)
    for f in files:
        for part in f.lower().split("/"):
            if part:
                tokens.add(part)

    m = MinHash(num_perm=num_perm)
    for token in tokens:
        m.update(token.encode("utf8"))
    return m


def _get_candidate_pairs_lsh(
    prs: list[dict[str, Any]], threshold: float = LSH_MIN_CANDIDATE_THRESHOLD, num_perm: int = LSH_NUM_PERM
) -> set[tuple[int, int]]:
    """Use MinHash LSH to find candidate similar PR pairs."""
    if len(prs) < 2:
        return set()

    lsh = MinHashLSH(threshold=threshold, num_perm=num_perm)
    minhashes: list[MinHash] = []

    for i, pr in enumerate(prs):
        m = _minhash_signature(pr, num_perm=num_perm)
        minhashes.append(m)
        lsh.insert(f"pr_{i}", m)

    candidates: set[tuple[int, int]] = set()
    for i in range(len(prs)):
        result = lsh.query(minhashes[i])
        for rid in result:
            j = int(rid.split("_")[1])
            if i < j:
                candidates.add((i, j))
            elif j < i:
                candidates.add((j, i))

    return candidates


def detect_duplicates(payload: dict[str, Any]) -> dict[str, Any]:
    prs = payload.get("pullRequests") or payload.get("prs") or []
    if not isinstance(prs, list):
        prs = []

    duplicate_threshold = float(payload.get("duplicateThreshold", 0.9))
    overlap_threshold = float(payload.get("overlapThreshold", 0.7))

    config = ProviderConfig.from_env()
    config.validate()  # Raises BackendConfigError if required config is missing
    embeddings: list[list[float]] | None = None
    degradation = {
        "embeddings_used": False,
        "heuristic_fallback": config.backend == "local",
        "fallback_reason": "local_backend" if config.backend == "local" else None,
    }
    if config.backend == "minimax" and config.minimax_api_key and prs:
        texts = [_embedding_text(pr) for pr in prs]
        try:
            embeddings = get_cached_embeddings(
                backend=config.backend,
                model=config.minimax_embed_model,
                texts=texts,
                embedder=lambda: minimax_embed_texts(
                    api_key=config.minimax_api_key,
                    model=config.minimax_embed_model,
                    texts=texts,
                ),
            )
            degradation = {
                "embeddings_used": True,
                "heuristic_fallback": False,
                "fallback_reason": None,
            }
        except MinimaxError:
            embeddings = None
            degradation = {
                "embeddings_used": False,
                "heuristic_fallback": True,
                "fallback_reason": "minimax_error",
            }
    elif config.backend == "voyage" and config.voyage_api_key and prs:
        texts = [_embedding_text(pr) for pr in prs]
        try:
            embeddings = get_cached_embeddings(
                backend=config.backend,
                model=config.voyage_model,
                texts=texts,
                embedder=lambda: voyage_embed_texts(
                    api_key=config.voyage_api_key,
                    model=config.voyage_model,
                    base_url=config.voyage_base_url,
                    texts=texts,
                ),
            )
            degradation = {
                "embeddings_used": True,
                "heuristic_fallback": False,
                "fallback_reason": None,
            }
        except VoyageError:
            embeddings = None
            degradation = {
                "embeddings_used": False,
                "heuristic_fallback": True,
                "fallback_reason": "voyage_error",
            }

    duplicates: dict[int, dict[str, Any]] = {}
    overlaps: dict[int, dict[str, Any]] = {}

    n = len(prs)
    if n < 100:
        pairs_to_check = [(i, j) for i in range(n) for j in range(i + 1, n)]
    else:
        lsh_threshold = max(overlap_threshold * 0.8, 0.5)
        pairs_to_check = _get_candidate_pairs_lsh(prs, threshold=lsh_threshold)
        if len(pairs_to_check) < n * (n - 1) // 4:
            pass
        else:
            pairs_to_check = [(i, j) for i in range(n) for j in range(i + 1, n)]

    for i, j in pairs_to_check:
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
        "degradation": degradation,
        "duplicates": duplicate_groups,
        "overlaps": overlap_groups,
    }
