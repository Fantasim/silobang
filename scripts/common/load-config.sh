#!/usr/bin/env bash
# scripts/common/load-config.sh
# Shared configuration loader — sourced by other scripts, never run directly.
# Requires: jq

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONFIG_FILE="$REPO_ROOT/scripts/.config.json"

# ============================================================================
# Validate dependencies
# ============================================================================

if ! command -v jq &>/dev/null; then
    echo "ERROR: jq is required but not installed." >&2
    echo "  Install: sudo apt-get install jq (Ubuntu) or brew install jq (macOS)" >&2
    exit 1
fi

if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE" >&2
    exit 1
fi

if ! jq empty "$CONFIG_FILE" 2>/dev/null; then
    echo "ERROR: Invalid JSON in $CONFIG_FILE" >&2
    exit 1
fi

# ============================================================================
# Read configuration values
# ============================================================================

APP_NAME=$(jq -r '.app_name' "$CONFIG_FILE")
APP_DISPLAY_NAME=$(jq -r '.app_display_name' "$CONFIG_FILE")
VERSION=$(jq -r '.version' "$CONFIG_FILE")
REPO_URL=$(jq -r '.repo_url' "$CONFIG_FILE")

# Derived values
TAG="v${VERSION}"
RELEASE_DIR="$REPO_ROOT/release"
VERSION_PACKAGE="silobang/internal/version"
LDFLAGS="-X ${VERSION_PACKAGE}.Version=${VERSION} -s -w"
ENTRY_POINT="./cmd/silobang"
FRONTEND_DIR="web-src"

# ============================================================================
# Colors
# ============================================================================

RED="\033[0;31m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
BLUE="\033[0;34m"
CYAN="\033[0;36m"
BOLD="\033[1m"
NC="\033[0m"

# ============================================================================
# Platform definitions
# ============================================================================

# Format: "label:GOOS:GOARCH:CC:archive_type"
ALL_PLATFORMS=(
    "linux-amd64:linux:amd64:gcc:tar.gz"
    "linux-arm64:linux:arm64:aarch64-linux-gnu-gcc:tar.gz"
    "macos-amd64:darwin:amd64:o64-clang:tar.gz"
    "macos-arm64:darwin:arm64:oa64-clang:tar.gz"
    "windows-amd64:windows:amd64:x86_64-w64-mingw32-gcc:zip"
)

# ============================================================================
# Helper functions
# ============================================================================

# Extract changelog text for current version from docs/CHANGELOG.md
# Returns all content between ## [version] and the next ## heading
get_version_changelog() {
    local changelog_file="$REPO_ROOT/docs/CHANGELOG.md"

    if [[ ! -f "$changelog_file" ]]; then
        echo ""
        return 0
    fi

    awk -v version="$VERSION" '
        /^## \[/ {
            if ($0 ~ "\\[" version "\\]") {
                in_section = 1
                next
            } else if (in_section) {
                exit
            }
        }
        in_section { print }
    ' "$changelog_file"
}

# Get GitHub repo slug from remote URL (e.g., "Fantasim/silobang")
get_repo_slug() {
    git -C "$REPO_ROOT" remote get-url origin 2>/dev/null \
        | sed 's/.*github\.com[:/]\(.*\)\.git$/\1/' \
        | sed 's/.*github\.com[:/]\(.*\)$/\1/'
}

echo -e "${CYAN}Config loaded: $APP_DISPLAY_NAME v$VERSION${NC}"
