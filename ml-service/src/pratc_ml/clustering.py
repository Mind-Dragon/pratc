from __future__ import annotations

from typing import Any

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
        similarities: list[float] = []
        for left_idx in range(len(members)):
            for right_idx in range(left_idx + 1, len(members)):
                left_position, left = members[left_idx]
                right_position, right = members[right_idx]
                if embeddings is not None:
                    left_embedding = embeddings[left_position]
                    right_embedding = embeddings[right_position]
                    score = (cosine_similarity(left_embedding, right_embedding) + 1.0) / 2.0
                else:
                    score = heuristic_similarity(left, right)
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
