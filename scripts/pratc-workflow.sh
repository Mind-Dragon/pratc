#!/usr/bin/env bash
set -euo pipefail

REPO="${1:-}"
if [[ -z "$REPO" ]]; then
  echo "usage: $0 owner/repo [bin_path]" >&2
  exit 2
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="${2:-$SCRIPT_DIR/../pratc}"
if [[ ! -x "$BIN" ]]; then
  if [[ -x "$SCRIPT_DIR/../bin/pratc" ]]; then
    BIN="$SCRIPT_DIR/../bin/pratc"
  else
    echo "pratc binary not found: $BIN" >&2
    exit 1
  fi
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) is required for this workflow" >&2
  exit 1
fi
if ! gh auth status -h github.com >/dev/null 2>&1; then
  echo "GitHub auth is required. Run 'gh auth login' first." >&2
  exit 1
fi

OUT_DIR="${WORKFLOW_OUT_DIR:-$SCRIPT_DIR/.pratc-workflow/$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$OUT_DIR"

RAW_LOG="$OUT_DIR/workflow.log"

printf '[%s] Repo: %s\n' "$(date +%H:%M:%S)" "$REPO" >&2
printf '[%s] Output: %s\n' "$(date +%H:%M:%S)" "$OUT_DIR" >&2
printf '[%s] Starting prATC workflow\n' "$(date +%H:%M:%S)" >&2

"$BIN" workflow --repo "$REPO" --out-dir "$OUT_DIR" --progress 2>&1 | tee "$RAW_LOG"
