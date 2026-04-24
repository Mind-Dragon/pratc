# AGENTS.md — internal/workqueue

**Package**: Action work queue with SQLite storage  
**Driver**: `modernc.org/sqlite` (pure Go, no CGO)

## Schema Version
Current: **1** (no migrations yet)

## Tables

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| `action_work_items` | Action work items | `id` PK, `repo`, `lane`, `state`, `lease_*` |
| `action_state_transitions` | State change audit trail | `id` PK, `work_item_id`, `from_state`, `to_state` |
| `action_proof_bundles` | Proof bundles for actions | `id` PK, `work_item_id`, `pr_number` |

## Key Methods (Queue)

```go
OpenSQLite(path string, opts ...Option) (*Queue, error)  // Opens queue DB, creates schema
Upsert(ctx context.Context, repo string, item types.ActionWorkItem) error
Claim(ctx context.Context, lane types.ActionLane) (types.ActionWorkItem, bool, error)
GetByPR(ctx context.Context, repo string, prNumber int) (types.ActionWorkItem, bool, error)
ListByRepo(ctx context.Context, repo string, lane types.ActionLane, state types.ActionWorkItemState) ([]types.ActionWorkItem, error)
UpdateState(ctx context.Context, id string, newState types.ActionWorkItemState, actor string, reason string) error
GetSummary(ctx context.Context, repo string) (map[types.ActionLane]map[types.ActionWorkItemState]int, error)
```

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `PRATC_QUEUE_DB_PATH` | `~/.pratc/queue.db` | Action work queue database |

## Pragmas Applied on Open

```sql
PRAGMA journal_mode=WAL;        -- Write-ahead logging
PRAGMA busy_timeout=5000;       -- 5s timeout for locked DB
```

## Action Table Schema

### action_work_items
- `id`: UUID string (primary key)
- `repo`, `pr_number`: PR reference
- `lane`: Action lane (review, approve, apply_fix, etc.)
- `state`: Work item state (proposed, claimed, in_progress, completed, failed, blocked)
- `lease_*`: Lease tracking for concurrent execution
- `payload_json`: Full ActionWorkItem as JSON (for flexibility)

### action_state_transitions
- Audit trail of state changes for debugging and compliance

### action_proof_bundles
- Stores proof bundles (test results, evidence) for completed actions

## Gotchas

- **Separate database from cache**: Queue DB is distinct from PR cache DB
- **WAL mode**: Journal files appear alongside DB file
- **Single connection**: `SetMaxOpenConns(1)` to avoid concurrent write issues
- **No migrations yet**: Schema is forward-only, no version tracking

## v2.0 Readiness

- ✓ Action work items table defined
- ✓ State transitions audit trail
- ✓ Proof bundle storage
- ✓ Lease mechanism for concurrent execution
- ⚠️ No migration system (schema v1)
- ⚠️ No down-migrations (forward-only only)
