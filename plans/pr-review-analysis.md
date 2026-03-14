# PRATC — PR Landscape Research & Gap Analysis

## Research Summary

This document captures the research findings that motivated the PRATC architecture, confirming two major unaddressed production gaps in the PR management tooling ecosystem.

---

## Core Problem Statement

Managing 5,500+ open PRs is **not a linear queue problem** — it’s a **combinatorial optimization problem**.

- With N PRs, there are 2^N possible merge subsets and N! possible orderings
- No existing tool enumerates or scores these; all treat PRs as independent units
- Same architecture as air traffic control: tiered strategies, on-the-fly plan generation, mode selection

**Modes:**
- **Combination mode**: Independent PRs (C(n,k) subsets) — no ordering constraints
- **Permutation mode**: Dependent PRs with ordering constraints (topological sort required)

---

## Repositories Analyzed

| Repo | Open PRs | Sample Size | Notes |
|------|----------|-------------|-------|
| openclaw/openclaw | ~5,506 | ~500 | Primary scale target |
| opencode-ai/opencode | 42 | All | Small repo baseline |
| code-yeongyu/oh-my-openagent | 126 | All | Mid-size baseline |

---

## Confirmed Production Gaps

### Gap 1: Proactive Conflict Prediction
- **Status**: Zero production tools exist
- **Current state**: Every tool today is **reactive** — detect/resolve conflicts *after* they occur
- **Opportunity**: Predict conflicts *before* merge attempts using file overlap analysis and AST diff
- **Building block**: Weave CLI (tree-sitter AST merge driver) resolves 31/31 conflicts vs Git’s 15/31

### Gap 2: PR Deduplication
- **Status**: Zero shipped production tools
- **Academic work**: PR-DupliChecker (2024) — paper exists, nothing in production
- **Opportunity**: Cluster semantically similar PRs (same intent, different implementations) for maintainer review
- **Building block**: HDBSCAN + sentence-transformers for intent-based grouping

### Gap 3: Cross-PR Awareness
- **Status**: No AI review tool understands inter-PR relationships
- **Current state**: Every AI PR reviewer treats each PR in isolation
- **Opportunity**: Graph-based relationship model (dependency, conflict, duplicate edges)
- **Building block**: D3.js force graph + topological sort for dependency ordering

---

## Building Blocks Identified

| Component | Tool/Approach | Capability |
|-----------|--------------|------------|
| Semantic merge | Weave CLI (tree-sitter) | 31/31 conflict resolution vs Git’s 15/31 |
| PR clustering | HDBSCAN + sentence-transformers | Intent-based grouping, noise-tolerant |
| Merge ordering | Topological sort + priority scoring | Dependency-aware merge sequencing |
| Deduplication | PR-DupliChecker approach (2024) | Semantic similarity clustering |
| Graph viz | D3.js force-directed | Dependency/conflict/duplicate visualization |

## Reference Tools Surveyed

- **Depviz** — PR dependency visualization (read-only, no merge intelligence)
- **stack-pr** — stacked PR management (linear chains only)
- **zhang-shasha** — tree edit distance algorithm (theoretical basis for AST diff)
- **Graphite / Trunk** — commercial stacked PR tools (not general-purpose, not self-hostable)

---

## PRATC Differentiation

| Feature | Existing Tools | PRATC |
|---------|---------------|-------|
| Conflict prediction | Reactive only | Proactive (file overlap + AST) |
| PR deduplication | None shipped | HDBSCAN semantic clustering |
| Cross-PR awareness | None | Full graph model (dep/conflict/dup) |
| Scale | Single PR focus | Optimized for 5,500+ PR repos |
| Deployment | SaaS/cloud | Self-hostable, repo-agnostic |
| License | Proprietary | Open source |

---

## Conclusion

The two confirmed production gaps (proactive conflict prediction, PR deduplication) represent genuinely unaddressed problems with clear building blocks available. PRATC’s combinatorial optimization framing is architecturally sound and differentiated from all surveyed tools.

Next step: implement v0.1 Foundation (SQLite schema + Go API scaffold + GitHub sync worker).
