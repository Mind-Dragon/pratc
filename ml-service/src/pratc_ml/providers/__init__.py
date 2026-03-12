from __future__ import annotations

import os
from dataclasses import dataclass
from typing import cast, Literal


Backend = Literal["local", "openrouter"]


@dataclass(frozen=True)
class ProviderConfig:
    backend: Backend = "local"
    openrouter_api_key: str | None = None
    openrouter_embed_model: str = "openai/text-embedding-3-large"
    openrouter_reason_model: str = "openai/gpt-5.4"

    @classmethod
    def from_env(cls) -> "ProviderConfig":
        backend = os.getenv("ML_BACKEND", "local").strip().lower() or "local"
        if backend not in {"local", "openrouter"}:
            backend = "local"

        return cls(
            backend=cast(Backend, backend),
            openrouter_api_key=os.getenv("OPENROUTER_API_KEY"),
            openrouter_embed_model=os.getenv(
                "OPENROUTER_EMBED_MODEL",
                "openai/text-embedding-3-large",
            ),
            openrouter_reason_model=os.getenv(
                "OPENROUTER_REASON_MODEL",
                "openai/gpt-5.4",
            ),
        )
