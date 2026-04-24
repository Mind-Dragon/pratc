# prATC HTTP Middleware Guide

**Last Updated:** 2026-04-24
**System Version:** v1.7.1 current, v2.0 target additions noted
**Location:** `internal/cmd/serve.go`

## Table of Contents

1. [Middleware Stack Overview](#middleware-stack-overview)
2. [CORS Middleware](#cors-middleware)
3. [Request/Response Flow](#requestresponse-flow)
4. [Route Handlers](#route-handlers)
5. [Request Validation](#request-validation)
6. [Response Helpers](#response-helpers)
7. [Error Handling](#error-handling)
8. [Configuration](#configuration)
9. [Security Considerations](#security-considerations)

---

## Middleware Stack Overview

The prATC HTTP server uses a simple middleware pattern with the standard Go `net/http` package.

```
Request
   │
   ▼
┌─────────────────┐
│  corsMiddleware │ ─── CORS headers, preflight handling
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  ServeMux       │ ─── Route matching
│  (http.NewServeMux)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Route Handler  │ ─── Business logic
│  (e.g., handleAnalyze)
└─────────────────┘
         │
         ▼
┌─────────────────┐
│  Response       │ ─── JSON or error response
└─────────────────┘
```

**Current middleware chain:**
1. `corsMiddleware` — CORS headers and preflight
2. `http.ServeMux` — Route dispatch

**Note:** No logging middleware, no auth middleware, no rate limiting middleware (rate limiting is in GitHub client, not HTTP layer).

---

## CORS Middleware

### Location

`internal/cmd/serve.go:827-874`

### Implementation

```go
func corsMiddleware(next http.Handler) http.Handler {
  allowedOrigins := corsAllowedOrigins()
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    origin := r.Header.Get("Origin")
    if r.Method == http.MethodOptions {
      if isOriginAllowed(origin, allowedOrigins) {
        w.Header().Set("Access-Control-Allow-Origin", origin)
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
      }
      w.WriteHeader(http.StatusOK)
      return
    }
    if isOriginAllowed(origin, allowedOrigins) {
      w.Header().Set("Access-Control-Allow-Origin", origin)
      w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
      w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    }
    next.ServeHTTP(w, r)
  })
}
```

### Configuration

**Configuration values:**
- Origins: `PRATC_CORS_ALLOWED_ORIGINS`, comma-separated. Empty disables CORS by default.
- Methods: `GET, POST, DELETE, OPTIONS`
- Headers: `Content-Type`

Browser dashboard origins are historical/deferred. The v2.0 dashboard is TUI-first, so no browser origin is enabled unless an operator explicitly configures it.

### Usage

Applied to all routes in server setup:

```go
server := &http.Server{
  Addr:    ":" + strconv.Itoa(port),
  Handler: corsMiddleware(mux),
}
```

---

## Request/Response Flow

### Standard Request Flow

```
┌─────────────┐
│ Client      │ GET /api/repos/owner/repo/analyze
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ CORS        │ Add CORS headers
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ ServeMux    │ Match route pattern
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ ensureGET   │ Validate HTTP method
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ ensureRepo  │ Validate repo parameter
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Service Call│ app.Service.Analyze()
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ writeHTTP   │ JSON response (200)
└─────────────┘
```

### Error Flow

```
┌─────────────┐
│ Client      │ GET /api/repos/owner/repo/analyze (invalid method)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ CORS        │ Add CORS headers
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ ServeMux    │ Match route
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ ensureGET   │ Reject (405 Method Not Allowed)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ writeHTTP   │ Error response
└─────────────┘
```

---

## Route Handlers

### Route Registration

```go
mux := http.NewServeMux()

// Health checks
mux.HandleFunc("/healthz", handleHealth)
mux.HandleFunc("/api/health", handleHealth)

// Settings
mux.HandleFunc("/api/settings", handleSettings)
mux.HandleFunc("/api/settings/export", handleExportSettings)
mux.HandleFunc("/api/settings/import", handleImportSettings)

// Legacy query-string routes
mux.HandleFunc("/analyze", handleAnalyzeLegacy)
mux.HandleFunc("/cluster", handleClusterLegacy)
mux.HandleFunc("/graph", handleGraphLegacy)
mux.HandleFunc("/plan", handlePlanLegacy)

// RESTful routes
mux.HandleFunc("/api/repos/", handleRepoAction)
```

### Route Handler Pattern

All handlers follow this structure:

```go
func handleXXX(w http.ResponseWriter, r *http.Request, service app.Service, repo string) {
  // 1. Validate method
  if !ensureGET(w, r) {
    return
  }
  
  // 2. Validate repo
  if !ensureRepo(w, repo) {
    return
  }
  
  // 3. Parse query params (for plan, etc.)
  params := parseParams(r)
  
  // 4. Call service
  result, err := service.XXX(r.Context(), repo, params)
  if err != nil {
    writeHTTPError(w, http.StatusInternalServerError, err.Error())
    return
  }
  
  // 5. Write response
  writeHTTPJSON(w, http.StatusOK, result)
}
```

### handleRepoAction Dispatcher

Parses RESTful paths and dispatches to appropriate handler:

```go
func handleRepoAction(w http.ResponseWriter, r *http.Request, service app.Service, syncAPI repoSyncAPI) {
  // Parse /api/repos/{owner}/{name}/{action}
  repo, action, ok := parseRepoActionPath(r.URL.Path)
  if !ok {
    writeHTTPError(w, http.StatusNotFound, "route not found")
    return
  }
  
  switch action {
  case "analyze":
    handleAnalyze(w, r, service, repo)
  case "cluster":
    handleCluster(w, r, service, repo)
  case "graph":
    handleGraph(w, r, service, repo)
  case "plan", "plans":
    handlePlan(w, r, service, repo)
  case "sync":
    handleSyncTrigger(w, r, syncAPI, repo)
  case "sync/stream":
    handleSyncStream(w, r, syncAPI, repo)
  case "sync/status":
    handleSyncStatus(w, r, repo)
  default:
    writeHTTPError(w, http.StatusNotFound, "route not found")
  }
}
```

### Path Parsing

```go
func parseRepoActionPath(path string) (repo string, action string, ok bool) {
  trimmed := strings.Trim(path, "/")
  parts := strings.Split(trimmed, "/")
  
  // Must be: api/repos/{owner}/{name}/{action}
  if len(parts) < 5 {
    return "", "", false
  }
  if parts[0] != "api" || parts[1] != "repos" {
    return "", "", false
  }
  
  // repo = "owner/name"
  // action = "analyze" or "sync/stream" etc.
  return parts[2] + "/" + parts[3], strings.Join(parts[4:], "/"), true
}
```

---

## Request Validation

### ensureGET

Validates HTTP method is GET.

```go
func ensureGET(w http.ResponseWriter, r *http.Request) bool {
  if r.Method == http.MethodGet {
    return true
  }
  w.WriteHeader(http.StatusMethodNotAllowed)
  return false
}
```

### ensureRepo

Validates repo parameter is non-empty.

```go
func ensureRepo(w http.ResponseWriter, repo string) bool {
  if strings.TrimSpace(repo) != "" {
    return true
  }
  writeHTTPError(w, http.StatusBadRequest, "missing repo parameter")
  return false
}
}
```

### Settings Request Validation

**GET /api/settings:**
- `repo` query param (optional, empty for global settings)

**POST /api/settings:**
- JSON body: `{scope, repo, key, value}`
- `key` is required
- `validateOnly` query param (optional)

**DELETE /api/settings:**
- Query params: `scope`, `repo`, `key`
- `key` is required

### Plan Query Parameter Validation

**Parameters:**

| Param | Type | Default | Validation |
|-------|------|---------|------------|
| `target` | int | 20 | > 0 |
| `mode` | string | "combination" | combination, permutation, with_replacement |
| `cluster_id` | string | - | No validation (reserved) |
| `exclude_conflicts` | bool | false | true/false |
| `stale_score_threshold` | float | 0.0 | 0.0 - 1.0 |
| `candidate_pool_cap` | int | 100 | 1 - 500 |
| `score_min` | float | 0.0 | 0.0 - 100.0 |

**Validation code (excerpt):**

```go
var validationErrors []string

// target: int > 0
target := 20
if raw := strings.TrimSpace(query.Get("target")); raw != "" {
  parsed, err := strconv.Atoi(raw)
  if err != nil || parsed <= 0 {
    validationErrors = append(validationErrors, "target must be a positive integer")
  } else {
    target = parsed
  }
}

// mode: enum
mode, err := parseMode(query.Get("mode"))
if err != nil {
  validationErrors = append(validationErrors, err.Error())
}

// Return all errors if any
if len(validationErrors) > 0 {
  writeHTTPError(w, http.StatusBadRequest, strings.Join(validationErrors, "; "))
  return
}
```

---

## Response Helpers

### writeHTTPJSON

Writes JSON response with proper Content-Type.

```go
func writeHTTPJSON(w http.ResponseWriter, status int, payload any) {
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(status)
  _ = json.NewEncoder(w).Encode(payload)
}
```

### writeHTTPError

Writes standardized error response.

```go
func writeHTTPError(w http.ResponseWriter, status int, message string) {
  writeHTTPJSON(w, status, map[string]string{"error": message})
}
```

**Error response format:**
```json
{
  "error": "error message"
}
```

### Special Responses

**Health check:**
```go
writeHTTPJSON(w, http.StatusOK, service.Health())
// Returns: {"status": "healthy", "version": "0.1.0"}
```

**DOT format (graph):**
```go
if strings.EqualFold(query.Get("format"), "dot") {
  w.Header().Set("Content-Type", "text/plain; charset=utf-8")
  w.WriteHeader(http.StatusOK)
  _, _ = fmt.Fprintln(w, response.DOT)
  return
}
```

**YAML export (settings):**
```go
w.Header().Set("Content-Type", "application/x-yaml")
w.WriteHeader(http.StatusOK)
_, _ = w.Write(content)
```

**Sync trigger (202 Accepted):**
```go
writeHTTPJSON(w, http.StatusAccepted, map[string]any{
  "started": true,
  "repo": repo,
})
```

---

## Error Handling

### HTTP Status Codes

| Code | Usage |
|------|-------|
| 200 OK | Successful GET, POST with result |
| 202 Accepted | Async operation started (sync) |
| 400 Bad Request | Invalid parameters |
| 404 Not Found | Route not found, repo not found |
| 405 Method Not Allowed | Wrong HTTP method |
| 500 Internal Server Error | Service errors, unexpected failures |

### Error Patterns

**Parameter validation:**
```go
if err != nil || parsed <= 0 {
  validationErrors = append(validationErrors, "target must be a positive integer")
}
// ... collect all errors
if len(validationErrors) > 0 {
  writeHTTPError(w, http.StatusBadRequest, strings.Join(validationErrors, "; "))
  return
}
```

**Service call errors:**
```go
response, err := service.Analyze(r.Context(), repo)
if err != nil {
  writeHTTPError(w, http.StatusInternalServerError, err.Error())
  return
}
```

**Sync API unavailable:**
```go
if syncAPI == nil {
  writeHTTPError(w, http.StatusInternalServerError, "sync API unavailable")
  return
}
```

---

## Configuration

### Server Configuration

```go
server := &http.Server{
  Addr:    ":" + strconv.Itoa(port),  // Default: :7400
  Handler: corsMiddleware(mux),
}
```

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `PRATC_PORT` | 7400 | Server port |

### Graceful Shutdown

```go
// In runServer()
go func() {
  <-ctx.Done()
  _ = server.Shutdown(context.Background())
}()

err := server.ListenAndServe()
if errors.Is(err, http.ErrServerClosed) {
  return nil  // Normal shutdown
}
return err
```

---

## Security Considerations

### Current Limitations

1. **No authentication** — API is open
2. **No rate limiting** — HTTP layer has no rate limiting (GitHub client has rate limiting)
3. **CORS hardcoded** — Browser-dashboard CORS examples are historical; v2.0 dashboard is TUI-first
4. **No request size limits** — Except settings import (1MB)
5. **No TLS** — HTTP only (use reverse proxy for HTTPS)

### Security Recommendations

1. **Add authentication middleware**
   ```go
   func authMiddleware(next http.Handler) http.Handler {
     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
       token := r.Header.Get("Authorization")
       if !validateToken(token) {
         writeHTTPError(w, http.StatusUnauthorized, "invalid token")
         return
       }
       next.ServeHTTP(w, r)
     })
   }
   ```

2. **Add rate limiting middleware**
   ```go
   func rateLimitMiddleware(next http.Handler) http.Handler {
     limiter := rate.NewLimiter(rate.Limit(10), 100) // 10 req/s, burst 100
     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
       if !limiter.Allow() {
         writeHTTPError(w, http.StatusTooManyRequests, "rate limit exceeded")
         return
       }
       next.ServeHTTP(w, r)
     })
   }
   ```

3. **Configurable CORS origins**
   ```go
   allowedOrigins := strings.Split(os.Getenv("PRATC_CORS_ORIGINS"), ",")
   ```

4. **Request logging middleware**
   ```go
   func loggingMiddleware(next http.Handler) http.Handler {
     return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
       start := time.Now()
       next.ServeHTTP(w, r)
       log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
     })
   }
   ```

---

## Route Reference

### Health Routes

| Method | Route | Response |
|--------|-------|----------|
| GET | `/healthz` | `{"status": "healthy", "version": "x.x.x"}` |
| GET | `/api/health` | Same as `/healthz` |

### Legacy Routes

| Method | Route | Params | Response |
|--------|-------|--------|----------|
| GET | `/analyze` | `repo` | AnalysisResponse |
| GET | `/cluster` | `repo` | ClusterResponse |
| GET | `/graph` | `repo`, `format` (dot/json) | GraphResponse or DOT |
| GET | `/plan` | `repo`, `target`, `mode`, ... | PlanResponse |

### RESTful Routes

| Method | Route | Params/Body | Response |
|--------|-------|-------------|----------|
| GET | `/api/repos/{o}/{n}/analyze` | - | AnalysisResponse |
| GET | `/api/repos/{o}/{n}/cluster` | - | ClusterResponse |
| GET | `/api/repos/{o}/{n}/graph` | `format` (dot/json) | GraphResponse or DOT |
| GET | `/api/repos/{o}/{n}/plan` | `target`, `mode`, ... | PlanResponse |
| POST | `/api/repos/{o}/{n}/sync` | - | `{"started": true}` (202) |
| GET | `/api/repos/{o}/{n}/sync/stream` | - | SSE events |
| GET | `/api/repos/{o}/{n}/sync/status` | - | Sync status JSON |

### Settings Routes

| Method | Route | Params/Body | Response |
|--------|-------|-------------|----------|
| GET | `/api/settings` | `repo` (query) | Settings JSON |
| POST | `/api/settings` | `{scope, repo, key, value}` | `{"updated": true}` |
| POST | `/api/settings?validateOnly=true` | Same as above | `{"valid": true}` |
| DELETE | `/api/settings` | `scope`, `repo`, `key` (query) | `{"deleted": true}` |
| GET | `/api/settings/export` | `scope`, `repo` (query) | YAML content |
| POST | `/api/settings/import` | YAML body (1MB max) | `{"imported": true}` |

---

## Complete Route Map

```
┌────────────────────────────────────────────────────────────────┐
│                         Route Tree                             │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  /healthz ................................... GET  -> health   │
│  /api/health ............................... GET  -> health    │
│                                                                │
│  /api/settings ............................. GET  -> get       │
│  /api/settings ............................. POST -> set       │
│  /api/settings ............................. DELETE -> delete  │
│  /api/settings/export ...................... GET  -> export    │
│  /api/settings/import ...................... POST -> import    │
│                                                                │
│  /analyze .................................. GET  -> analyze   │
│  /cluster .................................. GET  -> cluster   │
│  /graph .................................... GET  -> graph     │
│  /plan ..................................... GET  -> plan      │
│                                                                │
│  /api/repos/{owner}/{repo}/analyze ......... GET  -> analyze   │
│  /api/repos/{owner}/{repo}/cluster ......... GET  -> cluster   │
│  /api/repos/{owner}/{repo}/graph ........... GET  -> graph     │
│  /api/repos/{owner}/{repo}/plan ............ GET  -> plan      │
│  /api/repos/{owner}/{repo}/plans ........... GET  -> plan      │
│  /api/repos/{owner}/{repo}/sync ............ POST -> sync      │
│  /api/repos/{owner}/{repo}/sync/stream ..... GET  -> sync SSE  │
│  /api/repos/{owner}/{repo}/sync/status ..... GET  -> sync status│
│                                                                │
└────────────────────────────────────────────────────────────────┘
```
