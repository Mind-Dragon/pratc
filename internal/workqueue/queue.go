package workqueue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jeffersonnunn/pratc/internal/types"
	_ "modernc.org/sqlite"
)

type Queue struct {
	db  *sql.DB
	now func() time.Time
	mu  sync.Mutex
}

type Option func(*Queue)

func WithNow(now func() time.Time) Option {
	return func(q *Queue) {
		if now != nil {
			q.now = now
		}
	}
}

func OpenSQLite(path string, opts ...Option) (*Queue, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create queue db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open queue sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	q := &Queue{db: db, now: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(q)
	}
	if err := q.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return q, nil
}

func (q *Queue) Close() error {
	if q == nil || q.db == nil {
		return nil
	}
	return q.db.Close()
}

func (q *Queue) SetNow(now func() time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if now != nil {
		q.now = now
	}
}

func (q *Queue) init(ctx context.Context) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA busy_timeout=5000;`,
		`CREATE TABLE IF NOT EXISTS action_work_items (
			id TEXT PRIMARY KEY,
			repo TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			lane TEXT NOT NULL,
			state TEXT NOT NULL,
			priority_score REAL NOT NULL,
			confidence REAL NOT NULL,
			lease_claimed_by TEXT NOT NULL DEFAULT '',
			lease_claimed_at TEXT NOT NULL DEFAULT '',
			lease_expires_at TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_action_work_items_repo_lane_state ON action_work_items(repo, lane, state);`,
		`CREATE INDEX IF NOT EXISTS idx_action_work_items_lease_expires ON action_work_items(lease_expires_at);`,
		`CREATE TABLE IF NOT EXISTS action_state_transitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			work_item_id TEXT NOT NULL,
			from_state TEXT NOT NULL,
			to_state TEXT NOT NULL,
			actor TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := q.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("initialize workqueue schema: %w", err)
		}
	}
	return nil
}

func (q *Queue) Upsert(ctx context.Context, repo string, item types.ActionWorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if item.ID == "" {
		return fmt.Errorf("work item id is required")
	}
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	if item.State == "" {
		item.State = types.ActionWorkItemStateProposed
	}
	now := q.now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal work item: %w", err)
	}
	_, err = q.db.ExecContext(ctx, `
		INSERT INTO action_work_items (
			id, repo, pr_number, lane, state, priority_score, confidence,
			lease_claimed_by, lease_claimed_at, lease_expires_at, payload_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			repo = excluded.repo,
			pr_number = excluded.pr_number,
			lane = excluded.lane,
			state = excluded.state,
			priority_score = excluded.priority_score,
			confidence = excluded.confidence,
			lease_claimed_by = excluded.lease_claimed_by,
			lease_claimed_at = excluded.lease_claimed_at,
			lease_expires_at = excluded.lease_expires_at,
			payload_json = excluded.payload_json,
			updated_at = excluded.updated_at
	`, item.ID, repo, item.PRNumber, string(item.Lane), string(item.State), item.PriorityScore, item.Confidence,
		item.LeaseState.ClaimedBy, item.LeaseState.ClaimedAt, item.LeaseState.ExpiresAt, string(payload), now, now)
	if err != nil {
		return fmt.Errorf("upsert work item %s: %w", item.ID, err)
	}
	return nil
}

func (q *Queue) EnqueueActionPlan(ctx context.Context, plan types.ActionPlan) error {
	if plan.Repo == "" {
		return fmt.Errorf("action plan repo is required")
	}
	for _, item := range plan.WorkItems {
		item.State = types.ActionWorkItemStateClaimable
		item.LeaseState = types.ActionLease{}
		if err := q.Upsert(ctx, plan.Repo, item); err != nil {
			return err
		}
		if err := q.appendTransition(ctx, item.ID, types.ActionWorkItemStateProposed, types.ActionWorkItemStateClaimable, "system", "enqueue_action_plan"); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) Get(ctx context.Context, id string) (types.ActionWorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.getUnlocked(ctx, id)
}

func (q *Queue) Claim(ctx context.Context, id, workerID string, ttl time.Duration) (types.ActionWorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if workerID == "" {
		return types.ActionWorkItem{}, fmt.Errorf("worker id is required")
	}
	if ttl <= 0 {
		return types.ActionWorkItem{}, fmt.Errorf("lease ttl must be positive")
	}
	item, err := q.getUnlocked(ctx, id)
	if err != nil {
		return types.ActionWorkItem{}, err
	}
	now := q.now().UTC()
	expired := leaseExpired(item.LeaseState.ExpiresAt, now)
	sameWorker := item.State == types.ActionWorkItemStateClaimed && item.LeaseState.ClaimedBy == workerID
	if item.State != types.ActionWorkItemStateClaimable && !sameWorker && !expired {
		return types.ActionWorkItem{}, fmt.Errorf("work item %s is not claimable", id)
	}
	if item.State == types.ActionWorkItemStateClaimed && !sameWorker && expired {
		if err := q.appendTransitionUnlocked(ctx, id, item.State, types.ActionWorkItemStateClaimable, "system", "lease_expired_before_claim"); err != nil {
			return types.ActionWorkItem{}, err
		}
	}
	from := item.State
	item.State = types.ActionWorkItemStateClaimed
	item.LeaseState = types.ActionLease{
		ClaimedBy: workerID,
		ClaimedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(ttl).Format(time.RFC3339Nano),
	}
	if err := q.saveUnlocked(ctx, item); err != nil {
		return types.ActionWorkItem{}, err
	}
	if !sameWorker {
		if err := q.appendTransitionUnlocked(ctx, id, from, types.ActionWorkItemStateClaimed, workerID, "claim"); err != nil {
			return types.ActionWorkItem{}, err
		}
	}
	return item, nil
}

func (q *Queue) Release(ctx context.Context, id, workerID string) (types.ActionWorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, err := q.getUnlocked(ctx, id)
	if err != nil {
		return types.ActionWorkItem{}, err
	}
	if item.State != types.ActionWorkItemStateClaimed || item.LeaseState.ClaimedBy != workerID {
		return types.ActionWorkItem{}, fmt.Errorf("work item %s is not claimed by %s", id, workerID)
	}
	from := item.State
	item.State = types.ActionWorkItemStateClaimable
	item.LeaseState = types.ActionLease{}
	if err := q.saveUnlocked(ctx, item); err != nil {
		return types.ActionWorkItem{}, err
	}
	if err := q.appendTransitionUnlocked(ctx, id, from, item.State, workerID, "release"); err != nil {
		return types.ActionWorkItem{}, err
	}
	return item, nil
}

func (q *Queue) Heartbeat(ctx context.Context, id, workerID string, ttl time.Duration) (types.ActionWorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if ttl <= 0 {
		return types.ActionWorkItem{}, fmt.Errorf("lease ttl must be positive")
	}
	item, err := q.getUnlocked(ctx, id)
	if err != nil {
		return types.ActionWorkItem{}, err
	}
	if item.State != types.ActionWorkItemStateClaimed || item.LeaseState.ClaimedBy != workerID {
		return types.ActionWorkItem{}, fmt.Errorf("work item %s is not claimed by %s", id, workerID)
	}
	item.LeaseState.ExpiresAt = q.now().UTC().Add(ttl).Format(time.RFC3339Nano)
	if err := q.saveUnlocked(ctx, item); err != nil {
		return types.ActionWorkItem{}, err
	}
	return item, nil
}

func (q *Queue) ExpireLeases(ctx context.Context) ([]string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	rows, err := q.db.QueryContext(ctx, `SELECT id, payload_json FROM action_work_items WHERE state = ?`, string(types.ActionWorkItemStateClaimed))
	if err != nil {
		return nil, fmt.Errorf("query claimed work items: %w", err)
	}
	defer rows.Close()
	now := q.now().UTC()
	expired := []types.ActionWorkItem{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("scan claimed work item: %w", err)
		}
		var item types.ActionWorkItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, fmt.Errorf("decode claimed work item %s: %w", id, err)
		}
		if leaseExpired(item.LeaseState.ExpiresAt, now) {
			expired = append(expired, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(expired))
	for _, item := range expired {
		from := item.State
		item.State = types.ActionWorkItemStateClaimable
		item.LeaseState = types.ActionLease{}
		if err := q.saveUnlocked(ctx, item); err != nil {
			return nil, err
		}
		if err := q.appendTransitionUnlocked(ctx, item.ID, from, item.State, "system", "lease_expired"); err != nil {
			return nil, err
		}
		ids = append(ids, item.ID)
	}
	return ids, nil
}

func (q *Queue) GetClaimable(ctx context.Context, repo string, lane types.ActionLane, limit int) ([]types.ActionWorkItem, error) {
	if _, err := q.ExpireLeases(ctx); err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	query := `SELECT payload_json FROM action_work_items WHERE repo = ? AND state = ?`
	args := []any{repo, string(types.ActionWorkItemStateClaimable)}
	if lane != "" {
		query += ` AND lane = ?`
		args = append(args, string(lane))
	}
	query += ` ORDER BY priority_score DESC, pr_number ASC LIMIT ?`
	args = append(args, limit)
	rows, err := q.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query claimable work items: %w", err)
	}
	defer rows.Close()
	items := []types.ActionWorkItem{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item types.ActionWorkItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *Queue) Transition(ctx context.Context, id, actor string, from, to types.ActionWorkItemState, reason string) (types.ActionWorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	item, err := q.getUnlocked(ctx, id)
	if err != nil {
		return types.ActionWorkItem{}, err
	}
	if item.State != from {
		return types.ActionWorkItem{}, fmt.Errorf("work item %s state = %s, want %s", id, item.State, from)
	}
	if !validTransition(from, to) {
		return types.ActionWorkItem{}, fmt.Errorf("invalid work item transition %s -> %s", from, to)
	}
	item.State = to
	if to != types.ActionWorkItemStateClaimed {
		item.LeaseState = types.ActionLease{}
	}
	if err := q.saveUnlocked(ctx, item); err != nil {
		return types.ActionWorkItem{}, err
	}
	if err := q.appendTransitionUnlocked(ctx, id, from, to, actor, reason); err != nil {
		return types.ActionWorkItem{}, err
	}
	return item, nil
}

func (q *Queue) getUnlocked(ctx context.Context, id string) (types.ActionWorkItem, error) {
	var raw string
	err := q.db.QueryRowContext(ctx, `SELECT payload_json FROM action_work_items WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return types.ActionWorkItem{}, fmt.Errorf("work item %s not found", id)
	}
	if err != nil {
		return types.ActionWorkItem{}, fmt.Errorf("get work item %s: %w", id, err)
	}
	var item types.ActionWorkItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return types.ActionWorkItem{}, fmt.Errorf("decode work item %s: %w", id, err)
	}
	return item, nil
}

func (q *Queue) saveUnlocked(ctx context.Context, item types.ActionWorkItem) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal work item %s: %w", item.ID, err)
	}
	now := q.now().UTC().Format(time.RFC3339Nano)
	_, err = q.db.ExecContext(ctx, `
		UPDATE action_work_items
		SET state = ?, lane = ?, priority_score = ?, confidence = ?, lease_claimed_by = ?, lease_claimed_at = ?, lease_expires_at = ?, payload_json = ?, updated_at = ?
		WHERE id = ?
	`, string(item.State), string(item.Lane), item.PriorityScore, item.Confidence,
		item.LeaseState.ClaimedBy, item.LeaseState.ClaimedAt, item.LeaseState.ExpiresAt, string(payload), now, item.ID)
	if err != nil {
		return fmt.Errorf("save work item %s: %w", item.ID, err)
	}
	return nil
}

func (q *Queue) appendTransition(ctx context.Context, id string, from, to types.ActionWorkItemState, actor, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.appendTransitionUnlocked(ctx, id, from, to, actor, reason)
}

func (q *Queue) appendTransitionUnlocked(ctx context.Context, id string, from, to types.ActionWorkItemState, actor, reason string) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO action_state_transitions (work_item_id, from_state, to_state, actor, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, string(from), string(to), actor, reason, q.now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("append transition for %s: %w", id, err)
	}
	return nil
}

func leaseExpired(raw string, now time.Time) bool {
	if raw == "" {
		return true
	}
	expires, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return true
	}
	return !expires.After(now)
}

func validTransition(from, to types.ActionWorkItemState) bool {
	allowed := map[types.ActionWorkItemState][]types.ActionWorkItemState{
		types.ActionWorkItemStateProposed:             {types.ActionWorkItemStateClaimable, types.ActionWorkItemStateCanceled},
		types.ActionWorkItemStateClaimable:            {types.ActionWorkItemStateClaimed, types.ActionWorkItemStateCanceled, types.ActionWorkItemStateEscalated},
		types.ActionWorkItemStateClaimed:              {types.ActionWorkItemStateClaimable, types.ActionWorkItemStatePreflighted, types.ActionWorkItemStateFailed, types.ActionWorkItemStateEscalated},
		types.ActionWorkItemStatePreflighted:          {types.ActionWorkItemStatePatched, types.ActionWorkItemStateTested, types.ActionWorkItemStateApprovedForExecution, types.ActionWorkItemStateFailed},
		types.ActionWorkItemStatePatched:              {types.ActionWorkItemStateTested, types.ActionWorkItemStateFailed},
		types.ActionWorkItemStateTested:               {types.ActionWorkItemStateApprovedForExecution, types.ActionWorkItemStateFailed},
		types.ActionWorkItemStateApprovedForExecution: {types.ActionWorkItemStateExecuted, types.ActionWorkItemStateFailed},
		types.ActionWorkItemStateExecuted:             {types.ActionWorkItemStateVerified, types.ActionWorkItemStateFailed},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}
