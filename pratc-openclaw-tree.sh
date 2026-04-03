#!/usr/bin/env bash
# pratc-openclaw-tree.sh — Tree-mode omni analysis for openclaw/openclaw
#
# Batches analyze calls, omnis each batch, then final omni on all batches.
#
# Usage: ./pratc-openclaw-tree.sh [target] [batch_size] [api_port]
#
# Defaults: target=20, batch_size=50, api_port=8080

set -euo pipefail

REPO="${REPO:-openclaw/openclaw}"
TARGET="${1:-20}"
BATCH_SIZE="${2:-50}"
API_PORT="${3:-8080}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/bin/pratc"
OUTPUT_DIR="$SCRIPT_DIR/.pratc-tree/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTPUT_DIR"
START_TIME=$(date +%s)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[INFO]${NC}  [$(date +%H:%M:%S)] $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    [$(date +%H:%M:%S)] $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  [$(date +%H:%M:%S)] $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  [$(date +%H:%M:%S)] $*" >&2; exit 1; }

log() { echo "[$(date +%H:%M:%S)] $*" >> "$OUTPUT_DIR/run.log"; }

info "Repo:      $REPO"
info "Target:    $TARGET"
info "Batch:     $BATCH_SIZE PRs per analyze"
info "Output:    $OUTPUT_DIR"
info "API port:  $API_PORT"
echo ""

[[ -x "$BIN" ]] || fail "Binary not found: $BIN"

# =============================================================================
# Singleton Server Management
# =============================================================================

server_is_pratc() {
  local port="$1"
  local response=$(curl -sf --connect-timeout 2 "http://localhost:$port/healthz" 2>&1)
  [[ "$response" == *"version"* ]] && return 0 || return 1
}

server_is_responsive() {
  local port="$1"
  server_is_pratc "$port"
}

kill_server_on_port() {
  local port="$1"
  local pid=$(lsof -ti:"$port" 2>/dev/null || true)
  if [[ -n "$pid" ]]; then
    kill -9 "$pid" 2>/dev/null || true
    sleep 1
  fi
}

start_or_reuse_server() {
  local port="$1"
  
  if server_is_responsive "$port"; then
    warn "Server already running on port $port — reusing"
    SERVER_WAS_STARTED=false
    ok "Reusing existing server on port $port"
    return 0
  fi
  
  local existing_pid=$(lsof -ti:"$port" 2>/dev/null || true)
  if [[ -n "$existing_pid" ]]; then
    warn "Stale process on port $port (PID $existing_pid) — killing"
    kill_server_on_port "$port"
  fi
  
  info "Starting API server on port $port..."
  "$BIN" serve --port="$port" --repo="$REPO" &
  SERVER_PID=$!
  SERVER_WAS_STARTED=true
  
  local waited=0
  while [[ $waited -lt 10 ]]; do
    if server_is_responsive "$port"; then
      ok "API server started on port $port"
      return 0
    fi
    sleep 1
    waited=$((waited + 1))
    info "Waiting for server to start... ($waited/10s)"
  done
  
  fail "API server failed to start on port $port"
}

cleanup_server() {
  if [[ "$SERVER_WAS_STARTED" == "true" ]]; then
    info "Shutting down server (PID $SERVER_PID)..."
    kill $SERVER_PID 2>/dev/null || true
  else
    info "Server was pre-existing — leaving running"
  fi
}

trap 'cleanup_server' EXIT

start_or_reuse_server "$API_PORT"

# Step 1: Sync skipped (using cached PRs)
info "Step 1/5: Syncing $REPO (SKIPPED — using cached PRs)..."
CACHED_PR_COUNT=$(curl -sf "http://localhost:$API_PORT/api/repos/$REPO/sync/status" 2>/dev/null | jq -r '.pr_count // 0')
ok "Using $CACHED_PR_COUNT cached PRs (sync skipped)"
log "SYNC SKIPPED pr_count=$CACHED_PR_COUNT"
echo ""

# Step 2: Analyze with cached data
info "Step 2/5: Analyzing cached PRs..."
log "ANALYZE START"
ANALYZE_START=$(date +%s)
RESPONSE=$(curl -sf "http://localhost:$API_PORT/api/repos/$REPO/analyze?use_cache_first=true" 2>&1)
ANALYZE_END=$(date +%s)

if [[ -z "$RESPONSE" ]]; then
  fail "Analyze returned empty response"
fi

echo "$RESPONSE" > "$OUTPUT_DIR/analyze-full.json"
TOTAL=$(echo "$RESPONSE" | jq -r '.counts.total_prs // 0')
ok "Analyze complete — $TOTAL PRs in cache ($(($ANALYZE_END - $ANALYZE_START))s)"
log "ANALYZE END (took $((ANALYZE_END - $ANALYZE_START))s) total=$TOTAL"
echo ""

# Step 3: Extract all PR numbers
info "Step 3/5: Extracting PR numbers..."
ALL_PRS=$(echo "$RESPONSE" | jq -r '[.. | select(.number?) | .number] | unique | sort' 2>/dev/null)
PR_COUNT=$(echo "$ALL_PRS" | jq 'length')
if [[ "$PR_COUNT" -eq 0 ]] || [[ -z "$ALL_PRS" ]]; then
  fail "Could not extract PR numbers from analyze response"
fi
ok "Extracted $PR_COUNT unique PR numbers"
log "TOTAL_PRS=$PR_COUNT"

# Convert to comma-separated for omni API
ALL_PRS_LIST=$(echo "$ALL_PRS" | jq -r '. | join(",")')
echo "$ALL_PRS_LIST" > "$OUTPUT_DIR/all-prs.txt"
echo "PR_COUNT=$PR_COUNT" > "$OUTPUT_DIR/pr-count.txt"
echo ""

# Step 4: Omni each batch
info "Step 4/5: Running tree omni — $PR_COUNT PRs in batches of $BATCH_SIZE..."

BATCH_NUM=0
ALL_SELECTED=""
ALL_ORDERED=""
TOTAL_SELECTED=0

for START in $(seq 0 "$BATCH_SIZE" "$((PR_COUNT - 1))"); do
  BATCH_NUM=$((BATCH_NUM + 1))
  END=$((START + BATCH_SIZE - 1))
  if [[ $END -ge $PR_COUNT ]]; then END=$((PR_COUNT - 1)); fi

  # Get PR numbers for this batch (0-indexed positions)
  BATCH_PRS=$(echo "$ALL_PRS" | jq -r ".[$START:$((END + 1)) | join(',')")
  BATCH_COUNT=$(echo "$BATCH_PRS" | tr ',' '\n' | wc -l)

  info "Batch $BATCH_NUM: positions $START-$END ($BATCH_COUNT PRs)"
  log "BATCH $BATCH_NUM START pos=$START-$END count=$BATCH_COUNT"

  OMNI_START=$(date +%s)
  OMNI_RESP=$(curl -sf "http://localhost:$API_PORT/api/repos/$REPO/plan/omni?selector=${BATCH_PRS}&target=${TARGET}&stage_size=50" 2>&1)
  OMNI_END=$(date +%s)

  if [[ -z "$OMNI_RESP" ]]; then
    warn "Batch $BATCH_NUM: empty response, skipping"
    log "BATCH $BATCH_NUM EMPTY RESPONSE"
    continue
  fi

  echo "$OMNI_RESP" | jq '.' > "$OUTPUT_DIR/batch-$BATCH_NUM.json"

  BATCH_SELECTED=$(echo "$OMNI_RESP" | jq -r '.selected | join(",")')
  BATCH_ORDERING=$(echo "$OMNI_RESP" | jq -r '.ordering | join(",")')
  BATCH_SEL_COUNT=$(echo "$OMNI_RESP" | jq -r '.selected | length')

  if [[ -n "$ALL_SELECTED" ]]; then
    ALL_SELECTED="$ALL_SELECTED,$BATCH_SELECTED"
    ALL_ORDERED="$ALL_ORDERED,$BATCH_ORDERING"
  else
    ALL_SELECTED="$BATCH_SELECTED"
    ALL_ORDERED="$BATCH_ORDERING"
  fi
  TOTAL_SELECTED=$((TOTAL_SELECTED + BATCH_SEL_COUNT))

  log "BATCH $BATCH_NUM END selected=$BATCH_SEL_COUNT (took $((OMNI_END - OMNI_START))s)"
  ok "Batch $BATCH_NUM: $BATCH_SEL_COUNT selected ($(($OMNI_END - $OMNI_START))s)"
done

echo ""
info "All $BATCH_NUM batches complete"
log "ALL_BATCHES_COMPLETE total_selected=$TOTAL_SELECTED"
ok "Total selected across batches: $TOTAL_SELECTED"
echo ""

# Step 5: Final omni
info "Step 5/5: Final omni on all $(echo "$ALL_SELECTED" | tr ',' '\n' | wc -l) selected PRs..."

FINAL_START=$(date +%s)
FINAL_RESP=$(curl -sf "http://localhost:$API_PORT/api/repos/$REPO/plan/omni?selector=${ALL_SELECTED}&target=${TARGET}&stage_size=100" 2>&1)
FINAL_END=$(date +%s)

if [[ -z "$FINAL_RESP" ]]; then
  warn "Final omni failed — using last batch as final result"
  FINAL_RESP=$(cat "$OUTPUT_DIR/batch-$BATCH_NUM.json")
fi

echo "$FINAL_RESP" | jq '.' > "$OUTPUT_DIR/plan-final.json"
SELECTED_FINAL=$(echo "$FINAL_RESP" | jq -r '.selected | length')
STAGES=$(echo "$FINAL_RESP" | jq -r '.stageCount // 0')

log "FINAL_OMNI_COMPLETE selected=$SELECTED_FINAL stages=$STAGES (took $((FINAL_END - FINAL_START))s)"
ok "Final plan: $SELECTED_FINAL PRs in $STAGES stages ($(($FINAL_END - $FINAL_START))s)"
echo ""

# Step 6: Generate PDF report
info "Step 6/6: Generating PDF report..."
REPORT_OUTPUT="$OUTPUT_DIR/report.pdf"
max_retries=3
retry_count=0
report_success=false

while [[ $retry_count -lt $max_retries ]]; do
  if "$BIN" report --repo="$REPO" --input-dir="$OUTPUT_DIR" --output="$REPORT_OUTPUT" --format=pdf 2>/dev/null; then
    ok "Report generated successfully"
    log "REPORT GENERATION SUCCESS path=$REPORT_OUTPUT"
    report_success=true
    break
  else
    retry_count=$((retry_count + 1))
    if [[ $retry_count -lt $max_retries ]]; then
      warn "Report generation failed (attempt $retry_count/$max_retries), retrying..."
      log "REPORT GENERATION FAILED attempt=$retry_count retrying=true"
      sleep 2
    else
      warn "Report generation failed after $max_retries attempts - preserving other artifacts"
      log "REPORT GENERATION FAILED attempts=$max_retries preserving_artifacts=true"
    fi
  fi
done

echo ""

# Summary
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
DURATION_MIN=$((DURATION / 60))
DURATION_SEC=$((DURATION % 60))

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}Tree omni analysis complete for $REPO${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Results: $OUTPUT_DIR/"
echo ""
ls -la "$OUTPUT_DIR/" | grep -v '^total'
echo ""
echo "Quick stats:"
echo "  Total PRs analyzed: $TOTAL"
echo "  Batches run:       $BATCH_NUM (batch size $BATCH_SIZE)"
echo "  Final selected:     $SELECTED_FINAL"
echo "  Total duration:    ${DURATION_MIN}m ${DURATION_SEC}s"
if [[ "$report_success" == "true" ]]; then
  echo "  PDF report:        $REPORT_OUTPUT"
fi
echo ""
echo "View results:"
echo "  jq '.counts'           $OUTPUT_DIR/analyze-full.json"
echo "  jq '.selected'         $OUTPUT_DIR/plan-final.json"
echo "  cat                     $OUTPUT_DIR/run.log"
if [[ "$report_success" == "true" ]]; then
  echo "  PDF report:          $REPORT_OUTPUT"
fi
