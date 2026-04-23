# OpenClaw GitHub Test Prep — 20260423T221550Z

Purpose: prepare the next OpenClaw GitHub validation run after reprioritizing v1.8 toward ML automation and TUI feedback.

## Current preflight truth

- Repo: `openclaw/openclaw`
- Live GitHub open PR count: `6627`
- prATC preflight cache count: `6646` PRs, last synced `2026-04-21`
- SQLite raw cached rows for `openclaw/openclaw`: `6746`
- Delta from preflight: `0 PRs to fetch`
- Lock status: clear
- GitHub auth: active `Mind-Dragon`, token scopes include `repo`, `workflow`, `read:org`, `gist`
- Runtime health: `http://100.112.201.95:7400/healthz` returns `{"status":"ok","version":"1.7.0","api_version":"v1.6"}`

## Recommended test command

Use a cache-first full workflow first, because preflight says the OpenClaw cache is up to date and no sync is needed:

```bash
cd /home/agent/pratc
RUN_DIR=autonomous/runs/20260423T221550Z-openclaw-github-test
./bin/pratc workflow \
  --repo=openclaw/openclaw \
  --out-dir="$RUN_DIR" \
  --max-prs=0 \
  --sync-max-prs=0 \
  --progress=false
python3 scripts/audit_guideline.py "$RUN_DIR"
python3 scripts/gap_list_from_audit.py \
  --audit "$RUN_DIR/AUDIT_RESULTS.json" \
  --gap-list autonomous/GAP_LIST.md \
  --state autonomous/STATE.yaml
python3 scripts/autonomous_controller.py audit-state
```

## Optional fully-live recapture

Only use this if you explicitly want to ignore the current green cache and recapture GitHub state:

```bash
cd /home/agent/pratc
RUN_DIR=autonomous/runs/20260423T221550Z-openclaw-github-live
./bin/pratc workflow \
  --repo=openclaw/openclaw \
  --out-dir="$RUN_DIR" \
  --max-prs=0 \
  --sync-max-prs=0 \
  --resync \
  --refresh-sync \
  --progress=false
```

If the live path rate-limits, treat pause/resume state as expected, not as failure.

## Watch points

- Verify the run emits `analyze.json`, `step-2-analyze.json`, `step-3-cluster.json`, `step-4-graph.json`, `step-5-plan.json`, and `report.pdf`.
- Re-run `scripts/audit_guideline.py` against the new run directory before making any success claim.
- For v1.8 ML automation work, inspect duplicate/canonicalization quality, disposal false positives, confidence calibration, and selected-plan acceptance signals from the OpenClaw artifacts.
