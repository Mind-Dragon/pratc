# Production Execution Plan — openclaw/openclaw Omni Mode

## TL;DR

> **Quick Summary**: Execute full prATC pipeline on openclaw/openclaw (5,500+ PRs) using omni mode for comprehensive analysis, clustering, graphing, and planning.
> 
> **Deliverables**:
> - Complete repository sync (cold sync ~15-20 minutes)
> - Full omni analysis with all features enabled
> - Comprehensive PDF report with charts and detailed sections
> - Web dashboard accessible at http://localhost:3000
> - CLI commands for ongoing maintenance
> 
> **Estimated Effort**: Large (5.5k PR scale)
> **Critical Path**: Auth setup → Cold sync → Omni analyze → Generate report → Verify dashboard

---

## Context

### Target Repository
- **Repository**: `openclaw/openclaw`
- **Scale**: ~5,506 open PRs (primary scale target from research)
- **Challenge**: Largest test case for prATC's combinatorial optimization

### Omni Mode Features
From AGENTS.md and architecture plan:
- **Full ML clustering**: HDBSCAN + sentence-transformers for semantic grouping
- **Conflict prediction**: File overlap + AST-based conflict detection
- **Duplicate detection**: Semantic similarity clustering (>0.90 = duplicate)
- **Graph relationships**: Dependency, conflict, and duplicate edges
- **Combinatorial planning**: Multiple modes (combination, permutation, with_replacement)

### Environment Requirements
- **Authentication**: `GH_TOKEN` required for GitHub API access
- **ML Backend**: `ML_BACKEND=local` for safe testing (sklearn/tfidf)
- **Memory**: ≤2.5 GB RSS for CLI analyze (5.5k PR scale SLO)

---

## Work Objectives

### Core Objective
Successfully execute the complete prATC pipeline on openclaw/openclaw with omni mode enabled, producing actionable insights for the 5,500+ PR repository.

### Concrete Deliverables
- `~/.pratc/pratc.db` — SQLite cache with full PR metadata
- `~/.pratc/mirrors/openclaw/openclaw/` — Git mirror directory
- Comprehensive PDF report: `openclaw-openclaw-report.pdf`
- JSON analysis output: `openclaw-openclaw-analysis.json`
- Running web dashboard: http://localhost:3000
- Performance metrics meeting SLOs (analyze ≤300s, cluster ≤180s, etc.)

### Definition of Done
- [ ] `./bin/pratc sync --repo=openclaw/openclaw` → completes successfully
- [ ] `./bin/pratc analyze --repo=openclaw/openclaw --format=json` → exits 0, valid JSON
- [ ] `./bin/pratc cluster --repo=openclaw/openclaw --format=json` → exits 0, clusters detected
- [ ] `./bin/pratc graph --repo=openclaw/openclaw --format=dot` → exits 0, non-empty DOT
- [ ] `./bin/pratc plan --repo=openclaw/openclaw --target=50 --format=json` → exits 0, plans generated
- [ ] `./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=openclaw-report.pdf` → exits 0, PDF > 100KB
- [ ] Web dashboard loads at http://localhost:3000 with data

### Must Have
- All environment variables properly configured
- Cold sync completed before analysis
- Local ML backend to avoid external API dependencies
- Performance within SLOs for 5.5k PR scale
- All omni features enabled (clustering, conflicts, duplicates, graphs)

### Must NOT Have
- External ML API calls (use local only)
- Memory leaks or crashes during processing
- Incomplete sync leading to partial analysis
- Interactive prompts during automated execution

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go tests, web tests, Docker Compose)
- **Automated tests**: NONE for production run (this is live execution)
- **Framework**: Manual verification via CLI commands and dashboard

### QA Policy
Every step includes agent-executed verification commands. Evidence saved to `.sisyphus/evidence/production-openclaw/`.

---

## Execution Strategy

```
Wave 1 (Environment Setup — 3 tasks):
├── Task 1: Configure authentication and environment
├── Task 2: Verify build and dependencies
└── Task 3: Create evidence directory

Wave 2 (Cold Sync — 2 tasks):
├── Task 4: Execute cold sync (5.5k PRs)
└── Task 5: Verify sync completion and cache state

Wave 3 (Omni Analysis — 5 tasks):
├── Task 6: Execute full analyze with omni mode
├── Task 7: Execute clustering with local ML
├── Task 8: Generate dependency/conflict graph
├── Task 9: Generate merge plans (combination mode)
└── Task 10: Verify all analysis outputs

Wave 4 (Reporting & Dashboard — 3 tasks):
├── Task 11: Generate comprehensive PDF report
├── Task 12: Start web dashboard
└── Task 13: Verify dashboard data loading

Wave 5 (Performance Validation — 2 tasks):
├── Task 14: Measure execution times against SLOs
└── Task 15: Final compliance and success verification

Final Wave (2 parallel reviews):
├── F1: Production execution audit
└── F2: Performance and quality review
```

Critical Path: T1 → T2 → T4 → T6 → T7 → T8 → T9 → T11 → T12 → T14 → F1-F2

---

## TODOs

- [ ] 1. Configure authentication and environment

  **What to do**:
  - Export `GH_TOKEN=$(gh auth token)` for GitHub API access
  - Export `GITHUB_TOKEN=$GH_TOKEN` for sync worker compatibility
  - Set `ML_BACKEND=local` to use local ML (avoid external APIs)
  - Verify tokens work: `gh api repos/openclaw/openclaw` should return repo info
  - Set performance-related env vars if needed: `PRATC_CACHE_TTL=1h`

  **Must NOT do**:
  - Don't commit tokens to any files
  - Don't use external ML backends (voyage, etc.)

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: NO (blocks all other tasks)
  - **Blocks**: All subsequent tasks
  - **Blocked By**: None

  **References**:
  - AGENTS.md: Authentication section
  - README.md: 5k PR Live-Test Runbook

  **Acceptance Criteria**:
  - [ ] `GH_TOKEN` and `GITHUB_TOKEN` exported
  - [ ] `ML_BACKEND=local` set
  - [ ] `gh api repos/openclaw/openclaw` returns valid response

  **QA Scenarios**:
  ```
  Scenario: GitHub authentication configured correctly
    Tool: Bash
    Steps:
      1. export GH_TOKEN=$(gh auth token)
      2. export GITHUB_TOKEN=$GH_TOKEN  
      3. export ML_BACKEND=local
      4. gh api repos/openclaw/openclaw | jq '.name'
    Expected Result: Returns "openclaw" without authentication errors
    Evidence: .sisyphus/evidence/production-openclaw/task-1-auth.txt
  ```

  **Commit**: NO

  ---

- [ ] 2. Verify build and dependencies

  **What to do**:
  - Run `make build` to compile latest binary
  - Run `make test-quick` to verify basic functionality
  - Check Go version: `go version` (should be compatible)
  - Check Python dependencies: `uv python list` (for ML service)
  - Verify Docker Compose profiles available

  **Must NOT do**:
  - Don't skip build verification
  - Don't proceed with outdated binary

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 3)
  - **Blocks**: Tasks 4-15
  - **Blocked By**: Task 1

  **References**:
  - Makefile: build and test targets
  - README.md: Quick Start section

  **Acceptance Criteria**:
  - [ ] `make build` completes successfully
  - [ ] `./bin/pratc --help` shows all commands
  - [ ] Quick tests pass

  **QA Scenarios**:
  ```
  Scenario: Build and dependencies verified
    Tool: Bash
    Steps:
      1. make build
      2. ./bin/pratc --help | head -10
      3. make test-quick
    Expected Result: Build succeeds, help shows commands, tests pass
    Evidence: .sisyphus/evidence/production-openclaw/task-2-build.txt
  ```

  **Commit**: NO

  ---

- [ ] 3. Create evidence directory

  **What to do**:
  - Create `.sisyphus/evidence/production-openclaw/` directory
  - This will store all verification evidence and outputs

  **Must NOT do**:
  - Don't skip evidence collection

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 2)
  - **Blocks**: Nothing
  - **Blocked By**: None

  **Acceptance Criteria**:
  - [ ] Evidence directory exists

  **QA Scenarios**:
  ```
  Scenario: Evidence directory created
    Tool: Bash
    Steps:
      1. mkdir -p .sisyphus/evidence/production-openclaw/
      2. ls -la .sisyphus/evidence/production-openclaw/
    Expected Result: Directory exists and is empty
    Evidence: .sisyphus/evidence/production-openclaw/task-3-evidence-dir.txt
  ```

  **Commit**: NO

  ---

- [ ] 4. Execute cold sync (5.5k PRs)

  **What to do**:
  - Run `./bin/pratc sync --repo=openclaw/openclaw`
  - Monitor progress (expect ~15-20 minutes for 5.5k PRs)
  - Capture output and timing
  - Handle rate limiting gracefully (built-in retry logic)

  **Must NOT do**:
  - Don't interrupt sync process
  - Don't run analysis before sync completes

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: None required

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential, blocks analysis)
  - **Blocks**: Tasks 5-15
  - **Blocked By**: Tasks 1, 2

  **References**:
  - AGENTS.md: Sync worker documentation
  - README.md: 5k PR Live-Test Runbook

  **Acceptance Criteria**:
  - [ ] Sync command exits with code 0
  - [ ] SQLite database populated (~5.5k PRs)
  - [ ] Git mirror created in `~/.pratc/mirrors/`

  **QA Scenarios**:
  ```
  Scenario: Cold sync completes successfully
    Tool: Bash
    Steps:
      1. time ./bin/pratc sync --repo=openclaw/openclaw
      2. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"
      3. ls -la ~/.pratc/mirrors/openclaw/openclaw/
    Expected Result: Sync completes, PR count ~5500, mirror directory exists
    Evidence: .sisyphus/evidence/production-openclaw/task-4-sync.txt
  ```

  **Commit**: NO

  ---

- [ ] 5. Verify sync completion and cache state

  **What to do**:
  - Query SQLite database for PR count and status
  - Verify all PR fields are populated (title, body, files, etc.)
  - Check sync job status in database
  - Validate cache freshness and completeness

  **Must NOT do**:
  - Don't proceed to analysis with incomplete sync

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after sync)
  - **Blocks**: Tasks 6-15
  - **Blocked By**: Task 4

  **References**:
  - internal/cache/sqlite.go: Schema definition
  - AGENTS.md: SQLite schema documentation

  **Acceptance Criteria**:
  - [ ] PR count matches expected (~5500)
  - [ ] All critical fields populated (no null titles/bodies)
  - [ ] Sync job marked as completed

  **QA Scenarios**:
  ```
  Scenario: Sync cache verified complete
    Tool: Bash
    Steps:
      1. sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*), COUNT(title), COUNT(body) FROM pull_requests WHERE repo='openclaw/openclaw';"
      2. sqlite3 ~/.pratc/pratc.db "SELECT status FROM sync_jobs WHERE repo='openclaw/openclaw' ORDER BY started_at DESC LIMIT 1;"
    Expected Result: Title/body counts match PR count, sync job status = 'completed'
    Evidence: .sisyphus/evidence/production-openclaw/task-5-cache-verify.txt
  ```

  **Commit**: NO

  ---

- [ ] 6. Execute full analyze with omni mode

  **What to do**:
  - Run `./bin/pratc analyze --repo=openclaw/openclaw --format=json`
  - Capture JSON output to file
  - Measure execution time against 300s SLO
  - Verify all analysis features enabled (conflicts, duplicates, overlaps, staleness)

  **Must NOT do**:
  - Don't use cached results (ensure fresh analysis)
  - Don't skip any analysis phases

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential, blocks subsequent analysis)
  - **Blocks**: Tasks 7-10
  - **Blocked By**: Task 5

  **References**:
  - AGENTS.md: Analyze command documentation
  - Architecture plan: AnalysisResponse structure

  **Acceptance Criteria**:
  - [ ] Analyze command exits 0
  - [ ] JSON output contains all expected fields (counts, clusters, duplicates, conflicts, etc.)
  - [ ] Execution time ≤ 300s

  **QA Scenarios**:
  ```
  Scenario: Full analyze completes with omni features
    Tool: Bash
    Steps:
      1. time ./bin/pratc analyze --repo=openclaw/openclaw --format=json > openclaw-analysis.json
      2. jq 'keys' openclaw-analysis.json
      3. jq '.counts.total_prs' openclaw-analysis.json
    Expected Result: Keys include counts, clusters, duplicates, conflicts, overlaps, stalenessSignals; PR count ~5500
    Evidence: .sisyphus/evidence/production-openclaw/task-6-analyze.txt
  ```

  **Commit**: NO

  ---

- [ ] 7. Execute clustering with local ML

  **What to do**:
  - Run `./bin/pratc cluster --repo=openclaw/openclaw --format=json`
  - Capture output and measure time against 180s SLO
  - Verify HDBSCAN clustering produced meaningful groups
  - Check cluster count and distribution

  **Must NOT do**:
  - Don't use external ML backends
  - Don't skip clustering (core omni feature)

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after analyze)
  - **Blocks**: Tasks 8-10
  - **Blocked By**: Task 6

  **References**:
  - AGENTS.md: ML backend configuration
  - Research plan: HDBSCAN for 5.5k PR scale

  **Acceptance Criteria**:
  - [ ] Cluster command exits 0
  - [ ] JSON output contains clusters array
  - [ ] Execution time ≤ 180s
  - [ ] Clusters detected (count > 0)

  **QA Scenarios**:
  ```
  Scenario: Clustering produces meaningful groups
    Tool: Bash
    Steps:
      1. time ./bin/pratc cluster --repo=openclaw/openclaw --format=json > openclaw-clusters.json
      2. jq '.clusters | length' openclaw-clusters.json
      3. jq '.clusters[0].pr_ids | length' openclaw-clusters.json
    Expected Result: Cluster count > 0, individual clusters contain multiple PRs
    Evidence: .sisyphus/evidence/production-openclaw/task-7-cluster.txt
  ```

  **Commit**: NO

  ---

- [ ] 8. Generate dependency/conflict graph

  **What to do**:
  - Run `./bin/pratc graph --repo=openclaw/openclaw --format=dot`
  - Capture DOT output and measure time against 120s SLO
  - Verify graph contains nodes and edges
  - Convert to JSON format for web dashboard if needed

  **Must NOT do**:
  - Don't skip graph generation (core omni feature)
  - Don't use invalid formats

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential)
  - **Blocks**: Tasks 9-10
  - **Blocked By**: Task 6

  **References**:
  - AGENTS.md: Graph command documentation
  - Architecture plan: Graph API structure

  **Acceptance Criteria**:
  - [ ] Graph command exits 0
  - [ ] DOT output contains digraph with nodes/edges
  - [ ] Execution time ≤ 120s

  **QA Scenarios**:
  ```
  Scenario: Graph generation produces valid DOT
    Tool: Bash
    Steps:
      1. time ./bin/pratc graph --repo=openclaw/openclaw --format=dot > openclaw-graph.dot
      2. head -5 openclaw-graph.dot
      3. grep -c "digraph" openclaw-graph.dot
    Expected Result: Output starts with "digraph", contains valid DOT syntax
    Evidence: .sisyphus/evidence/production-openclaw/task-8-graph.txt
  ```

  **Commit**: NO

  ---

- [ ] 9. Generate merge plans (combination mode)

  **What to do**:
  - Run `./bin/pratc plan --repo=openclaw/openclaw --target=50 --format=json`
  - Use combination mode (default) for independent PR selection
  - Capture output and measure time against 90s SLO
  - Verify plans contain ranked PR selections with scores

  **Must NOT do**:
  - Don't enable dry_run=false (keep safe dry-run mode)
  - Don't use inappropriate target numbers

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential)
  - **Blocks**: Tasks 10-15
  - **Blocked By**: Task 6

  **References**:
  - AGENTS.md: Plan command documentation
  - Architecture plan: Combinatorial modes

  **Acceptance Criteria**:
  - [ ] Plan command exits 0
  - [ ] JSON output contains plans array with 50 PRs
  - [ ] Execution time ≤ 90s
  - [ ] Plans include scores and rankings

  **QA Scenarios**:
  ```
  Scenario: Merge planning produces ranked selections
    Tool: Bash
    Steps:
      1. time ./bin/pratc plan --repo=openclaw/openclaw --target=50 --format=json > openclaw-plans.json
      2. jq '.plans[0].prs | length' openclaw-plans.json
      3. jq '.plans[0].prs[0].score' openclaw-plans.json
    Expected Result: First plan contains 50 PRs, each with score > 0
    Evidence: .sisyphus/evidence/production-openclaw/task-9-plan.txt
  ```

  **Commit**: NO

  ---

- [ ] 10. Verify all analysis outputs

  **What to do**:
  - Validate all JSON/DOT outputs are well-formed
  - Cross-check data consistency between outputs
  - Verify PR counts match across all outputs
  - Ensure no critical errors in any output

  **Must NOT do**:
  - Don't proceed to reporting with invalid outputs

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential validation)
  - **Blocks**: Tasks 11-15
  - **Blocked By**: Tasks 6-9

  **Acceptance Criteria**:
  - [ ] All outputs are valid JSON/DOT
  - [ ] PR counts consistent across outputs
  - [ ] No error fields in responses

  **QA Scenarios**:
  ```
  Scenario: All analysis outputs validated
    Tool: Bash
    Steps:
      1. jq empty openclaw-analysis.json openclaw-clusters.json openclaw-plans.json
      2. dot -Tpng -o /dev/null openclaw-graph.dot 2>/dev/null && echo "DOT valid"
    Expected Result: jq validates JSON, dot validates DOT syntax
    Evidence: .sisyphus/evidence/production-openclaw/task-10-validate.txt
  ```

  **Commit**: NO

  ---

- [ ] 11. Generate comprehensive PDF report

  **What to do**:
  - Run `./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=openclaw-report.pdf`
  - Verify PDF contains all sections (Cover, Metrics, Conflicts, Duplicates, Overlaps, Staleness, Clusters, Charts)
  - Check PDF size (> 100KB) and page count (8-12 pages)
  - Extract text to verify content

  **Must NOT do**:
  - Don't skip chart rendering
  - Don't accept minimal/placeholder reports

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (sequential after analysis)
  - **Blocks**: Tasks 12-15
  - **Blocked By**: Task 10

  **References**:
  - report-website-full.md: PDF section requirements
  - AGENTS.md: Report command documentation

  **Acceptance Criteria**:
  - [ ] Report command exits 0
  - [ ] PDF file > 100KB
  - [ ] PDF contains all expected sections
  - [ ] Charts embedded as PNG images

  **QA Scenarios**:
  ```
  Scenario: Comprehensive PDF report generated
    Tool: Bash
    Steps:
      1. ./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=openclaw-report.pdf
      2. stat -c%s openclaw-report.pdf
      3. pdftotext openclaw-report.pdf - | grep -i "conflict\|duplicate\|cluster\|staleness" | wc -l
    Expected Result: File size > 100000, text contains all section keywords
    Evidence: .sisyphus/evidence/production-openclaw/task-11-report.txt
  ```

  **Commit**: NO

  ---

- [ ] 12. Start web dashboard

  **What to do**:
  - Start web dashboard: `cd web && bun run dev`
  - Verify it connects to prATC API at http://localhost:8080
  - Access http://localhost:3000 in browser
  - Wait for data to load

  **Must NOT do**:
  - Don't start dashboard before API is ready
  - Don't skip dashboard verification

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 13)
  - **Blocks**: Task 13
  - **Blocked By**: Task 10

  **References**:
  - web/README.md: Development instructions
  - AGENTS.md: Web dashboard documentation

  **Acceptance Criteria**:
  - [ ] Web server starts without errors
  - [ ] Connects to prATC API
  - [ ] Dashboard loads at http://localhost:3000

  **QA Scenarios**:
  ```
  Scenario: Web dashboard starts and connects
    Tool: Bash
    Steps:
      1. cd web && bun run dev &
      2. sleep 10
      3. curl -sS http://localhost:3000 | grep -i "pratc\|dashboard"
    Expected Result: Dashboard HTML contains prATC branding
    Evidence: .sisyphus/evidence/production-openclaw/task-12-dashboard.txt
  ```

  **Commit**: NO

  ---

- [ ] 13. Verify dashboard data loading

  **What to do**:
  - Navigate to http://localhost:3000
  - Verify openclaw/openclaw data loads
  - Check stats cards show correct PR counts
  - Verify ChartPanel displays staleness distribution
  - Confirm Download Report button works

  **Must NOT do**:
  - Don't accept "Disconnected" or placeholder data
  - Don't skip interactive verification

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: [`playwright`]

  **Parallelization**:
  - **Can Run In Parallel**: YES (with Task 12)
  - **Blocks**: Tasks 14-15
  - **Blocked By**: Tasks 10, 12

  **References**:
  - web/src/pages/index.tsx: Dashboard structure
  - AGENTS.md: Web component patterns

  **Acceptance Criteria**:
  - [ ] Dashboard shows openclaw/openclaw data
  - [ ] Stats cards display correct counts (~5500 PRs)
  - [ ] ChartPanel renders staleness distribution
  - [ ] Download Report button functional

  **QA Scenarios**:
  ```
  Scenario: Dashboard loads real data
    Tool: Playwright
    Steps:
      1. Open http://localhost:3000?repo=openclaw/openclaw
      2. Wait for data to load (timeout: 30s)
      3. Verify page title contains "openclaw/openclaw"
      4. Verify stats card shows ~5500 PRs
      5. Verify ChartPanel is visible
    Expected Result: Real data loaded, not placeholders
    Evidence: .sisyphus/evidence/production-openclaw/task-13-dashboard-data.png
  ```

  **Commit**: NO

  ---

- [ ] 14. Measure execution times against SLOs

  **What to do**:
  - Collect timing data from all previous steps
  - Compare against performance SLOs:
    - analyze ≤ 300s
    - cluster ≤ 180s  
    - graph ≤ 120s
    - plan ≤ 90s
  - Document actual vs target performance
  - Identify any bottlenecks

  **Must NOT do**:
  - Don't ignore performance metrics
  - Don't claim success if SLOs violated

  **Recommended Agent Profile**:
  - **Category**: `quick`

  **Parallelization**:
  - **Can Run In Parallel**: NO (final validation)
  - **Blocks**: Task 15, Final Wave
  - **Blocked By**: Tasks 6-9, 11

  **References**:
  - AGENTS.md: Performance SLOs section
  - Architecture plan: Performance targets

  **Acceptance Criteria**:
  - [ ] All operations within SLOs
  - [ ] Timing data documented
  - [ ] Any violations explained

  **QA Scenarios**:
  ```
  Scenario: Performance meets SLOs
    Tool: Bash
    Steps:
      1. Parse timing data from task evidence files
      2. Compare against SLO thresholds
      3. Generate performance summary
    Expected Result: All operations within specified time limits
    Evidence: .sisyphus/evidence/production-openclaw/task-14-performance.txt
  ```

  **Commit**: NO

  ---

- [ ] 15. Final compliance and success verification

  **What to do**:
  - Verify all Definition of Done criteria met
  - Check all Must Have items present
  - Confirm all Must NOT Have items absent
  - Compile final success report
  - Archive all evidence and outputs

  **Must NOT do**:
  - Don't skip final verification
  - Don't claim success prematurely

  **Recommended Agent Profile**:
  - **Category**: `oracle`

  **Parallelization**:
  - **Can Run In Parallel**: NO (final step)
  - **Blocks**: Final Wave
  - **Blocked By**: All previous tasks

  **References**:
  - This plan's Definition of Done section
  - Original architecture and research plans

  **Acceptance Criteria**:
  - [ ] All DoD criteria verified complete
  - [ ] Success report compiled
  - [ ] Evidence archived

  **QA Scenarios**:
  ```
  Scenario: All success criteria verified
    Tool: Read + Grep
    Steps:
      1. Verify each DoD item from this plan
      2. Cross-reference with evidence files
      3. Generate final verification report
    Expected Result: All criteria met, success confirmed
    Evidence: .sisyphus/evidence/production-openclaw/task-15-final-verification.txt
  ```

  **Commit**: NO

  ---

## Final Verification Wave (MANDATORY)

- [ ] F1. **Production Execution Audit** — `oracle`
  
  Review entire execution against original plans (architecture-plan.md, pr-review-analysis.md, report-website-full.md). Verify omni mode delivered all promised features.
  
  Output: `Features Delivered [N/N] | SLOs Met [N/N] | VERDICT: SUCCESS/FAILURE`

- [ ] F2. **Performance and Quality Review** — `unspecified-high`
  
  Review performance metrics, memory usage, error rates, and output quality. Ensure production readiness.
  
  Output: `Performance [PASS/FAIL] | Quality [PASS/FAIL] | Production Ready: YES/NO`

---

## Commit Strategy

- **NO COMMITS** — This is production execution, not development
- **All outputs saved externally**: JSON files, PDF reports, evidence directory
- **Environment cleanup**: Remove temporary files, stop background processes

---

## Success Criteria

### Verification Commands
```bash
# Authentication
gh api repos/openclaw/openclaw | jq '.name'

# Sync
./bin/pratc sync --repo=openclaw/openclaw
sqlite3 ~/.pratc/pratc.db "SELECT COUNT(*) FROM pull_requests WHERE repo='openclaw/openclaw';"

# Analysis
./bin/pratc analyze --repo=openclaw/openclaw --format=json > analysis.json
./bin/pratc cluster --repo=openclaw/openclaw --format=json > clusters.json  
./bin/pratc graph --repo=openclaw/openclaw --format=dot > graph.dot
./bin/pratc plan --repo=openclaw/openclaw --target=50 --format=json > plans.json

# Reporting
./bin/pratc report --repo=openclaw/openclaw --format=pdf --output=report.pdf
stat -c%s report.pdf  # Should be > 100000

# Dashboard
curl -sS http://localhost:3000 | grep -i "openclaw"
```

### Final Checklist
- [ ] All omni features delivered (clustering, conflicts, duplicates, graphs, planning)
- [ ] Performance within SLOs for 5.5k PR scale
- [ ] Comprehensive PDF report generated (8-12 pages with charts)
- [ ] Web dashboard functional with real data
- [ ] All evidence collected and archived
- [ ] Production execution successful