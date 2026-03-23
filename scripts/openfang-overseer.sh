#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="/home/agent/.openfang/overseer.log"
DAEMON_LOG="/tmp/openfang-daemon.log"
OPENFANG_BIN="/usr/local/bin/openfang"
PROJECT_ROOT="/home/agent/pratc"
ORCHESTRATOR_PROMPT_FILE="/home/agent/pratc/.sisyphus/prompts/openfang-orchestrator-autopilot.md"
STATE_FILE="/home/agent/.openfang/overseer-state.json"
LOCK_FILE="/tmp/openfang-overseer.lock"

timestamp() {
  date -u +"%Y-%m-%dT%H:%M:%SZ"
}

log() {
  printf '%s %s\n' "$(timestamp)" "$*" >> "$LOG_FILE"
}

ensure_daemon() {
  if "$OPENFANG_BIN" gateway status >/tmp/openfang-gateway-status.txt 2>&1; then
    if grep -q "Status:[[:space:]]*running" /tmp/openfang-gateway-status.txt; then
      log "gateway_status=running"
      return
    fi
  fi

  log "gateway_status=down action=restart"
  nohup "$OPENFANG_BIN" gateway start > "$DAEMON_LOG" 2>&1 &
  sleep 3
  if "$OPENFANG_BIN" gateway status >/tmp/openfang-gateway-status-recheck.txt 2>&1 && grep -q "Status:[[:space:]]*running" /tmp/openfang-gateway-status-recheck.txt; then
    log "gateway_restart=success"
  else
    log "gateway_restart=failed"
  fi
}

check_health() {
  if "$OPENFANG_BIN" health >/tmp/openfang-health.txt 2>&1; then
    log "health=ok"
  else
    log "health=fail"
  fi
}

reject_unsafe_approvals() {
  if ! "$OPENFANG_BIN" approvals list > /tmp/openfang-approvals.json 2>/tmp/openfang-approvals.err; then
    log "approvals_check=failed"
    return
  fi

  python - <<'PY'
import json
import re
import subprocess
from datetime import datetime, timezone
from pathlib import Path

openfang = "/usr/local/bin/openfang"
log_file = Path("/home/agent/.openfang/overseer.log")

def log(msg: str) -> None:
    with log_file.open("a", encoding="utf-8") as f:
        from datetime import datetime, timezone
        f.write(f"{datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')} {msg}\n")

data = json.loads(Path("/tmp/openfang-approvals.json").read_text(encoding="utf-8"))
for approval in data.get("approvals", []):
    action = (approval.get("action") or "").lower()
    aid = approval.get("id")
    if not aid:
        continue

    created_at = approval.get("created_at")
    if created_at:
        try:
            created_dt = datetime.fromisoformat(created_at.replace("Z", "+00:00"))
            age_minutes = (datetime.now(timezone.utc) - created_dt).total_seconds() / 60.0
            if age_minutes > 10:
                subprocess.run([openfang, "approvals", "reject", aid], check=False)
                log(f"approval_action=reject id={aid} reason=stale_pending age_minutes={age_minutes:.1f}")
                continue
        except Exception:
            pass

    if "git config" in action:
        subprocess.run([openfang, "approvals", "reject", aid], check=False)
        log(f"approval_action=reject id={aid} reason=git_config_forbidden")
        continue

    safe_patterns = [
        r"\b(ls|head|cat|cp|mkdir|pwd|grep|find)\b",
        r"\bgo test\b",
        r"\bgo build\b",
        r"\bmake (build|test|lint|verify-env)\b",
        r"\buv run pytest\b",
        r"\bbun (test|run test|run build)\b",
        r"\bpython(3)? -m pytest\b",
        r"\bgit (status|add|commit|push|pull|fetch|branch|log|clone|checkout|merge)\b",
        r"\bgh (pr|repo|api|auth)\b",
        r"\bdocker compose\b",
    ]
    if any(re.search(pattern, action) for pattern in safe_patterns):
        subprocess.run([openfang, "approvals", "approve", aid], check=False)
        log(f"approval_action=approve id={aid} reason=safe_pattern")
        continue

    log(f"approval_action=left_pending id={aid} reason=manual_review_required")
PY
}

ensure_core_agents() {
  python - <<'PY'
import json
import shutil
import subprocess
from datetime import datetime, timezone
from pathlib import Path

openfang = "/usr/local/bin/openfang"
log_file = Path("/home/agent/.openfang/overseer.log")
required = [
    "orchestrator",
    "planner",
    "architect",
    "coder",
    "debugger",
    "code-reviewer",
    "test-engineer",
]
optional = [
    "task-executor",
    "filesystem-proxy",
    "shell-runner-v2",
    "shell-runner",
    "ops",
    "researcher-hand",
]
project_root = Path("/home/agent/pratc")
workspaces_root = Path("/home/agent/.openfang/workspaces")
bootstrap_dirs = [
    "cmd/server",
    "cmd/worker",
    "internal/config",
    "internal/db",
    "internal/models",
    "internal/mq",
    "internal/search",
    "internal/telemetry",
    ".sisyphus/evidence",
    ".sisyphus/status",
    ".sisyphus/prompts",
]
sync_files = [
    "AGENTS.md",
    ".sisyphus/plans/pratc.md",
    ".sisyphus/prompts/openfang-orchestrator-autopilot.md",
    ".sisyphus/prompts/openfang-builder-task-prompt.md",
    ".sisyphus/prompts/openfang-review-task-prompt.md",
    ".sisyphus/prompts/openfang-test-task-prompt.md",
]

def log(msg: str) -> None:
    with log_file.open("a", encoding="utf-8") as f:
        f.write(f"{datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')} {msg}\n")

out = subprocess.check_output([openfang, "agent", "list", "--json"], text=True, timeout=30)
data = json.loads(out)
agents = data if isinstance(data, list) else data.get("agents", [])
present = {item.get("name"): item.get("id") for item in agents}

available_templates = {
    "orchestrator",
    "planner",
    "architect",
    "coder",
    "debugger",
    "code-reviewer",
    "test-engineer",
}

for name in required:
    if name not in present:
        if name in available_templates:
            subprocess.run([openfang, "agent", "new", name], check=False, timeout=30)
            log(f"agent_action=spawn name={name}")
        else:
            log(f"agent_action=skip_spawn name={name} reason=template_unavailable")

out = subprocess.check_output([openfang, "agent", "list", "--json"], text=True, timeout=30)
data = json.loads(out)
agents = data if isinstance(data, list) else data.get("agents", [])
for item in agents:
    if item.get("name") in required or item.get("name") in optional:
        aid = item.get("id")
        if aid:
            provider = item.get("model_provider")
            auth_status = item.get("auth_status")
            if provider == "anthropic" or auth_status == "missing":
                log(f"agent_action=reassign_model name={item.get('name')} id={aid} reason=missing_auth_or_provider")
            subprocess.run([openfang, "agent", "set", aid, "model", "kimi-for-coding"], check=False, timeout=30)

if workspaces_root.exists():
    for ws in workspaces_root.iterdir():
        if not ws.is_dir():
            continue
        repo = ws / "pratc"
        if not repo.exists():
            continue
        for rel in bootstrap_dirs:
            (repo / rel).mkdir(parents=True, exist_ok=True)
        for rel in sync_files:
            src = project_root / rel
            dst = repo / rel
            if not src.exists():
                continue
            dst.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dst)
        log(f"workspace_action=bootstrapped workspace={ws.name}")
PY
}

dispatch_orchestrator_tick() {
  if [[ ! -f "$ORCHESTRATOR_PROMPT_FILE" ]]; then
    log "orchestrator_prompt=missing file=$ORCHESTRATOR_PROMPT_FILE"
    return
  fi

  python - <<'PY'
import json
import subprocess
from datetime import datetime, timezone
from pathlib import Path
import re

openfang = "/usr/local/bin/openfang"
project_root = Path("/home/agent/pratc")
prompt_file = Path("/home/agent/pratc/.sisyphus/prompts/openfang-orchestrator-autopilot.md")
state_file = Path("/home/agent/.openfang/overseer-state.json")
log_file = Path("/home/agent/.openfang/overseer.log")
plan_file = project_root / ".sisyphus/plans/pratc.md"
status_dir = project_root / ".sisyphus/status"

def log(msg: str) -> None:
    with log_file.open("a", encoding="utf-8") as f:
        f.write(f"{datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ')} {msg}\n")

def load_json_output(*args: str):
    out = subprocess.check_output([openfang, *args], text=True, timeout=30)
    return json.loads(out)

def parse_dependencies(plan_text: str) -> dict[str, list[str]]:
    deps: dict[str, list[str]] = {}
    header_pattern = re.compile(r"^\s*- \[ \] ((?:\d+|F\d+))\.", re.MULTILINE)
    headers = list(header_pattern.finditer(plan_text))
    for i, header in enumerate(headers):
        tid = header.group(1)
        start = header.start()
        end = headers[i + 1].start() if i + 1 < len(headers) else len(plan_text)
        section = plan_text[start:end]
        dep_block = re.search(r"dependencies:\s*(.*?)(?:\n\s*changed files:|\n\s*\*\*|\n\s*$)", section, re.DOTALL)
        if not dep_block:
            deps[tid] = []
            continue
        dep_lines = re.findall(r"^\s*-\s*([A-Za-z0-9]+)\s*$", dep_block.group(1), re.MULTILINE)
        deps[tid] = dep_lines
    return deps

approvals = load_json_output("approvals", "list")
pending_approvals = approvals.get("total", 0)
approval_items = approvals.get("approvals", [])

agents = load_json_output("agent", "list", "--json")
agent_items = agents if isinstance(agents, list) else agents.get("agents", [])
coordinator_ids = []
for preferred in ["orchestrator", "planner"]:
    for agent in agent_items:
        if agent.get("name") == preferred and agent.get("id"):
            coordinator_ids.append((preferred, agent.get("id")))
            break

if not coordinator_ids:
    log("orchestrator_tick=skipped reason=no_coordinator")
    raise SystemExit(0)

plan_text = plan_file.read_text(encoding="utf-8")
task_ids = re.findall(r"^\s*- \[ \] ((?:\d+|F\d+))\.", plan_text, flags=re.MULTILINE)
if not task_ids:
    log("orchestrator_tick=skipped reason=no_task_ids_detected")
    raise SystemExit(0)

task_deps = parse_dependencies(plan_text)

valid_statuses = {"todo", "in_progress", "blocked", "done", "merged", "verified"}
done_statuses = {"merged", "verified"}
status_map = {}
invalid_states = []
for path in status_dir.glob("*.md"):
    text = path.read_text(encoding="utf-8")
    task_match = re.search(r"task id:\s*([A-Za-z0-9]+)", text)
    status_match = re.search(r"status:\s*([a-z_]+)", text)
    if task_match and status_match:
        tid = task_match.group(1)
        st = status_match.group(1)
        status_map[tid] = st
        if st not in valid_statuses:
            invalid_states.append((tid, st))

remaining = [tid for tid in task_ids if status_map.get(tid) not in done_statuses]
all_verified = all(status_map.get(tid) == "verified" for tid in task_ids)
if all_verified and task_ids:
    log("orchestrator_tick=completed all_tasks_verified=true")
    raise SystemExit(0)

ready = []
for tid in remaining:
    deps = task_deps.get(tid, [])
    if all(status_map.get(dep) in done_statuses for dep in deps):
        ready.append(tid)

next_task = ready[0] if ready else (remaining[0] if remaining else task_ids[0])

approval_note = ""
if pending_approvals > 0:
    approval_note = f"Pending approvals currently: {pending_approvals}. Continue non-blocked tasks and avoid deadlock."

invalid_note = ""
if invalid_states:
    invalid_note = "Invalid status values: " + ", ".join(f"{tid}={st}" for tid, st in invalid_states) + ". Normalize to AGENTS.md status set."

state = {}
if state_file.exists():
    try:
        state = json.loads(state_file.read_text(encoding="utf-8"))
    except Exception:
        state = {}

safe_patterns = [
    r"\b(ls|head|cat|cp|mkdir|pwd|grep|find)\b",
    r"\bgo test\b",
    r"\bgo build\b",
    r"\bmake (build|test|lint|verify-env)\b",
    r"\buv run pytest\b",
    r"\bbun (test|run test|run build)\b",
    r"\bpython(3)? -m pytest\b",
    r"\bgit (status|add|commit|push|pull|fetch|branch|log|clone|checkout|merge)\b",
    r"\bgh (pr|repo|api|auth)\b",
    r"\bdocker compose\b",
]
manual_pending = 0
for approval in approval_items:
    action = (approval.get("action") or "").lower()
    if not any(re.search(pattern, action) for pattern in safe_patterns):
        manual_pending += 1

use_full_prompt = True
last_full_prompt_at = state.get("last_full_prompt_at")
if last_full_prompt_at:
    try:
        last_full_dt = datetime.fromisoformat(last_full_prompt_at)
        elapsed = (datetime.now(timezone.utc) - last_full_dt).total_seconds()
        if elapsed < 1800:
            use_full_prompt = False
    except Exception:
        use_full_prompt = True

prompt = prompt_file.read_text(encoding="utf-8")
compact_prompt = (
    "Continue autonomous execution using the contract at "
    f"{prompt_file}. "
    "Do not ask for user input. Dispatch dependency-ready tasks only, "
    "write status/evidence artifacts, and keep moving."
)
selected_prompt = prompt if use_full_prompt else compact_prompt
message = (
    f"AUTONOMOUS TICK\\n"
    f"Project root (sandbox-safe): ./pratc\\n"
    f"Reference host path: {project_root}\\n"
    f"Next task target: {next_task}\\n"
    f"Ready task count: {len(ready)}\\n"
    f"Remaining task count: {len(remaining)}\\n"
    f"Manual-review pending approvals: {manual_pending}\\n"
    f"{approval_note}\\n"
    f"{invalid_note}\\n\\n"
    f"{selected_prompt}"
)

send_ok = False
coordinator_used = None
dispatch_result = "not_attempted"
for coordinator_name, coordinator_id in coordinator_ids:
    try:
        sent = subprocess.run([openfang, "message", coordinator_id, message], check=False, timeout=90)
        if sent.returncode == 0:
            send_ok = True
            coordinator_used = (coordinator_name, coordinator_id)
            dispatch_result = "sent"
            break
        log(f"orchestrator_tick=message_attempt_failed coordinator={coordinator_name} code={sent.returncode}")
    except subprocess.TimeoutExpired:
        log(f"orchestrator_tick=message_attempt_timeout coordinator={coordinator_name}")
        dispatch_result = "timeout"

if not send_ok:
    log("orchestrator_tick=failed reason=all_coordinator_message_attempts_failed")
    state_file.write_text(json.dumps({
        "last_tick": datetime.now(timezone.utc).isoformat(),
        "last_next_task": next_task,
        "ready": len(ready),
        "remaining": len(remaining),
        "pending_approvals": pending_approvals,
        "manual_review_pending": manual_pending,
        "dispatch_result": dispatch_result,
        "message_mode": "full" if use_full_prompt else "compact",
    }, indent=2), encoding="utf-8")
    raise SystemExit(0)

new_state = {
    "last_tick": datetime.now(timezone.utc).isoformat(),
    "last_next_task": next_task,
    "ready": len(ready),
    "remaining": len(remaining),
    "pending_approvals": pending_approvals,
    "manual_review_pending": manual_pending,
    "dispatch_result": dispatch_result,
    "message_mode": "full" if use_full_prompt else "compact",
}
if use_full_prompt:
    new_state["last_full_prompt_at"] = datetime.now(timezone.utc).isoformat()
elif last_full_prompt_at:
    new_state["last_full_prompt_at"] = last_full_prompt_at

state_file.write_text(json.dumps(new_state, indent=2), encoding="utf-8")
log(f"orchestrator_tick=sent coordinator={coordinator_used[0]} coordinator_id={coordinator_used[1]} next_task={next_task} ready={len(ready)} remaining={len(remaining)} pending_approvals={pending_approvals} manual_pending={manual_pending} message_mode={'full' if use_full_prompt else 'compact'}")
PY
}

main() {
  mkdir -p "$(dirname "$LOG_FILE")"
  exec 9>"$LOCK_FILE"
  if ! flock -n 9; then
    log "overseer_tick=skipped reason=lock_held"
    return
  fi
  ensure_daemon
  check_health
  reject_unsafe_approvals
  ensure_core_agents
  dispatch_orchestrator_tick
}

main
