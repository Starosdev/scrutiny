#!/bin/bash

set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
MOCK_BIN=$(mktemp -d)
trap 'rm -rf "$MOCK_BIN"' EXIT

cat > "$MOCK_BIN/git" <<'EOF'
#!/bin/bash
case "$1" in
    log)
        if [[ "$*" == *"--format=%as"* ]]; then
            echo "2026-09-05"
        else
            echo "2026-09-05T00:00:00Z"
        fi
        ;;
    rev-list)
        echo "released-commit"
        echo "wrapped-commit"
        echo "wrapped-merge"
        echo "released-merge"
        echo "stale-commit"
        echo "operational-commit"
        echo "operational-merge"
        echo "develop-only-commit"
        ;;
esac
EOF

cat > "$MOCK_BIN/gh" <<'EOF'
#!/bin/bash
case "${!#}" in
    */wrapped-commit/pulls)
        cat <<'JSON'
[{"number":45,"title":"fix: wrapped prose summary","merged_at":"2026-09-05T00:00:00Z","merge_commit_sha":"wrapped-merge","body":"## Product changes\n\n## Summary\n\nTen commits: two features, one documentation change and six\nfixes. Two of them are the reason to promote now.\n\nA second paragraph that also\nwraps across lines.\n\n- A real bullet\n\n## Test plan\n\n- test","base":{"ref":"master"},"head":{"ref":"develop"}}]
JSON
        ;;
    */released-commit/pulls)
        cat <<'JSON'
[{"number":42,"title":"feat: release change","merged_at":"2026-09-05T00:00:00Z","merge_commit_sha":"released-merge","body":"## Product changes\n\n## Summary\n\n- Visible change\n\nCloses #41\n\n## Test plan\n\n- test","base":{"ref":"master"},"head":{"ref":"develop"}}]
JSON
        ;;
    */stale-commit/pulls)
        cat <<'JSON'
[{"number":99,"title":"feat: stale association","merged_at":"2026-08-01T00:00:00Z","merge_commit_sha":"stale-merge","body":"## Product changes\n\n## Summary\n\n- Stale change","base":{"ref":"master"},"head":{"ref":"develop"}}]
JSON
        ;;
    */operational-commit/pulls)
        cat <<'JSON'
[{"number":44,"title":"fix(release): workflow repair","merged_at":"2026-09-05T00:00:00Z","merge_commit_sha":"operational-merge","body":"## Product changes\n\n## Summary\n\nNone.\n\n## Test plan\n\n- test","base":{"ref":"master"},"head":{"ref":"develop"}}]
JSON
        ;;
    */develop-only-commit/pulls)
        cat <<'JSON'
[{"number":43,"title":"feat: develop-only change","merged_at":"2026-09-05T00:00:00Z","merge_commit_sha":"develop-only-commit","body":"## Product changes\n\n## Summary\n\n- Develop-only change","base":{"ref":"develop"},"head":{"ref":"feature/develop"}},{"number":44,"title":"chore: release administration","merged_at":"2026-09-05T00:00:00Z","body":"## Summary\n\n- Internal change","base":{"ref":"master"},"head":{"ref":"chore/release"}}]
JSON
        ;;
esac
EOF

chmod +x "$MOCK_BIN/git" "$MOCK_BIN/gh"

NOTES=$(PATH="$MOCK_BIN:$PATH" "$ROOT/.github/scripts/generate-release-notes.sh" v1.0.0 v1.0.1)

grep -Fq "[#42](https://github.com/Starosdev/scrutiny/pull/42)" <<< "$NOTES"
grep -Fq "Visible change" <<< "$NOTES"
! grep -Fq "#43" <<< "$NOTES"
! grep -Fq "Develop-only change" <<< "$NOTES"
! grep -Fq "#44" <<< "$NOTES"
! grep -Fq "#99" <<< "$NOTES"
! grep -Fq "Stale change" <<< "$NOTES"

# Wrapped prose must join into one item per paragraph. Emitting one bullet per
# physical line split sentences mid-way in the published v1.72.0 notes.
grep -Fq "Ten commits: two features, one documentation change and six fixes. Two of them are the reason to promote now." <<< "$NOTES"
grep -Fq "A second paragraph that also wraps across lines." <<< "$NOTES"
# the fragment must not survive as its own item
! grep -qE '^[[:space:]]*-[[:space:]]+fixes\. Two of them' <<< "$NOTES"
# bullets alongside prose are still collected
grep -Fq "A real bullet" <<< "$NOTES"
