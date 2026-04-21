# Installation Guide

This guide covers installing prATC (PR Air Traffic Control) on Linux and macOS systems.

## Quick Install (Recommended)

One-line installation for most users:

```bash
curl -fsSL https://raw.githubusercontent.com/Mind-Dragon/pratc/main/scripts/install.sh | bash
```

This will:
1. Download the latest pre-built binary (or build from source)
2. Install to `~/.local/bin`
3. Add to your PATH
4. Create example configuration

**Post-install:** Restart your terminal or run `source ~/.bashrc` (or `~/.zshrc`).

## Manual Installation

### Prerequisites

| Tool | Required | Purpose |
|------|----------|---------|
| Go 1.21+ | Yes (for building) | Compile prATC binary |
| Git | Yes | Clone repository |
| Python 3.11+ | Optional | Local ML clustering service |
| uv | Optional | Python package management |
| Docker | Optional | Containerized deployment |
| Bun | Optional | Optional: web UI development only (deprecated in v1.6) |

### Install Prerequisites

**macOS (Homebrew):**
```bash
brew install go git python uv
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install golang-go git python3 python3-pip
pip3 install uv
```

**Arch Linux:**
```bash
sudo pacman -S go git python uv
```

### Build from Source

```bash
# Clone repository
git clone https://github.com/Mind-Dragon/pratc.git
cd pratc

# Verify environment
make verify-env

# Build binary
make build

# Run tests (optional)
make test

# Install binary
sudo cp bin/pratc /usr/local/bin/
```

### Verify Installation

```bash
pratc --help | sed -n '1,2p'
```

Expected output begins with:
```text
Harness Optimizer v1.6.0 built on <timestamp>
Using Config from: settings=... | cache=...
```

## Configuration

### Environment Variables

Create `~/.pratc/.env` only if you prefer environment-based auth. For most users, `gh auth login` is cleaner and keeps tokens out of shell history.

```bash
# GitHub authentication (preferred: `gh auth login`; env vars remain supported)
export GITHUB_TOKEN=<github-token>

# API server configuration
export PRATC_PORT=7400
export PRATC_DB_PATH=~/.pratc/pratc.db
export PRATC_SETTINGS_DB=~/.pratc/settings.db

# Rate limiting (optional, defaults shown)
export PRATC_RATE_LIMIT=5000
export PRATC_RESERVE_BUFFER=200
export PRATC_RESET_BUFFER=15

# HTTP client tuning (optional)
export PRATC_HTTP_MAX_IDLE=100
export PRATC_HTTP_MAX_IDLE_PER_HOST=10
export PRATC_HTTP_IDLE_TIMEOUT=90
export PRATC_HTTP_TIMEOUT=30

# ML backend (optional)
export PRATC_ANALYSIS_BACKEND=local  # or 'remote'
```

### GitHub Token Setup

prATC requires a GitHub personal access token (PAT) with `repo` scope:

1. Visit https://github.com/settings/tokens
2. Click "Generate new token (classic)"
3. Select scopes: `repo` (full control of private repositories)
4. Generate token and copy it
5. Set environment variable if you are not using `gh auth login`:
   ```bash
   export GITHUB_TOKEN=<github-token>
   ```

**Security note:** Never commit tokens or other credentials to version control. Prefer `gh auth login`, a secrets manager, or environment injection over shell-history copy/paste.

## Deployment Options

### Local Development

```bash
# Start API server
pratc serve --port=7400

# In another terminal, run the CLI for human operators
pratc analyze --repo=owner/repo --format=json

# Or query the API directly
curl http://localhost:7400/api/health
```

### Production Deployment

For production use, consider:

1. **Systemd service** — Run `pratc serve` as a background service
2. **Reverse proxy** — nginx or Caddy in front of prATC API
3. **TLS termination** — Use Let's Encrypt or enterprise CA
4. **Backup strategy** — Regular SQLite database backups
5. **Monitoring** — Logs at INFO level for sync, resume, and rate-limit events

## Directory Structure

After installation:

```
~/.pratc/
├── pratc.db              # Main SQLite cache
├── settings.db           # Settings database
├── config.example.json   # Example configuration
└── api-key              # API authentication key (mode 0600)

~/.cache/pratc/
└── repos/               # Git mirrors for synced repositories

~/.local/bin/
└── pratc                # Binary (or /usr/local/bin/pratc)
```

## Uninstallation

```bash
# Remove binary
rm ~/.local/bin/pratc  # or sudo rm /usr/local/bin/pratc

# Remove configuration (optional)
rm -rf ~/.pratc
rm -rf ~/.cache/pratc

# Remove from PATH (edit shell rc file)
# Remove these lines from ~/.bashrc or ~/.zshrc:
#   # prATC installation
#   export PATH="$HOME/.local/bin:$PATH"
```

## Troubleshooting

### "command not found: pratc"

Your PATH is not configured. Add to your shell rc file:
```bash
export PATH="$HOME/.local/bin:$PATH"
```

Then reload: `source ~/.bashrc` or `source ~/.zshrc`.

### "GITHUB_TOKEN is required"

Set your GitHub token:
```bash
export GITHUB_TOKEN=ghp_your_token_here
```

### "permission denied" on binary

Make executable:
```bash
chmod +x ~/.local/bin/pratc
```

### Build fails with "package not found"

Update Go modules:
```bash
go mod tidy
go mod download
```

### Python ML service fails

Install dependencies:
```bash
cd ml-service
uv sync
```

## Next Steps

After installation:

1. **Read [RATELIMITS.md](RATELIMITS.md)** — Understand GitHub API rate limits and cache-first defaults
2. **Run your first full workflow**: `pratc workflow --repo=owner/repo --progress`
3. **Query the API**: `curl http://localhost:7400/api/health` or use `pratc audit`
4. **Read [README.md](README.md)** — Feature overview, CLI commands, and architecture

For usage examples, see [README.md](README.md).

## Support

- **Documentation**: https://github.com/Mind-Dragon/pratc/tree/main/docs
- **Issues**: https://github.com/Mind-Dragon/pratc/issues
- **Commercial licensing**: jefferson@heimdallstrategy.com
