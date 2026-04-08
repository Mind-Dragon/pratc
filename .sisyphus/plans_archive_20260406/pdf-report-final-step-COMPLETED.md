# PDF Report Final Step Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore end-to-end PDF report generation in prATC so a real PDF becomes the final artifact of `pratc-openclaw-tree.sh`.

**Architecture:** Add a first-class `pratc report` CLI command that reads existing JSON artifacts and generates a real PDF. Update `pratc-openclaw-tree.sh` to call this command as the final step with retry-then-warn failure handling.

**Tech Stack:** Go (CLI, PDF generation), bash (script integration), internal/report package (PDF writer), internal/graph (DOT data)

---

## Requirements (from Design Session)

### Confirmed Requirements
- Target workflow: `pratc-openclaw-tree.sh`
- PDF content: Full artifact bundle (summary, metrics, graphs, clusters, plan, rejections, appendices)
- Failure behavior: Retry generation, then warn while preserving prior artifacts
- Test strategy: TDD for report generation and script integration
- Architecture: Reusable `pratc report` command invoked by shell script

### Must Have
- Real graph rendering (not placeholder boxes)
- Input validation for required JSON files
- Graceful error handling with clear messages
- PDF file created at specified output path
- Retry logic in shell script (bounded retries)
- All artifacts preserved on PDF failure

### Must NOT Have
- Placeholder chart boxes counted as "done"
- Script-only business logic duplication (must be CLI command)
- Mixed responsibilities (report separate from analyze/graph/plan)
- Silent failures (must log and warn clearly)

---

## Wave 1: Foundation

### Task 1: Report command skeleton + flags - [x] **COMPLETE**

**Files:**
- Create: `cmd/pratc/report.go`
- Modify: `internal/cmd/root.go`

**Steps:**

1. Create `cmd/pratc/report.go` with:
   - `RegisterReportCommand()` function
   - Flags: `--repo`, `--input-dir`, `--output`, `--format` (pdf|json)
   - Basic command structure calling registration in root.go

2. Modify `internal/cmd/root.go`:
   - Import report command package
   - Call `RegisterReportCommand()` in `init()` section (pattern at line 1582)

3. Test: Command appears in `--help`
   ```bash
   ./bin/pratc --help | grep report
   ```

4. Test: Basic flag parsing
   ```bash
   ./bin/pratc report --help
   ```

**Acceptance Criteria:**
- `pratc --help` shows `report` subcommand
- `pratc report --help` shows expected flags
- `make build` passes

**QA Scenarios:**
- Command registration visible in help
- Flag parsing works without crash

**Commit:** NO (scaffold only)

---

### Task 2: PDF writer interface + placeholder structure - [x] **COMPLETE**

**Files:**
- Read/Modify: `internal/report/pdf.go`
- Read: `internal/report/pdf_test.go`

**Steps:**

1. Read existing `internal/report/pdf.go` to understand current structure

2. Define interface for report sections:
   - `Title() string`
   - `Content() []byte`
   - `Render(pdf *Pdf) error`

3. Implement placeholder sections that write to PDF but don't render real data

4. Ensure PDF file is created at output path with basic structure

5. Test: PDF file exists after running report command
   ```bash
   ./bin/pratc report --repo=test --input-dir=/tmp/test --output=/tmp/test.pdf
   file /tmp/test.pdf
   ```

**Acceptance Criteria:**
- PDF file created at specified output path
- PDF contains basic structure with placeholder sections
- `make build` passes

**QA Scenarios:**
- PDF file creation verified
- PDF recognized as valid type

**Commit:** NO

---

### Task 3: Input validation + error handling - [x] **COMPLETE**

**Files:**
- Create: `internal/report/validator.go`
- Modify: `cmd/pratc/report.go`

**Steps:**

1. Create `internal/report/validator.go` with:
   - `ValidateInputDir(dir string) error` - checks directory exists
   - `ValidateRequiredFiles(dir string) []string` - returns missing files
   - Required files: `step-2-analyze.json`, `step-3-cluster.json`, `step-4-graph.json`, `step-5-plan.json`

2. Modify `cmd/pratc/report.go`:
   - Call validation before report generation
   - Add error logging to `.sisyphus/debug/pratc-report-*.log`
   - Return clear error messages for missing files

3. Test: Missing files error
   ```bash
   mkdir -p /tmp/empty-input
   ./bin/pratc report --repo=test --input-dir=/tmp/empty-input --output=/tmp/test.pdf
   echo "Exit code: $?"
   ```

4. Test: Valid input directory
   ```bash
   # Create input with required files
   ./bin/pratc report --repo=test --input-dir=/tmp/valid-input --output=/tmp/test.pdf
   echo "Exit code: $?"
   ```

**Acceptance Criteria:**
- Clear error message when required JSON files missing
- Graceful handling of malformed JSON
- Logging of validation failures
- `make build` passes

**QA Scenarios:**
- Missing required files → non-zero exit with clear error
- Valid input directory → zero exit, PDF created

**Commit:** NO

---

## Wave 2: Content Assembly

### Task 4: Executive summary section - [x] **COMPLETE**

**Files:**
- Create: `internal/report/summary_section.go`
- Modify: `cmd/pratc/report.go`

**Steps:**

1. Create `internal/report/summary_section.go` with:
   - `SummarySection` struct implementing section interface
   - Read `step-2-analyze.json` and `step-5-plan.json`
   - Extract: total PRs, clusters, conflicts, selected PRs, target

2. Render executive summary with:
   - Repository name and run metadata (timestamp, target, max PRs)
   - Summary of analysis results
   - High-level merge plan overview

3. Wire section into report pipeline in `cmd/pratc/report.go`

4. Test: Executive summary appears in PDF
   ```bash
   ./bin/pratc report --repo=openclaw/openclaw --input-dir=/tmp/valid --output=/tmp/test.pdf
   # Inspect PDF for summary section
   ```

**Acceptance Criteria:**
- Executive summary section rendered in PDF
- Contains repository name, timestamps, key metrics
- Data matches source JSON files
- `make build` passes

**QA Scenarios:**
- Summary section present with correct data
- Metrics match source JSON values

**Commit:** NO

---

### Task 5: Metrics dashboard section - [x] **COMPLETE**

**Files:**
- Create: `internal/report/metrics_section.go`

**Steps:**

1. Create `internal/report/metrics_section.go` with:
   - `MetricsSection` struct
   - Extract metrics from all JSON artifacts:
     - Total PRs, clusters, conflicts, overlaps, duplicates (from analyze.json)
     - Graph nodes and edges (from graph.json)
     - Plan statistics: selected vs rejections (from plan.json)

2. Create visual dashboard layout with key metrics displayed prominently

3. Render to PDF with proper formatting

4. Test: Metrics dashboard appears with correct values

**Acceptance Criteria:**
- Metrics dashboard section rendered
- All key metrics displayed with correct values
- Clean, readable layout
- `make build` passes

**QA Scenarios:**
- All metrics present and accurate
- Layout is readable

**Commit:** NO

---

### Task 6: Graph rendering section (real charts) - [x] **COMPLETE**

**Files:**
- Create: `internal/report/graph_section.go`
- Read: `internal/graph/graph.go`

**Steps:**

1. Create `internal/report/graph_section.go` with:
   - `GraphSection` struct
   - Read `step-4-graph.json` for dependency/conflict graph data
   - Implement actual graph rendering (NOT placeholder boxes)

2. Graph rendering options (implement in priority order):
   - Option A: Convert DOT to image using `dot` command line tool
   - Option B: Simple text-based graph visualization (fallback)
   - Option C: Basic SVG rendering

3. Implement fallback logic: if external tools unavailable, use text visualization

4. Render graph section to PDF with proper formatting

5. Test: Graph section shows actual structure
   ```bash
   ./bin/pratc report --repo=openclaw/openclaw --input-dir=/tmp/valid --output=/tmp/test.pdf
   # Inspect PDF for real graph, not empty boxes
   ```

**Acceptance Criteria:**
- Graph section rendered with actual structure
- No placeholder boxes used
- Fallback to text visualization if needed
- `make build` passes

**QA Scenarios:**
- Graph displays dependency/conflict relationships
- Fallback works when dot unavailable

**Commit:** NO

---

### Task 7: Cluster analysis section - [x] **COMPLETE**

**Files:**
- Create: `internal/report/cluster_section.go`

**Steps:**

1. Create `internal/report/cluster_section.go` with:
   - `ClusterSection` struct
   - Read `step-3-cluster.json` for cluster data
   - Extract: cluster IDs, PR assignments, cluster characteristics

2. Create cluster analysis section with:
   - Summary of number of clusters
   - List of clusters with their PR members
   - Cluster metadata (size, characteristics)

3. Render to PDF with proper formatting

4. Test: Cluster analysis appears with correct data

**Acceptance Criteria:**
- Cluster analysis section rendered
- All clusters listed with PR members
- Data matches source JSON file
- `make build` passes

**QA Scenarios:**
- Complete cluster listing present
- PR assignments are correct

**Commit:** NO

---

### Task 8: Merge plan section - [x] **COMPLETE**

**Files:**
- Create: `internal/report/plan_section.go`

**Steps:**

1. Create `internal/report/plan_section.go` with:
   - `PlanSection` struct
   - Read `step-5-plan.json` for merge plan data
   - Extract: selected PRs, ordering, rejection reasons

2. Create merge plan section with:
   - List of selected PRs with ordering
   - Rejection list with reasons
   - Plan metadata (target, strategy)

3. Render to PDF with proper formatting

4. Test: Merge plan appears with correct data

**Acceptance Criteria:**
- Merge plan section rendered
- Selected PRs listed with ordering
- Rejections listed with reasons
- Data matches source JSON file
- `make build` passes

**QA Scenarios:**
- Complete merge plan present
- Rejection reasons included

**Commit:** YES
- Message: `feat(report): add PDF report generation command`
- Files: `cmd/pratc/report.go`, `internal/report/*.go`
- Pre-commit: `make build && make test`

---

## Wave 3: Integration

### Task 9: Shell script integration + retry logic - [x] **COMPLETE**

**Files:**
- Modify: `pratc-openclaw-tree.sh`

**Steps:**

1. Modify `pratc-openclaw-tree.sh`:
   - Add Step 7: Report generation (after summary.txt step)
   - Call: `./bin/pratc report --repo="$REPO" --input-dir="$OUTPUT_DIR" --output="$OUTPUT_DIR/report.pdf"`
   - Implement retry logic:
     ```bash
     max_retries=3
     retry_count=0
     while [[ $retry_count -lt $max_retries ]]; do
       if ./bin/pratc report ...; then
         ok "Report generated successfully"
         break
       else
         retry_count=$((retry_count + 1))
         if [[ $retry_count -lt $max_retries ]]; then
           warn "Report generation failed (attempt $retry_count/$max_retries), retrying..."
           sleep 2
         else
           warn "Report generation failed after $max_retries attempts - preserving other artifacts"
         fi
       fi
     done
     ```
   - Update final summary to include PDF output location

2. Test: Successful PDF generation
   ```bash
   ./pratc-openclaw-tree.sh --skip-sync
   ls -la .pratc-tree/*/report.pdf
   ```

3. Test: Failure handling
   - Inject failure condition
   - Verify script retries and warns
   - Verify existing artifacts preserved

**Acceptance Criteria:**
- Report step added to workflow
- Retry logic implemented (3 retries)
- Graceful failure handling
- PDF output location in final summary
- `make build` passes

**QA Scenarios:**
- Successful PDF generation
- Failure handling preserves artifacts

**Commit:** YES
- Message: `feat(report): integrate PDF generation into tree workflow`
- Files: `pratc-openclaw-tree.sh`
- Pre-commit: `make build`

---

## Wave FINAL: Verification

### F1. Plan compliance audit — `oracle`

Read the plan end-to-end. For each "Must Have": verify implementation exists. For each "Must NOT Have": search for forbidden patterns. Check evidence files exist.

**Output:** `Must Have [N/N] | Must NOT Have [N/N] | Tasks [N/N] | VERDICT: APPROVE/REJECT`

---

### F2. Code quality review

Run `make build` + `make test` + `go vet ./...`. Review all changed files for issues.

**Output:** `Build [PASS/FAIL] | Lint [PASS/FAIL] | Tests [N pass/N fail] | VERDICT: APPROVE/REJECT`

---

### F3. Real manual QA

Execute EVERY QA scenario from EVERY task. Save to `.sisyphus/evidence/final-qa/`.

**Output:** `Scenarios [N/N pass] | VERDICT: APPROVE/REJECT`

---

### F4. Scope fidelity check — `deep`

For each task: verify implementation matches spec. Check "Must NOT do" compliance.

**Output:** `Tasks [N/N compliant] | VERDICT: APPROVE/REJECT`

---

## Success Criteria

### Build Verification
```bash
make build  # Expected: Build succeeds
make test   # Expected: All tests pass
```

### Command Verification
```bash
./bin/pratc --help           # Expected: Shows report subcommand
./bin/pratc report --help    # Expected: Shows flags
```

### Functional Verification
```bash
./pratc-openclaw-tree.sh     # Expected: Runs full workflow including PDF
ls .pratc-tree/*/report.pdf  # Expected: PDF file exists
```

### Final Checklist
- [ ] All "Must Have" present (report command, PDF generation, shell integration)
- [ ] All "Must NOT Have" absent (no placeholder charts, no script-only logic)
- [ ] All tests pass
- [ ] PDF output is real content, not placeholder boxes
- [ ] Retry-then-warn behavior implemented
- [ ] Existing artifacts preserved on failure

---

## Final Verification Wave — Checklist

- [x] **F1**: Plan compliance — APPROVE
- [x] **F2**: Code quality — APPROVE
- [x] **F3**: Manual QA — APPROVE
- [x] **F4**: Scope fidelity — APPROVE

**Status**: COMPLETED
