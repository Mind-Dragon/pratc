#!/usr/bin/env bash
# run-tree-analysis.sh — Single script: kill old server, fix DB, start server, run analysis
# Usage: ./run-tree-analysis.sh [port] [target] [batch_size]
# Example: ./run-tree-analysis.sh 9999 20 50

set -euo pipefail

PORT="${1:-9999}"
TARGET="${2:-20}"
BATCH="${3:-50}"
REPO="openclaw/openclaw"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/bin/pratc"
OUT="$SCRIPT_DIR/.pratc-tree/$(date +%Y%m%d-%H%M%S)"

# Colors
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*" >&2; exit 1; }

echo "============================================"
echo "  prATC Tree-Mode Omni Analysis"
echo "============================================"
echo "  Port:    $PORT"
echo "  Target:  $TARGET"
echo "  Batch:   $BATCH"
echo "  Repo:    $REPO"
echo "  Output:  $OUT"
echo "============================================"
echo ""

# Check binary
[[ -x "$BIN" ]] || fail "Binary not found: $BIN (run 'make build' first)"

# Step 1: Kill existing server on this port
info "Step 1/5: Clearing port $PORT..."
EXISTING=$(lsof -ti :"$PORT" 2>/dev/null || true)
if [[ -n "$EXISTING" ]]; then
    kill -9 $EXISTING 2>/dev/null || true
    sleep 1
    # Verify
    REMAINING=$(lsof -ti :"$PORT" 2>/dev/null || true)
    if [[ -n "$REMAINING" ]]; then
        warn "Port $PORT still in use by PID $REMAINING, trying harder..."
        kill -9 $REMAINING 2>/dev/null || true
        sleep 2
    fi
fi
ok "Port $PORT is free"

# Step 2: Fix DB sync state
info "Step 2/5: Fixing cache state..."
DB="$HOME/.pratc/pratc.db"
if [[ -f "$DB" ]]; then
    sqlite3 "$DB" "UPDATE sync_progress SET last_sync_at=strftime('%Y-%m-%dT%H:%M:%SZ','now') WHERE repo='$REPO';"
    sqlite3 "$DB" "UPDATE sync_jobs SET status='completed' WHERE status='in_progress' AND repo='$REPO';"
    PR_COUNT=$(sqlite3 "$DB" "SELECT COUNT(*) FROM pull_requests WHERE repo='$REPO';")
    ok "Cache fixed ($PR_COUNT PRs in DB)"
else
    warn "DB not found at $DB — sync may be needed first"
fi

# Step 3: Start server
info "Step 3/5: Starting server on port $PORT..."
mkdir -p "$OUT"
nohup "$BIN" serve --port="$PORT" --repo="$REPO" --use-cache-first > "$OUT/server.log" 2>&1 &
SERVER_PID=$!
trap "kill $SERVER_PID 2>/dev/null || true" EXIT

# Wait for server
for i in $(seq 1 30); do
    if curl -sf "http://localhost:$PORT/healthz" > /dev/null 2>&1; then
        ok "Server ready (PID $SERVER_PID)"
        break
    fi
    if [[ $i -eq 30 ]]; then
        fail "Server failed to start. Check $OUT/server.log"
    fi
    sleep 1
done

# Step 4: Analyze
info "Step 4/5: Analyzing $REPO (this may take a few minutes)..."
ANALYZE_START=$(date +%s)
R=$(curl -sf --max-time 600 "http://localhost:$PORT/api/repos/$REPO/analyze?use_cache_first=true&max_prs=1000" 2>&1) || {
    fail "Analyze request failed. Server log: $OUT/server.log"
}
ANALYZE_END=$(date +%s)

# Check for errors
ERR=$(echo "$R" | jq -r '.error // empty' 2>/dev/null || true)
if [[ -n "$ERR" ]]; then
    MSG=$(echo "$R" | jq -r '.message // "unknown"' 2>/dev/null || true)
    fail "Analyze error: $ERR — $MSG"
fi

N=$(echo "$R" | jq -r '.counts.total_prs // 0' 2>/dev/null || echo "0")
if [[ "$N" -eq 0 ]]; then
    fail "Analyze returned 0 PRs. Response: $(echo "$R" | head -c 200)"
fi
ok "Analyzed $N PRs in $((ANALYZE_END - ANALYZE_START))s"
echo "$R" > "$OUT/analyze.json"

# Step 5: Omni batches
NUM_BATCHES=$(( (N + BATCH - 1) / BATCH ))
info "Step 5/5: Running $NUM_BATCHES omni batches..."

B=0; ACC=""
for S in $(seq 0 "$BATCH" "$((N-1))"); do
    B=$((B+1)); E=$((S+BATCH-1)); [[ $E -ge $N ]] && E=$((N-1))

    SEL=$(echo "$R" | jq -r --argjson a "$S" --argjson b "$((E+1))" \
        '.prs[$a:$b] | map(.number|tostring) | join(" OR ")')

    echo -n "  batch $B/$NUM_BATCHES ($S-$E): "

    X=$(curl -sf --max-time 30 \
        "http://localhost:$PORT/api/repos/$REPO/plan/omni?selector=${SEL}&target=${TARGET}&stage_size=50" 2>&1) || true

    if [[ -z "$X" ]]; then
        echo -e "${RED}FAIL (empty)${NC}"
        continue
    fi

    XERR=$(echo "$X" | jq -r '.error // empty' 2>/dev/null || true)
    if [[ -n "$XERR" ]]; then
        echo -e "${RED}FAIL ($XERR)${NC}"
        continue
    fi

    C=$(echo "$X" | jq '.selected | length')
    echo -e "${GREEN}${C} selected${NC}"
    echo "$X" > "$OUT/batch-$B.json"

    V=$(echo "$X" | jq -r '.selected | join(",")')
    [[ -n "$V" && -n "$ACC" ]] && ACC="$ACC,$V"
    [[ -n "$V" && -z "$ACC" ]] && ACC="$V"
done

# Final omni
echo ""
if [[ -z "$ACC" ]]; then
    warn "No PRs selected across all batches"
else
    info "Final omni on accumulated selections..."
    FSEL=$(echo "$ACC" | tr ',' '\n' | sort -nu | paste -sd" OR ")
    F=$(curl -sf --max-time 120 \
        "http://localhost:$PORT/api/repos/$REPO/plan/omni?selector=${FSEL}&target=${TARGET}&stage_size=100" 2>&1) || true

    if [[ -n "$F" ]] && ! echo "$F" | jq -e '.error' > /dev/null 2>&1; then
        echo "$F" | jq '.' > "$OUT/plan-final.json"
        FC=$(echo "$F" | jq '.selected | length')
        ok "Final: $FC PRs selected"
        echo ""
        echo "Selected PRs: $(echo "$F" | jq -r '.selected | join(", ")')"
    else
        warn "Final omni failed"
    fi
fi

echo ""
echo "============================================"
echo -e "  ${GREEN}Done!${NC} Results: $OUT/"
echo "============================================"
ls -la "$OUT/"
