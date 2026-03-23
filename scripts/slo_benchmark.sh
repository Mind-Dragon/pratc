#!/bin/bash
#
# slo_benchmark.sh — Reproducible v0.1 SLO benchmark harness
#
# Measures runtime for analyze/cluster/graph/plan commands against frozen fixtures.
# Warm-cache precondition: runs each command once before measuring to populate OS-level caches.
#
# SLO Thresholds (from AGENTS.md):
#   analyze <= 300s
#   cluster <= 180s
#   graph   <= 120s
#   plan    <=  90s
#
# Usage:
#   ./scripts/slo_benchmark.sh [--repo owner/repo] [--target 20] [--output-dir .sisyphus/evidence]
#

set -euo pipefail

REPO="${REPO:-opencode-ai/opencode}"
TARGET="${TARGET:-20}"
OUTPUT_DIR="${OUTPUT_DIR:-.sisyphus/evidence}"
FIXTURE_REPO="opencode-ai/opencode"

# SLO thresholds in seconds
SLO_ANALYZE=300
SLO_CLUSTER=180
SLO_GRAPH=120
SLO_PLAN=90

# Binary name
BINARY="./bin/pratc"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

declare -A RESULTS
declare -A PASS_FAIL
declare -A SLO

SLO[analyze]=$SLO_ANALYZE
SLO[cluster]=$SLO_CLUSTER
SLO[graph]=$SLO_GRAPH
SLO[plan]=$SLO_PLAN

function log_info() {
    echo -e "${NC}[INFO] $*"
}

function log_pass() {
    echo -e "${GREEN}[PASS] $*${NC}"
}

function log_fail() {
    echo -e "${RED}[FAIL] $*${NC}"
}

function log_warn() {
    echo -e "${YELLOW}[WARN] $*${NC}"
}

function get_slo() {
    local cmd="$1"
    case "$cmd" in
        analyze) echo "$SLO_ANALYZE" ;;
        cluster) echo "$SLO_CLUSTER" ;;
        graph) echo "$SLO_GRAPH" ;;
        plan) echo "$SLO_PLAN" ;;
        *) echo "0" ;;
    esac
}

function duration_sec() {
    local ns="$1"
    awk "BEGIN {printf \"%.3f\", $ns / 1000000000}"
}

function le_check() {
    local a="$1"
    local b="$2"
    awk "BEGIN {print ($a <= $b) ? 1 : 0}"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --repo)
            REPO="$2"
            shift 2
            ;;
        --target)
            TARGET="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

mkdir -p "$OUTPUT_DIR"

log_info "=== prATC v0.1 SLO Benchmark Harness ==="
log_info "Repository: $REPO"
log_info "Fixture repo: $FIXTURE_REPO"
log_info "Target PRs for plan: $TARGET"
log_info "Output directory: $OUTPUT_DIR"
echo ""

if [[ ! -f "$BINARY" ]]; then
    log_info "Binary not found, building..."
    make build
fi

log_info "Binary: $BINARY"
echo ""

measure_command() {
    local name="$1"
    local slo="$2"
    shift 2
    local cmd=("$@")

    log_info "--- Benchmarking: $name ---"

    log_info "Warm-cache run..."
    if ! "${cmd[@]}" > /dev/null 2>&1; then
        log_warn "Warm-cache run for $name had non-zero exit (may be expected)"
    fi

    log_info "Measured run..."
    local start_ns=$(date +%s%N)
    local exit_code=0
    local output
    output=$("${cmd[@]}" 2>&1) || exit_code=$?
    local end_ns=$(date +%s%N)

    local duration_ns=$((end_ns - start_ns))
    local duration_sec=$(duration_sec "$duration_ns")

    RESULTS["${name}_duration"]="$duration_sec"
    RESULTS["${name}_exit_code"]="$exit_code"
    RESULTS["${name}_output"]="$output"

    local slo_check=$(le_check "$duration_sec" "$slo")
    if [[ "$slo_check" -eq 1 && "$exit_code" -eq 0 ]]; then
        PASS_FAIL["$name"]="PASS"
        log_pass "$name completed in ${duration_sec}s (SLO: ${slo}s)"
    elif [[ "$exit_code" -ne 0 ]]; then
        PASS_FAIL["$name"]="FAIL"
        log_fail "$name failed with exit code $exit_code"
    else
        PASS_FAIL["$name"]="FAIL"
        log_fail "$name completed in ${duration_sec}s (SLO: ${slo}s) - EXCEEDED"
    fi

    echo ""
}

# Run benchmarks
log_info "=== WARM-CACHE BENCHMARKS ==="
echo ""

# Analyze
measure_command "analyze" "$SLO_ANALYZE" "$BINARY" analyze --repo="$FIXTURE_REPO" --format=json

# Cluster
measure_command "cluster" "$SLO_CLUSTER" "$BINARY" cluster --repo="$FIXTURE_REPO" --format=json

# Graph
measure_command "graph" "$SLO_GRAPH" "$BINARY" graph --repo="$FIXTURE_REPO" --format=dot

# Plan
measure_command "plan" "$SLO_PLAN" "$BINARY" plan --repo="$FIXTURE_REPO" --target="$TARGET" --format=json

# Generate evidence files
log_info "=== GENERATING EVIDENCE FILES ==="
echo ""

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
HOSTNAME=$(hostname)
GO_VERSION=$(go version 2>/dev/null | awk '{print $3}' || echo "unknown")

# Text evidence file
TEXT_EVIDENCE_FILE="$OUTPUT_DIR/task-13-slo-benchmarks.txt"
log_info "Writing text evidence to: $TEXT_EVIDENCE_FILE"

cat > "$TEXT_EVIDENCE_FILE" << EOF
================================================================================
prATC v0.1 SLO Benchmark Evidence
================================================================================
Timestamp:  $TIMESTAMP
Hostname:   $HOSTNAME
Repository: $REPO
Fixture:    $FIXTURE_REPO
Target PRs: $TARGET
Go Version: $GO_VERSION

--------------------------------------------------------------------------------
BENCHMARK RESULTS (Warm-Cache)
--------------------------------------------------------------------------------

Command: analyze
  Duration:  ${RESULTS[analyze_duration]}s
  SLO Limit: ${SLO_ANALYZE}s
  Exit Code: ${RESULTS[analyze_exit_code]}
  Status:    ${PASS_FAIL[analyze]}

Command: cluster
  Duration:  ${RESULTS[cluster_duration]}s
  SLO Limit: ${SLO_CLUSTER}s
  Exit Code: ${RESULTS[cluster_exit_code]}
  Status:    ${PASS_FAIL[cluster]}

Command: graph
  Duration:  ${RESULTS[graph_duration]}s
  SLO Limit: ${SLO_GRAPH}s
  Exit Code: ${RESULTS[graph_exit_code]}
  Status:    ${PASS_FAIL[graph]}

Command: plan
  Duration:  ${RESULTS[plan_duration]}s
  SLO Limit: ${SLO_PLAN}s
  Exit Code: ${RESULTS[plan_exit_code]}
  Status:    ${PASS_FAIL[plan]}

--------------------------------------------------------------------------------
SUMMARY
--------------------------------------------------------------------------------
EOF

# Add summary to text file
OVERALL_PASS="true"
for cmd in analyze cluster graph plan; do
    if [[ "${PASS_FAIL[$cmd]}" != "PASS" ]]; then
        OVERALL_PASS="false"
    fi
    echo "  $cmd: ${PASS_FAIL[$cmd]} (${RESULTS[${cmd}_duration]:-N/A}s)" >> "$TEXT_EVIDENCE_FILE"
done

echo "" >> "$TEXT_EVIDENCE_FILE"
if [[ "$OVERALL_PASS" == "true" ]]; then
    echo "OVERALL: PASS - All commands within SLO thresholds" >> "$TEXT_EVIDENCE_FILE"
else
    echo "OVERALL: FAIL - One or more commands exceeded SLO thresholds" >> "$TEXT_EVIDENCE_FILE"
fi

echo "=================================================================================" >> "$TEXT_EVIDENCE_FILE"

# JSON evidence file
JSON_EVIDENCE_FILE="$OUTPUT_DIR/task-13-slo-benchmarks.json"
log_info "Writing JSON evidence to: $JSON_EVIDENCE_FILE"

cat > "$JSON_EVIDENCE_FILE" << EOF
{
  "timestamp": "$TIMESTAMP",
  "hostname": "$HOSTNAME",
  "repository": "$REPO",
  "fixture_repository": "$FIXTURE_REPO",
  "target_prs": $TARGET,
  "go_version": "$GO_VERSION",
  "benchmark_type": "slo_warm_cache",
  "commands": {
    "analyze": {
      "duration_seconds": ${RESULTS[analyze_duration]},
      "slo_threshold_seconds": $SLO_ANALYZE,
      "exit_code": ${RESULTS[analyze_exit_code]},
      "status": "${PASS_FAIL[analyze]}"
    },
    "cluster": {
      "duration_seconds": ${RESULTS[cluster_duration]},
      "slo_threshold_seconds": $SLO_CLUSTER,
      "exit_code": ${RESULTS[cluster_exit_code]},
      "status": "${PASS_FAIL[cluster]}"
    },
    "graph": {
      "duration_seconds": ${RESULTS[graph_duration]},
      "slo_threshold_seconds": $SLO_GRAPH,
      "exit_code": ${RESULTS[graph_exit_code]},
      "status": "${PASS_FAIL[graph]}"
    },
    "plan": {
      "duration_seconds": ${RESULTS[plan_duration]},
      "slo_threshold_seconds": $SLO_PLAN,
      "exit_code": ${RESULTS[plan_exit_code]},
      "status": "${PASS_FAIL[plan]}"
    }
  },
  "overall_status": "$([ "$OVERALL_PASS" == "true" ] && echo "PASS" || echo "FAIL")"
}
EOF

# Timeout test evidence
TIMEOUT_EVIDENCE_FILE="$OUTPUT_DIR/task-13-slo-timeout.txt"
log_info "Writing timeout evidence to: $TIMEOUT_EVIDENCE_FILE"

cat > "$TIMEOUT_EVIDENCE_FILE" << EOF
================================================================================
prATC v0.1 SLO Timeout Verification
================================================================================
Timestamp:  $TIMESTAMP
Hostname:   $HOSTNAME

--------------------------------------------------------------------------------
SLO TIMEOUT THRESHOLDS
--------------------------------------------------------------------------------

  analyze:  ${SLO_ANALYZE}s (300s)
  cluster:  ${SLO_CLUSTER}s (180s)
  graph:    ${SLO_GRAPH}s (120s)
  plan:     ${SLO_PLAN}s (90s)

--------------------------------------------------------------------------------
COMMAND EXIT CODE VERIFICATION
--------------------------------------------------------------------------------

EOF

for cmd in analyze cluster graph plan; do
    exit_code="${RESULTS[${cmd}_exit_code]}"
    status="${PASS_FAIL[$cmd]}"
    duration="${RESULTS[${cmd}_duration]}s"
    echo "  $cmd: exit=$exit_code status=$status duration=$duration" >> "$TIMEOUT_EVIDENCE_FILE"
done

echo "" >> "$TIMEOUT_EVIDENCE_FILE"
echo "--------------------------------------------------------------------------------" >> "$TIMEOUT_EVIDENCE_FILE"
echo "VERIFICATION RESULT" >> "$TIMEOUT_EVIDENCE_FILE"
echo "--------------------------------------------------------------------------------" >> "$TIMEOUT_EVIDENCE_FILE"

if [[ "$OVERALL_PASS" == "true" ]]; then
    cat >> "$TIMEOUT_EVIDENCE_FILE" << EOF
PASS: All commands completed within SLO thresholds without timeout.

Commands verified:
EOF
    for cmd in analyze cluster graph plan; do
        duration="${RESULTS[${cmd}_duration]:-N/A}"
        slo="$(get_slo "$cmd")"
        echo "  - $cmd: ${duration}s <= ${slo}s" >> "$TIMEOUT_EVIDENCE_FILE"
    done
else
    cat >> "$TIMEOUT_EVIDENCE_FILE" << EOF
FAIL: One or more commands exceeded SLO thresholds or failed.

Commands requiring attention:
EOF
    for cmd in analyze cluster graph plan; do
        if [[ "${PASS_FAIL[$cmd]}" != "PASS" ]]; then
            duration="${RESULTS[${cmd}_duration]:-N/A}"
            slo="$(get_slo "$cmd")"
            echo "  - $cmd: ${duration}s > ${slo}s (${PASS_FAIL[$cmd]})" >> "$TIMEOUT_EVIDENCE_FILE"
        fi
    done
fi

echo "" >> "$TIMEOUT_EVIDENCE_FILE"
echo "=================================================================================" >> "$TIMEOUT_EVIDENCE_FILE"

log_info "Evidence files generated successfully."
echo ""

log_info "Evidence files generated successfully."
echo ""

echo "=== FINAL RESULTS ==="
echo ""
for cmd in analyze cluster graph plan; do
    slo="$(get_slo "$cmd")"
    printf "  %-10s %s (%ss limit)\n" "$cmd" "${PASS_FAIL[$cmd]:-UNKNOWN}" "$slo"
done
echo ""
if [[ "$OVERALL_PASS" == "true" ]]; then
    log_pass "All SLO benchmarks PASSED"
    exit 0
else
    log_fail "Some SLO benchmarks FAILED"
    exit 1
fi
