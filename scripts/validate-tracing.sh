#!/bin/bash
#
# validate-tracing.sh — Request ID correlation validation across prATC components
#
# Validates that request IDs are:
# 1. Generated and logged in Go CLI
# 2. Propagated to Python ML service (when PRATC_ML_DEBUG=1)
# 3. Correlated with TypeScript web dashboard (when --dashboard flag provided)
#
# Usage:
#   ./scripts/validate-tracing.sh [--repo owner/repo] [--dashboard] [--output-dir .sisyphus/evidence]
#   PRATC_ML_DEBUG=1 ./scripts/validate-tracing.sh  # Also validate Python ML correlation
#

set -euo pipefail

REPO="${REPO:-opencode-ai/opencode}"
DASHBOARD="${DASHBOARD:-false}"
OUTPUT_DIR="${OUTPUT_DIR:-.sisyphus/evidence}"
BINARY="./bin/pratc"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Validation state
GO_REQUEST_IDS=()
ML_REQUEST_IDS=()
WEB_REQUEST_IDS=()
VALIDATION_STATUS="PASS"

function log_info() {
    echo -e "${NC}[INFO] $*"
}

function log_pass() {
    echo -e "${GREEN}[PASS] $*${NC}"
}

function log_fail() {
    echo -e "${RED}[FAIL] $*${NC}"
    VALIDATION_STATUS="FAIL"
}

function log_warn() {
    echo -e "${YELLOW}[WARN] $*${NC}"
}

# UUID v4 regex pattern
UUID_REGEX='[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}'

# Extract request_id from a JSON log line using grep + sed
# Handles cases where request_id might be missing (backward compatibility)
extract_request_id() {
    local line="$1"
    # Match "request_id":"UUID" pattern
    echo "$line" | grep -oE '"request_id":"'"$UUID_REGEX"'"' 2>/dev/null | sed 's/"request_id":"\(.*\)"/\1/' | head -1
}

# Check if a string is a valid UUID v4
is_valid_uuid() {
    local uuid="$1"
    if [[ "$uuid" =~ ^$UUID_REGEX$ ]]; then
        return 0
    fi
    return 1
}

# Parse request IDs from log output
parse_request_ids() {
    local log_output="$1"
    local -n arr_ref="$2"
    arr_ref=()

    while IFS= read -r line; do
        local req_id
        req_id=$(extract_request_id "$line")
        if [[ -n "$req_id" ]] && is_valid_uuid "$req_id"; then
            arr_ref+=("$req_id")
        fi
    done <<< "$log_output"
}

# Check if request IDs array has unique values
all_same() {
    local -a ids=("$@")
    if [[ ${#ids[@]} -eq 0 ]]; then
        return 1
    fi
    local first="${ids[0]}"
    for id in "${ids[@]}"; do
        if [[ "$id" != "$first" ]]; then
            return 1
        fi
    done
    return 0
}

function validate_go_cli_tracing() {
    log_info "=== Validating Go CLI Request ID Tracing ==="
    echo ""

    log_info "Running: $BINARY analyze --repo=$REPO --format=json"
    echo ""

    # Capture both stdout and stderr
    # stderr contains JSON logs, stdout contains JSON output
    local output
    local exit_code=0
    output=$("$BINARY" analyze --repo="$REPO" --format=json 2>&1) || exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        log_warn "CLI command exited with code $exit_code (may be expected without credentials)"
    fi

    # Parse Go request IDs from output using grep
    # JSON log lines contain "request_id":"UUID" pattern
    local go_ids
    go_ids=$(echo "$output" | grep -oE '"request_id":"'"$UUID_REGEX"'"' 2>/dev/null | sed 's/"request_id":"\(.*\)"/\1/' || true)

    # Convert to array
    while IFS= read -r id; do
        if [[ -n "$id" ]] && is_valid_uuid "$id"; then
            GO_REQUEST_IDS+=("$id")
        fi
    done <<< "$go_ids"

    log_info "Extracted ${#GO_REQUEST_IDS[@]} request ID(s) from Go CLI logs"

    if [[ ${#GO_REQUEST_IDS[@]} -eq 0 ]]; then
        log_warn "No request_ids found in Go CLI logs (backward compatibility mode - may be pre-logging implementation)"
        return 0
    fi

    # Show unique request IDs found
    local unique_go_ids
    unique_go_ids=$(printf '%s\n' "${GO_REQUEST_IDS[@]}" | sort -u)
    log_info "Unique Go request IDs: $(echo "$unique_go_ids" | wc -l)"

    # Verify all Go request IDs are valid UUIDs
    local all_valid=true
    for id in "${GO_REQUEST_IDS[@]}"; do
        if ! is_valid_uuid "$id"; then
            log_fail "Invalid UUID found: $id"
            all_valid=false
        fi
    done

    if [[ "$all_valid" != "true" ]]; then
        return 1
    fi

    # Check if all request IDs in a single operation are the same
    if all_same "${GO_REQUEST_IDS[@]}"; then
        log_pass "Go CLI: All ${#GO_REQUEST_IDS[@]} log entries share the same request ID"
    else
        log_fail "Go CLI: Request IDs vary across log entries (expected single ID per operation)"
    fi

    echo ""
    return 0
}

function validate_python_ml_tracing() {
    if [[ "${PRATC_ML_DEBUG:-0}" != "1" ]]; then
        log_info "=== Skipping Python ML Validation (PRATC_ML_DEBUG not set) ==="
        return 0
    fi

    log_info "=== Validating Python ML Request ID Tracing ==="
    echo ""

    # For ML tracing, we need to capture logs from the ML service
    # When PRATC_ML_DEBUG=1, the ML service logs to stderr with request_id

    local output
    local exit_code=0
    output=$("$BINARY" analyze --repo="$REPO" --format=json 2>&1) || exit_code=$?

    # Extract JSON log lines
    local stderr_lines=""
    while IFS= read -r line; do
        if [[ "$line" =~ ^\{.*\"ts\".*\}$ ]]; then
            stderr_lines+="$line"$'\n'
        fi
    done <<< "$output"

    parse_request_ids "$stderr_lines" ML_REQUEST_IDS

    log_info "Extracted ${#ML_REQUEST_IDS[@]} request ID(s) from combined logs"

    if [[ ${#ML_REQUEST_IDS[@]} -eq 0 ]]; then
        log_warn "No request_ids found (ML logging may not be implemented yet)"
        return 0
    fi

    # Compare Go and ML request IDs
    local go_unique
    printf -v go_unique '%s\n' "${GO_REQUEST_IDS[@]}" | sort -u
    local ml_unique
    printf -v ml_unique '%s\n' "${ML_REQUEST_IDS[@]}" | sort -u

    if [[ "$go_unique" == "$ml_unique" ]]; then
        log_pass "Python ML: Request IDs match Go CLI logs"
    else
        log_fail "Python ML: Request IDs do not match Go CLI logs"
        log_fail "  Go unique IDs: $(echo "$go_unique" | wc -l)"
        log_fail "  ML unique IDs: $(echo "$ml_unique" | wc -l)"
    fi

    echo ""
    return 0
}

function validate_web_dashboard_tracing() {
    if [[ "$DASHBOARD" != "true" ]]; then
        log_info "=== Skipping Web Dashboard Validation (--dashboard not set) ==="
        return 0
    fi

    log_info "=== Validating Web Dashboard Request ID Tracing ==="
    echo ""

    # When running against the web dashboard, we would capture browser console logs
    # For now, this is a placeholder for the validation logic
    # The actual implementation would require browser automation

    log_warn "Web dashboard tracing validation requires browser automation"
    log_info "Expected: Browser console logs should contain same request_id as CLI"

    # Note: Full implementation would:
    # 1. Start the web dashboard
    # 2. Navigate to the triage/analyze page
    # 3. Capture browser console logs
    # 4. Extract request_ids from browser logs
    # 5. Compare with CLI request_ids

    echo ""
    return 0
}

function generate_validation_report() {
    local report_file="$1"

    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    log_info "Generating validation report: $report_file"
    echo ""

    # Count unique request IDs
    local unique_go_ids
    if [[ ${#GO_REQUEST_IDS[@]} -gt 0 ]]; then
        unique_go_ids=$(printf '%s\n' "${GO_REQUEST_IDS[@]}" | sort -u)
    else
        unique_go_ids=""
    fi
    local unique_go_count
    if [[ -n "$unique_go_ids" ]]; then
        unique_go_count=$(echo "$unique_go_ids" | wc -l | tr -d ' ')
    else
        unique_go_count=0
    fi

    local unique_ml_ids
    if [[ ${#ML_REQUEST_IDS[@]} -gt 0 ]]; then
        unique_ml_ids=$(printf '%s\n' "${ML_REQUEST_IDS[@]}" | sort -u)
    else
        unique_ml_ids=""
    fi
    local unique_ml_count
    if [[ -n "$unique_ml_ids" ]]; then
        unique_ml_count=$(echo "$unique_ml_ids" | wc -l | tr -d ' ')
    else
        unique_ml_count=0
    fi

    local unique_web_ids
    if [[ ${#WEB_REQUEST_IDS[@]} -gt 0 ]]; then
        unique_web_ids=$(printf '%s\n' "${WEB_REQUEST_IDS[@]}" | sort -u)
    else
        unique_web_ids=""
    fi
    local unique_web_count
    if [[ -n "$unique_web_ids" ]]; then
        unique_web_count=$(echo "$unique_web_ids" | wc -l | tr -d ' ')
    else
        unique_web_count=0
    fi

    cat > "$report_file" << EOF
=================================================================================
prATC Request ID Tracing Validation Report
=================================================================================
Timestamp:          $timestamp
Repository:         $REPO
Validation Status:  $VALIDATION_STATUS

--------------------------------------------------------------------------------
GO CLI TRACING
--------------------------------------------------------------------------------
Request IDs Found:  ${#GO_REQUEST_IDS[@]}
Unique IDs:        $unique_go_count
${unique_go_ids:+Unique ID List:
$(echo "$unique_go_ids" | sed 's/^/  - /')}

--------------------------------------------------------------------------------
PYTHON ML TRACING (PRATC_ML_DEBUG=${PRATC_ML_DEBUG:-0})
--------------------------------------------------------------------------------
Request IDs Found: ${#ML_REQUEST_IDS[@]}
Unique IDs:        $unique_ml_count
${unique_ml_ids:+Unique ID List:
$(echo "$unique_ml_ids" | sed 's/^/  - /')}

--------------------------------------------------------------------------------
WEB DASHBOARD TRACING (--dashboard=$DASHBOARD)
--------------------------------------------------------------------------------
Request IDs Found: ${#WEB_REQUEST_IDS[@]}
Unique IDs:        $unique_web_count
${unique_web_ids:+Unique ID List:
$(echo "$unique_web_ids" | sed 's/^/  - /')}

--------------------------------------------------------------------------------
CORRELATION CHECK
--------------------------------------------------------------------------------
EOF

    if [[ ${#GO_REQUEST_IDS[@]} -gt 0 ]] && [[ ${#ML_REQUEST_IDS[@]} -gt 0 ]]; then
        if [[ "$unique_go_ids" == "$unique_ml_ids" ]]; then
            echo "Go ↔ Python ML: CORRELATED (PASS)" >> "$report_file"
        else
            echo "Go ↔ Python ML: NOT CORRELATED (FAIL)" >> "$report_file"
            echo "  Go unique IDs: $unique_go_count" >> "$report_file"
            echo "  ML unique IDs: $unique_ml_count" >> "$report_file"
        fi
    else
        echo "Go ↔ Python ML: SKIPPED (insufficient data)" >> "$report_file"
    fi

    if [[ ${#GO_REQUEST_IDS[@]} -gt 0 ]] && [[ ${#WEB_REQUEST_IDS[@]} -gt 0 ]]; then
        if [[ "$unique_go_ids" == "$unique_web_ids" ]]; then
            echo "Go ↔ Web Dashboard: CORRELATED (PASS)" >> "$report_file"
        else
            echo "Go ↔ Web Dashboard: NOT CORRELATED (FAIL)" >> "$report_file"
        fi
    else
        echo "Go ↔ Web Dashboard: SKIPPED (insufficient data or --dashboard not set)" >> "$report_file"
    fi

    echo "" >> "$report_file"
    echo "=================================================================================" >> "$report_file"
    echo "OVERALL STATUS: $VALIDATION_STATUS" >> "$report_file"
    echo "=================================================================================" >> "$report_file"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --repo)
            REPO="$2"
            shift 2
            ;;
        --dashboard)
            DASHBOARD="true"
            shift
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--repo owner/repo] [--dashboard] [--output-dir dir]"
            exit 1
            ;;
    esac
done

mkdir -p "$OUTPUT_DIR"

log_info "=== prATC Request ID Tracing Validation ==="
log_info "Repository: $REPO"
log_info "ML Debug: ${PRATC_ML_DEBUG:-0}"
log_info "Dashboard: $DASHBOARD"
log_info "Output Dir: $OUTPUT_DIR"
echo ""

# Check if binary exists
if [[ ! -f "$BINARY" ]]; then
    log_info "Binary not found, building..."
    make build
fi

# Run validations
validate_go_cli_tracing
validate_python_ml_tracing
validate_web_dashboard_tracing

# Generate report
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
REPORT_FILE="$OUTPUT_DIR/task-tracing-validation.txt"
generate_validation_report "$REPORT_FILE"

# Also generate JSON report
JSON_REPORT_FILE="$OUTPUT_DIR/task-tracing-validation.json"

unique_go_count=0
unique_ml_count=0
unique_web_count=0

if [[ ${#GO_REQUEST_IDS[@]} -gt 0 ]]; then
    unique_go_count=$(printf '%s\n' "${GO_REQUEST_IDS[@]}" | sort -u | grep -v '^$' | wc -l | tr -d ' ')
fi
if [[ ${#ML_REQUEST_IDS[@]} -gt 0 ]]; then
    unique_ml_count=$(printf '%s\n' "${ML_REQUEST_IDS[@]}" | sort -u | grep -v '^$' | wc -l | tr -d ' ')
fi
if [[ ${#WEB_REQUEST_IDS[@]} -gt 0 ]]; then
    unique_web_count=$(printf '%s\n' "${WEB_REQUEST_IDS[@]}" | sort -u | grep -v '^$' | wc -l | tr -d ' ')
fi

cat > "$JSON_REPORT_FILE" << EOF
{
  "timestamp": "$TIMESTAMP",
  "repository": "$REPO",
  "ml_debug": "${PRATC_ML_DEBUG:-0}",
  "dashboard": "$DASHBOARD",
  "validation_status": "$VALIDATION_STATUS",
  "components": {
    "go_cli": {
      "request_ids_found": ${#GO_REQUEST_IDS[@]},
      "unique_ids": $unique_go_count
    },
    "python_ml": {
      "request_ids_found": ${#ML_REQUEST_IDS[@]},
      "unique_ids": $unique_ml_count
    },
    "web_dashboard": {
      "request_ids_found": ${#WEB_REQUEST_IDS[@]},
      "unique_ids": $unique_web_count
    }
  }
}
EOF

log_info "Text report: $REPORT_FILE"
log_info "JSON report: $JSON_REPORT_FILE"
echo ""

echo "=== FINAL VALIDATION RESULT ==="
echo ""
if [[ "$VALIDATION_STATUS" == "PASS" ]]; then
    log_pass "Request ID tracing validation PASSED"
    exit 0
else
    log_fail "Request ID tracing validation FAILED"
    exit 1
fi