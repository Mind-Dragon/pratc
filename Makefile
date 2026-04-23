SHELL := /bin/bash
PATH := /opt/homebrew/bin:$(PATH)

GO ?= go
PYTHON ?= python3
UV ?= uv
NODE ?= node
BUN ?= bun
DOCKER_COMPOSE ?= docker-compose
VERSION ?= 1.7.0
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
DIRTY ?= $(shell test -z "$$(git status --short 2>/dev/null)" && echo false || echo true)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/jeffersonnunn/pratc/internal/version.Version=$(VERSION) -X github.com/jeffersonnunn/pratc/internal/version.Commit=$(COMMIT) -X github.com/jeffersonnunn/pratc/internal/version.Dirty=$(DIRTY) -X github.com/jeffersonnunn/pratc/internal/version.BuildDate=$(BUILD_DATE)
GO_BUILD_CACHE := $(CURDIR)/.cache/go-build
GO_MOD_CACHE := $(CURDIR)/.cache/go-mod
GO_ENV := GOCACHE=$(GO_BUILD_CACHE) GOMODCACHE=$(GO_MOD_CACHE)
UV_CACHE_DIR := $(CURDIR)/.cache/uv

.PHONY: deps verify-env dev build test test-go test-python test-web lint docker-up docker-down clean

deps:
	@echo "Dependencies are managed per ecosystem during each task."

verify-env:
	@command -v $(GO) >/dev/null 2>&1 || { echo "missing go"; exit 1; }
	@command -v $(PYTHON) >/dev/null 2>&1 || { echo "missing python3"; exit 1; }
	@$(PYTHON) -c 'import sys; raise SystemExit(0 if sys.version_info >= (3, 11) else 1)' || { echo "python3.11+ required"; exit 1; }
	@command -v $(UV) >/dev/null 2>&1 || { echo "missing uv"; exit 1; }
	@command -v docker >/dev/null 2>&1 || { echo "missing docker"; exit 1; }
	@command -v $(DOCKER_COMPOSE) >/dev/null 2>&1 || { echo "missing docker-compose"; exit 1; }
	@echo "Core dependencies verified. Node/Bun optional for web UI development only."

dev:
	@echo "Run 'make dev-api' or 'make dev-web' after the next scaffold tasks land."

build:
	@mkdir -p bin
	@mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE)
	@$(GO_ENV) $(GO) build -ldflags "$(LDFLAGS)" -o bin/pratc ./cmd/pratc/

test: test-go test-python test-web

test-go:
	@if [ -d internal ]; then mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) && $(GO_ENV) $(GO) test -race -v ./...; else echo "skipping go tests"; fi

test-python:
	@if [ -f ml-service/pyproject.toml ]; then \
		mkdir -p $(UV_CACHE_DIR); \
		cd ml-service; \
		if UV_CACHE_DIR=$(UV_CACHE_DIR) $(UV) run --extra dev pytest -v; then \
			exit 0; \
		elif [ -x .venv/bin/python ]; then \
			echo "uv run failed; falling back to .venv/bin/python"; \
			./.venv/bin/python -m pytest -v tests; \
		else \
			echo "uv run failed; falling back to python3 -m pytest"; \
			$(PYTHON) -m pytest -v tests; \
		fi; \
	else \
		echo "skipping python tests; ml-service/pyproject.toml not present"; \
	fi

test-web:
	@if [ -f web/package.json ]; then \
		cd web; \
		if [ ! -d node_modules ]; then $(BUN) install; fi; \
		$(BUN) run test; \
	else \
		echo "skipping web tests; web/package.json not present"; \
	fi

lint:
	@if [ -d internal ]; then mkdir -p $(GO_BUILD_CACHE) $(GO_MOD_CACHE) && $(GO_ENV) $(GO) vet ./...; else echo "skipping go vet"; fi

docker-up:
	@$(DOCKER_COMPOSE) --profile local-ml up --build -d

docker-down:
	@$(DOCKER_COMPOSE) down --remove-orphans

clean:
	@rm -rf bin
