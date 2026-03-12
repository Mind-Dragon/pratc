# prATC ML Service

Python machine‑learning service for PR clustering, duplicate detection, and overlap analysis.

It provides a simple JSON‑over‑STDIN/STDOUT CLI used by the Go server.

## Usage

```bash
python -m pratc_ml.cli < payload.json
```

The payload must contain an `action` field (`health`, `cluster`, `duplicates`, `overlap`).

## Development

- Install dependencies: `uv sync`
- Run tests: `uv run pytest -v`

## License

MIT
