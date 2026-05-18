#!/bin/bash
# Generate release notes from merged PRs between two tags.
# Usage: ./generate-release-notes.sh <previous-tag> <new-tag>
#
# This script is deterministic:
# - merged PR metadata is the source of truth
# - PR ## Summary blocks provide the actual note content
# - linked issues come from PR bodies
# - completeness is validated before notes are emitted

set -euo pipefail

PREV_TAG="${1:-$(git describe --tags --abbrev=0 HEAD~1 2>/dev/null || echo "")}"
NEW_TAG="${2:-$(git describe --tags --abbrev=0 HEAD 2>/dev/null || echo "HEAD")}"
REPO="${GITHUB_REPOSITORY:-Starosdev/scrutiny}"
MAX_SUMMARY_BULLETS=8

if [ -z "$PREV_TAG" ]; then
    echo "Error: Could not determine previous tag" >&2
    exit 1
fi

echo "Generating release notes for $PREV_TAG..$NEW_TAG" >&2

PREV_DATE=$(git log -1 --format=%aI "$PREV_TAG" 2>/dev/null || echo "1970-01-01T00:00:00Z")
NEW_DATE=$(git log -1 --format=%aI "$NEW_TAG" 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
RELEASE_DATE=$(git log -1 --format=%as "$NEW_TAG" 2>/dev/null || date +%Y-%m-%d)

DEVELOP_JSON=$(mktemp)
MASTER_JSON=$(mktemp)
INTEGRATION_JSON=$(mktemp)
DEVELOP_FILTERED_FILE=$(mktemp)
OUTPUT_FILE=$(mktemp)
EXPECTED_FILE=$(mktemp)
trap 'rm -f "$DEVELOP_JSON" "$MASTER_JSON" "$INTEGRATION_JSON" "$DEVELOP_FILTERED_FILE" "$OUTPUT_FILE" "$EXPECTED_FILE"' EXIT

gh pr list --repo "$REPO" --state merged --base develop --limit 200 \
    --json number,title,mergedAt,body,headRefName \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\" and .mergedAt <= \"$NEW_DATE\")]" \
    > "$DEVELOP_JSON" 2>/dev/null || echo "[]" > "$DEVELOP_JSON"

gh pr list --repo "$REPO" --state merged --base master --limit 200 \
    --json number,title,mergedAt,body,headRefName \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\" and .mergedAt <= \"$NEW_DATE\") | select(.headRefName != \"develop\")]" \
    > "$MASTER_JSON" 2>/dev/null || echo "[]" > "$MASTER_JSON"

gh pr list --repo "$REPO" --state merged --base master --limit 200 \
    --json number,title,mergedAt,body,headRefName \
    --jq "[.[] | select(.mergedAt > \"$PREV_DATE\" and .mergedAt <= \"$NEW_DATE\") | select(.headRefName == \"develop\")]" \
    > "$INTEGRATION_JSON" 2>/dev/null || echo "[]" > "$INTEGRATION_JSON"

COVERED_PRS=$(
    jq -r '.[].body // "", .[].title // ""' "$INTEGRATION_JSON" 2>/dev/null \
        | { grep -oE '(Closes|Fixes|Resolves) #[0-9]+|\(#[0-9]+\)' || true; } \
        | { grep -oE '[0-9]+' || true; } \
        | sort -u \
        | paste -sd '|' - 2>/dev/null || echo ""
)

if [ -n "$COVERED_PRS" ]; then
    DEVELOP_FILTERED=$(jq "[.[] | select(.number | tostring | test(\"^($COVERED_PRS)$\") | not)]" "$DEVELOP_JSON")
else
    DEVELOP_FILTERED=$(cat "$DEVELOP_JSON")
fi

echo "$DEVELOP_FILTERED" > "$DEVELOP_FILTERED_FILE"
MERGED_JSON=$(jq -s \
    --slurpfile master "$MASTER_JSON" \
    --slurpfile integration "$INTEGRATION_JSON" \
    '.[0] + $master[0] + $integration[0] | unique_by(.number) | sort_by(.mergedAt, .number)' \
    "$DEVELOP_FILTERED_FILE")

get_summary_block() {
    local body="$1"
    [ -z "$body" ] && return
    echo "$body" | tr -d '\r' | sed -n '/^## Summary/,/^## /{/^## /d; p;}'
}

clean_text() {
    sed 's/\*\*//g; s/`//g; s/^[[:space:]]*//; s/[[:space:]]*$//'
}

extract_summary_items() {
    local body="$1"
    local summary_block
    summary_block=$(get_summary_block "$body")
    [ -z "$summary_block" ] && return

    local prose_lines bullet_lines
    prose_lines=$(
        echo "$summary_block" \
            | grep -v '^[[:space:]]*$' \
            | grep -v '^[[:space:]]*[-*] ' \
            | grep -v '^#' \
            | grep -vE '^(Closes|Fixes|Resolves) #[0-9]+$' \
            | clean_text || true
    )
    bullet_lines=$(
        echo "$summary_block" \
            | sed -n 's/^[[:space:]]*[-*] //p' \
            | clean_text || true
    )

    if [ -n "$prose_lines" ]; then
        printf '%s\n' "$prose_lines"
        if [ -n "$bullet_lines" ]; then
            printf '%s\n' "$bullet_lines" | head -n "$MAX_SUMMARY_BULLETS"
        fi
    elif [ -n "$bullet_lines" ]; then
        printf '%s\n' "$bullet_lines" | head -n "$MAX_SUMMARY_BULLETS"
    fi
}

extract_closes() {
    local body="$1"
    [ -z "$body" ] && return

    echo "$body" \
        | grep -oE '(Closes|Fixes|Resolves) #[0-9]+' \
        | awk '!seen[$0]++' \
        | while IFS= read -r ref; do
            local keyword issue_num
            keyword=$(echo "$ref" | grep -oE '^(Closes|Fixes|Resolves)')
            issue_num=$(echo "$ref" | grep -oE '[0-9]+$')
            echo "$keyword [#$issue_num](https://github.com/$REPO/issues/$issue_num)"
        done || true
}

clean_title() {
    local title="$1"
    title=$(echo "$title" | sed -E 's/^(feat|fix|refactor|docs|ci|build|perf|chore)(\(.+\))?!?:[[:space:]]*//')
    title=$(echo "$title" | sed -E 's/[[:space:]]*\((#[0-9]+|SCR-[0-9]+)\)[[:space:]]*$//')
    echo "$(echo "${title:0:1}" | tr '[:lower:]' '[:upper:]')${title:1}"
}

append_expected_items() {
    local pr_num="$1"
    local items="$2"
    [ -z "$items" ] && return

    while IFS= read -r item; do
        [ -n "$item" ] && printf '%s\t%s\n' "$pr_num" "$item" >> "$EXPECTED_FILE"
    done <<< "$items"
}

format_entry() {
    local pr_num="$1"
    local pr_title="$2"
    local pr_body="$3"
    local link="https://github.com/$REPO/pull/$pr_num"

    local title closes closes_inline summary_items
    title=$(clean_title "$pr_title")
    closes=$(extract_closes "$pr_body")
    summary_items=$(extract_summary_items "$pr_body")

    append_expected_items "$pr_num" "$summary_items"

    ENTRY="- **$title** ([#$pr_num]($link))"
    if [ -n "$closes" ]; then
        closes_inline=$(echo "$closes" | awk 'NR>1{printf ", "} {printf "%s", $0} END{print ""}')
        ENTRY="$ENTRY - $closes_inline"
    fi
    ENTRY="$ENTRY"$'\n'

    if [ -n "$summary_items" ]; then
        while IFS= read -r item; do
            [ -n "$item" ] && ENTRY="$ENTRY""  - $item"$'\n'
        done <<< "$summary_items"
    fi
}

declare -a FEATURES FIXES REFACTORS DOCS DEPS CICD HIGHLIGHTS OTHER
HAS_ENTRIES=0

PR_COUNT=$(echo "$MERGED_JSON" | jq 'length')

for ((i = 0; i < PR_COUNT; i++)); do
    pr_num=$(echo "$MERGED_JSON" | jq -r ".[$i].number")
    pr_title=$(echo "$MERGED_JSON" | jq -r ".[$i].title")
    pr_body=$(echo "$MERGED_JSON" | jq -r ".[$i].body // \"\"")
    pr_head=$(echo "$MERGED_JSON" | jq -r ".[$i].headRefName // \"\"")

    [ -z "$pr_num" ] && continue

    if [[ "$pr_title" =~ ^Release:|^chore\(release\) ]]; then
        continue
    fi

    format_entry "$pr_num" "$pr_title" "$pr_body"

    if [ "$pr_head" = "develop" ]; then
        HIGHLIGHTS+=("$ENTRY")
    elif [[ "$pr_title" =~ ^feat(\(.+\))?:|^feat!(\(.+\))?: ]]; then
        FEATURES+=("$ENTRY")
    elif [[ "$pr_title" =~ ^fix(\(.+\))?:|^fix!(\(.+\))?: ]]; then
        FIXES+=("$ENTRY")
    elif [[ "$pr_title" =~ ^refactor(\(.+\))?: ]]; then
        REFACTORS+=("$ENTRY")
    elif [[ "$pr_title" =~ ^docs(\(.+\))?: ]]; then
        DOCS+=("$ENTRY")
    elif [[ "$pr_title" =~ ^ci(\(.+\))?:|^chore\((ci|docker)\): ]]; then
        CICD+=("$ENTRY")
    elif [[ "$pr_title" =~ ^chore\(deps\):|[Dd]ependen|[Uu]pdate.*go\.(mod|sum) ]]; then
        DEPS+=("$ENTRY")
    elif [[ ! "$pr_title" =~ ^chore ]]; then
        OTHER+=("$ENTRY")
    fi
done

print_section() {
    local heading="$1"
    shift
    local entries=("$@")

    [ ${#entries[@]} -eq 0 ] && return

    {
        echo "### $heading"
        echo ""
        for entry in "${entries[@]}"; do
            echo "$entry"
        done
        echo ""
    } >> "$OUTPUT_FILE"
}

{
    echo "## [$NEW_TAG](https://github.com/$REPO/compare/$PREV_TAG...$NEW_TAG) ($RELEASE_DATE)"
    echo ""
} > "$OUTPUT_FILE"

if [ ${#FEATURES[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Features" "${FEATURES[@]}"
fi
if [ ${#FIXES[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Bug Fixes" "${FIXES[@]}"
fi
if [ ${#HIGHLIGHTS[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Release Highlights" "${HIGHLIGHTS[@]}"
fi
if [ ${#REFACTORS[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Refactoring" "${REFACTORS[@]}"
fi
if [ ${#DOCS[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Documentation" "${DOCS[@]}"
fi
if [ ${#DEPS[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Dependencies" "${DEPS[@]}"
fi
if [ ${#CICD[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "CI/CD" "${CICD[@]}"
fi
if [ ${#OTHER[@]} -gt 0 ]; then
    HAS_ENTRIES=1
    print_section "Other Changes" "${OTHER[@]}"
fi

if [ "$HAS_ENTRIES" -eq 0 ]; then
    {
        echo "### Changes"
        echo ""
        echo "See [commit history](https://github.com/$REPO/compare/$PREV_TAG...$NEW_TAG) for details."
        echo ""
    } >> "$OUTPUT_FILE"
fi

validate_output() {
    local missing=0
    while IFS=$'\t' read -r pr_num item; do
        [ -z "$item" ] && continue
        if ! grep -Fq -- "  - $item" "$OUTPUT_FILE"; then
            echo "Missing summary item from PR #$pr_num: $item" >&2
            missing=1
        fi
    done < "$EXPECTED_FILE"

    if [ "$missing" -ne 0 ]; then
        echo "Release note validation failed; refusing to emit incomplete raw notes." >&2
        exit 1
    fi
}

validate_output
cat "$OUTPUT_FILE"
