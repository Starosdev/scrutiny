#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="${SCRUTINY_REPO_DIR:-/mnt/user/appdata/scrutiny-develop/repo}"
ENV_FILE="${SCRUTINY_ENV_FILE:-/mnt/user/appdata/scrutiny-develop/testing.env}"
APPDATA_ROOT="${SCRUTINY_APPDATA_ROOT:-${REPO_DIR%/repo}}"
COMPOSE_FILE="${SCRUTINY_COMPOSE_FILE:-$APPDATA_ROOT/docker-compose.yml}"
PROJECT="${SCRUTINY_PROJECT_NAME:-scrutiny-develop}"
BRANCH="${SCRUTINY_DEPLOY_BRANCH:-develop}"

echo "=== Scrutiny Develop Deploy ==="
echo "Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Target appdata root: $APPDATA_ROOT"

cd "$REPO_DIR"
git fetch origin "$BRANCH"
git checkout "$BRANCH"
git reset --hard "origin/$BRANCH"

docker compose \
  -p "$PROJECT" \
  -f "$COMPOSE_FILE" \
  --env-file "$ENV_FILE" \
  pull

docker compose \
  -p "$PROJECT" \
  -f "$COMPOSE_FILE" \
  --env-file "$ENV_FILE" \
  up -d --remove-orphans

docker compose \
  -p "$PROJECT" \
  -f "$COMPOSE_FILE" \
  --env-file "$ENV_FILE" \
  ps

BASE_URL="$(grep -m1 '^SCRUTINY_BASE_URL=' "$ENV_FILE" | cut -d'=' -f2- || true)"
if [[ -n "$BASE_URL" ]]; then
  bash "$REPO_DIR/ops/smoke_test.sh" --base-url "$BASE_URL"
fi
