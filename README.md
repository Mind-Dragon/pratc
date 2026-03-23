# prATC

prATC (PR Air Traffic Control) is a self-hostable system for large-scale pull request triage and merge planning.

## Features

- **CLI Analysis**: Analyze, cluster, graph, and plan merges for GitHub repositories
- **Web Dashboard**: ATC overview, triage inbox, dependency graphs, and merge planning
- **ML Clustering**: Python service for PR clustering and duplicate detection
- **SQLite Cache**: Incremental GitHub sync with persisted state
- **Rate-Limit Aware**: Built-in retry and budget management

## Quick Start

```bash
# Build everything
make verify-env
make build
make test

# Run with Docker Compose
docker-compose --profile local-ml up
docker-compose --profile minimax-light up
```

## CLI Commands

### analyze
Analyze pull requests for a repository.

```bash
pratc analyze --repo=owner/repo --format=json
```

Output includes: `repo`, `generatedAt`, `counts`, `clusters`, `duplicates`, `overlaps`, `conflicts`, `stalenessSignals`.

### cluster
Cluster pull requests by similarity.

```bash
pratc cluster --repo=owner/repo --format=json
```

Output includes: `repo`, `generatedAt`, `model`, `thresholds`, `clusters`.

### graph
Generate dependency/conflict graphs.

```bash
pratc graph --repo=owner/repo --format=dot    # Default DOT output
pratc graph --repo=owner/repo --format=json   # JSON output
```

### plan
Generate a ranked merge plan. **Dry-run by default** (no changes executed).

```bash
pratc plan --repo=owner/repo --target=20
pratc plan --repo=owner/repo --target=20 --mode=combination
pratc plan --repo=owner/repo --target=20 --include-bots
pratc plan --repo=owner/repo --target=20 --dry-run=false   # Execute mode
```

**Flags:**
- `--target`: Number of PRs to include (default: 20)
- `--mode`: Formula mode - `combination` (default), `permutation`, `with_replacement`
- `--dry-run`: Plan only, do not execute (default: `true`, always true if not explicitly set)
- `--include-bots`: Include bot PRs in merge plan (default: `false`)

### serve
Start the API server.

```bash
pratc serve --port=8080
pratc serve --port=8080 --repo=owner/repo   # Default repo for API
```

### sync
Sync repository metadata and refs.

```bash
pratc sync --repo=owner/repo
pratc sync --repo=owner/repo --watch --interval=5m
```

### audit
Query the audit log.

```bash
pratc audit --limit=20 --format=json
```

## API Routes

### Health
- `GET /healthz` - Health check
- `GET /api/health` - Health check (alias)

### Settings
- `GET /api/settings?repo=` - Get settings
- `POST /api/settings` - Set setting
- `DELETE /api/settings?scope=&repo=&key=` - Delete setting
- `GET /api/settings/export?scope=&repo=` - Export as YAML
- `POST /api/settings/import?scope=&repo=` - Import YAML

### Analysis
Legacy routes (query string repo):
- `GET /analyze?repo=` - Analyze PRs
- `GET /cluster?repo=` - Cluster PRs
- `GET /graph?repo=&format=` - Graph PRs (add `&format=dot` for DOT output)
- `GET /plan?repo=&target=&mode=` - Plan PRs

RESTful routes (`/api/repos/:owner/:repo/...`):
- `GET /api/repos/:owner/:repo/analyze`
- `GET /api/repos/:owner/:repo/cluster`
- `GET /api/repos/:owner/:repo/graph`
- `GET /api/repos/:owner/:repo/plan` or `/api/repos/:owner/:repo/plans`

### Sync
- `POST /api/repos/:owner/:repo/sync` - Trigger sync
- `GET /api/repos/:owner/:repo/sync/stream` - Stream sync progress

### Plan Query Parameters
When calling plan endpoints, these query parameters are supported:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `target` | int | 20 | Number of PRs to include |
| `mode` | string | `combination` | Formula mode: `combination`, `permutation`, `with_replacement` |
| `cluster_id` | string | - | Filter to specific cluster |
| `exclude_conflicts` | bool | false | Exclude conflicting PRs |
| `stale_score_threshold` | float | 0.0 | Staleness threshold (0-1) |
| `candidate_pool_cap` | int | 100 | Max candidate pool size (1-500) |
| `score_min` | float | 0.0 | Minimum PR score (0-100) |
| `dry_run` | bool | true | Plan only, do not execute |

## Web Dashboard

The web dashboard runs at `http://localhost:3000`.

### Routes

| Route | Description |
|-------|-------------|
| `/` | ATC overview dashboard |
| `/inbox` | PR inbox view |
| `/triage` | Sequential triage workflow |
| `/plan` | Merge plan panel |
| `/graph` | Interactive dependency graph |
| `/settings` | Configuration settings |

## Docker Compose Profiles

Two profiles are available for different deployment scenarios:

### local-ml
Full local ML stack with Python clustering service.

```bash
docker-compose --profile local-ml up
```

### minimax-light
Lightweight profile without local ML (uses external APIs).

```bash
docker-compose --profile minimax-light up
```

**Note:** The `minimax-light` profile replaces the previous `openrouter-light` naming.

## Repository Layout

```text
cmd/pratc/          # CLI entrypoints
internal/           # Go packages
  cmd/              # CLI command implementations
  app/              # Service layer
  cache/            # SQLite persistence
  github/           # GitHub client
  filter/           # Pre-filter pipeline
  formula/          # Combinatorial formula engine
  graph/            # Dependency graph engine
  planner/          # Merge planning
  settings/         # Settings management
  sync/             # Background sync
  types/            # Shared types
ml-service/         # Python ML service
web/                # TypeScript Next.js dashboard
fixtures/           # Test fixtures
```

## Configuration

Environment variables:

| Variable | Description |
|----------|-------------|
| `PRATC_PORT` | API server port (default: 8080) |
| `PRATC_DB_PATH` | SQLite database path |
| `PRATC_SETTINGS_DB` | Settings database path |
| `PRATC_ANALYSIS_BACKEND` | Analysis backend: `local` or `remote` |
| `GITHUB_TOKEN` | GitHub API token |

## Testing

```bash
# Go tests
go test -race -v ./...

# Python tests
uv run pytest -v

# Web tests
bun run test

# All tests
make test
```

## Exit Codes

- `0` - Success
- `1` - Runtime failure
- `2` - Invalid arguments

## Duplicate Classification

- Similarity > 0.90: duplicate
- Similarity 0.70-0.90: overlapping

## License

MIT
