# Autonomous Runbook

Use this file for exact operator and controller commands. `AUTONOMOUS.md` defines policy; this file defines execution mechanics. `VERSION2.0.md` defines the active action-engine buildout plan.

## Bootstrap

### 1. Baseline repo truth

```bash
cd /home/agent/pratc
git status --short
git rev-parse --short HEAD
go test ./...
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
```

### 2. Build and verify binary provenance

```bash
cd /home/agent/pratc
make build
./bin/pratc --help | head -5
./bin/pratc version
```

If `./bin/pratc version` is unavailable or the banner does not identify the current release/commit, autonomous runtime proof is incomplete. Fix the version surface before closing the loop.

### 3. Start or verify API service

The CLI-only autonomous workflow can run without the API service, but service proof is required before claiming API runtime readiness.

```bash
cd /home/agent/pratc
mkdir -p autonomous/runtime/logs
: > autonomous/runtime/logs/serve.log
if tmux has-session -t pratc-autonomous 2>/dev/null; then tmux kill-session -t pratc-autonomous; fi
tmux new-session -d -s pratc-autonomous -c /home/agent/pratc \
  'echo "[tmux-start] $(date -u +%Y-%m-%dT%H:%M:%SZ)"; ./bin/pratc version; PRATC_SETTINGS_DB=/home/agent/pratc/autonomous/runtime/pratc-settings.db PRATC_DB_PATH=/home/agent/.pratc/pratc.db ./bin/pratc serve --port 7400 --force-cache 2>&1 | tee -a autonomous/runtime/logs/serve.log'
tmux capture-pane -pt pratc-autonomous -S -80
```

Health probes:

```bash
curl -sf http://100.112.201.95:7400/healthz
curl -sf http://127.0.0.1:7400/healthz
```

Prefer the real VPN IPv4 probe for runtime proof. If service proof is intentionally skipped, record the reason in `autonomous/runs/<run-id>/run-metadata.yaml`.

### 4. Start the monitor in a separate pane

```bash
cd /home/agent/pratc
./bin/pratc monitor
# or: go run ./cmd/pratc monitor
```

The monitor is observational in v1.x. In v2.0 it becomes the TUI dashboard for action lanes, queue leases, executor state, and audit stream. Completion proof still comes from artifacts and audits, not from visual state alone.

### 5. Initialize controller checkpoint

Historical green audit available for bootstrap:

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py init \
  --repo openclaw/openclaw \
  --corpus-dir projects/openclaw_openclaw/runs/release-audit-20260421T023224Z
```

Fresh current-HEAD run is still required before setting `phase: complete` or `last_green_commit` to the current commit.

## Full autonomous loop

### Promotion rule during execution

When a new issue is discovered, promote it immediately into exactly one of:

- an audit failure
- a gap in `autonomous/GAP_LIST.md`
- a live todo item with a verification command
- a documented blocker/non-goal

Do not leave findings stranded only in chat.

### Audit latest known historical run

```bash
cd /home/agent/pratc
python3 scripts/audit_guideline.py projects/openclaw_openclaw/runs/release-audit-20260421T023224Z
python3 scripts/gap_list_from_audit.py \
  --audit projects/openclaw_openclaw/runs/release-audit-20260421T023224Z/AUDIT_RESULTS.json \
  --gap-list autonomous/GAP_LIST.md \
  --state autonomous/STATE.yaml
```

### Validate the current control plane before starting a wave

```bash
cd /home/agent/pratc
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
go test ./...
```

### Create a fresh current-HEAD run directory

```bash
cd /home/agent/pratc
RUN_ID=$(date -u +%Y%m%dT%H%M%SZ)
RUN_DIR=autonomous/runs/$RUN_ID
mkdir -p "$RUN_DIR/subagent-results"
```

### Write run metadata

```bash
cat > "$RUN_DIR/run-metadata.yaml" <<EOF
repo: openclaw/openclaw
branch: $(git branch --show-current)
commit: $(git rev-parse --short HEAD)
binary: ./bin/pratc
binary_help_head: |
$(./bin/pratc --help | head -5 | sed 's/^/  /')
service_health_url: http://100.112.201.95:7400/healthz
service_health_status: pending
corpus_source: cache-first
sync_refreshed: false
max_prs_applied: 0
created_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF
```

Update `service_health_status` after probing, or record an explicit skip reason.

### Rerun after a fix wave or for fresh closeout

Full-corpus cache-backed run, intentionally no analysis cap:

```bash
cd /home/agent/pratc
./bin/pratc analyze --repo openclaw/openclaw --force-cache --format=json > "$RUN_DIR/analyze.json"
./bin/pratc cluster --repo openclaw/openclaw --force-cache --format=json > "$RUN_DIR/step-3-cluster.json"
./bin/pratc graph --repo openclaw/openclaw --force-cache --format=json > "$RUN_DIR/step-4-graph.json"
./bin/pratc plan --repo openclaw/openclaw --force-cache --target 20 --format=json > "$RUN_DIR/step-5-plan.json"
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=advisory --format=json > "$RUN_DIR/action-plan.json"
./bin/pratc report --repo openclaw/openclaw --input-dir "$RUN_DIR" --output "$RUN_DIR/report.pdf"
python3 scripts/audit_guideline.py "$RUN_DIR"
python3 scripts/gap_list_from_audit.py \
  --audit "$RUN_DIR/AUDIT_RESULTS.json" \
  --gap-list autonomous/GAP_LIST.md \
  --state autonomous/STATE.yaml
```

If a cap is intentionally used for smoke testing, record it in `run-metadata.yaml`; do not call the result full-corpus.

### Run controller reconciliation only

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py reconcile
```

### Mark the next wave active

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py next-wave
```

### Resume after interruption

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py resume
```

### Pause intentionally

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py pause --reason "operator hold"
```

### Mark success after verified green closeout

Only after a fresh current-HEAD run has all required audit checks passing:

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py complete --reason "fresh current-HEAD audit passed"
```

If manual audit checks remain, record explicit operator acceptance in `autonomous/runs/<run-id>/wave-summary.md` before marking success.

## Expected controller flow

1. Read `GUIDELINE.md`, `ARCHITECTURE.md`, `VERSION2.0.md`, `AUTONOMOUS.md`, `TODO.md`
2. Read `autonomous/STATE.yaml` and `autonomous/GAP_LIST.md`
3. Reconcile live repo state and latest run artifacts
4. Seed the Hermes session todo with the current wave
5. Dispatch subagents for non-trivial gap work
6. Verify locally with build + tests
7. Rerun analyze/cluster/graph/plan/report and regenerate the audit/gap list
8. Update checkpoint and continue or stop

## Recovery notes

### If the service is down

- restart `pratc serve` if API proof is required
- confirm `/healthz` on the real interface when possible
- do not trust stale TUI output from a dead process
- if CLI-only run is acceptable, record service skip reason in run metadata

### If audit output is missing

- rerun `scripts/audit_guideline.py`
- do not patch `GAP_LIST.md` by hand as a substitute for a fresh audit

### If `STATE.yaml` is stale relative to HEAD

- run `python3 scripts/autonomous_controller.py reconcile`
- if `last_audit_path` is missing, downgrade to bootstrap/fresh-run-required
- do not mark complete until a fresh current-HEAD audit exists

### If a wave fails without progress

- leave failed gaps open
- record blocker notes in `GAP_LIST.md`
- set `stop_reason` in `STATE.yaml` if halting

## Current canonical corpus

Current v1.7.1 action-engine baseline:

- repo: `openclaw/openclaw`
- corpus dir: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z`
- action plan: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z/action-plan.json`
- audit: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z/AUDIT_RESULTS.json` (`22` pass, `0` fail, `0` manual)
- report: `projects/openclaw_openclaw/runs/v171-head-20260424T153126Z/report.pdf`
- runtime proof: `autonomous/runtime/runtime-proof.json`
- tmux session: `pratc-autonomous` on port `7400`
- status: advisory ActionPlan and snapshot packet; not an execution manifest

Required next corpus for guarded/autonomous v2.0:

- repo: `openclaw/openclaw`
- corpus dir: `autonomous/runs/<fresh-current-head-run-id>`
- policy: `guarded` or `autonomous` only after operator enablement
- required new artifact: executor ledger plus post-action verification results
- status: pending implementation

## Wave B Commands

### Preflight Checker Usage

The live preflight checker enforces all 9 gates before any GitHub mutation:

```bash
cd /home/agent/pratc
./bin/pratc actions --repo openclaw/openclaw --force-cache --policy=guarded --format=json > "$RUN_DIR/action-plan.json"
```

Preflight checks include:
- PR still open
- head SHA unchanged or revalidated
- base branch allowed
- CI/checks green for merge
- mergeability clean for merge
- branch protection/review requirements satisfied
- token permission sufficient
- rate-limit budget sufficient
- policy allows action
- idempotency key not already executed

### Guarded Comment/Label Executor Usage

The guarded executor performs safe non-destructive mutations:

```bash
cd /home/agent/pratc
./bin/pratc execute --repo openclaw/openclaw --work-item-id <ID> --policy=guarded --format=json
```

Available actions in guarded mode:
- `comment` - Add PR comment
- `label` - Add/remove PR labels
- `status` - Update PR status (if implemented)

### Ledger Persistence

The executor ledger uses SQLite for crash recovery:

```bash
# View ledger entries
sqlite3 /home/agent/.pratc/pratc.db "SELECT * FROM executor_ledger ORDER BY timestamp DESC LIMIT 20;"

# Check idempotency
sqlite3 /home/agent/.pratc/pratc.db "SELECT intent_id, transition FROM executor_ledger WHERE intent_id='<INTENT_ID>';"
```

Ledger schema:
- `executor_ledger` table with append-only writes
- Tracks: intent_id, transition, preflight_snapshot, mutation_snapshot, timestamp
- Enforces exactly-once execution via UNIQUE(intent_id, transition)

### Fix-and-Merge Sandbox Usage

The sandbox generates local proof bundles for `fix_and_merge` items:

```bash
cd /home/agent/pratc
./bin/pratc sandbox --repo openclaw/openclaw --work-item-id <ID> --format=json > "$RUN_DIR/proof-bundle.json"
```

Sandbox workflow:
1. Create isolated worktree/checkout per work item
2. Apply patch/rebase and capture proof
3. Run test command and capture output/exit code
4. Attach proof bundle to queue item
5. Dry-run only until v2 gates green

### Wave B Verification Commands

```bash
cd /home/agent/pratc
go test ./internal/executor ./internal/github ./internal/cache ./internal/monitor/...
./bin/pratc monitor --once || true
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
python3 scripts/autonomous_controller.py audit-state
```

### Wave B Completion Proof

Current Wave B implementation status:
- TUI panels render with live data (verified)
- Live preflight checker enforces all 9 gates
- Guarded comment/label executor works with fake GitHub
- Ledger persistence survives restart
- Post-action verification confirms mutations
- Fix-and-merge sandbox produces valid proof bundles
- E2E harness passes all audit checks
- Docs aligned with implementation

## Wave C Commands

### GitHub Token Environment Variables

Live GitHub mutations require a GitHub Personal Access Token (PAT). The token is read from environment variables with the following precedence:

1. `GITHUB_PAT` (recommended for local development)
2. `GITHUB_TOKEN` (common in CI environments)

**Never commit tokens to version control.** Use `psst SECRET_NAME -- <command>` or a secret manager.

**Example:**
```bash
export GITHUB_PAT="ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
./bin/pratc execute --repo owner/repo --work-item-id 123 --live
```

**Validation:** When live mode is enabled, the token must be non-empty (trimmed of whitespace). The helper `GetGitHubToken()` enforces this.

**Security:** Never log the token value; only log "token present" or "token missing".
