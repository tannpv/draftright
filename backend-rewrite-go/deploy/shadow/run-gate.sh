#!/usr/bin/env bash
# =============================================================================
# deploy/shadow/run-gate.sh
# End-to-end dev parity gate. Run ON the dev VPS from a checkout of this branch.
#
#   ./deploy/shadow/run-gate.sh [/path/to/.env.shadow]
#
# Steps:
#   1. (re)build the Go shadow image from THIS checkout (so the gate tests the
#      branch's binary, not a stale :latest).
#   2. build the frozen template DB from draftright_dev + augment.sql.
#   3. create both per-backend shadow DBs from the template.
#   4. bring up both shadow backends (node:3200, go:3201) sharing one JWT secret.
#   5. run shadowdiff with per-fixture reset + coverage + token bootstrap.
#
# Exits non-zero on any fixture diff or coverage gap. Touches ONLY the
# draftright_shadow_* databases — never draftright_dev / prod.
# =============================================================================
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
ENVFILE="${1:-$ROOT/.env.shadow}"

TMPL=draftright_shadow_tmpl
DB_NODE=draftright_shadow_node
DB_GO=draftright_shadow_go

[ -f "$ENVFILE" ] || { echo "missing env file: $ENVFILE (needs JWT_SECRET + JWT_REFRESH_SECRET + NODE_DATABASE_URL + GO_DATABASE_URL + MAINT_DSN)"; exit 2; }

# Source the env-file so MAINT_DSN (+ optional PGCONT/PGUSER overrides) are
# available to this script. The same file is passed to docker compose via
# --env-file below, so both layers read one source of truth — no creds in git.
set -a; . "$ENVFILE"; set +a

# Container + role used for in-container psql (trust/peer auth, no password).
# Overridable from the env-file if the dev stack names them differently.
PGCONT="${PGCONT:-draftright-dev-postgres-1}"
PGUSER="${PGUSER:-draftright}"
: "${MAINT_DSN:?set MAINT_DSN in $ENVFILE (host-reachable postgres-db DSN with real password)}"

psql_postgres() { docker exec -i "$PGCONT" psql -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 "$@"; }

echo "== [1/5] build Go shadow image from this checkout =="
docker build -t draftright-rewrite-go:latest "$ROOT"

echo "== [2/5] build template DB =="
"$HERE/make-template.sh"

echo "== [3/5] create per-backend shadow DBs from template =="
for db in "$DB_NODE" "$DB_GO"; do
  psql_postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$db' AND pid <> pg_backend_pid();"
  psql_postgres -c "DROP DATABASE IF EXISTS $db;"
  psql_postgres -c "CREATE DATABASE $db TEMPLATE $TMPL;"
done

echo "== [4/5] up shadow backends =="
docker compose -f "$HERE/docker-compose.shadow.yml" --env-file "$ENVFILE" up -d backend-node-shadow backend-go-shadow
echo "   waiting for health..."
for i in $(seq 1 30); do
  if curl -fsS http://localhost:3200/health >/dev/null 2>&1 && curl -fsS http://localhost:3201/health >/dev/null 2>&1; then
    echo "   both healthy"; break
  fi
  sleep 1
  [ "$i" = 30 ] && { echo "shadow backends did not become healthy"; exit 2; }
done

echo "== [5/5] run shadowdiff =="
go run "$ROOT/cmd/shadowdiff" \
  --node=http://localhost:3200 --go=http://localhost:3201 \
  --fixtures="$ROOT/cmd/shadowdiff/fixtures" \
  --routes="$HERE/routes.txt" \
  --maint-dsn="$MAINT_DSN" \
  --template="$TMPL" \
  --db-node="$DB_NODE" --db-go="$DB_GO" \
  --user-email=shadow-user@draftright.info  --user-pass=ShadowPass123 \
  --admin-email=shadow-admin@draftright.info --admin-pass=ShadowPass123
