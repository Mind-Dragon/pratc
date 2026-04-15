# prATC v1.4a — Remediation Action Plan

Generated from codebase analysis. 30 items across Performance (9), Security (5), Slop (16).

---

## Performance (9 items)

---

### perf-01 — O(n*k) slice manipulation
**file:** `internal/formula/modes.go:139`

**action:** Replace `append(available[:choiceIndex], available[choiceIndex+1:]...)` with an index-tracking approach using a pre-allocated `removed` bool slice. Track chosen indices, filter once at end. Or swap-with-last and shrink length (O(1) instead of O(n)).

**test:** Benchmark `Count()` and `GenerateByIndex()` with k=50, n=5000 before/after. Target: same output, faster.
**verify:** `go test -bench=. ./internal/formula/ -benchmem`

---

### perf-02 — Sequential file fetching
**file:** `internal/app/service.go:1175-1188`

**action:** Modify `enrichFromGraphQL()` to batch file fetches. GitHub GraphQL supports `pullRequestFiles(first: 100)` with pagination. Fetch files for up to 10 PRs concurrently using goroutines with semaphore (max 10 concurrent). Cache results keyed by PR number.

**test:** Instrument timing with 100 PRs that have files. Log time per PR before/after. Target: parallel phase < 5s for 100 PRs.
**verify:** Add `t.Logf("enrichDuration=%s", time.Since(start))` and run `pratc analyze` with trace logging.

---

### perf-03 — No ML result caching
**file:** `internal/ml/bridge.go:165-172`

**action:** Add an in-memory LRU cache keyed by hash of input PR slice. TTL of 5 minutes. Invalidate on PR update. Cache `ClusterResult`, `DuplicateResult` separately.

**test:** Run `pratc analyze` twice on same repo. Second run should hit cache and complete in <500ms. First run is baseline.
**verify:** Add cache hit/miss logging. Count hits on second identical call.

---

### perf-04 — conflictCounts() inside tier loop
**file:** `internal/formula/engine.go:96`

**action:** Move `conflictCounts(pool)` outside the tier-building loop. It depends only on `pool`, not on `tier`. Compute once before the loop.

**test:** Run `pratc plan --target=10` with 1000 PRs. Profile with `go test -cpuprofile=cpu.out`. Before: conflictCounts called n times. After: called once.
**verify:** `go test -bench=. ./internal/formula/ -benchmem` and compare allocations.

---

### perf-05 — PairwiseExecutor dead code
**file:** `internal/planning/pairwise.go`

**action:** Two options: (A) wire it into the production `graph.Build()` path as an alternative executor, or (B) delete it entirely with a commit message noting removal. Recommendation: profile it first, then decide. Add a comment at the top of the file explicitly marking it as "UNWIRED - evaluate for v1.7".

**test:** If wiring: ensure `go test ./internal/planning/ -run Pairwise` passes. If deleting: verify no calls to `PairwiseExecutor` exist in codebase.
**verify:** `grep -r "PairwiseExecutor" internal/` returns zero production call sites.

---

### perf-06 — Manual string conversion
**file:** `internal/planning/pool.go:586-609`

**action:** Replace `lowercaseFold()` with `strings.EqualFold()` (single call, no allocation). Replace `findSubstring()` with `strings.Contains()`. Delete the custom functions.

**test:** Run existing pool scoring tests. Verify output unchanged.
**verify:** `go test ./internal/planning/ -run Pool -v`

---

### perf-07 — ListPRs() no pagination
**file:** `internal/cache/sqlite.go:114-181`

**action:** Add `ListPRsPaginated(repo, cursor, limit int)` that returns `[]*PR, nextCursor`. Add `ListPRsStream(repo, ch chan<- *PR)` for bulk operations. Update callers to use cursor pagination in batches of 500.

**test:** Load 5500 PRs. Time `ListPRs()` call. With pagination (500/batch): memory stable. Without: RSS grows to ~500MB+.
**verify:** Run with `GOGC=50` and monitor RSS via `ps -o rss= -p $(pgrep pratc)`.

---

### perf-08 — math/rand for jitter
**file:** `internal/github/client.go:474-476`

**action:** Replace `rand.Int63n(...)` from `math/rand` with `crypto/rand.Int63()`. Read bytes via `crypto/rand.Int63()`. Fall back to `math/rand` with `rand.NewSource(time.Now().UnixNano())` only if crypto/rand fails (defensive).

**test:** Run 1000 jittered backoff samples. Verify distribution is uniform over [d, d*1.25]. Statistical test: Kolmogorov-Smirnov, p>0.01.
**verify:** `go test -run=Jitter ./internal/github/ -v`

---

### perf-09 — Bootstrap loads all PRs
**file:** `internal/sync/worker.go:73-81`

**action:** Add cursor-based pagination to bootstrap. Fetch PRs in batches of 500 using GitHub's `after: cursor` GraphQL param. Stream into SQLite rather than holding in memory.

**test:** With 5500 PRs: monitor RSS during sync. Before: loads all into slice. After: streaming insert, RSS should stay <100MB.
**verify:** `time pratc sync --repo owner/name` and `ps aux | grep pratc` RSS column.

---

## Security (5 items)

---

### sec-01 — Python path hijacking
**file:** `internal/ml/bridge.go:259-267`

**action:** Hardcode `python3` path via build-time flag or env var `PRATC_PYTHON_BIN`. Do not search PATH at runtime for the python binary. Validate `workDir` is under `$HOME/pratc` or the repo root, not arbitrary user-controlled paths.

**test:** Set a malicious `python` in PATH before `$PATH`. Run ML bridge. Verify it uses the hardcoded/configured python, not the PATH one.
**verify:** `strace -f -e execve pratc analyze 2>&1 | grep python` — should show only the intended binary path.

---

### sec-02 — math/rand for jitter (same root cause as perf-08)
**file:** `internal/github/client.go:475`

**action:** Same fix as perf-08 — use `crypto/rand`. Document that jitter must be unpredictable to prevent timing attacks on rate limit windows.

**test:** Same as perf-08.
**verify:** Same as perf-08.

---

### sec-03 — Race condition in WebSocket origins
**file:** `internal/monitor/server/websocket.go:17-20`

**action:** Add a `sync.RWMutex` protecting `allowedOrigins` and `parsedAllowedOrigins`. `SetAllowedOrigins()` acquires write lock. `isValidOriginURL()` acquires read lock. Alternatively move these into a struct with a mutex and pass as dependency.

**test:** Run `go race -test ./internal/monitor/server/ -run WebSocket`. 10 goroutines calling `SetAllowedOrigins` concurrently with 100 goroutines reading.
**verify:** `go test -race ./internal/monitor/server/`

---

### sec-04 — API key not constant-time
**file:** `internal/cmd/serve.go:911`

**action:** Replace `providedKey != apiKey` with `subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) == 1`. Import `crypto/subtle`.

**test:** Verify key comparison still works for correct keys and rejects incorrect keys. Add test with keys of same length and different length.
**verify:** `go test ./internal/cmd/ -run APIKey -v`

---

### sec-05 — YAML import without validation
**file:** `internal/settings/store.go:175`

**action:** Add a schema validation step after `yaml.Unmarshal`. Use manual field validation. Reject YAML with unexpected top-level keys. Log import source (filepath) for audit.

**test:** Import a YAML with extra fields `{extra: true}`. Should either strip unknowns or error. Import YAML with `../etc/passwd` path traversal in a field — should reject.
**verify:** `go test ./internal/settings/ -run Import -v`

---

## Slop (16 items)

---

### slop-01 — Port 8080 default
**file:** `internal/cmd/serve.go:79`, `cmd/pratc/main_test.go:67-68`

**action:** Change default from `8080` to `7400`. Update test assertion from `"8080"` to `"7400"`. Update AGENTS.md if it documents this.

**test:** Run `pratc serve` with no flags. Verify port 7400 is shown in startup log.
**verify:** `go test ./cmd/pratc/ -run DefaultPort -v`

---

### slop-02 — Bare return err (settings)
**file:** `internal/settings/store.go:113,117,126,146,172,180,187,277`

**action:** Wrap each bare `return err` with `fmt.Errorf("store.operationName: %w", err)` with appropriate context string (e.g., `"store.list: %w"`, `"store.upsert: %w"`).

**test:** `go vet ./internal/settings/` — should report zero issues after fix.
**verify:** `go test ./internal/settings/ -v`

---

### slop-03 — Bare return err (repo)
**file:** `internal/repo/mirror.go:155,252,268`

**action:** Same pattern — `fmt.Errorf("repo.mirror.context: %w", err)`.

**test:** `go vet ./internal/repo/`
**verify:** `go test ./internal/repo/ -v`

---

### slop-04 — Bare return err (workflow)
**file:** `internal/cmd/workflow.go:77,82,102,105,120,124,127,133,136,142,145,151,154`

**action:** Wrap with `fmt.Errorf("workflow.actionName: %w", err)` per call site.

**test:** `go vet ./internal/cmd/`
**verify:** `go test ./internal/cmd/ -run Workflow -v`

---

### slop-05 — Bare return err (serve + analyze)
**file:** `internal/cmd/serve.go:924,941,1036`, `internal/cmd/analyze.go:210,219`

**action:** Same wrapping pattern with context prefixes.

**test:** `go vet ./internal/cmd/`
**verify:** `go test ./internal/cmd/ -v`

---

### slop-06 — Bare return err (plan)
**file:** `internal/cmd/plan.go:45,62`

**action:** Same wrapping pattern.

**test:** `go vet ./internal/cmd/`
**verify:** `go test ./internal/cmd/ -run Plan -v`

---

### slop-07 — 6,651 LOC dead code in planning/
**file:** `internal/planning/`

**action:** Either (A) write integration test wiring PoolSelector into Plan() and profile improvement, or (B) delete the directory and add a commit note. If keeping, add `//go:build wired` build tag to files that are production-ready, separate from `//go:build unwired` or no tag.

**test:** If deleting: verify `go build ./...` still works, grep for any remaining references.
**verify:** `go test ./internal/planning/ -list .` — if dead, tests may not exist.

---

### slop-08 — Magic scoring numbers
**file:** `internal/filter/scorer.go:14-51`

**action:** Extract named constants at top of file:
```go
const (
    ciSuccessScore     = 3
    ciPendingScore     = 1
    ciFailureScore     = -2
    approvedScore      = 2
    mergeableScore     = 1
    ageDivisor         = 15
    maxAgeBoost        = 2
    botBoost           = 0.5
)
```
Replace all numeric literals in scoring logic with constants. Document each in comments.

**test:** Existing scorer tests should pass unchanged. Add unit test explicitly checking constant values.
**verify:** `go test ./internal/filter/ -run Scorer -v`

---

### slop-09 — Magic DefaultPoolCap=100
**file:** `internal/filter/pipeline.go:34`

**action:** Add `const DefaultPoolCap = 100` to `internal/filter/pipeline.go` or `internal/types/`. Use constant name instead of literal `100`. Add godoc explaining why 100.

**test:** Run filter pipeline with various pool sizes. Output unchanged.
**verify:** `go test ./internal/filter/ -v`

---

### slop-10 — Analyze()/Plan() too large
**file:** `internal/app/service.go`

**action:** Extract sub-functions. For `Analyze()`: pull `enrichFromGraphQL()`, `classifyDuplicates()`, `buildClusters()` into separate methods. For `Plan()`: extract `buildCandidatePool()`, `computeTiering()`, `generateCombinations()` into separate methods. Each <50 LOC.

**test:** All existing service tests pass. No behavior change.
**verify:** `go test ./internal/app/ -v`

---

### slop-11 — Settings API route mismatch
**file:** `internal/cmd/serve.go`, `web/src/lib/api.ts`

**action:** Align server and client. Pick one convention: either all `/api/settings` (repo-scope via header) or `/api/settings?repo=owner/name`. Document in `contracts/` directory. Update server route and client fetch call.

**test:** Run web app, navigate to settings. Verify GET and PUT work end-to-end.
**verify:** `curl localhost:7400/api/settings` and `curl -X PUT localhost:7400/api/settings -d '{}'`

---

### slop-12 — ETag TODOs
**file:** `internal/github/etag.go:87,95,96`

**action:** Either implement ETag support (store etags in `ci_status` table, send `If-None-Match` header, treat 304 as cache hit) or remove the TODO comments and add `// TODO(v2.0): implement ETag conditional requests` at package level. Don't leave half-baked code.

**test:** If implementing: add test that 304 response skips JSON parsing. If removing: verify no callers rely on etag functions.
**verify:** `go test ./internal/github/ -run ETag -v`

---

### slop-13 — print() in Python ML service
**file:** `ml-service/src/pratc_ml/models.py:524`

**action:** Remove `print()` call. If debugging output is needed, use a logger (e.g., `logging.getLogger(__name__).debug(...)`). Ensure stdout is clean for JSON protocol.

**test:** Run ML service, call an action. Verify stdout is valid JSON with no stray output.
**verify:** `echo '{"action":"health"}' | python3 -m pratc_ml.cli > /tmp/out.json && cat /tmp/out.json` — must be valid JSON.

---

### slop-14 — CORS hardcoded
**file:** `internal/cmd/serve.go`

**action:** Add env var `PRATC_CORS_ORIGINS` (comma-separated). Default to `localhost:3000` for dev. Parse and validate URLs (must have scheme `http://` or `https://`). Reject `file://` or wildcard in production.

**test:** Set `PRATC_CORS_ORIGINS=https://example.com`. Send request with `Origin: https://example.com` — should succeed. Send `Origin: https://evil.com` — should 403.
**verify:** `curl -H "Origin: https://evil.com" -I localhost:7400/api/health`

---

### slop-15 — Zero test coverage
**file:** `internal/settings/`, `internal/repo/`, `internal/sync/`

**action:** Write table-driven tests for each exported function. Minimum coverage target: 60% for each package. Use `go test -coverprofile=c.out ./internal/X/` and `go tool cover -html=c.out`.

**test:** `go test ./internal/settings/ ./internal/repo/ ./internal/sync/ -coverprofile=c.out`
**verify:** `go tool cover -func=c.out | grep total` — should show coverage %.

---

### slop-16 — Test expects port 8080
**file:** `cmd/pratc/main_test.go:67-68`

**action:** Change expected default from `8080` to `7400`. Run `go test ./cmd/pratc/ -run TestDefaultPort -v`.

**test:** `go test ./cmd/pratc/ -run TestDefaultPort -v`
**verify:** Test passes with output showing `7400`.

---

## Summary

| Category | Count | Quick Wins |
|----------|-------|------------|
| PERF | 9 | perf-06, perf-08 |
| SEC  | 5 | sec-02, sec-04 |
| SLOP | 16 | slop-01, slop-02..06, slop-13, slop-16 |
| **TOTAL** | **30** | **14** |
