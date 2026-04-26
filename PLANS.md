# prATC Development Roadmap — Version Plan

> Canonical project plan for prATC (Pipeline for Automated Triaging & Corrections).  
> Live file: `PLAN.md` at repo root.  
> Last updated: Wave C implementation phase

---

## Current Status

**Version:** 2.0-dev (Wave C)  
**Branch:** main @ `1160092` (81 commits ahead of origin/main)  
**Test Status:** 37/37 passing (github + executor packages)  
**Hermes Integration:** 12 providers active, RunPod removed, vault migrated to env-file

---

## Architecture Overview

```
prATC = Pipeline reconciler that detects divergence between codebase reality and planning documents,
        then autonomously proposes/executes corrective actions via GitHub.

Components:
- Analyzer  : Reads codebase, compares against DESIGN.md / TODO.md / ARCHITECTURE.md
- Planner   : Generates actionable correction tasks (diffs, refactors, file moves)
- Executor  : Dispatches tasks to providers + applies changes
- LiveMutator: GitHub API integration for PR creation, merging, closing
- WorkQueue : Persistent job queue with claim/lease/transition states
- Ledger    : Audit trail of all actions + verification results
- Serve     : HTTP API + CLI server; orchestrates full pipeline
- TUI       : Terminal UI for viewing pipeline state, interventions, logs
```

---

## Version History

### v0.1–0.5 (Completed — commits 1–30)
| Version | Features | Status |
|---------|----------|--------|
| 0.1 | Repo scaffolding, go.mod, basic CLI skeleton | ✓ |
| 0.2 | Work queue (Redis/SQLite backend), state machine | ✓ |
| 0.3 | Analyzer: doc-codebase reconciliation pass 1 | ✓ |
| 0.4 | Planner: diff generation, plan → task breakdown | ✓ |
| 0.5 | Executor: provider routing, dry-run mode, basic logging | ✓ |

### v1.0 (Completed — commits 31–60)
| Feature | Description |
|---------|-------------|
| Serve command | `pratc serve` HTTP server with health, metrics, API endpoints |
| TUI | `pratc tui` interactive terminal dashboard (bubbletea) |
| Doc sync | Two-way DESIGN.md ⇄ codebase reconciliation |
| Provider framework | Multi-LLM routing, Hermes integration, Fallback logic |
| Test suite | Unit tests (github, executor), e2e sandbox harness |
| CI | GitHub Actions: lint, test, gate checks |

---

## v2.0 — Wave-Based Release Plan

Release strategy: incremental waves, each adding orthogonal capabilities.  
All mutations default to `dry-run`; live GitHub ops gated behind `--live`.

### Wave A — Foundation (commits 61–70) ✓ COMPLETE
- **LiveGitHubMutator** stub + `github.Client` wrapper
- `internal/github` API methods: PR create/update, issues, labels, comments
- `internal/executor/live_mutator.go` with dry-run guards
- Work queue `GetClaimable` / `Transition` state transitions
- Executor plumbing: `ExecuteIntent` accepts `Mutator` interface
- **Tests:** 37/37 green (github + executor packages)

### Wave B — Safety & Observability (commits 71–75) — IN PROGRESS
- **Circuit breaker**: maxConcurrentMutations per repo + global limit
- **Ledger**: `internal/executor/ledger.go` — action log, PR linkage, audit fields
- **Verification**: post-execution diff + LLM-as-judge correctness check
- **Guarded executor**: wrap `ExecuteIntent` with pre/post verification hooks
- **Auth**: provider token sourcing from `~/.vault/llm-provider-keys.env`

### Wave C (current) — Live Flag & Worker Pool (commits 76–80) — ACTIVE
- **`--live` flag**: added to `internal/cmd/serve.go RegisterServeCommand`
- **`runServer` signature extended**: `(..., live bool)`
- **Intent linkage**: `ActionIntent.WorkItemID` links executable decisions to queue work items
- **Intent persistence**: `internal/workqueue` stores `action_intents` and exposes `GetIntentsForWorkItem()`
- **Live mutator**: `LiveGitHubMutator` satisfies `executor.GitHubMutator` with dry-run-aware methods
- **Worker spawn**: `serve --live` starts `executor.Worker` instead of embedding mutation loops in `serve.go`
- **Integration**: worker claims `ActionWorkItem`, loads persisted intents, and executes them through `executor.ExecuteIntent`
- **Gate check**: focused tests, `make build`, `serve --help`, and `git diff --check` pass locally

### Wave D — Merge & Close Actions (commits 81–85)
- **Merge strategies**: squash, rebase, merge commit; configurable per intent
- **Close action**: PR/issue close with comment reason
- **Retry logic**: exponential backoff on 5xx/rate-limit from GitHub
- **State transitions**: `Claimed → Executed → Verified` (or `Failed`)

### Wave E — Ledger & UI Integration (commits 86–90)
- **Ledger persistence**: SQLite table `action_ledger` with indexes
- **TUI updates**: live view of mutation queue, worker status, ledger entries
- **API endpoints**: `/api/v1/ledger`, `/api/v1/queue/stats`
- **Notification**: optional Slack/Discord webhook on merge/failure

### Wave F — Sandbox & E2E Tests (commits 91–95)
- **Sandbox repo**: `pratc-e2e-test` org, disposable feature branches
- **E2E scenarios**: dry-run → review → live merge full cycle
- **Chaos tests**: simulate GitHub API 500/rate-limit; verify retry/backoff
- **CI gate**: e2e test block until all Wave F scenarios pass

### Wave G — Documentation & Runbook (commits 96–98)
- **RUNBOOK.md**: operational procedures (restart, recover failed workers, vault key rotation)
- **VERSION2.0.md**: release notes, migration guide from v1.x
- **API reference**: OpenAPI spec for `serve` endpoints
- **Provider guide**: adding new LLM backends (env keys, model mapping)

### Wave H — Polishing & Optimization (commits 99–100)
- **Concurrency tuning**: configurable worker pool size via flag/env
- **Metrics**: Prometheus metrics (queue depth, mutation latency, success rate)
- **Profiling**: pprof endpoints, flamegraph generation
- **Final gate**: code review checklist, dependency audit, version bump

---

## v3.0 — Planned Features (post-2.0)

| Feature | Target Wave | Notes |
|---------|-------------|-------|
| **Multi-repo support** | A | Single serve instance, multiple queue shards |
| **Batch operations** | A | Apply same fix across 10+ repos (dependency upgrades) |
| **AI review** | B | LLM-as-judge on PR diffs before auto-merge |
| **Rollback** | C | Revert bad mutations via ledger |
| **Webhooks** | C | GitHub webhook-driven pipeline (no polling) |
| **RBAC** | D | Token-scoped permissions (write vs admin) |
| **Plugin system** | D | External executor plugins (Python/JS) |

---

## Gate Checklist (per wave)

- [ ] `go vet ./...` passes
- [ ] `go mod tidy` produces no changes
- [ ] `gofmt -l` reports no modified files
- [ ] Unit tests ≥ 80% coverage on new code
- [ ] E2E tests pass (if applicable for wave)
- [ ] Running `pratc serve --help` shows new flags
- [ ] Docs updated in `/docs` or `RUNBOOK.md`
- [ ] No credentials in logs or error messages
- [ ] Graceful shutdown verified (SIGTERM → worker stop)
- [ ] Hermes provider list still ≥ 10 active providers

---

## Provider Status (Hermes)

| Provider | Status | Model(s) |
|----------|--------|----------|
| kilocode | ✓ active | qwen3-235b, qwen3-32b |
| nacrof | ✓ active | k3-mini, k2.6 |
| deepseek | ✓ active | v3, r1 |
| fireworks | ✓ active | qwen3-235b, v3 |
| exa | ✓ active | exa-large, exa-pro |
| kimi | ✓ active | k2.5, k2.5-thinking |
| inception | ✓ active | incester-v3 |
| xai | ✓ active | grok-3 |
| zai | ✓ active | z-ai-pro |
| openrouter | ✓ active | claude-3.7, gpt-4.1 |
| opencode | ✓ active | o4-mini, o3 |
| synthetic | ✓ active | synthetic-v1 |

All keys sourced from `~/.vault/llm-provider-keys.env`.  
RunPod removed (balance zero).

---

## Change Log

| Date | Version | Change |
|------|---------|--------|
| 2026-04-26 | 2.0-dev Wave C | Added `--live` flag, persisted executable intents, and wired serve to central executor worker |
| 2026-04-26 | 2.0-dev Wave B | Vault migrated to env-file; Hermes provider config refreshed |
| 2026-04-26 | 1.0 | First stable release; serve + TUI + provider routing |

---

## Contact

Maintained by Nous Research / Hermes Agent session.  
Issues: `github.com/jeffersonnunn/pratc`