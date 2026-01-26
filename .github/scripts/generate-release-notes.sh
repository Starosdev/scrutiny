#!/bin/bash
# Generate release notes from merged PRs between two tags
# Usage: ./generate-release-notes.sh <previous-tag> <new-tag>

set -e

PREV_TAG="${1:-$(git describe --tags --abbrev=0 HEAD~1 2>/dev/null || echo "")}"
NEW_TAG="${2:-$(git describe --tags --abbrev=0 HEAD 2>/dev/null || echo "HEAD")}"
REPO="${GITHUB_REPOSITORY:-Starosdev/scrutiny}"

if [ -z "$PREV_TAG" ]; then
    echo "Error: Could not determine previous tag" >&2
    exit 1
fi

echo "Generating release notes for $PREV_TAG..$NEW_TAG"

# Get the date of the previous tag
PREV_DATE=$(git log -1 --format=%aI "$PREV_TAG" 2>/dev/null || echo "1970-01-01T00:00:00Z")

# Initialize arrays for categorizing changes
declare -a FEATURES FIXES REFACTORS DOCS DEPS CICD OTHER

# Get all PRs merged after the previous tag
# We look at PR titles which follow conventional commit format
MERGED_PRS=$(gh pr list --repo "$REPO" --state merged --base master --json number,title,mergedAt,labels \
    --jq ".[] | select(.mergedAt > \"$PREV_DATE\") | \"\(.number)|\(.title)\"" 2>/dev/null || echo "")

# Also check PRs merged to develop that were included in release PRs
DEVELOP_PRS=$(gh pr list --repo "$REPO" --state merged --base develop --json number,title,mergedAt \
    --jq ".[] | select(.mergedAt > \"$PREV_DATE\") | \"\(.number)|\(.title)\"" 2>/dev/null || echo "")

# Combine and deduplicate
ALL_PRS=$(echo -e "$MERGED_PRS\n$DEVELOP_PRS" | sort -u | grep -v "^$" || true)

# Categorize PRs based on conventional commit prefixes in titles
while IFS='|' read -r pr_num pr_title; do
    [ -z "$pr_num" ] && continue

    # Skip release merge PRs
    if [[ "$pr_title" =~ ^Release:|^chore\(release\) ]]; then
        continue
    fi

    # Extract type from conventional commit format
    if [[ "$pr_title" =~ ^feat(\(.+\))?:|^feat!(\(.+\))?: ]]; then
        FEATURES+=("* ${pr_title#feat*: } ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    elif [[ "$pr_title" =~ ^fix(\(.+\))?:|^fix!(\(.+\))?: ]]; then
        FIXES+=("* ${pr_title#fix*: } ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    elif [[ "$pr_title" =~ ^refactor(\(.+\))?: ]]; then
        REFACTORS+=("* ${pr_title#refactor*: } ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    elif [[ "$pr_title" =~ ^docs(\(.+\))?: ]]; then
        DOCS+=("* ${pr_title#docs*: } ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    elif [[ "$pr_title" =~ ^ci(\(.+\))?: ]]; then
        CICD+=("* ${pr_title#ci*: } ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    elif [[ "$pr_title" =~ [Dd]ependen|[Uu]pdate.*go\.(mod|sum) ]]; then
        DEPS+=("* $pr_title ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    elif [[ ! "$pr_title" =~ ^chore ]]; then
        OTHER+=("* $pr_title ([#$pr_num](https://github.com/$REPO/pull/$pr_num))")
    fi
done <<< "$ALL_PRS"

# Generate markdown output
echo "## [$NEW_TAG](https://github.com/$REPO/compare/$PREV_TAG...$NEW_TAG) ($(date +%Y-%m-%d))"
echo ""

if [ ${#FEATURES[@]} -gt 0 ]; then
    echo "### Features"
    echo ""
    printf '%s\n' "${FEATURES[@]}"
    echo ""
fi

if [ ${#FIXES[@]} -gt 0 ]; then
    echo "### Bug Fixes"
    echo ""
    printf '%s\n' "${FIXES[@]}"
    echo ""
fi

if [ ${#REFACTORS[@]} -gt 0 ]; then
    echo "### Refactoring"
    echo ""
    printf '%s\n' "${REFACTORS[@]}"
    echo ""
fi

if [ ${#DOCS[@]} -gt 0 ]; then
    echo "### Documentation"
    echo ""
    printf '%s\n' "${DOCS[@]}"
    echo ""
fi

if [ ${#DEPS[@]} -gt 0 ]; then
    echo "### Dependencies"
    echo ""
    printf '%s\n' "${DEPS[@]}"
    echo ""
fi

if [ ${#CICD[@]} -gt 0 ]; then
    echo "### CI/CD"
    echo ""
    printf '%s\n' "${CICD[@]}"
    echo ""
fi

if [ ${#OTHER[@]} -gt 0 ]; then
    echo "### Other Changes"
    echo ""
    printf '%s\n' "${OTHER[@]}"
    echo ""
fi
