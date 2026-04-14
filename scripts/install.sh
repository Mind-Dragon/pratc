#!/usr/bin/env bash
#
# prATC Installer
# Installs PR Air Traffic Control (prATC) on Linux/macOS systems
#
# Usage: curl -fsSL https://raw.githubusercontent.com/Mind-Dragon/pratc/main/scripts/install.sh | bash
#
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="Mind-Dragon/pratc"
INSTALL_DIR="${PRATC_INSTALL_DIR:-$HOME/.local/bin}"
CACHE_DIR="${PRATC_CACHE_DIR:-$HOME/.cache/pratc}"
CONFIG_DIR="${PRATC_CONFIG_DIR:-$HOME/.pratc}"

# Helper functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=()
    
    # Check for curl
    if ! command -v curl &> /dev/null; then
        missing+=("curl")
    fi
    
    # Check for git
    if ! command -v git &> /dev/null; then
        missing+=("git")
    fi
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        log_error "Go is required but not installed"
        log_info "Install Go: https://go.dev/dl/"
        exit 1
    fi
    
    # Check Go version (1.21+)
    local go_version
    go_version=$(go version 2>&1 | cut -d' ' -f3 | cut -c3-)
    local major minor
    major=$(echo "$go_version" | cut -d'.' -f1)
    minor=$(echo "$go_version" | cut -d'.' -f2)
    
    if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 21 ]; }; then
        log_error "Go 1.21+ required (found: $go_version)"
        log_info "Install Go: https://go.dev/dl/"
        exit 1
    fi
    
    log_success "Go $go_version detected"
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        log_info "Install with your package manager, e.g.:"
        log_info "  macOS: brew install ${missing[*]}"
        log_info "  Ubuntu: sudo apt install ${missing[*]}"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

detect_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) 
            log_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
}

detect_os() {
    local os
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$os" in
        linux) echo "linux" ;;
        darwin) echo "darwin" ;;
        *) 
            log_error "Unsupported OS: $os"
            exit 1
            ;;
    esac
}

setup_directories() {
    log_info "Setting up directories..."
    
    mkdir -p "$INSTALL_DIR"
    mkdir -p "$CACHE_DIR"
    mkdir -p "$CONFIG_DIR"
    
    log_success "Directories created:"
    echo "  Install: $INSTALL_DIR"
    echo "  Cache:   $CACHE_DIR"
    echo "  Config:  $CONFIG_DIR"
}

build_from_source() {
    log_info "Building prATC from source..."
    
    local temp_dir
    temp_dir=$(mktemp -d)
    trap "rm -rf '$temp_dir'" EXIT
    
    log_info "Cloning repository..."
    git clone --depth 1 "https://github.com/${REPO}.git" "$temp_dir"
    cd "$temp_dir"
    
    log_info "Building binary..."
    if ! make build; then
        log_error "Build failed"
        exit 1
    fi
    
    log_success "Build complete"
    cp bin/pratc "${INSTALL_DIR}/pratc"
    chmod +x "${INSTALL_DIR}/pratc"
}

setup_environment() {
    log_info "Setting up environment..."
    
    # Add to PATH if not already present
    local shell_rc=""
    case "$SHELL" in
        */bash) shell_rc="$HOME/.bashrc" ;;
        */zsh) shell_rc="$HOME/.zshrc" ;;
        */fish) 
            echo "set -gx PATH $INSTALL_DIR \$PATH" >> "$HOME/.config/fish/config.fish"
            log_info "Added $INSTALL_DIR to fish PATH"
            return
            ;;
        *) 
            log_warn "Unknown shell: $SHELL. Please add $INSTALL_DIR to your PATH manually."
            return
            ;;
    esac
    
    if ! grep -q "$INSTALL_DIR" "$shell_rc" 2>/dev/null; then
        echo "" >> "$shell_rc"
        echo "# prATC installation" >> "$shell_rc"
        echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$shell_rc"
        log_info "Added $INSTALL_DIR to PATH in $shell_rc"
        log_info "Run 'source $shell_rc' or restart your terminal to apply"
    else
        log_success "PATH already configured"
    fi
}

create_example_config() {
    local config_file="$CONFIG_DIR/config.example.json"
    
    if [ ! -f "$config_file" ]; then
        cat > "$config_file" << 'EOF'
{
  "github": {
    "token_env": "GITHUB_TOKEN",
    "rate_limit": 5000,
    "reserve_buffer": 200
  },
  "api": {
    "port": 7400,
    "cors_origins": ["http://localhost:3000"]
  },
  "cache": {
    "dir": "~/.cache/pratc",
    "ttl_hours": 24
  },
  "ml": {
    "backend": "local",
    "duplicate_threshold": 0.90,
    "overlap_threshold": 0.70
  }
}
EOF
        log_success "Created example config: $config_file"
    fi
}

verify_installation() {
    log_info "Verifying installation..."
    
    if "${INSTALL_DIR}/pratc" --help > /dev/null 2>&1; then
        local version
        version=$("${INSTALL_DIR}/pratc" version 2>&1 | head -1)
        log_success "prATC installed successfully!"
        echo ""
        echo "  $version"
        echo ""
        echo "Next steps:"
        echo "  1. Set your GitHub token: export GITHUB_TOKEN=ghp_..."
        echo "  2. Sync a repo: pratc sync --repo=owner/repo"
        echo "  3. Analyze: pratc analyze --repo=owner/repo"
        echo "  4. Start dashboard: pratc monitor"
        echo ""
        echo "Documentation: https://github.com/${REPO}/blob/main/README.md"
        echo "Rate Limits:   https://github.com/${REPO}/blob/main/RATELIMITS.md"
        return 0
    else
        log_error "Installation verification failed"
        exit 1
    fi
}

# Main installation flow
main() {
    echo "╔════════════════════════════════════════╗"
    echo "║         prATC Installer                ║"
    echo "║   PR Air Traffic Control for GitHub    ║"
    echo "╚════════════════════════════════════════╝"
    echo ""
    
    check_prerequisites
    setup_directories
    
    local arch os
    arch=$(detect_arch)
    os=$(detect_os)
    
    log_info "Target: ${os}/${arch}"
    log_info "Note: prATC builds from source (no pre-built binaries)"
    
    build_from_source
    setup_environment
    create_example_config
    verify_installation
}

# Run main function
main "$@"
