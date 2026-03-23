from __future__ import annotations

import os
from dataclasses import dataclass
from typing import Literal, cast


class BackendConfigError(RuntimeError):
    """Raised when backend configuration is invalid or missing required values."""

    pass


Backend = Literal["local", "minimax", "voyage"]


@dataclass(frozen=True)
class ProviderConfig:
    backend: Backend = "local"
    minimax_api_key: str | None = None
    minimax_embed_model: str = "abab6.5t"
    minimax_reason_model: str = "abab6.5t"
    voyage_api_key: str | None = None
    voyage_model: str = "voyage-code-3"
    voyage_base_url: str = "https://api.voyageai.com/v1"

    @classmethod
    def from_env(cls) -> "ProviderConfig":
        backend = os.getenv("ML_BACKEND", "local").strip().lower() or "local"
        if backend not in {"local", "minimax", "voyage"}:
            backend = "local"

        return cls(
            backend=cast(Backend, backend),
            minimax_api_key=os.getenv("MINIMAX_API_KEY"),
            minimax_embed_model=os.getenv(
                "MINIMAX_EMBED_MODEL",
                "abab6.5t",
            ),
            minimax_reason_model=os.getenv(
                "MINIMAX_REASON_MODEL",
                "abab6.5t",
            ),
            voyage_api_key=os.getenv("VOYAGE_API_KEY"),
            voyage_model=os.getenv("VOYAGE_MODEL", "voyage-code-3"),
            voyage_base_url=os.getenv("VOYAGE_BASE_URL", "https://api.voyageai.com/v1"),
        )

    def validate(self) -> None:
        """Validate backend configuration and raise BackendConfigError if required values are missing.

        For minimax backend: MINIMAX_API_KEY must be set
        For voyage backend: VOYAGE_API_KEY must be set
        For local backend: no validation required
        """
        if self.backend == "minimax" and not self.minimax_api_key:
            raise BackendConfigError(
                "MINIMAX_API_KEY environment variable is required when ML_BACKEND=minimax. "
                "Set MINIMAX_API_KEY to your Minimax API key."
            )
        if self.backend == "voyage" and not self.voyage_api_key:
            raise BackendConfigError(
                "VOYAGE_API_KEY environment variable is required when ML_BACKEND=voyage. "
                "Set VOYAGE_API_KEY to your Voyage AI API key."
            )
        # local backend requires no validation
