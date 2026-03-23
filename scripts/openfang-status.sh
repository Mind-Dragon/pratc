#!/usr/bin/env bash
set -euo pipefail

PROJECT_ROOT="/home/agent/pratc"
STATE_FILE="/home/agent/.openfang/overseer-state.json"
LOG_FILE="/home/agent/.openfang/overseer.log"
PLAN_FILE="$PROJECT_ROOT/.sisyphus/plans/pratc.md"
STATUS_DIR="$PROJECT_ROOT/.sisyphus/status"

WATCH_MODE=0
INTERVAL=5

while [[ $# -gt 0 ]]; do
  case "$1" in
    --watch)
      WATCH_MODE=1
      shift
      ;;
    --interval)
      INTERVAL="${2:-5}"
      shift 2
      ;;
    --once)
      WATCH_MODE=0
      shift
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

render() {
  python - <<'PY'
import json
import os
import re
import subprocess
from datetime import datetime, timezone
from pathlib import Path

project_root = Path("/home/agent/pratc")
state_file = Path("/home/agent/.openfang/overseer-state.json")
log_file = Path("/home/agent/.openfang/overseer.log")
plan_file = project_root / ".sisyphus/plans/pratc.md"
status_dir = project_root / ".sisyphus/status"
progress_file = Path("/home/agent/.openfang/status-progress.json")

# Liveness thresholds (seconds)
# OPENFANG_IDLE_SECONDS: running agent with age <= threshold is HEALTHY
# OPENFANG_STALE_SECONDS: running agent with age > threshold is STALE
# OPENFANG_OVERSEER_WARN_SECONDS: last_tick age over threshold is WARN
# OPENFANG_NO_PROGRESS_TICKS: unchanged remaining count over threshold is CRITICAL

def env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or raw.strip() == "":
        return default
    try:
        val = int(raw)
        return val if val >= 0 else default
    except Exception:
        return default

IDLE_SECONDS = env_int("OPENFANG_IDLE_SECONDS", 900)
STALE_SECONDS = env_int("OPENFANG_STALE_SECONDS", 3600)
OVERSEER_WARN_SECONDS = env_int("OPENFANG_OVERSEER_WARN_SECONDS", 900)
NO_PROGRESS_TICKS = env_int("OPENFANG_NO_PROGRESS_TICKS", 3)

def classify_agent(state: str, ready: bool, auth_status: str, age_s: int | None) -> str:
    if state != "Running" or not ready:
        return "UNREADY"
    if age_s is None:
        return "UNKNOWN"
    if age_s > STALE_SECONDS:
        if auth_status in {"unknown", "missing", "missing_auth", ""}:
            return "AUTH_ISSUE"
        return "STALE"
    if age_s > IDLE_SECONDS:
        return "IDLE"
    return "HEALTHY"

def parse_ts(ts: str):
    try:
        return datetime.fromisoformat(ts.replace("Z", "+00:00"))
    except Exception:
        return None

now = datetime.now(timezone.utc)

try:
    plan_text = plan_file.read_text(encoding="utf-8") if plan_file.exists() else ""
    task_ids = re.findall(r"^\s*- \[ \] ((?:\d+|F\d+))\.", plan_text, flags=re.MULTILINE)
    total = len(task_ids)

    status_map = {}
    for p in status_dir.glob("task-*.md"):
        t = p.read_text(encoding="utf-8")
        tid = re.search(r"task id:\s*([A-Za-z0-9]+)", t)
        st = re.search(r"status:\s*([a-z_]+)", t)
        if tid and st:
            status_map[tid.group(1)] = st.group(1)

    verified = sum(1 for tid in task_ids if status_map.get(tid) == "verified")
    merged = sum(1 for tid in task_ids if status_map.get(tid) == "merged")
    done_like = sum(1 for tid in task_ids if status_map.get(tid) in {"done", "merged", "verified"})
    remaining = max(total - done_like, 0)

    state = {}
    if state_file.exists():
        try:
            state = json.loads(state_file.read_text(encoding="utf-8"))
        except Exception:
            state = {}

    approvals_total = "?"
    manual_pending = state.get("manual_review_pending", 0)
    try:
        raw = subprocess.check_output(["openfang", "approvals", "list"], text=True, timeout=10)
        approvals = json.loads(raw)
        approvals_total = approvals.get("total", 0)
    except Exception:
        pass

    last_tick = state.get("last_tick", "unknown")
    dispatch = state.get("dispatch_result", "unknown")
    message_mode = state.get("message_mode", "unknown")
    next_task = state.get("last_next_task", "unknown")

    tick_age_s = None
    age = "unknown"
    tick_dt = parse_ts(last_tick) if isinstance(last_tick, str) else None
    if tick_dt:
        tick_age_s = max(int((now - tick_dt).total_seconds()), 0)
        age = f"{tick_age_s}s"

    recent = []
    if log_file.exists():
        lines = log_file.read_text(encoding="utf-8").splitlines()
        recent = lines[-8:]

    # Agent liveness snapshot
    agent_items = []
    agent_list_error = None
    try:
        raw_agents = subprocess.check_output(["openfang", "agent", "list", "--json"], text=True, timeout=10)
        parsed_agents = json.loads(raw_agents)
        agent_items = parsed_agents if isinstance(parsed_agents, list) else parsed_agents.get("agents", [])
    except Exception as exc:
        agent_list_error = str(exc)

    agent_rows = []
    class_counts = {"HEALTHY": 0, "IDLE": 0, "STALE": 0, "UNREADY": 0, "AUTH_ISSUE": 0, "UNKNOWN": 0}
    required_agents = {"orchestrator", "planner", "coder", "test-engineer", "code-reviewer", "debugger", "architect"}
    required_bad = []

    for item in agent_items:
        name = item.get("name") or "unknown"
        state_val = item.get("state") or "unknown"
        ready = bool(item.get("ready"))
        auth_status = (item.get("auth_status") or "").strip().lower()
        last_active_raw = item.get("last_active")
        age_s = None
        if isinstance(last_active_raw, str):
            last_active_dt = parse_ts(last_active_raw)
            if last_active_dt:
                age_s = max(int((now - last_active_dt).total_seconds()), 0)

        health_class = classify_agent(state_val, ready, auth_status, age_s)
        class_counts[health_class] = class_counts.get(health_class, 0) + 1
        if name in required_agents and health_class in {"UNREADY", "STALE", "AUTH_ISSUE", "UNKNOWN"}:
            required_bad.append(name)

        agent_rows.append({
            "name": name,
            "state": state_val,
            "ready": ready,
            "auth": auth_status or "n/a",
            "age_s": age_s,
            "class": health_class,
        })

    # Use overseer-provided remaining when available for cadence consistency.
    # Fallback to computed remaining from status artifacts.
    progress_remaining = remaining
    state_remaining = state.get("remaining")
    if isinstance(state_remaining, int) and state_remaining >= 0:
        progress_remaining = state_remaining

    # No-progress tracking snapshot
    ticks_without_progress = 0
    last_remaining_change_at = now.isoformat()
    try:
        progress = {}
        if progress_file.exists():
            try:
                progress = json.loads(progress_file.read_text(encoding="utf-8"))
            except Exception:
                progress = {}

        prev_remaining = progress.get("last_remaining")
        prev_tick = progress.get("last_observed_tick")
        prev_ticks = int(progress.get("ticks_without_progress", 0)) if str(progress.get("ticks_without_progress", "")).isdigit() else 0
        prev_change_at = progress.get("last_remaining_change_at") or now.isoformat()
        tick_advanced = isinstance(last_tick, str) and last_tick not in {"", "unknown"} and last_tick != prev_tick

        if isinstance(prev_remaining, int) and prev_remaining == progress_remaining:
            # Only count no-progress when the overseer tick actually advanced.
            if tick_advanced:
                ticks_without_progress = prev_ticks + 1
                last_remaining_change_at = prev_change_at
            else:
                ticks_without_progress = prev_ticks
                last_remaining_change_at = prev_change_at
        else:
            ticks_without_progress = 0
            last_remaining_change_at = now.isoformat()

        progress_file.parent.mkdir(parents=True, exist_ok=True)
        progress_file.write_text(json.dumps({
            "last_remaining": progress_remaining,
            "last_next_task": next_task,
            "last_remaining_change_at": last_remaining_change_at,
            "ticks_without_progress": ticks_without_progress,
            "last_observed_tick": last_tick,
            "updated_at": now.isoformat(),
        }, indent=2), encoding="utf-8")
    except Exception:
        ticks_without_progress = 0

    # System severity + wait guidance
    severity = "OK"
    wait_guidance = "System active. Wait up to 12m before escalation."

    warn_reasons = []
    critical_reasons = []
    if tick_age_s is None:
        warn_reasons.append("tick_age_unknown")
    elif tick_age_s > OVERSEER_WARN_SECONDS:
        warn_reasons.append("overseer_tick_overdue")

    if dispatch in {"timeout", "not_attempted"}:
        warn_reasons.append(f"dispatch_{dispatch}")

    if required_bad:
        warn_reasons.append("required_agents_unhealthy")

    if isinstance(manual_pending, int) and manual_pending > 0:
        warn_reasons.append("manual_approvals_pending")

    if ticks_without_progress >= NO_PROGRESS_TICKS:
        critical_reasons.append("no_progress_threshold_exceeded")

    if dispatch == "timeout" and ticks_without_progress >= NO_PROGRESS_TICKS:
        critical_reasons.append("timeout_plus_stagnation")

    if critical_reasons:
        severity = "CRITICAL"
        wait_guidance = "Critical stall indicators detected. Intervene now: run one manual orchestrator tick and inspect approvals/logs."
    elif warn_reasons:
        severity = "WARN"
        wait_guidance = "Potential slowdown detected. If unchanged for 15m, run one manual orchestrator tick."

    print("OpenFang Build Status")
    print("====================")
    print(f"total_tasks={total} done_like={done_like} verified={verified} merged={merged} remaining={remaining}")
    print(f"next_task={next_task} dispatch={dispatch} message_mode={message_mode} approvals_total={approvals_total} manual_review_pending={manual_pending}")
    print(f"last_tick={last_tick} age={age}")
    print("recent_events:")
    for line in recent:
        print(f"  {line}")

    print("\nAgent Health")
    print("============")
    if agent_list_error:
        print(f"agent_status=unavailable error={agent_list_error}")
    else:
        print(
            "class_counts="
            f"healthy:{class_counts['HEALTHY']} idle:{class_counts['IDLE']} stale:{class_counts['STALE']} "
            f"auth_issue:{class_counts['AUTH_ISSUE']} unready:{class_counts['UNREADY']} unknown:{class_counts['UNKNOWN']}"
        )
        for row in sorted(agent_rows, key=lambda r: r["name"]):
            age_display = f"{row['age_s']}s" if row["age_s"] is not None else "unknown"
            print(
                f"- {row['name']}: state={row['state']} ready={str(row['ready']).lower()} "
                f"auth={row['auth']} last_active_age={age_display} class={row['class']}"
            )

    print("\nSystem Health")
    print("=============")
    print(f"severity={severity}")
    print(f"ticks_without_progress={ticks_without_progress} threshold={NO_PROGRESS_TICKS}")
    if warn_reasons:
        print(f"warn_reasons={','.join(warn_reasons)}")
    if critical_reasons:
        print(f"critical_reasons={','.join(critical_reasons)}")

    print("\nOperator Guidance")
    print("================")
    print(wait_guidance)
    print(
        "thresholds="
        f"idle:{IDLE_SECONDS}s stale:{STALE_SECONDS}s overseer_warn:{OVERSEER_WARN_SECONDS}s no_progress_ticks:{NO_PROGRESS_TICKS}"
    )

except Exception as exc:
    print("OpenFang Build Status")
    print("====================")
    print(f"status=degraded error={exc}")
    print("Operator Guidance")
    print("================")
    print("Status rendering failed unexpectedly. Run `openfang health` and `openfang status`, then retry.")
PY
}

if [[ "$WATCH_MODE" -eq 1 ]]; then
  while true; do
    clear
    render
    sleep "$INTERVAL"
  done
else
  render
fi
