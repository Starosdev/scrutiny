#!/usr/bin/env bash
set -euo pipefail

BASE_URL=""
HEALTH_PATH="/api/health"
ROOT_PATH="/"
TIMEOUT=10

usage() {
  cat <<'EOF'
Usage: ops/smoke_test.sh --base-url <url> [--health-path <path>] [--root-path <path>]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --base-url)
      BASE_URL="${2:-}"
      shift 2
      ;;
    --health-path)
      HEALTH_PATH="${2:-}"
      shift 2
      ;;
    --root-path)
      ROOT_PATH="${2:-}"
      shift 2
      ;;
    *)
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$BASE_URL" ]]; then
  usage
  exit 1
fi

check_url() {
  local url="$1"
  local label="$2"
  local expected_pattern="$3"
  local code

  code="$(curl -fsS -o /dev/null -w "%{http_code}" --connect-timeout "$TIMEOUT" "$url")"
  if [[ ! "$code" =~ $expected_pattern ]]; then
    echo "Smoke test failed for $label: HTTP $code ($url)"
    exit 1
  fi
  echo "$label ok: HTTP $code"
}

check_url "${BASE_URL%/}${HEALTH_PATH}" "health" '^200$'
check_url "${BASE_URL%/}${ROOT_PATH}" "root" '^(200|301|302|307|308)$'
