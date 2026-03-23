# AGENTS.md — ML Service

**Scope:** Python CLI for PR clustering/duplicates/overlap via JSON stdin/stdout.

## Provider System

`ML_BACKEND` env var controls embedding source:
- `local` (default) — heuristic similarity only, no API calls
- `minimax` — requires `MINIMAX_API_KEY`, uses `api.minimaxi.chat/v1`
- `voyage` — requires `VOYAGE_API_KEY`, uses `voyageai.com/v1`

`ProviderConfig.from_env()` → `config.validate()` raises `BackendConfigError` on missing keys.

## Error Hierarchy

- `BackendConfigError(RuntimeError)` — config validation failures
- `MinimaxError(RuntimeError)` — Minimax API errors (5xx, auth, rate limit)
- `VoyageError(RuntimeError)` — Voyage API errors

All use `from exc` chaining. CLI catches `BackendConfigError` and returns structured error response.

## CLI Contract (`cli.py`)

Reads JSON from stdin, dispatches by `action`:
- `health` → `{"status": "ok"}`
- `cluster` → grouped PR clusters with centroid PRs
- `duplicates` → pairwise similarity matrix
- `overlap` → file overlap conflicts

Exit: `0` on success, `1` on error. Errors include `"error"` key in response.

## Similarity Functions (`similarity.py`)

- `cosine_similarity(left, right)` — clamped to `[-1, 1]`, numpy vectors
- `jaccard(left, right)` — tokenized word set intersection/union
- `heuristic_similarity(left, right)` — weighted: `0.6*title + 0.3*files + 0.1*body`

Thresholds: `>0.90` = duplicate, `0.70-0.90` = overlapping.

## Embedding Text Format

Shared by `clustering.py` and `duplicates.py`:

```
"{title}\n{body}\n{first 5 files}"
```

File paths truncated to first 5 to limit token usage.

## Fallback Chain

1. Try API embeddings (minimax/voyage)
2. On any API error, fallback to `heuristic_similarity`
3. Local backend skips step 1 entirely

## Models (`models.py`)

Pydantic `BaseModel` with bootstrap fallback: if Pydantic unavailable, uses `@dataclass` wrapper with JSON serde. All fields use `snake_case` keys matching Go/TypeScript contracts.

## Test Conventions

- `@pytest.mark.unit` on all test functions
- `monkeypatch.setenv()` / `monkeypatch.delenv()` for env manipulation
- No fixtures in `conftest.py` (minimal setup only)
- Run: `uv run pytest -v`

## Gotchas

- **Never** print to stdout (breaks JSON protocol). Use `sys.stderr` for logs.
- **Always** flush stdout after JSON dump (buffering issues in pipe).
- Embedding APIs have no built-in retry; Go side handles retries.
- `heuristic_similarity` weights must match Go `internal/app/service.go` exactly.
- Thresholds are soft; Go backend may apply additional filtering.
