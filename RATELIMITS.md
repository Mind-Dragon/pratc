# GitHub API Rate Limits

prATC is designed to work respectfully within GitHub's API rate limits. This document explains how rate limiting works and how prATC manages it.

## GitHub Rate Limit Tiers

### Authenticated Requests
When using a personal access token (PAT) or OAuth token:
- **Limit**: 5,000 requests per hour
- **Reset**: Rolling window (resets every hour from first request)
- **Check**: `curl -H "Authorization: token YOUR_TOKEN" https://api.github.com/rate_limit`

### Unauthenticated Requests
Without authentication:
- **Limit**: 60 requests per hour per IP
- **Not recommended** for prATC — always use authentication

### Secondary Rate Limits
GitHub also enforces secondary rate limits to prevent abuse:
- **Content creation**: Limited POST/PUT/PATCH requests
- **Search**: 30 requests per minute
- **GraphQL**: Complex queries count as multiple requests
- **Concurrent connections**: Too many simultaneous requests triggers throttling

## How prATC Manages Rate Limits

### Budget Tracking
prATC tracks your API budget in real-time:
```
Budget: 5000/5000 remaining (0 reserved), resets in 59m59s
```

- **Total**: Your hourly limit (5000 for authenticated)
- **Remaining**: Requests left in current window
- **Reserved**: Buffer kept for critical operations
- **Resets**: Time until limit resets

### Automatic Throttling
When rate limit is approached:
1. **Reserve buffer**: Keep 200 requests for emergency operations
2. **Pause sync**: Stop background sync when budget is low
3. **Queue requests**: Defer non-critical API calls
4. **Resume automatically**: Continue when budget resets

### Retry Logic
prATC handles rate limit responses gracefully:

| Status Code | Behavior | Retry Strategy |
|-------------|----------|----------------|
| 403 (rate limit) | Pause until reset | Wait until `X-RateLimit-Reset` + 15s |
| 403 (secondary) | Exponential backoff | 2s → 4s → 8s → ... → 60s (max 8 retries) |
| 5xx (server error) | Exponential backoff | 1s → 2s → 4s → ... → 30s (max 6 retries) |
| 429 (too many) | Immediate backoff | Use `Retry-After` header |

### Configuration Options

#### Environment Variables
```bash
# API rate limit (default: 5000)
export PRATC_RATE_LIMIT=5000

# Reserve buffer (default: 200)
export PRATC_RESERVE_BUFFER=200

# Seconds to wait after reset (default: 15)
export PRATC_RESET_BUFFER=15
```

#### CLI Flags
```bash
# Custom rate limit
pratc sync --repo=owner/repo --rate-limit=4000 --reserve-buffer=500

# Watch mode with automatic resume
pratc sync --repo=owner/repo --watch --interval=5m
```

## Best Practices

### 1. Use Authentication
Prefer `gh auth login` so tokens do not end up in shell history. Environment variables remain supported when automation needs them:
```bash
gh auth login
# or
export GITHUB_TOKEN=<github-token>
```

### 2. Warm Cache First
Initial sync is expensive. After that, prefer the cache-first workflow and reuse the local snapshot:
```bash
# First time (full sync or live validation)
pratc workflow --repo=owner/repo --refresh-sync --force-live --progress

# Default day-to-day path (cache-first)
pratc workflow --repo=owner/repo --progress
```

### 3. Use Cache-First Mode
For routine analysis, the default path should stay local unless you explicitly need a fresh recapture:
```bash
pratc workflow --repo=owner/repo --progress
pratc analyze --repo=owner/repo --use-cache-first
```

### 4. Batch Operations
prATC automatically batches GraphQL queries. For large repos:
```bash
# Process in stages
pratc sync --repo=owner/repo --watch --interval=10m
```

### 5. Monitor Budget
Use the dashboard to watch rate limit status:
```bash
# Terminal dashboard
pratc monitor

# Or API endpoint
curl http://localhost:7400/api/health
```

## Rate Limit Headers

GitHub includes these headers in every response:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Your total hourly limit |
| `X-RateLimit-Remaining` | Requests left in window |
| `X-RateLimit-Reset` | Unix timestamp when limit resets |
| `X-RateLimit-Used` | Requests used in window |

prATC reads these headers to track your budget accurately.

## Troubleshooting

### "secondary rate limit hit"
You're making too many requests too fast. prATC will automatically back off. To reduce frequency:
```bash
# Increase interval between sync checks
pratc sync --repo=owner/repo --watch --interval=15m
```

### "rate limit exceeded"
Your hourly budget is exhausted. Options:
1. Wait for reset (check `X-RateLimit-Reset`)
2. Reduce `--rate-limit` to be more conservative
3. Increase `--reserve-buffer` to save more for critical ops

### "API token expired"
Refresh your token:
```bash
# Revoke old token at https://github.com/settings/tokens
# Create new token with 'repo' scope
export GITHUB_TOKEN=ghp_new_token
```

## Enterprise GitHub

GitHub Enterprise has different limits:
- **Enterprise Cloud**: 15,000 requests/hour
- **Enterprise Server**: Configurable by admin

Set accordingly:
```bash
export PRATC_RATE_LIMIT=15000
```

## Monitoring

prATC logs rate limit events at INFO level:
```json
{"level":"INFO","msg":"budget initialized","budget":"Budget: 5000/5000 remaining"}
{"level":"INFO","msg":"secondary rate limit hit, retrying","retry_after":3089}
{"level":"WARN","msg":"rate limit critical","remaining":50,"action":"pausing sync"}
```

Enable verbose logging to see every API call:
```bash
pratc sync --repo=owner/repo --log-level=debug
```

## Resources

- [GitHub Rate Limits Documentation](https://docs.github.com/en/rest/overview/rate-limits-for-the-rest-api)
- [GraphQL Rate Limits](https://docs.github.com/en/graphql/overview/rate-limiting-and-node-limiting)
- [Best Practices for Integrators](https://docs.github.com/en/rest/guides/best-practices-for-integrators)
