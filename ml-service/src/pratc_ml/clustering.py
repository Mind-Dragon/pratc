from __future__ import annotations

from typing import Any

import numpy as np

from pratc_ml.providers import ProviderConfig
from pratc_ml.providers.minimax import MinimaxError, embed_texts as minimax_embed_texts
from pratc_ml.providers.voyage import VoyageError, embed_texts as voyage_embed_texts
from pratc_ml.similarity import cosine_similarity, heuristic_similarity


def _embedding_text(pr: dict[str, Any]) -> str:
    files = " ".join(pr.get("files_changed", [])[:5])
    return f"{pr.get('title', '')}\n{pr.get('body', '')}\n{files}".strip()


def _cluster_key(pr: dict[str, Any]) -> str:
    title = str(pr.get("title", "")).lower()
    labels = [str(label).lower() for label in pr.get("labels", [])]
    if pr.get("is_bot") or any("depend" in label for label in labels):
        return "dependency"
    if "planner" in title:
        return "planner"
    if labels:
        return labels[0]
    return title.split(" ")[0] if title else "general"


def _compute_similarity_matrix(
    embeddings: list[list[float]], indices: list[int]
) -> np.ndarray:
    """Compute pairwise cosine similarity matrix for given embedding indices."""
    n = len(indices)
    if n < 2:
        return np.zeros((n, n))

    vecs = np.array([embeddings[i] for i in indices], dtype=np.float64)
    norms = np.linalg.norm(vecs, axis=1, keepdims=True)
    norms = np.where(norms == 0, 1, norms)
    normalized = vecs / norms
    sim_matrix = normalized @ normalized.T
    return (sim_matrix + 1.0) / 2.0


def cluster_pull_requests(payload: dict[str, Any]) -> dict[str, Any]:
    prs = payload.get("pullRequests") or payload.get("prs") or []
    if not isinstance(prs, list):
        prs = []

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

    indexed_prs = list(enumerate(prs))

    grouped: dict[str, list[tuple[int, dict[str, Any]]]] = {}
    for pr_index, pr in indexed_prs:
        key = _cluster_key(pr)
        grouped.setdefault(key, []).append((pr_index, pr))

    clusters: list[dict[str, Any]] = []
    for idx, (key, members) in enumerate(sorted(grouped.items()), start=1):
        positions = [pos for pos, _ in members]

        if embeddings is not None and len(members) >= 2:
            sim_matrix = _compute_similarity_matrix(embeddings, positions)
            n = len(members)
            similarities = []
            for left_idx in range(n):
                for right_idx in range(left_idx + 1, n):
                    similarities.append(round(float(sim_matrix[left_idx, right_idx]), 4))
        elif embeddings is not None and len(members) == 1:
            similarities = []
        else:
            member_dicts = [member for _, member in members]
            titles = [m.get("title", "") for m in member_dicts]
            bodies = [m.get("body", "") for m in member_dicts]
            files_list = [m.get("files_changed", []) for m in member_dicts]

            from pratc_ml.similarity import tokenize

            tokenized_titles = [tokenize(t) for t in titles]
            tokenized_bodies = [tokenize(b) for b in bodies]

            similarities = []
            for i in range(len(members)):
                for j in range(i + 1, len(members)):
                    left = member_dicts[i]
                    right = member_dicts[j]
                    score = _heuristic_similarity_precomputed(
                        left, right, tokenized_titles[i], tokenized_titles[j],
                        tokenized_bodies[i], tokenized_bodies[j], files_list[i], files_list[j]
                    )
                    similarities.append(score)

        average_similarity = (
            round(sum(similarities) / len(similarities), 4) if similarities else 1.0
        )

        health = "green"
        member_values = [member for _, member in members]

        if any(
            member.get("mergeable") == "conflicting" or member.get("ci_status") == "failure"
            for member in member_values
        ):
            health = "red"
        elif any(member.get("ci_status") in {"pending", "unknown"} for member in member_values):
            health = "yellow"

        clusters.append(
            {
                "cluster_id": f"{key}-{idx:02d}",
                "cluster_label": key.replace("_", " ").title(),
                "summary": f"{len(member_values)} pull requests in {key} lane",
                "pr_ids": [int(member.get("number", 0)) for member in member_values],
                "health_status": health,
                "average_similarity": average_similarity,
                "sample_titles": [str(member.get("title", "")) for member in member_values[:3]],
            }
        )

    return {
        "action": "cluster",
        "status": "ok",
        "repo": payload.get("repo"),
        "model": (
            config.minimax_embed_model
            if config.backend == "minimax" and config.minimax_api_key
            else config.voyage_model
            if config.backend == "voyage" and config.voyage_api_key
            else "heuristic-fallback"
        ),
        "clusters": clusters,
    }


def _heuristic_similarity_precomputed(
    left: dict, right: dict,
    left_title_tokens: tuple[str, ...], right_title_tokens: tuple[str, ...],
    left_body_tokens: tuple[str, ...], right_body_tokens: tuple[str, ...],
    left_files: list, right_files: list
) -> float:
    """Heuristic similarity with pre-computed tokenized title and body."""
    left_title_set = set(left_title_tokens)
    right_title_set = set(right_title_tokens)
    if not left_title_set and not right_title_set:
        title_score = 0.0
    else:
        union = left_title_set | right_title_set
        title_score = len(left_title_set & right_title_set) / len(union) if union else 0.0

    left_body_set = set(left_body_tokens)
    right_body_set = set(right_body_tokens)
    if not left_body_set and not right_body_set:
        body_score = 0.0
    else:
        union = left_body_set | right_body_set
        body_score = len(left_body_set & right_body_set) / len(union) if union else 0.0

    left_files_set = {f.strip().lower() for f in left_files if f and f.strip()}
    right_files_set = {f.strip().lower() for f in right_files if f and f.strip()}
    if not left_files_set and not right_files_set:
        files_score = 0.5
    else:
        union = left_files_set | right_files_set
        files_score = len(left_files_set & right_files_set) / len(union) if union else 0.0

    return round((0.6 * title_score) + (0.3 * files_score) + (0.1 * body_score), 4)
