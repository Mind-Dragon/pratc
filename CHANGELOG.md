# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added `ROADMAP.md` to document near-term priorities for the review-engine release track.
- Added a v1.3 review-engine design document under `docs/plans/` for evidence-backed PR review workflows.

### Changed

- Updated `README.md` to surface current project status, the reserved prATC port range, and links to roadmap/review design docs.
- Hardened `.gitignore` to exclude local secret material, TypeScript incremental build outputs, and stray browser screenshot artifacts from release commits.

### Removed

- Removed a tracked `web/tsconfig.tsbuildinfo` build artifact from version control.

## [0.2.0] - 2026-03-23

### Added

- Added `PRATC_CACHE_DIR` environment variable to control mirror storage location
- Added `mirror list` command to display all synced repositories with size and last sync timestamp
- Added `mirror info` command to show detailed mirror statistics including disk usage
- Added `mirror clean` command to remove all mirrors
- Added `--use-cache-first` flag to all CLI commands for cache-first analysis workflow
- Added automatic fallback to live fetch when cache data is missing or expired
- Added `GetChangedFiles` method to extract changed files from git mirror using diff
- Added batch git fetch operations processing up to 100 refs per fetch call
- Added parallel file extraction with worker pool supporting up to 10 concurrent git operations
- Added background sync auto-trigger when analyzing a repository for the first time
- Added CLI warning when no recent sync data exists, with recommended workflow guidance
- Added `--force` flag to skip sync warnings and proceed with live fetch

### Changed

- Changed default mirror storage location from project directory to `~/.cache/pratc/repos/` (XDG cache directory)
- Modified service layer to check SQLite cache and git mirror before live fetch when `--use-cache-first` is enabled
- Reduced git fetch operations from N sequential calls to ceil(N/100) batched calls
- File content retrieval now uses git mirror diff instead of sequential GraphQL calls when mirror is available

### Fixed

- Fixed mirror storage polluting project workspace by relocating to standard cache directory
- Fixed excessive GitHub API calls for file content by using local mirror instead of GraphQL

## [0.1.0] - 2026-03-15

### Added

- Initial release of prATC (PR Air Traffic Control)
- CLI commands: `analyze`, `cluster`, `graph`, `plan`, `serve`, `sync`, `audit`
- Web dashboard with ATC overview, triage inbox, dependency graphs, and merge planning
- Python ML service for PR clustering and duplicate detection
- SQLite cache with incremental GitHub sync
- GitHub GraphQL client with rate limiting and retry logic
- Pre-filter pipeline for draft, conflict, CI status, and bot detection
- Combinatorial formula engine for merge planning
- Dependency graph generation with DOT output
- Settings management with YAML import/export
- Docker Compose profiles for local-ml and minimax-light deployments

[Unreleased]: https://github.com/owner/repo/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/owner/repo/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/owner/repo/releases/tag/v0.1.0
