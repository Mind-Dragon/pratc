# OpenClaw GitHub Test Prep — 20260423T221550Z

Purpose: prepare the next OpenClaw GitHub validation run after reprioritizing v1.8 toward ML automation and TUI feedback.

## Current preflight truth

- Repo: `openclaw/openclaw`
- Live GitHub access: OK via `gh`, active account `Mind-Dragon`, scopes include `repo`, `workflow`, `read:org`, `gist`
- GitHub core rate during prep: `4991/5000` remaining
- Runtime health: `http://100.112.201.95:7400/healthz` returns `{"status":"ok","version":"1.7.0","api_version":"v1.6"}`
- Runtime binary restarted under tmux `pratc-autonomous` with `commit=b4f266eefca9`, runtime DB env set
- Lock status: no active `pratc workflow` process after cleanup

## Live smoke attempt result

A 25-PR workflow smoke was attempted against live OpenClaw GitHub. It did not reach useful artifacts because GitHub returned a secondary-rate-limit retry window before sync work began:

- Retry reset epoch: `1776986427`
- Reset time: `2026-04-23T23:20:27Z`
- Core quota was still healthy, so this is secondary throttling, not core exhaustion
- The orphaned workflow process was killed and verified gone

## Full test command after secondary reset

Run the full live validation only after the secondary window clears. Use tmux because the workflow may legitimately wait through GitHub backoff windows.

```bash
cd /home/agent/pratc
RUN_DIR=autonomous/runs/$(date -u +%Y%m%dT%H%M%SZ)-openclaw-github-live
tmux new-session -d -s pratc-openclaw-live \
  "cd /home/agent/pratc && ./bin/pratc workflow \
    --repo=openclaw/openclaw \
    --out-dir=\"$RUN_DIR\" \
    --max-prs=0 \
    --sync-max-prs=0 \
    --resync \
    --refresh-sync \
    --progress=true \
    > \"$RUN_DIR.log\" 2>&1"
```

Then verify:

```bash
python3 scripts/audit_guideline.py "$RUN_DIR"
python3 scripts/gap_list_from_audit.py \
  --audit "$RUN_DIR/AUDIT_RESULTS.json" \
  --gap-list autonomous/GAP_LIST.md \
  --state autonomous/STATE.yaml
python3 scripts/autonomous_controller.py audit-state
```

## Cache-backed fallback

If live secondary throttling persists, use the existing OpenClaw snapshot for a non-live regression run, then label it cache-backed in the run notes. Do not claim it as a fresh GitHub recapture.

## Watch points for v1.8 ML automation

- Duplicate/canonicalization quality
- Disposal false positives
- Bucket/category override candidates
- Selected-plan acceptance/rejection signals
- Confidence calibration and reason-trail quality
- Places where TUI feedback would be faster than editing JSON/YAML artifacts
