# Autonomous Runbook

Use this file for exact operator and controller commands. `AUTONOMOUS.md` defines policy; this file defines execution mechanics.

## Bootstrap

### 1. Baseline repo truth

```bash
cd /home/agent/pratc
git status --short
git rev-parse --short HEAD
go test ./...
```

### 2. Start or verify API service

```bash
cd /home/agent/pratc
./pratc-bin serve --port 7400
# or: go run ./cmd/pratc serve --port 7400
curl -sf http://127.0.0.1:7400/healthz
```

### 3. Start the monitor in a separate pane

```bash
cd /home/agent/pratc
./pratc-bin monitor
# or: go run ./cmd/pratc monitor
```

### 4. Initialize controller checkpoint

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py init \
  --repo openclaw/openclaw \
  --corpus-dir projects/openclaw_openclaw/runs/final-wave
```

## Full autonomous loop

### Promotion rule during execution

When a new issue is discovered, promote it immediately into exactly one of:
- an audit failure
- a gap in `autonomous/GAP_LIST.md`
- a live todo item with a verification command
- a documented blocker/non-goal

Do not leave findings stranded only in chat.

### Audit latest known run

```bash
cd /home/agent/pratc
python3 scripts/audit_guideline.py projects/openclaw_openclaw/runs/final-wave
python3 scripts/gap_list_from_audit.py \
  --audit projects/openclaw_openclaw/runs/final-wave/AUDIT_RESULTS.json \
  --gap-list autonomous/GAP_LIST.md \
  --state autonomous/STATE.yaml
```

### Validate the current control plane before starting a wave

```bash
cd /home/agent/pratc
go test ./...
python -m pytest -q tests/test_audit_guideline.py scripts/test_autonomous_controller.py
```

### Rerun after a fix wave

```bash
cd /home/agent/pratc
./bin/pratc analyze --repo openclaw/openclaw --force-cache --max-prs 5000 --format=json > <fresh-run-dir>/analyze.json
./bin/pratc cluster --repo openclaw/openclaw --force-cache --format=json > <fresh-run-dir>/step-3-cluster.json
./bin/pratc graph --repo openclaw/openclaw --force-cache --max-prs 5000 --format=json > <fresh-run-dir>/step-4-graph.json
./bin/pratc plan --repo openclaw/openclaw --force-cache --max-prs 5000 --target 20 --format=json > <fresh-run-dir>/step-5-plan.json
./bin/pratc report --repo openclaw/openclaw --input-dir <fresh-run-dir> --output <fresh-run-dir>/report.pdf
python3 scripts/audit_guideline.py <fresh-run-dir>
python3 scripts/gap_list_from_audit.py \
  --audit <fresh-run-dir>/AUDIT_RESULTS.json \
  --gap-list autonomous/GAP_LIST.md \
  --state autonomous/STATE.yaml
```

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

```bash
cd /home/agent/pratc
python3 scripts/autonomous_controller.py complete --reason "all required audit checks passed"
```

## Expected controller flow

1. Read `GUIDELINE.md`, `ARCHITECTURE.md`, `AUTONOMOUS.md`, `TODO.md`
2. Read `autonomous/STATE.yaml` and `autonomous/GAP_LIST.md`
3. Reconcile live repo state and latest run artifacts
4. Seed the Hermes session todo with the current wave
5. Dispatch subagents for non-trivial gap work
6. Verify locally with build + tests
7. Rerun analyze/cluster/graph/plan/report and regenerate the audit/gap list
8. Update checkpoint and continue or stop

## Recovery notes

### If the service is down
- restart `pratc serve`
- confirm `/healthz`
- do not trust stale TUI output from a dead process

### If audit output is missing
- rerun `scripts/audit_guideline.py`
- do not patch `GAP_LIST.md` by hand as a substitute

### If `STATE.yaml` is stale relative to HEAD
- run `python3 scripts/autonomous_controller.py reconcile`
- this should refresh branch, commit, timestamps, and resume pointers

### If a wave fails without progress
- leave failed gaps open
- record blocker notes in `GAP_LIST.md`
- set `stop_reason` in `STATE.yaml` if halting

## Current canonical corpus

Until replaced by a fresher verified run, use:
- repo: `openclaw/openclaw`
- corpus dir: `projects/openclaw_openclaw/runs/final-wave`
