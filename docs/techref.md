# prATC Technical Reference

> **Note:** This file was previously `docs/architecture.md`. It serves as the detailed technical reference for component breakdowns, API routes, data model, and SLOs. For the system design philosophy and layered decision architecture, see [ARCHITECTURE.md](../ARCHITECTURE.md). For operating rules and bucket definitions, see [GUIDELINE.md](../GUIDELINE.md).

**Last Updated:** 2026-03-23  
**System Version:** v0.1  
**Scope:** prATC (PR Air Traffic Control) — self-hostable, repo-agnostic system for large-scale PR triage and merge planning.

## Table of Contents

1. [System Overview](#system-overview)
2. [High-Level Architecture](#high-level-architecture)
3. [Component Breakdown](#component-breakdown)
4. [Data Flow](#data-flow)
5. [Technology Stack](#technology-stack)
6. [Cross-Cutting Concerns](#cross-cutting-concerns)
7. [Performance SLOs](#performance-slos)
8. [Environment Configuration](#environment-configuration)

---

## System Overview

prATC is a three-tier system for analyzing GitHub repositories at scale. It ingests PR metadata, applies ML clustering, builds dependency graphs, and generates optimized merge plans.

### Key Capabilities

- **Analyze**: Pull PR metadata, detect duplicates, overlaps, conflicts, staleness
- **Cluster**: Group similar PRs using ML-based semantic clustering
- **Graph**: Build dependency/conflict graphs with DOT visualization
- **Plan**: Generate ranked merge plans with combinatorial optimization
- **Sync**: Incremental GitHub sync with cursor persistence
- **Dashboard**: Web UI for interactive triage and visualization

---

## High-Level Architecture

```
                    ┌─────────────────────────────────────────────────────────────┐
                    │                        Client Layer                         │
                    │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
                    │  │ CLI (pratc)  │  │   Web UI     │  │  External Tools  │  │
                    │  │   Go binary  │  │  Next.js 15  │  │   (CI/CD, etc)   │  │
                    │  └──────┬───────┘  └──────┬───────┘  └────────┬─────────┘  │
                    └─────────┼─────────────────┼───────────────────┼───────────┘
                              │                 │                   │
                              │ HTTP/JSON       │ HTTP/JSON         │ HTTP/JSON
                              ▼                 ▼                   ▼
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                                     API Layer (Go)                                       │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐│
│  │                               internal/cmd (HTTP Server)                            ││
│  │  • CORS middleware (hardcoded localhost:3000)                                       ││
│  │  • Route handlers: /healthz, /api/*, /analyze, /cluster, /graph, /plan              ││
│  │  • Settings API: CRUD with YAML import/export                                       ││
│  │  • SSE streaming for sync progress                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────┘│
│                                          │                                               │
│                                          ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐│
│  │                               internal/app (Service)                                ││
│  │  • Service facade: Analyze, Cluster, Graph, Plan                                    ││
│  │  • Orchestrates filter, graph, planner, github packages                             ││
│  │  • Handles ML bridge communication                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────────────────┘
                                          │
        ┌─────────────────────────────────┼─────────────────────────────────┐
        │                                 │                                 │
        ▼                                 ▼                                 ▼
┌───────────────┐              ┌──────────────────┐              ┌──────────────────┐
│  Core Engine  │              │  Data Layer      │              │  External APIs   │
│  (Go)         │              │  (SQLite)        │              │                  │
│               │              │                  │              │                  │
│ • filter/     │              │ • pull_requests  │              │ • GitHub GraphQL │
│   Pipeline    │              │ • pr_files       │              │ • Rate limiting  │
│ • graph/      │◄────────────►│ • pr_reviews     │              │ • Retry logic    │
│   Dependency  │              │ • ci_status      │              │                  │
│ • planner/    │              │ • sync_jobs      │              │ • ML Service     │
│   Merge plans │              │ • sync_progress  │              │   (Python)       │
│ • formula/    │              │ • schema_migr.   │              │   stdin/stdout   │
│   Combinatorics│             │ • audit_log      │              │   JSON IPC       │
│ • planning/   │              └──────────────────┘              └──────────────────┘
│   Pool select │                       │
│ • analysis/   │                       │
│   Bot detect  │                       │
│ • sync/       │                       │
│   Background  │                       │
│ • repo/       │                       │
│   Git mirrors │                       │
└───────────────┘                       │
                                        │
                              ┌─────────▼──────────┐
                              │   ML Service       │
                              │   (Python 3.11+)   │
                              │                    │
                              │ • Clustering       │
                              │ • Duplicate detect │
                              │ • Overlap analysis │
                              │ • Semantic conf.   │
                              └────────────────────┘
```

---

## Component Breakdown

### CLI Layer (`cmd/pratc/`)

Entry point for all operations. Uses Cobra for command structure.

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `analyze` | Full PR analysis | `--repo`, `--format`, `--use-cache-first` |
| `cluster` | ML clustering only | `--repo`, `--format` |
| `graph` | Dependency graph | `--repo`, `--format` (dot/json) |
| `plan` | Merge planning | `--repo`, `--target`, `--mode`, `--dry-run` |
| `report` | Generate PDF report | `--repo`, `--input-dir`, `--output`, `--format` |
| `serve` | Start API server | `--port`, `--repo` |
| `sync` | GitHub sync | `--repo`, `--watch`, `--interval` |
| `audit` | Query audit log | `--limit`, `--format` |
| `mirror` | Git mirror mgmt | `list`, `info`, `prune`, `clean` |

### API Layer (`internal/cmd/`)

HTTP server with RESTful and legacy query-string routes.

**RESTful Routes:**
```
GET  /api/repos/{owner}/{repo}/analyze
GET  /api/repos/{owner}/{repo}/cluster
GET  /api/repos/{owner}/{repo}/graph
GET  /api/repos/{owner}/{repo}/plan
POST /api/repos/{owner}/{repo}/sync
GET  /api/repos/{owner}/{repo}/sync/stream  (SSE)
```

**Legacy Routes:**
```
GET /analyze?repo=owner/repo
GET /cluster?repo=owner/repo
GET /graph?repo=owner/repo&format=dot
GET /plan?repo=owner/repo&target=20
```

### Service Layer (`internal/app/`)

Central facade coordinating all operations.

```go
// Service interface (conceptual)
type Service interface {
    Analyze(ctx context.Context, repo string) (*AnalysisResponse, error)
    Cluster(ctx context.Context, repo string) (*ClusterResponse, error)
    Graph(ctx context.Context, repo string) (*GraphResponse, error)
    Plan(ctx context.Context, repo string, target int, mode formula.Mode) (*PlanResponse, error)
    Health() *HealthResponse
}
```

**Key thresholds (defined in service.go):**
- Duplicate threshold: 0.90 (similarity > 0.90 = duplicate)
- Overlap threshold: 0.70 (similarity 0.70-0.90 = overlapping)

### Core Engine Packages

#### `internal/filter/` - Pre-filter Pipeline

Sequential filtering before heavy computation:

1. **Draft Filter** — exclude draft PRs
2. **Conflict Filter** — detect file-level conflicts
3. **CI Filter** — check CI status gates
4. **Bot Filter** — exclude bot-authored PRs (optional)
5. **Scorer** — assign priority scores

#### `internal/graph/` - Dependency Graph

Builds conflict and dependency graphs between PRs.

```go
// Key methods
func (g *Graph) Build(prs []types.PR) error
func (g *Graph) TopologicalOrder() ([]int, error)
func (g *Graph) DOT() string  // Graphviz format
func (g *Graph) BuildIncremental(prs []types.PR, fingerprints map[string]string) error
```

**Fingerprint-based incremental updates:** First call uses `Build()`, subsequent calls use `BuildIncremental()` with prior fingerprint map.

#### `internal/planner/` - Merge Planning

Core planner with functional options pattern.

```go
p := planner.New(prs,
    planner.WithNow(customTime),
    planner.WithIncludeBots(true),
    planner.WithMode(formula.ModeCombination),
)
plan, err := p.Generate(target)
```

**Modes:**
- `combination` — C(n,k), select k from n without order
- `permutation` — P(n,k), select k from n with order
- `with_replacement` — n^k, select with replacement

#### `internal/formula/` - Combinatorial Engine

Handles large combinatorial calculations using `math/big` (P(5000,20) overflows uint64).

```go
func Count(n, k int64, mode Mode) *big.Int
func GenerateByIndex(n, k int64, mode Mode, idx *big.Int) ([]int64, error)
```

#### `internal/github/` - GitHub Client

GraphQL client with rate limiting and retry logic.

- **Rate limit reserve:** 200 requests/hour minimum
- **Secondary rate limit (403):** Exponential backoff 2s→60s, max 8 retries
- **5xx errors:** Exponential backoff 1s→30s, max 6 retries
- **Cursor persistence:** Resumes from last cursor on rate limit exhaustion

#### `internal/ml/` - ML Bridge

JSON over stdin/stdout via `exec.CommandContext`.

**Actions:** `health`, `cluster`, `duplicates`, `overlap`

Default timeout: 30 seconds. Large repos may exceed this; caller handles with partial results.

#### `internal/sync/` - Background Sync

Incremental GitHub synchronization with cursor persistence.

```go
type SyncJob struct {
    ID        string
    Repo      string
    Status    string  // pending, running, completed, failed
    Cursor    string  // GitHub GraphQL cursor (opaque)
    ErrorMsg  string
}
```

### Data Layer (`internal/cache/`)

SQLite with forward-only migrations.

**Schema version:** 2

**Pragmas applied on open:**
```sql
PRAGMA journal_mode=WAL;        -- Write-ahead logging
PRAGMA busy_timeout=5000;       -- 5s timeout for locked DB
PRAGMA foreign_keys=ON;         -- Enforce FK constraints
```

**Tables:** `pull_requests`, `pr_files`, `pr_reviews`, `ci_status`, `sync_jobs`, `sync_progress`, `merged_pr_index`, `schema_migrations`, `audit_log`

### ML Service (`ml-service/`)

Python 3.11+ service for ML operations.

**Responsibilities:**
- Semantic clustering of PRs
- Duplicate detection (cosine similarity)
- Overlap analysis
- Semantic conflict detection

**Communication:** JSON over stdin/stdout from Go bridge.

### Web Dashboard (`web/`)

Next.js 15 + React 19 + bun.

**Architecture:**
- Pages use `getServerSideProps` (no SWR, no React Query)
- Single global CSS (`styles/globals.css`)
- No Tailwind, no CSS modules, no styled-components
- Dynamic imports with `ssr: false` for force graph

**Routes:**
- `/` — Dashboard overview
- `/inbox` — PR inbox (alias to triage)
- `/triage` — Sequential PR review
- `/plan` — Merge plan panel
- `/graph` — Interactive dependency graph
- `/settings` — Configuration (9 sections)

---

## Data Flow

### 1. Analyze Flow

```
CLI: analyze --repo=owner/repo
        │
        ▼
┌───────────────┐
│  cache.List   │ ─── Check for cached PRs
└───────┬───────┘
        │ cache miss
        ▼
┌───────────────┐
│ github.Fetch  │ ─── GraphQL query with pagination
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ cache.Upsert  │ ─── Store PRs, files, reviews
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ filter.Run    │ ─── Apply pipeline filters
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ ml.Cluster    │ ─── Python ML service
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ planner.Dups  │ ─── Duplicate detection
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ graph.Build   │ ─── Dependency/conflict graph
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ Output JSON   │ ─── AnalysisResponse
└───────────────┘
```

### 2. Plan Flow

```
CLI: plan --repo=owner/repo --target=20
        │
        ▼
┌───────────────┐
│ filter.Run    │ ─── Pre-filter pipeline
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ planner.New   │ ─── Create planner with options
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ formula.Count │ ─── Calculate combination count
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ planner.Gen   │ ─── Generate candidates
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ graph.Order   │ ─── Topological sort
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ Output JSON   │ ─── PlanResponse
└───────────────┘
```

### 3. Sync Flow

```
CLI: sync --repo=owner/repo --watch
        │
        ▼
┌───────────────┐
│ sync.Create   │ ─── Create sync job
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ cache.Resume  │ ─── Check for existing cursor
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ github.Fetch  │ ─── Paginated GraphQL
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ cache.Update  │ ─── Upsert PRs, update cursor
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ sync.Complete │ ─── Mark job complete
└───────────────┘
        │
        ▼ (watch mode)
   [sleep interval]
        │
        └──────► Repeat
```

---

## Technology Stack

| Layer | Technology | Version | Notes |
|-------|-----------|---------|-------|
| CLI/API | Go | 1.23+ | Pure Go, no CGO |
| SQLite Driver | modernc.org/sqlite | Latest | Pure Go SQLite |
| HTTP Router | stdlib | - | `net/http` + ServeMux |
| CLI Framework | Cobra | Latest | Command structure |
| ML Service | Python | 3.11+ | uv package manager |
| ML Framework | scikit-learn | Latest | Clustering algorithms |
| Web Dashboard | Next.js | 15 | React 19 |
| Web Runtime | bun | Latest | Package manager + runtime |
| Testing (Go) | stdlib | - | `testing` package only |
| Testing (Web) | vitest | Latest | + @testing-library/react |

---

## Cross-Cutting Concerns

### Type Consistency

All three languages share identical type definitions with `snake_case` JSON keys.

| Language | File | Key Type |
|----------|------|----------|
| Go | `internal/types/models.go` | snake_case |
| Python | `ml-service/src/pratc_ml/models.py` | snake_case |
| TypeScript | `web/src/types/api.ts` | snake_case |

### API Contracts

**Success Response:**
```json
{
  "repo": "owner/repo",
  "generatedAt": "2026-03-23T10:00:00Z",
  "...": "operation-specific payload"
}
```

**Error Response:**
```json
{
  "error": "error_code",
  "message": "Human readable description",
  "status": "error"
}
```

### Configuration Flow

```
Environment Variables
        │
        ▼
┌───────────────┐
│ Go root.go    │ ─── Parse env vars
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ Go service.go │ ─── Apply to service config
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ Python ML     │ ─── Via stdin JSON
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ React (web)   │ ─── NEXT_PUBLIC_* vars
└───────────────┘
```

### Sorting Convention

Every sort must use PR number as final tiebreaker for deterministic output:

```go
sort.SliceStable(prs, func(i, j int) bool {
    if prs[i].Score != prs[j].Score {
        return prs[i].Score > prs[j].Score
    }
    return prs[i].Number < prs[j].Number // tiebreaker
})
```

### Error Wrapping

Component prefix style (not verb prefix):

```go
// Standard Go:
fmt.Errorf("failed to fetch: %w", err)

// This codebase:
fmt.Errorf("github client: %w", err)
```

---

## Performance SLOs

For 5,500 PR scale:

| Operation | Cold | Warm | Notes |
|-----------|------|------|-------|
| Sync | ≤20min | ≤3min | Incremental with cursor |
| Analyze | ≤300s | - | Includes ML clustering |
| Cluster | ≤180s | - | ML service call |
| Graph | ≤120s | - | Build + DOT generation |
| Plan | - | ≤90s | Pre-filtered pool |
| API /analyze | - | p95 ≤5s | With cache |
| API /cluster | - | p95 ≤3s | With cache |
| API /graph | - | p95 ≤2s | With cache |
| API /plan | - | p95 ≤2s | With cache |
| Memory (CLI analyze) | - | ≤2.5 GB | RSS ceiling |

---

## Environment Configuration

### Required Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `GITHUB_TOKEN` | - | GitHub API authentication |
| `PRATC_PORT` | 8080 | API server port |
| `PRATC_DB_PATH` | ~/.pratc/pratc.db | Main cache database |
| `PRATC_SETTINGS_DB` | ./pratc-settings.db | Settings database |

### Optional Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PRATC_ANALYSIS_BACKEND` | local | Analysis backend: `local` or `remote` |
| `NEXT_PUBLIC_PRATC_API_URL` | http://localhost:8080 | Web dashboard API URL |

### Secret Management

Never store secrets in code or config files. Use `psst`:

```bash
psst GITHUB_TOKEN -- pratc analyze --repo=owner/repo
```

---

## Code Smells (Known)

1. **`app/` duplicates `planner/` (~70% overlap)**
   - Both implement: `jaccard()`, `tokenize()`, `buildClusters()`
   - Rule: New clustering logic goes in `planner/`

2. **`planning/` is wired as of v1.4 Phase 0**
   - `HierarchicalPlanner`, `PoolSelector`, `PairwiseExecutor`, `TimeDecayWindow` are integrated into `app/service.go`
   - See ROADMAP.md Phase 0 for details

3. **`filter/scorer.go` bubble sort — FIXED**
   - `rankByConflictScore()` was O(n²) — FIXED, now uses `sort.Slice`

---

## Scope Guardrails

### Must Have (v0.1)
- Rate-limit-aware GitHub client
- Pre-filter pipeline
- Dry-run default for all plan operations
- Audit logging

### Must Not Have (v0.1)
- GitHub App/OAuth/webhooks
- ML feedback loops
- Multi-repo UI
- gRPC
- Auto PR actions
- Nx/Turborepo

---

## Testing Commands

```bash
# All tests
make test

# Go only
make test-go

# Python only
make test-python

# Web only
make test-web

# Build verification
make build
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime failure |
| 2 | Invalid arguments |

---

## File Locations for AI Agents

| Purpose | Path |
|---------|------|
| CLI commands | `cmd/pratc/*.go` |
| API routes | `internal/cmd/root.go` |
| Service facade | `internal/app/service.go` |
| Type definitions | `internal/types/models.go` |
| Pre-filter | `internal/filter/pipeline.go` |
| Combinatorial | `internal/formula/modes.go` |
| Pool selection | `internal/planning/pool.go` |
| Graph builder | `internal/graph/graph.go` |
| SQLite store | `internal/cache/sqlite.go` |
| ML bridge | `internal/ml/bridge.go` |
| Settings | `internal/settings/store.go` |
| GitHub client | `internal/github/client.go` |
| Fixtures | `internal/testutil/fixture_loader.go` |
| Web types | `web/src/types/api.ts` |
| Web API client | `web/src/lib/api.ts` |
| Web pages | `web/src/pages/*.tsx` |
