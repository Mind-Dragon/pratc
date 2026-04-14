# Roadmap

prATC development roadmap for versions 1.4 through 1.6.

## Version 1.4 — Planning Integration (Q2 2026)

### Goal
Wire up the sophisticated planning algorithms in `internal/planning/` to production, replacing the current simple scoring approach.

### Deliverables

#### PoolSelector Integration
- Weighted multi-component priority scoring (5 components, weights sum to 1.0)
  - Staleness (0.30): Anti-starvation for old PRs
  - CI Status (0.25): Prefer green CI
  - Security Labels (0.20): Elevate security PRs
  - Cluster Coherence (0.15): Batch similar PRs
  - Time Decay (0.10): Recency weighting
- Settings integration: `PriorityWeights.ToSettings()` / `FromSettings()`
- Explainable scoring: reason codes for each component

#### HierarchicalPlanner
- 3-level planning reducing complexity from O(C(n,k)) to O(C(clusters,c) × C(avg,s))
  - Level 1: Select clusters by score
  - Level 2: Rank PRs within clusters
  - Level 3: Topological sort with dependency ordering
- Configurable via `HierarchicalConfig`
- Dependency ordering toggle (currently hardcoded to true)

#### PairwiseExecutor
- Sharded parallel conflict detection with worker pool
- Early exit on first conflict found
- Shard metrics for performance monitoring

#### TimeDecayWindow
- Exponential decay: `score = e^(-ln(2) × ageHours / halfLifeHours)`
- Protected lane for security/urgent PRs (bypass decay after `ProtectedHours`)
- Old critical PRs get `MinScore` floor to prevent starvation

### Decision Point
If planning integration proves too complex or provides insufficient benefit over current scoring, delete `internal/planning/` before v1.4 release to reduce maintenance burden.

### Success Metrics
- Planning time for 5500 PRs: <90s (current: ~180s)
- Merge plan quality: 20% reduction in conflict detection post-merge
- User satisfaction: operators report better prioritization

---

## Version 1.5 — Dashboard Enhancements (Q3 2026)

### Goal
Transform the web dashboard from a monitoring tool into a full PR triage and merge execution interface.

### Deliverables

#### Triage Inbox
- Filterable PR list with analyzer findings displayed inline
- Bulk actions: approve for merge, request changes, mark as duplicate
- Keyboard shortcuts for rapid triage (vim-style navigation)

#### Merge Execution
- **Read-only merge planning** (no auto-merge)
- Generate merge instructions with conflict warnings
- Copy-to-clipboard merge commands for operators
- Integration with GitHub CLI for one-click merge execution

#### Real-Time Collaboration
- WebSocket-based live updates for multi-operator workflows
- Operator presence indicators ("Alice is reviewing PR #123")
- Comment threads on PRs (stored locally, not pushed to GitHub)

#### Mobile Responsiveness
- Full-featured mobile interface for on-call operators
- Push notifications for high-priority PRs (via Telegram integration)
- Offline mode with local cache

### Guardrails
- No auto-merge or auto-approve behavior (read-only planning only)
- All merge execution requires explicit operator confirmation
- No GitHub App or webhook expansion (manual trigger only)

### Success Metrics
- Triage time per PR: <30 seconds (current: ~2 minutes)
- Operator adoption: 80% of team using dashboard daily
- Merge conflict rate: <5% of merged PRs

---

## Version 1.6 — Evidence Enrichment (Q4 2026)

### Goal
Enhance analyzer evidence beyond metadata to include diff analysis, subsystem detection, and test coverage impact.

### Deliverables

#### Diff Analysis
- Parse unified diff to detect:
  - Files touched by subsystem (security/, auth/, api/, etc.)
  - Risky patterns (SQL queries, auth checks, crypto operations)
  - Test file changes (coverage impact estimation)
- Integrate with `internal/review/` analyzers for evidence-backed findings

#### Dependency Impact
- Detect changes to:
  - Public APIs (breaking change detection)
  - Shared libraries (impact on downstream consumers)
  - Configuration schemas (migration requirements)
- Cross-reference with dependency graph for downstream impact

#### Test Evidence
- Identify test files changed alongside source files
- Estimate coverage impact (lines changed in tested vs untested code)
- Flag PRs that modify production code without corresponding test changes

#### Evidence Presentation
- Structured evidence in review output:
  ```json
  {
    "analyzer_name": "security",
    "finding": "auth_bypass_risk",
    "confidence": 0.85,
    "evidence": [
      {"type": "diff_pattern", "pattern": "if user.isAdmin", "line": 142},
      {"type": "subsystem", "path": "src/auth/", "risk": "high"},
      {"type": "test_coverage", "changed": false, "coverage": 0.23}
    ]
  }
  ```

### Guardrails
- Evidence is advisory only — operators make final decisions
- No automated risk scoring or merge blocking
- Diff analysis is read-only (no code modification)

### Success Metrics
- Evidence coverage: 90% of high-risk PRs have diff-based evidence
- False positive rate: <10% of flagged risks are false alarms
- Operator trust: 85% of operators report "evidence is helpful"

---

## Beyond 1.6 (Future Considerations)

### Multi-Repo Support
- Aggregate analysis across multiple repositories
- Cross-repo dependency detection
- Unified merge planning for monorepo-style workflows

### ML Feedback Loop
- Operator decisions as training signals
- Improve duplicate detection accuracy over time
- Personalized scoring based on team preferences

### GitHub App Integration
- OAuth-based authentication (no PAT management)
- Webhook-triggered analysis (real-time PR updates)
- Status check integration (block merge on high-risk findings)

**Note:** These are exploratory ideas. None are committed to a release timeline. Focus remains on v1.4-v1.6 deliverables.

---

## Guardrails (All Versions)

The following principles apply to all future development:

1. **No auto-merge or auto-approve behavior** — prATC is advisory only
2. **No GitHub App, OAuth, or webhook expansion** — manual trigger only (unless explicitly approved for v1.6+)
3. **No claims of high-confidence merge safety from weak metadata-only signals** — evidence must be strong
4. **Non-commercial use only** — FSL-1.1-Apache-2.0 license (converts to Apache 2.0 after 2 years)
5. **Read-only by default** — all destructive operations require explicit opt-in

---

## How to Contribute

prATC welcomes contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

For roadmap discussions, open a GitHub Issue with the `roadmap` label.

For commercial licensing inquiries, contact: jefferson@heimdallstrategy.com
