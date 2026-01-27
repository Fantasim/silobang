#!/usr/bin/env bash
# scripts/release-clean.sh
# Clean up a failed release: delete local tag, remote tag, and GitHub release.
# Use this to retry a release after fixing issues.
#
# Usage: ./scripts/release-clean.sh
#   or:  make release-clean

set -euo pipefail

source "$(dirname "$0")/common/load-config.sh"

echo -e "${BOLD}${CYAN}$APP_DISPLAY_NAME Release Cleanup${NC}"
echo -e "${CYAN}==================================${NC}"
echo ""
echo -e "${BOLD}Version:${NC} $VERSION"
echo -e "${BOLD}Tag:${NC}     $TAG"
echo ""

SLUG=$(get_repo_slug)
CLEANED=0

# ============================================================================
# 1. Delete GitHub release (if exists)
# ============================================================================

if command -v gh &>/dev/null; then
    if gh release view "$TAG" --repo "$SLUG" &>/dev/null 2>&1; then
        echo -e "${YELLOW}Deleting GitHub release $TAG...${NC}"
        gh release delete "$TAG" --repo "$SLUG" --yes --cleanup-tag 2>/dev/null || true
        echo -e "${GREEN}  GitHub release deleted${NC}"
        CLEANED=1
    else
        echo -e "  No GitHub release for $TAG"
    fi
else
    echo -e "${YELLOW}  Skipping GitHub release check (gh CLI not installed)${NC}"
fi

# ============================================================================
# 2. Delete remote tag (if exists)
# ============================================================================

if git -C "$REPO_ROOT" ls-remote --tags origin "$TAG" 2>/dev/null | grep -q "$TAG"; then
    echo -e "${YELLOW}Deleting remote tag $TAG...${NC}"
    git -C "$REPO_ROOT" push origin --delete "$TAG" 2>/dev/null || true
    echo -e "${GREEN}  Remote tag deleted${NC}"
    CLEANED=1
else
    echo -e "  No remote tag $TAG"
fi

# ============================================================================
# 3. Delete local tag (if exists)
# ============================================================================

if git -C "$REPO_ROOT" rev-parse "$TAG" >/dev/null 2>&1; then
    echo -e "${YELLOW}Deleting local tag $TAG...${NC}"
    git -C "$REPO_ROOT" tag -d "$TAG"
    echo -e "${GREEN}  Local tag deleted${NC}"
    CLEANED=1
else
    echo -e "  No local tag $TAG"
fi

# ============================================================================
# 4. Clean local release artifacts (if any)
# ============================================================================

if [[ -d "$RELEASE_DIR" ]]; then
    echo -e "${YELLOW}Removing local release artifacts...${NC}"
    rm -rf "$RELEASE_DIR"
    echo -e "${GREEN}  Release artifacts removed${NC}"
    CLEANED=1
fi

echo ""
if [[ "$CLEANED" -gt 0 ]]; then
    echo -e "${BOLD}${GREEN}Cleanup complete.${NC}"
    echo -e "You can now run ${CYAN}make release${NC} to retry."
else
    echo -e "Nothing to clean up for $TAG."
fi
echo ""
