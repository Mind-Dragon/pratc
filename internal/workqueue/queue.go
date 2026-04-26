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
		`CREATE TABLE IF NOT EXISTS action_intents (
			id TEXT PRIMARY KEY,
			repo TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			action TEXT NOT NULL,
			dry_run INTEGER NOT NULL,
			policy_profile TEXT NOT NULL,
			ordinal INTEGER NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_action_intents_work_item ON action_intents(repo, work_item_id, ordinal);`,
		`CREATE INDEX IF NOT EXISTS idx_action_intents_idempotency ON action_intents(idempotency_key);`,
		`CREATE TABLE IF NOT EXISTS action_state_transitions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			work_item_id TEXT NOT NULL,
			from_state TEXT NOT NULL,
			to_state TEXT NOT NULL,
			actor TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS action_proof_bundles (
			id TEXT PRIMARY KEY,
			work_item_id TEXT NOT NULL,
			pr_number INTEGER NOT NULL,
			summary TEXT NOT NULL,
			evidence_refs_json TEXT NOT NULL,
			artifact_refs_json TEXT NOT NULL,
			test_commands_json TEXT NOT NULL,
			test_results_json TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_action_proof_bundles_work_item_id ON action_proof_bundles(work_item_id);`,
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
	for _, intent := range plan.ActionIntents {
		if intent.WorkItemID == "" {
			return fmt.Errorf("action intent %s missing work item id", intent.ID)
		}
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
	for ordinal, intent := range plan.ActionIntents {
		if err := q.UpsertIntent(ctx, plan.Repo, intent, ordinal); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) UpsertIntent(ctx context.Context, repo string, intent types.ActionIntent, ordinal int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.upsertIntentUnlocked(ctx, repo, intent, ordinal)
}

func (q *Queue) upsertIntentUnlocked(ctx context.Context, repo string, intent types.ActionIntent, ordinal int) error {
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	if intent.ID == "" {
		return fmt.Errorf("action intent id is required")
	}
	if intent.WorkItemID == "" {
		return fmt.Errorf("action intent %s missing work item id", intent.ID)
	}
	if intent.IdempotencyKey == "" {
		return fmt.Errorf("action intent %s missing idempotency key", intent.ID)
	}
	now := q.now().UTC().Format(time.RFC3339Nano)
	payload, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("marshal action intent %s: %w", intent.ID, err)
	}
	_, err = q.db.ExecContext(ctx, `
		INSERT INTO action_intents (
			id, repo, work_item_id, pr_number, action, dry_run, policy_profile, ordinal, idempotency_key, payload_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			repo = excluded.repo,
			work_item_id = excluded.work_item_id,
			pr_number = excluded.pr_number,
			action = excluded.action,
			dry_run = excluded.dry_run,
			policy_profile = excluded.policy_profile,
			ordinal = excluded.ordinal,
			idempotency_key = excluded.idempotency_key,
			payload_json = excluded.payload_json,
			updated_at = excluded.updated_at
	`, intent.ID, repo, intent.WorkItemID, intent.PRNumber, string(intent.Action), boolToInt(intent.DryRun), string(intent.PolicyProfile), ordinal, intent.IdempotencyKey, string(payload), now, now)
	if err != nil {
		return fmt.Errorf("upsert action intent %s: %w", intent.ID, err)
	}
	return nil
}

func (q *Queue) GetIntentsForWorkItem(ctx context.Context, repo, workItemID string) ([]types.ActionIntent, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if repo == "" {
		return nil, fmt.Errorf("repo is required")
	}
	if workItemID == "" {
		return nil, fmt.Errorf("work item id is required")
	}
	rows, err := q.db.QueryContext(ctx, `
		SELECT payload_json FROM action_intents
		WHERE repo = ? AND work_item_id = ?
		ORDER BY ordinal ASC, id ASC
	`, repo, workItemID)
	if err != nil {
		return nil, fmt.Errorf("query action intents for work item %s: %w", workItemID, err)
	}
	defer rows.Close()
	intents := []types.ActionIntent{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var intent types.ActionIntent
		if err := json.Unmarshal([]byte(raw), &intent); err != nil {
			return nil, fmt.Errorf("decode action intent for work item %s: %w", workItemID, err)
		}
		intents = append(intents, intent)
	}
	return intents, rows.Err()
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

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
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

// GetLaneCounts returns claimable and claimed counts per lane for a given repo.
func (q *Queue) GetLaneCounts(ctx context.Context, repo string) (map[types.ActionLane]struct{ Claimable, Claimed int }, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Query counts grouped by lane and state
	query := `SELECT lane, state, COUNT(*) FROM action_work_items WHERE repo = ? GROUP BY lane, state`
	rows, err := q.db.QueryContext(ctx, query, repo)
	if err != nil {
		return nil, fmt.Errorf("query lane counts: %w", err)
	}
	defer rows.Close()

	counts := make(map[types.ActionLane]struct{ Claimable, Claimed int })
	for rows.Next() {
		var laneStr, stateStr string
		var cnt int
		if err := rows.Scan(&laneStr, &stateStr, &cnt); err != nil {
			return nil, fmt.Errorf("scan lane count: %w", err)
		}
		lane := types.ActionLane(laneStr)
		state := types.ActionWorkItemState(stateStr)
		entry := counts[lane]
		if state == types.ActionWorkItemStateClaimable {
			entry.Claimable = cnt
		} else if state == types.ActionWorkItemStateClaimed {
			entry.Claimed = cnt
		}
		counts[lane] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return counts, nil
}

// AttachProof validates and attaches a proof bundle to a claimed work item.
// If the bundle ID already exists for the work item, returns the existing bundle idempotently.
// On successful attach, appends bundle.ID to item.ProofBundleRefs and transitions item to preflighted.
func (q *Queue) AttachProof(ctx context.Context, workItemID, workerID string, bundle types.ProofBundle) (types.ProofBundle, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Get work item
	item, err := q.getUnlocked(ctx, workItemID)
	if err != nil {
		return types.ProofBundle{}, err
	}

	// Check if bundle ID already exists for this work item (idempotent)
	existing, err := q.getProofUnlocked(ctx, bundle.ID)
	if err == nil {
		// Bundle already exists; ensure it's for the same work item
		if existing.WorkItemID != workItemID {
			return types.ProofBundle{}, fmt.Errorf("proof bundle %s belongs to different work item %s", bundle.ID, existing.WorkItemID)
		}
		// Ensure ProofBundleRefs includes the bundle ID (should already)
		found := false
		for _, ref := range item.ProofBundleRefs {
			if ref == bundle.ID {
				found = true
				break
			}
		}
		if !found {
			item.ProofBundleRefs = append(item.ProofBundleRefs, bundle.ID)
			if err := q.saveUnlocked(ctx, item); err != nil {
				return types.ProofBundle{}, fmt.Errorf("save work item after proof attach (idempotent): %w", err)
			}
		}
		// Return existing bundle idempotently
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return types.ProofBundle{}, fmt.Errorf("checking existing proof bundle: %w", err)
	}

	// Bundle ID not found; validate item state and lease ownership
	if item.State != types.ActionWorkItemStateClaimed {
		return types.ProofBundle{}, fmt.Errorf("work item %s is not claimed (state=%s)", workItemID, item.State)
	}
	if item.LeaseState.ClaimedBy != workerID {
		return types.ProofBundle{}, fmt.Errorf("work item %s not claimed by worker %s", workItemID, workerID)
	}
	if leaseExpired(item.LeaseState.ExpiresAt, q.now()) {
		return types.ProofBundle{}, fmt.Errorf("lease for work item %s has expired", workItemID)
	}

	// Validate proof bundle fields
	if err := types.ValidateProofBundle(item, bundle); err != nil {
		return types.ProofBundle{}, fmt.Errorf("proof bundle validation failed: %w", err)
	}

	// Insert new proof bundle
	evidenceRefsJSON, err := json.Marshal(bundle.EvidenceRefs)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("marshal evidence refs: %w", err)
	}
	artifactRefsJSON, err := json.Marshal(bundle.ArtifactRefs)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("marshal artifact refs: %w", err)
	}
	testCommandsJSON, err := json.Marshal(bundle.TestCommands)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("marshal test commands: %w", err)
	}
	testResultsJSON, err := json.Marshal(bundle.TestResults)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("marshal test results: %w", err)
	}

	_, err = q.db.ExecContext(ctx, `
		INSERT INTO action_proof_bundles (
			id, work_item_id, pr_number, summary,
			evidence_refs_json, artifact_refs_json,
			test_commands_json, test_results_json,
			created_by, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, bundle.ID, bundle.WorkItemID, bundle.PRNumber, bundle.Summary,
		string(evidenceRefsJSON), string(artifactRefsJSON),
		string(testCommandsJSON), string(testResultsJSON),
		bundle.CreatedBy, bundle.CreatedAt)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("insert proof bundle: %w", err)
	}

	// Append bundle ID to item.ProofBundleRefs if absent
	found := false
	for _, ref := range item.ProofBundleRefs {
		if ref == bundle.ID {
			found = true
			break
		}
	}
	if !found {
		item.ProofBundleRefs = append(item.ProofBundleRefs, bundle.ID)
	}

	// Transition claimed -> preflighted once
	if item.State == types.ActionWorkItemStateClaimed {
		item.State = types.ActionWorkItemStatePreflighted
		// Update work item with new state and ProofBundleRefs
		if err := q.saveUnlocked(ctx, item); err != nil {
			return types.ProofBundle{}, fmt.Errorf("save work item after proof attach: %w", err)
		}
		// Record transition
		if err := q.appendTransitionUnlocked(ctx, item.ID, types.ActionWorkItemStateClaimed, types.ActionWorkItemStatePreflighted, workerID, "proof_attached"); err != nil {
			return types.ProofBundle{}, fmt.Errorf("append transition: %w", err)
		}
	}

	return bundle, nil
}

// GetProof retrieves a proof bundle by its ID.
func (q *Queue) GetProof(ctx context.Context, id string) (types.ProofBundle, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.getProofUnlocked(ctx, id)
}

// getProofUnlocked retrieves a proof bundle by ID (internal, assumes lock held).
func (q *Queue) getProofUnlocked(ctx context.Context, id string) (types.ProofBundle, error) {
	var (
		workItemID           string
		prNumber             int
		summary              string
		evidenceRefsJSON     string
		artifactRefsJSON     string
		testCommandsJSON     string
		testResultsJSON      string
		createdBy, createdAt string
	)
	err := q.db.QueryRowContext(ctx, `
		SELECT work_item_id, pr_number, summary, evidence_refs_json, artifact_refs_json,
		       test_commands_json, test_results_json, created_by, created_at
		FROM action_proof_bundles WHERE id = ?
	`, id).Scan(&workItemID, &prNumber, &summary, &evidenceRefsJSON, &artifactRefsJSON,
		&testCommandsJSON, &testResultsJSON, &createdBy, &createdAt)
	if err != nil {
		return types.ProofBundle{}, fmt.Errorf("get proof bundle %s: %w", id, err)
	}
	var evidenceRefs, artifactRefs, testCommands, testResults []string
	if err := json.Unmarshal([]byte(evidenceRefsJSON), &evidenceRefs); err != nil {
		return types.ProofBundle{}, fmt.Errorf("unmarshal evidence refs: %w", err)
	}
	if err := json.Unmarshal([]byte(artifactRefsJSON), &artifactRefs); err != nil {
		return types.ProofBundle{}, fmt.Errorf("unmarshal artifact refs: %w", err)
	}
	if err := json.Unmarshal([]byte(testCommandsJSON), &testCommands); err != nil {
		return types.ProofBundle{}, fmt.Errorf("unmarshal test commands: %w", err)
	}
	if err := json.Unmarshal([]byte(testResultsJSON), &testResults); err != nil {
		return types.ProofBundle{}, fmt.Errorf("unmarshal test results: %w", err)
	}
	return types.ProofBundle{
		ID:           id,
		WorkItemID:   workItemID,
		PRNumber:     prNumber,
		Summary:      summary,
		EvidenceRefs: evidenceRefs,
		ArtifactRefs: artifactRefs,
		TestCommands: testCommands,
		TestResults:  testResults,
		CreatedBy:    createdBy,
		CreatedAt:    createdAt,
	}, nil
}

// GetProofsForItem retrieves all proof bundles attached to a work item.
func (q *Queue) GetProofsForItem(ctx context.Context, workItemID string) ([]types.ProofBundle, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	rows, err := q.db.QueryContext(ctx, `
		SELECT id FROM action_proof_bundles WHERE work_item_id = ?
	`, workItemID)
	if err != nil {
		return nil, fmt.Errorf("query proof bundles for work item %s: %w", workItemID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan proof bundle id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	bundles := make([]types.ProofBundle, 0, len(ids))
	for _, id := range ids {
		bundle, err := q.getProofUnlocked(ctx, id)
		if err != nil {
			return nil, err
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}

// GetActionQueueStats returns nested map: byLane[lane][state] = count
func (q *Queue) GetActionQueueStats() (byLane map[string]map[string]int, err error) {
	ctx := context.Background()
	q.mu.Lock()
	defer q.mu.Unlock()

	byLane = make(map[string]map[string]int)

	rows, err := q.db.QueryContext(ctx, `SELECT lane, state, COUNT(*) FROM action_work_items GROUP BY lane, state`)
	if err != nil {
		return nil, fmt.Errorf("query action queue stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var lane, state string
		var count int
		if err := rows.Scan(&lane, &state, &count); err != nil {
			return nil, fmt.Errorf("scan action queue stats: %w", err)
		}
		if byLane[lane] == nil {
			byLane[lane] = make(map[string]int)
		}
		byLane[lane][state] = count
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return byLane, nil
}

// ExecutorLedger contains transitions and proof bundles for executor audit
type ExecutorLedger struct {
	Transitions  []ActionTransition `json:"transitions"`
	ProofBundles []ProofBundleView  `json:"proof_bundles"`
}

// ActionTransition represents a state change in the action work queue
type ActionTransition struct {
	WorkItemID string    `json:"work_item_id"`
	FromState  string    `json:"from_state"`
	ToState    string    `json:"to_state"`
	Actor      string    `json:"actor"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

// ProofBundleView represents a proof bundle for audit purposes
type ProofBundleView struct {
	ID           string    `json:"id"`
	WorkItemID   string    `json:"work_item_id"`
	PRNumber     int       `json:"pr_number"`
	Summary      string    `json:"summary"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	TestCommands []string  `json:"test_commands"`
	TestResults  []string  `json:"test_results"`
}

// GetExecutorLedger returns structured ledger with transitions and proof bundles
func (q *Queue) GetExecutorLedger(limit int) (ExecutorLedger, error) {
	ctx := context.Background()
	q.mu.Lock()
	defer q.mu.Unlock()

	if limit <= 0 {
		limit = 100
	}

	// Query transitions
	transRows, err := q.db.QueryContext(ctx, `
		SELECT work_item_id, from_state, to_state, actor, reason, created_at
		FROM action_state_transitions
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return ExecutorLedger{}, fmt.Errorf("query transitions: %w", err)
	}
	defer transRows.Close()

	var transitions []ActionTransition
	for transRows.Next() {
		var t ActionTransition
		var createdAtStr string
		if err := transRows.Scan(&t.WorkItemID, &t.FromState, &t.ToState, &t.Actor, &t.Reason, &createdAtStr); err != nil {
			return ExecutorLedger{}, fmt.Errorf("scan transition: %w", err)
		}
		t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			t.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr) // Try RFC3339 as fallback
		}
		transitions = append(transitions, t)
	}

	if err := transRows.Err(); err != nil {
		return ExecutorLedger{}, err
	}

	// Query proof bundles
	bundleRows, err := q.db.QueryContext(ctx, `
		SELECT id, work_item_id, pr_number, summary, test_commands_json, test_results_json, created_by, created_at
		FROM action_proof_bundles
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return ExecutorLedger{}, fmt.Errorf("query proof bundles: %w", err)
	}
	defer bundleRows.Close()

	var bundles []ProofBundleView
	for bundleRows.Next() {
		var b ProofBundleView
		var testCommandsJSON, testResultsJSON, createdAtStr string
		if err := bundleRows.Scan(&b.ID, &b.WorkItemID, &b.PRNumber, &b.Summary, &testCommandsJSON, &testResultsJSON, &b.CreatedBy, &createdAtStr); err != nil {
			return ExecutorLedger{}, fmt.Errorf("scan proof bundle: %w", err)
		}
		b.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
		if err != nil {
			b.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr) // Try RFC3339 as fallback
		}
		if err := json.Unmarshal([]byte(testCommandsJSON), &b.TestCommands); err != nil {
			return ExecutorLedger{}, fmt.Errorf("unmarshal test commands: %w", err)
		}
		if err := json.Unmarshal([]byte(testResultsJSON), &b.TestResults); err != nil {
			return ExecutorLedger{}, fmt.Errorf("unmarshal test results: %w", err)
		}
		bundles = append(bundles, b)
	}

	if err := bundleRows.Err(); err != nil {
		return ExecutorLedger{}, err
	}

	return ExecutorLedger{
		Transitions:  transitions,
		ProofBundles: bundles,
	}, nil
}

// QueueSummary contains summary statistics for the work queue
type QueueSummary struct {
	TotalItems    int            `json:"total_items"`
	ByState       map[string]int `json:"by_state"`
	ByLane        map[string]int `json:"by_lane"`
	ExpiredLeases int            `json:"expired_leases"`
}

// GetQueueSummary returns summary statistics for the work queue
func (q *Queue) GetQueueSummary() (QueueSummary, error) {
	ctx := context.Background()
	q.mu.Lock()
	defer q.mu.Unlock()

	var summary QueueSummary
	summary.ByState = make(map[string]int)
	summary.ByLane = make(map[string]int)

	// Total items
	err := q.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM action_work_items`).Scan(&summary.TotalItems)
	if err != nil {
		return QueueSummary{}, fmt.Errorf("query total items: %w", err)
	}

	// By state
	stateRows, err := q.db.QueryContext(ctx, `SELECT state, COUNT(*) FROM action_work_items GROUP BY state`)
	if err != nil {
		return QueueSummary{}, fmt.Errorf("query by state: %w", err)
	}
	defer stateRows.Close()

	for stateRows.Next() {
		var state string
		var count int
		if err := stateRows.Scan(&state, &count); err != nil {
			return QueueSummary{}, fmt.Errorf("scan by state: %w", err)
		}
		summary.ByState[state] = count
	}

	if err := stateRows.Err(); err != nil {
		return QueueSummary{}, err
	}

	// By lane
	laneRows, err := q.db.QueryContext(ctx, `SELECT lane, COUNT(*) FROM action_work_items GROUP BY lane`)
	if err != nil {
		return QueueSummary{}, fmt.Errorf("query by lane: %w", err)
	}
	defer laneRows.Close()

	for laneRows.Next() {
		var lane string
		var count int
		if err := laneRows.Scan(&lane, &count); err != nil {
			return QueueSummary{}, fmt.Errorf("scan by lane: %w", err)
		}
		summary.ByLane[lane] = count
	}

	if err := laneRows.Err(); err != nil {
		return QueueSummary{}, err
	}

	// Expired leases
	err = q.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM action_work_items
		WHERE lease_expires_at < ? AND state IN (?, ?)
	`, q.now().UTC().Format(time.RFC3339Nano), string(types.ActionWorkItemStateClaimed), string(types.ActionWorkItemStatePreflighted)).Scan(&summary.ExpiredLeases)
	if err != nil {
		return QueueSummary{}, fmt.Errorf("query expired leases: %w", err)
	}

	return summary, nil
}
