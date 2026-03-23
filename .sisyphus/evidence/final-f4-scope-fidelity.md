# F4 Scope Fidelity Check — FINAL

Date: 2026-03-23
Reviewer: Deep (ses_2e8042bd8ffekJ9lzc0EdxPt6O)
Verdict: APPROVE

## Must-Have Status
| Item | Status | Evidence |
|------|--------|----------|
| Rate-limit-aware client | ✅ | handleRateLimit(), reserveRequests=200, exponential backoff with 8 retries |
| Pre-filter pipeline | ✅ | BuildCandidatePool() with ApplyFilters, AssignClusterIDs, SortPoolByPriority |
| Dry-run default | ✅ | --dry-run defaults true; no mutation code paths |
| Audit logging | ✅ | AuditEntry, Store interface; CLI command at cmd/pratc/audit.go |
| SQLite cache | ✅ | schema_migrations table, fail-fast on future schema versions |
| Formula engine | ✅ | P(n,k), C(n,k), ModeWithReplacement |
| Graph engine | ✅ | TopologicalOrder(), DOT() output with digraph declaration |
| Bot detection | ✅ | hardcoded patterns for dependabot[bot], renovate[bot], etc. |

## Must-Not-Have Status
| Forbidden Item | Found? | Details |
|---------------|--------|---------|
| OAuth/webhooks | NO | Only appears in AGENTS.md as forbidden |
| ML feedback loops | NO | No feedback loop patterns in ml-service/ |
| Multi-repo UI | NO | Web components are single-repo focused |
| gRPC | NO | No .proto files or grpc imports |
| Auto PR execution | NO | No GitHub mutation API calls found |
| Nx/Turborepo | NO | No nx.json or turborepo.json files |

## Dry-Run Enforcement
No GitHub mutation code paths exist. All code is read-only GraphQL queries. Audit subsystem only appends entries. Plan command produces plans without execution.

## Backward Compatibility
All required routes verified present: /healthz, /api/health, /analyze, /cluster, /graph, /plan, /inbox, /triage, RESTful /api/repos/:owner/:repo/* routes.

## OpenRouter Migration
✅ docker-compose.yml: minimax-light profile present, no openrouter-light
✅ ML provider code: minimax.py with API integration
✅ AGENTS.md: openrouter-light reference fixed to minimax-light (commit 65f0854)

## Blocking Issues
None.
