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

echo "Generating release notes for $PREV_TAG..$NEW_TAG" >&2

# Get the date of the previous tag
PREV_DATE=$(git log -1 --format=%aI "$PREV_TAG" 2>/dev/null || echo "1970-01-01T00:00:00Z")

# Create temp files for PR data
MASTER_JSON=$(mktemp)
DEVELOP_JSON=$(mktemp)
trap 'rm -f "$MASTER_JSON" "$DEVELOP_JSON"' EXIT

# Fetch PRs as JSON arrays (handles bodies with special characters safely)
gh pr list --repo "$REPO" --state merged --base master \
    --json number,title,mergedAt,body \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\")]" \
    > "$MASTER_JSON" 2>/dev/null || echo "[]" > "$MASTER_JSON"

gh pr list --repo "$REPO" --state merged --base develop \
    --json number,title,mergedAt,body \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\")]" \
    > "$DEVELOP_JSON" 2>/dev/null || echo "[]" > "$DEVELOP_JSON"

# Merge and deduplicate by PR number
MERGED_JSON=$(jq -s '.[0] + .[1] | unique_by(.number)' "$MASTER_JSON" "$DEVELOP_JSON")

# Extract summary from PR body. Looks for a ## Summary section and returns
# the first 1-2 sentences (max 200 chars). Returns empty string if not found.
extract_summary() {
    local body="$1"
    [ -z "$body" ] && return

    # Extract text between ## Summary and the next ## heading
    local summary_block
    summary_block=$(echo "$body" | sed -n '/^## Summary/,/^## /{/^## /d; p;}')
    [ -z "$summary_block" ] && return

    # If the summary is a bulleted list, take just the first bullet
    local summary
    local first_bullet
    first_bullet=$(echo "$summary_block" | sed -n 's/^[[:space:]]*[-*] //p' | head -1)
    if [ -n "$first_bullet" ]; then
        summary="$first_bullet"
    else
        # Not a list, take first few lines as prose
        summary=$(echo "$summary_block" | head -3 | tr '\n' ' ')
    fi

    # Strip markdown formatting
    summary=$(echo "$summary" \
        | sed 's/\*\*//g' \
        | sed 's/`//g' \
        | sed 's/[[:space:]]\{2,\}/ /g' \
        | sed 's/^[[:space:]]*//' \
        | sed 's/[[:space:]]*$//')
    [ -z "$summary" ] && return

    # Trim to ~200 chars at a sentence boundary
    if [ ${#summary} -gt 200 ]; then
        # Try to cut at a sentence boundary
        local trimmed
        trimmed=$(echo "${summary:0:200}" | sed 's/\.[^.]*$/\./')
        if [ ${#trimmed} -gt 20 ]; then
            summary="$trimmed"
        else
            summary="${summary:0:197}..."
        fi
    fi

    echo "$summary"
}

# Initialize arrays for categorizing changes
declare -a FEATURES FIXES REFACTORS DOCS DEPS CICD OTHER

# Get count of PRs
PR_COUNT=$(echo "$MERGED_JSON" | jq 'length')

for i in $(seq 0 $((PR_COUNT - 1))); do
    pr_num=$(echo "$MERGED_JSON" | jq -r ".[$i].number")
    pr_title=$(echo "$MERGED_JSON" | jq -r ".[$i].title")
    pr_body=$(echo "$MERGED_JSON" | jq -r ".[$i].body // \"\"")

    [ -z "$pr_num" ] && continue

    # Skip release merge PRs
    if [[ "$pr_title" =~ ^Release:|^chore\(release\) ]]; then
        continue
    fi

    # Build the entry line
    local_title="$pr_title"
    link="https://github.com/$REPO/pull/$pr_num"
    summary=$(extract_summary "$pr_body")
    entry="* $local_title ([#$pr_num]($link))"
    if [ -n "$summary" ]; then
        entry="$entry"$'\n'"  - $summary"
    fi

    # Categorize by conventional commit prefix
    if [[ "$pr_title" =~ ^feat(\(.+\))?:|^feat!(\(.+\))?: ]]; then
        local_title="${pr_title#feat*: }"
        entry="* $local_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        FEATURES+=("$entry")
    elif [[ "$pr_title" =~ ^fix(\(.+\))?:|^fix!(\(.+\))?: ]]; then
        local_title="${pr_title#fix*: }"
        entry="* $local_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        FIXES+=("$entry")
    elif [[ "$pr_title" =~ ^refactor(\(.+\))?: ]]; then
        local_title="${pr_title#refactor*: }"
        entry="* $local_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        REFACTORS+=("$entry")
    elif [[ "$pr_title" =~ ^docs(\(.+\))?: ]]; then
        local_title="${pr_title#docs*: }"
        entry="* $local_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        DOCS+=("$entry")
    elif [[ "$pr_title" =~ ^ci(\(.+\))?: ]]; then
        local_title="${pr_title#ci*: }"
        entry="* $local_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        CICD+=("$entry")
    elif [[ "$pr_title" =~ [Dd]ependen|[Uu]pdate.*go\.(mod|sum) ]]; then
        entry="* $pr_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        DEPS+=("$entry")
    elif [[ ! "$pr_title" =~ ^chore ]]; then
        entry="* $pr_title ([#$pr_num]($link))"
        [ -n "$summary" ] && entry="$entry"$'\n'"  - $summary"
        OTHER+=("$entry")
    fi
done

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

# If no PRs were categorized, add a note about direct commits
TOTAL=$((${#FEATURES[@]} + ${#FIXES[@]} + ${#REFACTORS[@]} + ${#DOCS[@]} + ${#DEPS[@]} + ${#CICD[@]} + ${#OTHER[@]}))
if [ "$TOTAL" -eq 0 ]; then
    echo "### Changes"
    echo ""
    echo "See [commit history](https://github.com/$REPO/compare/$PREV_TAG...$NEW_TAG) for details."
    echo ""
fi
