#!/usr/bin/env bash
# scripts/release.sh
# Create a new release by tagging and pushing to GitHub.
# This triggers the GitHub Actions release workflow.
#
# Usage: ./scripts/release.sh
#   or:  make release

set -euo pipefail

source "$(dirname "$0")/common/load-config.sh"

echo -e "${BOLD}${CYAN}Release Preparation for $APP_DISPLAY_NAME${NC}"
echo -e "${CYAN}===================================${NC}"
echo ""
echo -e "${BOLD}Version:${NC} ${GREEN}$VERSION${NC}"
echo -e "${BOLD}Tag:${NC}     ${GREEN}$TAG${NC}"
echo ""

# ============================================================================
# 1. Pre-flight checks
# ============================================================================

echo -e "${BOLD}${CYAN}Pre-flight checks:${NC}"
echo -e "${CYAN}-------------------${NC}"

# Check for uncommitted changes
if ! git -C "$REPO_ROOT" diff-index --quiet HEAD -- 2>/dev/null; then
    echo -e "${RED}Error: You have uncommitted changes.${NC}"
    echo -e "${YELLOW}Please commit or stash your changes before creating a release.${NC}"
    exit 1
fi
echo -e "${GREEN}  No uncommitted changes${NC}"

# Check current branch
CURRENT_BRANCH=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD)
echo -e "${BOLD}  Branch:${NC} $CURRENT_BRANCH"

# Check if tag already exists locally
if git -C "$REPO_ROOT" rev-parse "$TAG" >/dev/null 2>&1; then
    echo -e "${RED}Error: Tag $TAG already exists locally.${NC}"
    echo -e "${YELLOW}Update the version in scripts/.config.json or run: make release-clean${NC}"
    exit 1
fi
echo -e "${GREEN}  Tag $TAG does not exist locally${NC}"

# Check if remote release exists (requires gh CLI)
if command -v gh &>/dev/null; then
    if gh release view "$TAG" --repo "$(get_repo_slug)" &>/dev/null 2>&1; then
        echo -e "${RED}Error: GitHub release $TAG already exists.${NC}"
        echo -e "${YELLOW}Run: make release-clean${NC}"
        exit 1
    fi
    echo -e "${GREEN}  No existing GitHub release for $TAG${NC}"
else
    echo -e "${YELLOW}  Skipping remote check (gh CLI not installed)${NC}"
fi

# ============================================================================
# 2. Run tests
# ============================================================================

echo ""
echo -e "${BOLD}${CYAN}Running tests...${NC}"
if ! go test -C "$REPO_ROOT" ./... 2>&1; then
    echo -e "${RED}Tests failed! Fix them before releasing.${NC}"
    exit 1
fi
echo -e "${GREEN}All tests passed${NC}"

# ============================================================================
# 3. Extract and display changelog
# ============================================================================

echo ""
echo -e "${BOLD}${CYAN}Changelog for v$VERSION:${NC}"
echo -e "${CYAN}-------------------------------------------${NC}"

CHANGELOG_CONTENT=$(get_version_changelog)

if [[ -z "$CHANGELOG_CONTENT" ]]; then
    echo -e "${RED}Error: No changelog found for version $VERSION in docs/CHANGELOG.md${NC}"
    echo -e "${YELLOW}Please add release notes. Expected format:${NC}"
    echo ""
    echo "## [$VERSION] - $(date +%Y-%m-%d)"
    echo ""
    echo "### Added"
    echo "- Description of changes..."
    exit 1
fi

echo "$CHANGELOG_CONTENT"
echo -e "${CYAN}-------------------------------------------${NC}"
echo ""

# ============================================================================
# 4. Confirm and create release
# ============================================================================

read -p "Create release $TAG? (y/N): " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo -e "${YELLOW}Release cancelled.${NC}"
    exit 0
fi

echo ""
echo -e "${BOLD}${CYAN}Creating release...${NC}"

# Create lightweight tag
echo -e "${YELLOW}Creating tag $TAG...${NC}"
git -C "$REPO_ROOT" tag "$TAG"

# Push tag to trigger GitHub Actions
echo -e "${YELLOW}Pushing tag to GitHub...${NC}"
git -C "$REPO_ROOT" push origin "$TAG"

echo ""
echo -e "${BOLD}${GREEN}Release initiated successfully!${NC}"
echo ""
echo -e "${CYAN}GitHub Actions is now:${NC}"
echo -e "  1. Validating version"
echo -e "  2. Running tests"
echo -e "  3. Building frontend"
echo -e "  4. Cross-compiling binaries for 5 platforms"
echo -e "  5. Packaging archives with checksums"
echo -e "  6. Publishing GitHub Release"
echo ""

SLUG=$(get_repo_slug)
echo -e "${BOLD}Monitor progress:${NC}"
echo -e "  https://github.com/$SLUG/actions"
echo ""
echo -e "${BOLD}Release page (once complete):${NC}"
echo -e "  https://github.com/$SLUG/releases/tag/$TAG"
echo ""
echo -e "Run ${CYAN}make release-status${NC} to check workflow status."
echo ""
