#!/usr/bin/env bash
# scripts/release-status.sh
# Check the status of the current version's release workflow.
#
# Usage: ./scripts/release-status.sh
#   or:  make release-status

set -euo pipefail

source "$(dirname "$0")/common/load-config.sh"

echo -e "${BOLD}${CYAN}$APP_DISPLAY_NAME Release Status${NC}"
echo -e "${CYAN}================================${NC}"
echo ""
echo -e "${BOLD}Config version:${NC} $VERSION"
echo -e "${BOLD}Expected tag:${NC}   $TAG"

# Check if tag exists locally
if git -C "$REPO_ROOT" rev-parse "$TAG" >/dev/null 2>&1; then
    echo -e "${BOLD}Local tag:${NC}      ${GREEN}EXISTS${NC} ($(git -C "$REPO_ROOT" rev-parse --short "$TAG"))"
else
    echo -e "${BOLD}Local tag:${NC}      ${YELLOW}NOT FOUND${NC}"
    echo ""
    echo -e "Run ${CYAN}make release${NC} to create a release."
    exit 0
fi

echo -e "${BOLD}Current commit:${NC} $(git -C "$REPO_ROOT" rev-parse --short HEAD)"
echo -e "${BOLD}Branch:${NC}         $(git -C "$REPO_ROOT" branch --show-current)"
echo ""

# Check GitHub CLI
if ! command -v gh &>/dev/null; then
    echo -e "${RED}Error: GitHub CLI (gh) is not installed.${NC}"
    echo -e "${YELLOW}Install: https://cli.github.com/${NC}"
    exit 1
fi

# ============================================================================
# Check workflow status
# ============================================================================

echo -e "${BOLD}${CYAN}GitHub Actions Workflow:${NC}"
echo ""

WORKFLOW_RUN=$(gh run list \
    --workflow=release.yml \
    --json status,conclusion,createdAt,databaseId \
    --limit 1 \
    --branch "$TAG" \
    --repo "$(get_repo_slug)" 2>/dev/null || echo "[]")

if [[ "$WORKFLOW_RUN" == "[]" || -z "$WORKFLOW_RUN" ]]; then
    echo -e "${YELLOW}No workflow runs found for tag $TAG${NC}"
    echo -e "The workflow may not have started yet, or the tag was not pushed."
    echo ""
    SLUG=$(get_repo_slug)
    echo -e "${BOLD}Check all workflows:${NC}"
    echo -e "  https://github.com/$SLUG/actions"
    exit 0
fi

STATUS=$(echo "$WORKFLOW_RUN" | jq -r '.[0].status')
CONCLUSION=$(echo "$WORKFLOW_RUN" | jq -r '.[0].conclusion')
RUN_ID=$(echo "$WORKFLOW_RUN" | jq -r '.[0].databaseId')
SLUG=$(get_repo_slug)

echo -e "${BOLD}Status:${NC} $STATUS"

if [[ "$STATUS" == "completed" ]]; then
    if [[ "$CONCLUSION" == "success" ]]; then
        echo -e "${BOLD}Result:${NC} ${GREEN}SUCCESS${NC}"
        echo ""

        # Check if release exists
        if gh release view "$TAG" --repo "$SLUG" &>/dev/null; then
            echo -e "${BOLD}${GREEN}Release published!${NC}"
            echo ""
            echo -e "${BOLD}URL:${NC}"
            echo -e "  https://github.com/$SLUG/releases/tag/$TAG"
            echo ""
            echo -e "${BOLD}Assets:${NC}"
            gh release view "$TAG" --repo "$SLUG" --json assets -q '.assets[] | "  \(.name)"'
            echo ""
            echo -e "${BOLD}Downloads:${NC}"
            gh release view "$TAG" --repo "$SLUG" --json assets -q '.assets[] | "  \(.name): \(.downloadCount) downloads"'
        else
            echo -e "${YELLOW}Workflow succeeded but release not found yet.${NC}"
        fi
    elif [[ "$CONCLUSION" == "failure" ]]; then
        echo -e "${BOLD}Result:${NC} ${RED}FAILED${NC}"
        echo ""
        echo -e "${BOLD}View logs:${NC}"
        echo -e "  gh run view $RUN_ID --log-failed"
        echo -e "  https://github.com/$SLUG/actions/runs/$RUN_ID"
        echo ""
        echo -e "${BOLD}To retry:${NC}"
        echo -e "  1. Fix any issues"
        echo -e "  2. Run: ${CYAN}make release-clean${NC}"
        echo -e "  3. Run: ${CYAN}make release${NC}"
    else
        echo -e "${BOLD}Result:${NC} ${YELLOW}$CONCLUSION${NC}"
    fi
elif [[ "$STATUS" == "in_progress" ]]; then
    echo -e "${YELLOW}Workflow is currently running...${NC}"
    echo ""
    echo -e "${BOLD}Watch progress:${NC}"
    echo -e "  gh run watch $RUN_ID"
    echo -e "  https://github.com/$SLUG/actions/runs/$RUN_ID"
else
    echo -e "${YELLOW}Status: $STATUS${NC}"
fi

echo ""
echo -e "${BOLD}Workflow URL:${NC}"
echo -e "  https://github.com/$SLUG/actions/runs/$RUN_ID"
echo ""
