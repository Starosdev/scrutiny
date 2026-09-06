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
        echo "develop-only-commit"
        ;;
esac
EOF

cat > "$MOCK_BIN/gh" <<'EOF'
#!/bin/bash
case "${!#}" in
    */released-commit/pulls)
        cat <<'JSON'
[{"number":42,"title":"feat: release change","merged_at":"2026-09-05T00:00:00Z","body":"## Summary\n\n- Visible change\n\nCloses #41\n\n## Test plan\n\n- test","base":{"ref":"master"},"head":{"ref":"feature/released"}}]
JSON
        ;;
    */develop-only-commit/pulls)
        cat <<'JSON'
[{"number":43,"title":"feat: unreleased change","merged_at":"2026-09-05T00:00:00Z","body":"## Summary\n\n- Unreleased change","base":{"ref":"develop"},"head":{"ref":"feature/develop"}},{"number":44,"title":"chore: release administration","merged_at":"2026-09-05T00:00:00Z","body":"## Summary\n\n- Internal change","base":{"ref":"master"},"head":{"ref":"chore/release"}}]
JSON
        ;;
esac
EOF

chmod +x "$MOCK_BIN/git" "$MOCK_BIN/gh"

NOTES=$(PATH="$MOCK_BIN:$PATH" "$ROOT/.github/scripts/generate-release-notes.sh" v1.0.0 v1.0.1)

grep -Fq "[#42](https://github.com/Starosdev/scrutiny/pull/42)" <<< "$NOTES"
grep -Fq "Visible change" <<< "$NOTES"
! grep -Fq "#43" <<< "$NOTES"
! grep -Fq "#44" <<< "$NOTES"
