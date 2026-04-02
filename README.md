# prATC

prATC (PR Air Traffic Control) is a self-hostable system for large-scale pull request triage and merge planning.

## Features

- **CLI Analysis**: Analyze, cluster, graph, and plan merges for GitHub repositories
- **Web Dashboard**: ATC overview, triage inbox, dependency graphs, and merge planning
- **ML Clustering**: Python service for PR clustering and duplicate detection
- **SQLite Cache**: Incremental GitHub sync with persisted state
- **Omni Batch Planning**: Select PRs by ID ranges and boolean expressions via selector syntax
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
- `GET /api/repos/:owner/:repo/plan/omni` - Omni batch plan with selector

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

## Omni Batch Planning

Omni mode lets you select specific PRs by ID using a selector expression. This is useful when you already know which PRs you want to plan against, bypassing the normal scoring and filtering pipeline.

### API Endpoint

```
GET /api/repos/:owner/:repo/plan/omni
```

### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `selector` | string | *(required)* | PR selector expression (see syntax below) |
| `target` | int | 20 | Number of PRs to select from the first stage |
| `stage_size` | int | 64 | Maximum PRs per processing stage |

### Selector Syntax

Selectors use a simple expression language to reference PRs by number.

**Grammar:**

```
expr       → orExpr
orExpr     → andExpr (OR andExpr)*
andExpr    → term (AND term)*
term       → '(' expr ')' | atomic
atomic     → ID | RANGE
```

- **ID**: A single PR number (e.g., `123`)
- **RANGE**: Two IDs separated by a hyphen, inclusive (e.g., `100-200`)
- **AND**: Intersection of two sets (binds tighter than OR)
- **OR**: Union of two sets (loosest precedence)
- **Parentheses**: Override default precedence

### Selector Examples

```bash
# Single PR
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=42"

# Range of PRs (inclusive)
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=1-100"

# Multiple ranges combined with AND (intersection)
# Selects PRs present in both ranges: {100, 101, ..., 150}
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=50-150+AND+100-200"
# Note: URL-encode spaces as + or %20

# Multiple ranges combined with OR (union)
# Selects PRs from either range
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=1-50+OR+200-250"

# Grouping with parentheses
# Selects intersection of (1-100 OR 300-400) with 50-150
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=(1-100+OR+300-400)+AND+50-150"
# Result: {50, 51, ..., 100}

# With custom stage size and target
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=1-200&target=10&stage_size=32"
```

### Response Format

```json
{
  "repo": "owner/repo",
  "generatedAt": "2026-04-02T12:00:00Z",
  "selector": "1-100",
  "mode": "omni_batch",
  "stageCount": 2,
  "stages": [
    { "stage": 1, "stageSize": 64, "matched": 64, "selected": 20 },
    { "stage": 2, "stageSize": 64, "matched": 37, "selected": 0 }
  ],
  "selected": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20],
  "ordering": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20]
}
```

- **stages**: The matched PRs are divided into batches of `stage_size`. Only the first stage populates the `selected` list, up to `target` PRs.
- **selected**: PR IDs chosen from the first stage (limited by `target`).
- **ordering**: Merge ordering for the selected PRs.

### Integration with Other Commands

Omni mode works alongside the standard analyze and config workflow:

```bash
# 1. Analyze the repo to understand the PR landscape
pratc analyze --repo=owner/repo --format=json

# 2. Configure planning settings
pratc config set --scope=repo --repo=owner/repo planning.target 20

# 3. Use omni mode for targeted planning on a specific PR range
curl "http://localhost:8080/api/repos/owner/repo/plan/omni?selector=50-150"
```

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
