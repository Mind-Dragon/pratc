#!/usr/bin/env bash
# pratc-openclaw.sh — Full prATC analysis pipeline with OpenClaw auth fallback
#
# Usage:
#   ./pratc-openclaw.sh <owner/repo> [target]
#
# Examples:
#   ./pratc-openclaw.sh opencode-ai/opencode
#   ./pratc-openclaw.sh opencode-ai/opencode 50
#
# Prerequisites:
#   - gh CLI installed and authenticated
#   - Go toolchain
#   - (optional) Python 3.11+ with uv for ML clustering

set -euo pipefail

# ── Colors ──────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*" >&2; exit 1; }

# ── Args ────────────────────────────────────────────────────────────────
REPO="${1:-}"
TARGET="${2:-20}"

if [[ -z "$REPO" ]]; then
  echo "Usage: $0 <owner/repo> [target]"
  echo ""
  echo "Runs the full prATC analysis pipeline:"
  echo "  1. Auth check (gh CLI fallback)"
  echo "  2. Build"
  echo "  3. Sync (populate cache)"
  echo "  4. Analyze"
  echo "  5. Cluster"
  echo "  6. Graph"
  echo "  7. Plan"
  echo ""
  echo "Examples:"
  echo "  $0 opencode-ai/opencode"
  echo "  $0 opencode-ai/opencode 50"
  exit 1
fi

# ── Directories ─────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/bin/pratc"
OUTPUT_DIR="$SCRIPT_DIR/.pratc-output/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTPUT_DIR"

# ── Environment ─────────────────────────────────────────────────────────
# Clear env vars to test gh CLI fallback
unset GH_TOKEN GITHUB_PAT GITHUB_TOKEN 2>/dev/null || true

export ML_BACKEND="${ML_BACKEND:-local}"

info "Repo:     $REPO"
info "Target:   $TARGET"
info "Output:   $OUTPUT_DIR"
info "ML:       $ML_BACKEND"
echo ""

# ── Step 1: Prerequisites ──────────────────────────────────────────────
info "Checking prerequisites..."

command -v go   >/dev/null 2>&1 || fail "Go not found in PATH"
command -v gh   >/dev/null 2>&1 || fail "gh CLI not found — install: https://cli.github.com/"
command -v make >/dev/null 2>&1 || warn "make not found — will build manually"

# Verify gh is authenticated
if ! gh auth status >/dev/null 2>&1; then
  fail "gh CLI not authenticated. Run: gh auth login"
fi
ok "gh CLI authenticated"

# ── Step 2: Build ───────────────────────────────────────────────────────
info "Building prATC..."
if command -v make >/dev/null 2>&1; then
  make build 2>&1 || fail "Build failed"
else
  go build -o "$BIN" ./cmd/pratc/ 2>&1 || fail "Build failed"
fi
ok "Binary: $BIN"
echo ""

# ── Step 3: Sync ────────────────────────────────────────────────────────
info "Syncing $REPO (this may take a while for large repos)..."
"$BIN" sync --repo="$REPO" 2>&1 | tee "$OUTPUT_DIR/sync.log" || warn "Sync had issues (may be partial)"
ok "Sync complete"
echo ""

# ── Step 4: Analyze ─────────────────────────────────────────────────────
info "Analyzing $REPO..."
"$BIN" analyze --repo="$REPO" --format=json 2>&1 | tee "$OUTPUT_DIR/analyze.json" || fail "Analyze failed"
TOTAL=$(jq -r '.counts.total_prs // 0' "$OUTPUT_DIR/analyze.json" 2>/dev/null || echo "?")
ok "Analyze complete — $TOTAL PRs"
echo ""

# ── Step 5: Cluster ─────────────────────────────────────────────────────
info "Clustering $REPO..."
"$BIN" cluster --repo="$REPO" --format=json 2>&1 | tee "$OUTPUT_DIR/cluster.json" || warn "Cluster failed (ML backend may not be ready)"
CLUSTERS=$(jq -r '.clusters | length // 0' "$OUTPUT_DIR/cluster.json" 2>/dev/null || echo "?")
ok "Cluster complete — $CLUSTERS clusters"
echo ""

# ── Step 6: Graph ───────────────────────────────────────────────────────
info "Generating dependency graph for $REPO..."
"$BIN" graph --repo="$REPO" --format=dot 2>&1 | tee "$OUTPUT_DIR/graph.dot" || warn "Graph failed"
ok "Graph complete"
echo ""

# ── Step 7: Plan ────────────────────────────────────────────────────────
info "Generating merge plan (target=$TARGET) for $REPO..."
"$BIN" plan --repo="$REPO" --target="$TARGET" --format=json 2>&1 | tee "$OUTPUT_DIR/plan.json" || fail "Plan failed"
SELECTED=$(jq -r '.selected | length // 0' "$OUTPUT_DIR/plan.json" 2>/dev/null || echo "?")
ok "Plan complete — $SELECTED PRs selected"
echo ""

# ── Summary ─────────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}Analysis complete for $REPO${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Results saved to: $OUTPUT_DIR/"
echo ""
ls -la "$OUTPUT_DIR/"
echo ""
echo "Quick stats:"
echo "  Total PRs:    $TOTAL"
echo "  Clusters:     $CLUSTERS"
echo "  Plan target:  $TARGET"
echo "  Selected:     $SELECTED"
echo ""
echo "View results:"
echo "  jq '.counts'           $OUTPUT_DIR/analyze.json"
echo "  jq '.clusters[]'       $OUTPUT_DIR/cluster.json"
echo "  jq '.selected'         $OUTPUT_DIR/plan.json"
echo "  cat                    $OUTPUT_DIR/graph.dot"
