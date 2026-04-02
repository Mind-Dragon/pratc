# prATC Debug Run - OpenClaw
# Timestamp: 2026-04-02_144020
# Repository: openclaw/openclaw

## Environment
- pratc binary: /home/agent/pratc/bin/pratc
- prATC version: Unknown (no --version flag)
- GitHub token: Not set in environment (using unauthenticated?)

## Repository Info
- OpenClaw GitHub: https://github.com/openclaw/openclaw
- Estimated open PRs: ~6,646 (from analyze output)

## Commands Run

### 1. Sync Command
```
pratc sync --repo=openclaw/openclaw
```
Log: pratc-openclaw-sync-20260402_144020.log
Result: Started background sync job

### 2. Analyze Command (cached)
```
pratc analyze --repo=openclaw/openclaw --format=json
```
Log: pratc-openclaw-analyze-20260402_144020.log
Result: Sync in progress - returned JSON indicating "sync_status": "in_progress"

### 3. Cluster Command (no cache)
```
pratc cluster --repo=openclaw/openclaw --format=json
```
Log: pratc-openclaw-cluster-20260402_144020.log
Result: ERROR - "sync first: run `pratc sync --repo=openclaw/openclaw` before analyze"

### 4. Graph Command (no cache)
```
pratc graph --repo=openclaw/openclaw --format=dot
```
Log: pratc-openclaw-graph-20260402_144020.log
Result: ERROR - "sync first: run `pratc sync --repo=openclaw/openclaw` before analyze"

### 5. Analyze with --force-live (all PRs)
```
pratc analyze --repo=openclaw/openclaw --format=json --force-live
```
Log: pratc-openclaw-analyze-live-20260402_144020.log
Result: TIMEOUT after 300s - fetched 4400+ PRs, still fetching at timeout
Note: --max-prs flag appears to not limit fetch stage (was fetching 5400+ even with --max-prs=100)

### 6. Analyze with --force-live --max-prs=100
```
pratc analyze --repo=openclaw/openclaw --format=json --force-live --max-prs=100
```
Log: pratc-openclaw-analyze-100pr-20260402_144020.log
Result: TIMEOUT after 300s - fetched 5400+ PRs, --max-prs appears broken
Note: Still fetching way more than 100 PRs - potential bug in max-prs implementation

## Observations

### Rate Limiting
- Initial rate limit remaining: 4999
- After fetching ~5400 PRs: 4901 remaining
- Uses ~1 API call per PR fetched

### Log Format
JSON structured logs to stderr:
- Fields: ts, level, msg, request_id, repo, [additional fields]
- Levels: INFO, ERROR only (no DEBUG/WARN)

### Issues Found

1. **max-prs flag not working**: Both runs with --max-prs=100 fetched 5400+ PRs
   - Expected: Limit to 100 PRs
   - Actual: Fetched all available PRs

2. **cluster/graph require prior sync**: Cannot run without cache
   - Error message mentions "before analyze" but command was cluster/graph

3. **No version flag**: Cannot determine prATC version

4. **No GitHub token**: Running unauthenticated (4999 initial rate limit suggests this)

## prATC CLI Reference (from explore)

### Core Commands
pratc analyze --repo=owner/repo [--format=json] [--use-cache-first] [--force-live] [--max-prs=N]
pratc cluster --repo=owner/repo [--format=json] [--use-cache-first]
pratc graph --repo=owner/repo [--format=dot|json] [--use-cache-first]
pratc plan --repo=owner/repo [--target=N] [--mode=combination|permutation|with_replacement] [--format=json] [--dry-run] [--include-bots] [--use-cache-first]
pratc serve [--port=8080] [--repo=owner/repo] [--use-cache-first]
pratc sync --repo=owner/repo [--watch] [--interval=5m]
pratc audit [--limit=20] [--format=json]

### Environment Variables
PRATC_PORT=8080
PRATC_DB_PATH=~/.pratc/pratc.db
PRATC_SETTINGS_DB=./pratc-settings.db
PRATC_CACHE_TTL=1h
GITHUB_TOKEN=
GITHUB_PAT=

### Debug/Logging
- No DEBUG flag exists
- Logger only has INFO and ERROR levels
- JSON logs to stderr with: ts, level, component, request_id, repo, job_id, msg
