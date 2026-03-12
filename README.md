# prATC

prATC (PR Air Traffic Control) is a self-hostable system for large-scale pull request triage and merge planning.

## Planned Deliverables

- Go CLI: `pratc analyze`, `cluster`, `graph`, `plan`, `serve`
- Python ML service for clustering and duplicate detection
- TypeScript dashboard with ATC overview and triage inbox
- Docker Compose stack for local ML and hosted-light profiles
- SQLite cache with incremental GitHub sync

## Scaffold Status

This repository currently contains the Wave 1 foundation scaffold:

- Go module initialized at `github.com/jeffersonnunn/pratc`
- CLI entrypoint at `cmd/pratc/main.go`
- Build orchestration in `Makefile`
- Container stubs in `Dockerfile.cli`, `Dockerfile.web`, and `docker-compose.yml`
- Root directory layout for Go, Python, web, and fixtures workstreams

## Commands

```bash
make verify-env
make build
make test
docker-compose config
```

## Repository Layout

```text
cmd/pratc/
internal/
ml-service/
web/
fixtures/
```
