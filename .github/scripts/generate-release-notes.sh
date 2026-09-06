#!/bin/bash
# Generate release notes from PRs represented by commits between two tags.
# Usage: ./generate-release-notes.sh <previous-tag> <new-tag>
#
# This script is deterministic:
# - commits reachable from the release tag determine included PRs
# - PR ## Summary blocks provide normal note content
# - Release promotion PRs preserve authored sections through ## Test plan
# - linked issues come from PR bodies
# - completeness is validated before notes are emitted

set -euo pipefail

PREV_TAG="${1:-$(git describe --tags --abbrev=0 HEAD~1 2>/dev/null || echo "")}"
NEW_TAG="${2:-$(git describe --tags --abbrev=0 HEAD 2>/dev/null || echo "HEAD")}"
REPO="${GITHUB_REPOSITORY:-Starosdev/scrutiny}"
MAX_SUMMARY_BULLETS=8
# Operational-only PRs excluded from user-facing release notes.
EXCLUDED_PRS_REGEX='^(701|719|724|726)$'

if [ -z "$PREV_TAG" ]; then
    echo "Error: Could not determine previous tag" >&2
    exit 1
fi

echo "Generating release notes for $PREV_TAG..$NEW_TAG" >&2

RELEASE_DATE=$(git log -1 --format=%as "$NEW_TAG" 2>/dev/null || date +%Y-%m-%d)

PRS_JSON=$(mktemp)
OUTPUT_FILE=$(mktemp)
EXPECTED_FILE=$(mktemp)
trap 'rm -f "$PRS_JSON" "$OUTPUT_FILE" "$EXPECTED_FILE"' EXIT

while IFS= read -r commit; do
    [ -z "$commit" ] && continue
    gh api --paginate "repos/$REPO/commits/$commit/pulls" \
        | jq '[.[] | select(.base.ref == "master" and .merged_at != null) | {
            number,
            title,
            mergedAt: .merged_at,
            body,
            headRefName: .head.ref
        }]' >> "$PRS_JSON"
done < <(git rev-list "$PREV_TAG..$NEW_TAG")

MERGED_JSON=$(jq -s 'flatten | unique_by(.number) | sort_by(.mergedAt, .number)' "$PRS_JSON")

get_summary_block() {
    local body="$1"
    [ -z "$body" ] && return

    if echo "$body" | grep -q '^## Product changes$'; then
        echo "$body" | awk '
            /^## Summary$/ { in_release=1 }
            /^## Test plan$/ { in_release=0 }
            in_release { print }
        '
        return
    fi

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

declare -a FEATURES=()
declare -a FIXES=()
declare -a REFACTORS=()
declare -a DOCS=()
declare -a DEPS=()
declare -a CICD=()
declare -a HIGHLIGHTS=()
declare -a OTHER=()
HAS_ENTRIES=0

PR_COUNT=$(echo "$MERGED_JSON" | jq 'length')

for ((i = 0; i < PR_COUNT; i++)); do
    pr_num=$(echo "$MERGED_JSON" | jq -r ".[$i].number")
    pr_title=$(echo "$MERGED_JSON" | jq -r ".[$i].title")
    pr_body=$(echo "$MERGED_JSON" | jq -r ".[$i].body // \"\"")
    pr_head=$(echo "$MERGED_JSON" | jq -r ".[$i].headRefName // \"\"")

    [ -z "$pr_num" ] && continue

    if [[ "$pr_num" =~ $EXCLUDED_PRS_REGEX ]]; then
        continue
    fi

    if [[ "$pr_title" =~ ^(docs|style|chore|test|ci)(\(.+\))?!?: ]]; then
        continue
    fi

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
    else
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
        if ! grep -Fq -- "$item" "$OUTPUT_FILE"; then
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
