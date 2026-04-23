# prATC TODO — ML Reliability + Honesty Slice

## Goal

Make the optional ML path honest about what it did, reduce avoidable repeat cost, and remove the most misleading shortcuts without expanding into speculative roadmap work.

This slice is done when:
- ML-backed paths clearly report whether they used embeddings or heuristic fallback
- fallback is visible and test-covered instead of silently collapsing to a model string
- embedding requests stop recomputing the same texts every run
- embedding text stops pretending the first 5 files are the whole PR
- heuristic scoring constants and duplicate-candidate settings are explicit, centralized, and covered by tests
- the Python `analyze` action no longer ships as an undocumented empty-success stub

## Source of truth

- `GUIDELINE.md` — honesty, explainability, no silent exclusion
- `ARCHITECTURE.md` — optional ML bridge, explicit layered behavior
- `internal/ml/bridge.go` — Go↔Python subprocess contract
- `internal/app/service.go` — cluster/analyze orchestration and fallback surface
- `ml-service/src/pratc_ml/cli.py` — action dispatch and analyze behavior
- `ml-service/src/pratc_ml/clustering.py` — embedding use and cluster response surface
- `ml-service/src/pratc_ml/duplicates.py` — duplicate candidate generation and embedding path
- `ml-service/src/pratc_ml/similarity.py` — heuristic scoring

## Workstream 1 — Make ML fallback explicit

- [x] Extend the Go/Python ML response contract so cluster/duplicate/analyze paths can say:
  - whether embeddings were used
  - whether heuristic fallback was used
  - why fallback happened (`backend_unavailable`, `provider_error`, `timeout`, `local_backend`, etc.)
- [x] Stop treating ML failure as a silent success path in the bridge/service layer; preserve optional behavior, but surface the degradation explicitly in output/logs
- [x] Add focused tests covering:
  - bridge unavailable
  - provider error
  - local backend intentional heuristic mode
  - successful embedding path

Files:
- `internal/ml/bridge.go`
- `internal/app/service.go`
- `ml-service/src/pratc_ml/cli.py`
- `ml-service/src/pratc_ml/clustering.py`
- `ml-service/src/pratc_ml/duplicates.py`

## Workstream 2 — Replace the analyze stub with an honest contract

- [x] Remove the current “empty analyzers” stub behavior as an implicit success case
- [x] Make `action=analyze` return an explicit not-implemented/degraded response until there is real analyzer output, or wire a minimal real analyzer if that is lower risk
- [x] Add CLI tests locking the response shape so Go can distinguish “no findings” from “feature not implemented”

Files:
- `ml-service/src/pratc_ml/cli.py`
- `internal/ml/bridge.go`
- `ml-service/tests/test_cli.py`

## Workstream 3 — Add embedding-result caching

- [x] Add a small, local embedding cache keyed by backend + model + normalized embedding text
- [x] Use the cache in both clustering and duplicate detection
- [x] Make cache hits/misses observable in tests and logs
- [x] Keep the first implementation local and simple; no distributed cache, no cross-host coordination

Files:
- `ml-service/src/pratc_ml/clustering.py`
- `ml-service/src/pratc_ml/duplicates.py`
- supporting ML service module(s) as needed
- `ml-service/tests/test_clustering.py`
- `ml-service/tests/test_duplicates.py`

## Workstream 4 — Make embedding text less misleading

- [x] Replace the current “title + body + first 5 files” embedding text builder with a bounded full-file summary that accounts for all changed files
- [x] Keep the text bounded and deterministic for large PRs
- [x] Add tests proving:
  - files beyond the fifth affect the embedding text
  - very large file lists stay bounded
  - ordering is deterministic

Files:
- `ml-service/src/pratc_ml/clustering.py`
- `ml-service/src/pratc_ml/duplicates.py`
- shared helper module if needed
- `ml-service/tests/test_clustering.py`
- `ml-service/tests/test_duplicates.py`

## Workstream 5 — Centralize heuristic knobs and LSH settings

- [x] Move heuristic weights and duplicate-candidate parameters out of scattered literals into one shared constant/config surface
- [x] Cover at minimum:
  - title/files/body heuristic weights
  - MinHash `num_perm`
  - LSH threshold derivation
- [x] Add regression tests so these settings are explicit and stable
- [x] If `num_perm=128` remains the default, document and test it as an intentional choice instead of an unexplained literal

Files:
- `ml-service/src/pratc_ml/similarity.py`
- `ml-service/src/pratc_ml/duplicates.py`
- `ml-service/src/pratc_ml/clustering.py`
- `ml-service/tests/test_duplicates.py`
- `ml-service/tests/test_clustering.py`

## Execution rules

- [x] Keep this slice limited to ML reliability/honesty; no speculative roadmap expansion
- [x] Every contract change gets a focused failing test before code changes
- [x] Preserve optional-ML behavior; do not make external embeddings mandatory
- [x] Do not claim semantic analysis where the system is still using heuristics
- [x] Keep output deterministic and machine-readable

## Out of scope for this slice

- [ ] ML feedback loops, operator-training capture, or model retraining
- [ ] Multi-repo ML behavior
- [ ] New hosted provider integrations
- [ ] Replacing heuristic clustering with a new ML architecture
- [ ] Large-scale planner/review pipeline changes outside the ML bridge contract
- [ ] Dashboard/UI work

## Verification commands

```bash
cd /home/agent/pratc && go test ./internal/ml ./internal/app
cd /home/agent/pratc/ml-service && UV_CACHE_DIR=/home/agent/pratc/.cache/uv uv run --extra dev pytest -v tests/test_cli.py tests/test_clustering.py tests/test_duplicates.py
cd /home/agent/pratc && make test-python
cd /home/agent/pratc && make test-go
cd /home/agent/pratc && go test -race ./...
```
