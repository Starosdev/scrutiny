#!/bin/bash
# Generate release notes from merged PRs between two tags
# Usage: ./generate-release-notes.sh <previous-tag> <new-tag>
#
# Produces structured markdown with bold titles, descriptions, and bullet points.
# Output can be piped through polish-release-notes.sh for OpenAI polishing.

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
DEVELOP_JSON=$(mktemp)
MASTER_JSON=$(mktemp)
trap 'rm -f "$DEVELOP_JSON" "$MASTER_JSON"' EXIT

# Fetch PRs merged to develop -- these are the actual feature/fix PRs.
gh pr list --repo "$REPO" --state merged --base develop \
    --json number,title,mergedAt,body \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\")]" \
    > "$DEVELOP_JSON" 2>/dev/null || echo "[]" > "$DEVELOP_JSON"

# Also include PRs merged directly to master (hotfixes, direct deploys)
# but exclude develop->master integration PRs which duplicate develop PRs.
gh pr list --repo "$REPO" --state merged --base master \
    --json number,title,mergedAt,body,headRefName \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\") | select(.headRefName != \"develop\")]" \
    > "$MASTER_JSON" 2>/dev/null || echo "[]" > "$MASTER_JSON"

# Merge and deduplicate by PR number
MERGED_JSON=$(jq -s '.[0] + .[1] | unique_by(.number)' "$DEVELOP_JSON" "$MASTER_JSON")

# Extract summary block from PR body (text between ## Summary and next ## heading).
get_summary_block() {
    local body="$1"
    [ -z "$body" ] && return
    echo "$body" | sed -n '/^## Summary/,/^## /{/^## /d; p;}'
}

# Extract the first non-bullet line as a description. If none, use first bullet.
# Sets HAS_PROSE_DESC=1 if a prose line was found, 0 if fell back to first bullet.
HAS_PROSE_DESC=0
extract_description() {
    local summary_block
    summary_block=$(get_summary_block "$1")
    [ -z "$summary_block" ] && return

    # Try prose line first (non-empty, non-bullet, non-heading, not just a Closes/Fixes reference)
    local description
    description=$(echo "$summary_block" \
        | grep -v '^[[:space:]]*$' \
        | grep -v '^[[:space:]]*[-*] ' \
        | grep -v '^#' \
        | grep -vE '^(Closes|Fixes|Resolves) #[0-9]+$' \
        | head -1 \
        | sed 's/\*\*//g; s/`//g; s/^[[:space:]]*//')

    if [ -n "$description" ]; then
        HAS_PROSE_DESC=1
        echo "$description"
        return
    fi

    # Fall back to first bullet
    HAS_PROSE_DESC=0
    description=$(echo "$summary_block" | sed -n 's/^[[:space:]]*[-*] //p' | head -1 \
        | sed 's/\*\*//g; s/`//g')
    echo "$description"
}

# Extract up to 3 bullet points from ## Summary, stripped of bold markdown.
# If the description was taken from the first bullet (HAS_PROSE_DESC=0),
# skip the first bullet to avoid duplication.
extract_bullets() {
    local summary_block
    summary_block=$(get_summary_block "$1")
    [ -z "$summary_block" ] && return

    local all_bullets
    all_bullets=$(echo "$summary_block" | sed -n 's/^[[:space:]]*[-*] //p' | sed 's/\*\*//g')
    [ -z "$all_bullets" ] && return

    if [ "$HAS_PROSE_DESC" -eq 0 ]; then
        # Skip first bullet (already used as description), take next 3
        echo "$all_bullets" | tail -n +2 | head -3
    else
        echo "$all_bullets" | head -3
    fi
}

# Extract "Closes #XXX" / "Fixes #XXX" / "Resolves #XXX" from the PR body.
extract_closes() {
    local body="$1"
    [ -z "$body" ] && return

    echo "$body" | grep -oE '(Closes|Fixes|Resolves) #[0-9]+' | head -3
}

# Strip conventional commit prefix, trailing issue references, and capitalize first letter.
clean_title() {
    local title="$1"
    # Remove conventional commit prefix
    title=$(echo "$title" | sed -E 's/^(feat|fix|refactor|docs|ci|build|perf|chore)(\(.+\))?!?:[[:space:]]*//')
    # Remove trailing issue/PR references like (#123), (SCR-123)
    title=$(echo "$title" | sed -E 's/[[:space:]]*\((#[0-9]+|SCR-[0-9]+)\)[[:space:]]*$//')
    # Capitalize first letter
    echo "$(echo "${title:0:1}" | tr '[:lower:]' '[:upper:]')${title:1}"
}

# Format a single entry with bold title, description, and bullets.
# Output is stored in the ENTRY variable (multi-line).
format_entry() {
    local pr_num="$1"
    local pr_title="$2"
    local pr_body="$3"
    local link="https://github.com/$REPO/pull/$pr_num"

    local title
    title=$(clean_title "$pr_title")

    local description
    description=$(extract_description "$pr_body")

    local closes
    closes=$(extract_closes "$pr_body")

    # Title line: **Clean Title** ([#PR](link)) - Closes #XXX
    local title_line="**$title** ([#$pr_num]($link))"
    if [ -n "$closes" ]; then
        local closes_inline
        closes_inline=$(echo "$closes" | paste -sd ', ' -)
        title_line="$title_line - $closes_inline"
    fi

    ENTRY="$title_line"
    ENTRY="$ENTRY"$'\n'

    # Description line
    if [ -n "$description" ]; then
        ENTRY="$ENTRY"$'\n'"$description"
        ENTRY="$ENTRY"$'\n'
    fi

    # Bullet points (up to 3)
    local bullets
    bullets=$(extract_bullets "$pr_body")
    if [ -n "$bullets" ]; then
        ENTRY="$ENTRY"$'\n'
        while IFS= read -r bullet; do
            [ -n "$bullet" ] && ENTRY="$ENTRY""- $bullet"$'\n'
        done <<< "$bullets"
    fi
}

# Initialize arrays for categorizing entries
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

    # Format the entry
    format_entry "$pr_num" "$pr_title" "$pr_body"

    # Categorize by conventional commit prefix
    if [[ "$pr_title" =~ ^feat(\(.+\))?:|^feat!(\(.+\))?: ]]; then
        FEATURES+=("$ENTRY")
    elif [[ "$pr_title" =~ ^fix(\(.+\))?:|^fix!(\(.+\))?: ]]; then
        FIXES+=("$ENTRY")
    elif [[ "$pr_title" =~ ^refactor(\(.+\))?: ]]; then
        REFACTORS+=("$ENTRY")
    elif [[ "$pr_title" =~ ^docs(\(.+\))?: ]]; then
        DOCS+=("$ENTRY")
    elif [[ "$pr_title" =~ ^ci(\(.+\))?: ]]; then
        CICD+=("$ENTRY")
    elif [[ "$pr_title" =~ [Dd]ependen|[Uu]pdate.*go\.(mod|sum) ]]; then
        DEPS+=("$ENTRY")
    elif [[ ! "$pr_title" =~ ^chore ]]; then
        OTHER+=("$ENTRY")
    fi
done

# Print a category section with --- separators between entries
print_section() {
    local heading="$1"
    shift
    local entries=("$@")

    [ ${#entries[@]} -eq 0 ] && return

    echo "### $heading"
    echo ""

    local count=0
    for entry in "${entries[@]}"; do
        if [ $count -gt 0 ]; then
            echo "---"
            echo ""
        fi
        echo "$entry"
        count=$((count + 1))
    done
    echo ""
}

# Generate markdown output
echo "## [$NEW_TAG](https://github.com/$REPO/compare/$PREV_TAG...$NEW_TAG) ($(date +%Y-%m-%d))"
echo ""

print_section "Features" "${FEATURES[@]}"
print_section "Bug Fixes" "${FIXES[@]}"
print_section "Refactoring" "${REFACTORS[@]}"
print_section "Documentation" "${DOCS[@]}"
print_section "Dependencies" "${DEPS[@]}"
print_section "CI/CD" "${CICD[@]}"
print_section "Other Changes" "${OTHER[@]}"

# If no PRs were categorized, add a note about direct commits
TOTAL=$((${#FEATURES[@]} + ${#FIXES[@]} + ${#REFACTORS[@]} + ${#DOCS[@]} + ${#DEPS[@]} + ${#CICD[@]} + ${#OTHER[@]}))
if [ "$TOTAL" -eq 0 ]; then
    echo "### Changes"
    echo ""
    echo "See [commit history](https://github.com/$REPO/compare/$PREV_TAG...$NEW_TAG) for details."
    echo ""
fi
