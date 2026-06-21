#!/usr/bin/env bash
# =============================================================================
# deploy/shadow/make-template.sh
# Purpose: Build the frozen template DB (draftright_shadow_tmpl) from the
#          live dev DB (draftright_dev), then apply the idempotent fixture seed
#          (augment.sql) so the template contains known test data.
#
# Run ON the VPS by an operator.  This script is safe to re-run — it drops
# and recreates the template fresh each time.
#
# Prerequisites:
#   - draftright-dev-postgres-1 container is running
#   - augment.sql is in the same directory as this script (deploy/shadow/)
#
# Usage:
#   cd /opt/draftright/backend-rewrite-go   # or wherever the repo is checked out
#   bash deploy/shadow/make-template.sh
# =============================================================================
set -euo pipefail

# Overridable via env so the same script drives the VPS dev stack OR a fully
# local rig (different container name / source db). Defaults = VPS dev stack.
POSTGRES_CONTAINER="${PGCONT:-draftright-dev-postgres-1}"
POSTGRES_USER="${PGUSER:-draftright}"
SOURCE_DB="${SHADOW_SOURCE_DB:-draftright_dev}"
TEMPLATE_DB="${SHADOW_TEMPLATE_DB:-draftright_shadow_tmpl}"

# Resolve script directory so augment.sql path is always correct regardless of
# where the operator calls the script from.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AUGMENT_SQL="${SCRIPT_DIR}/augment.sql"

echo "=== Shadow template builder ==="
echo "  Source DB  : ${SOURCE_DB}"
echo "  Template DB: ${TEMPLATE_DB}"
echo "  Container  : ${POSTGRES_CONTAINER}"
echo ""

# ─── Step 0: Lock the source DB against new connections ──────────────────────
# CREATE DATABASE … TEMPLATE requires ZERO active connections to the source.
# Just terminating is racy: a live app pool (e.g. the dev backend) reconnects in
# the gap before CREATE runs → "source database is being accessed by other
# users". Flip ALLOW_CONNECTIONS off so terminated clients can't re-attach during
# the clone; a trap restores it on ANY exit so the source never stays locked.
restore_source_connections() {
    docker exec "${POSTGRES_CONTAINER}" psql \
        -U "${POSTGRES_USER}" -d postgres \
        -c "ALTER DATABASE ${SOURCE_DB} WITH ALLOW_CONNECTIONS true;" --quiet || true
}
trap restore_source_connections EXIT

echo "[0/5] Locking '${SOURCE_DB}' against new connections (clone window)..."
docker exec "${POSTGRES_CONTAINER}" psql \
    -U "${POSTGRES_USER}" \
    -d postgres \
    -v ON_ERROR_STOP=1 \
    -c "ALTER DATABASE ${SOURCE_DB} WITH ALLOW_CONNECTIONS false;" \
    --quiet

# ─── Step 1: Terminate connections to source DB ───────────────────────────────
echo "[1/5] Terminating active connections to '${SOURCE_DB}'..."
docker exec "${POSTGRES_CONTAINER}" psql \
    -U "${POSTGRES_USER}" \
    -d postgres \
    -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${SOURCE_DB}' AND pid <> pg_backend_pid();" \
    --quiet

# ─── Step 2: Terminate connections to existing template (if any) ─────────────
echo "[2/5] Terminating active connections to '${TEMPLATE_DB}' (if exists)..."
docker exec "${POSTGRES_CONTAINER}" psql \
    -U "${POSTGRES_USER}" \
    -d postgres \
    -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '${TEMPLATE_DB}' AND pid <> pg_backend_pid();" \
    --quiet

# ─── Step 3: Drop + recreate template DB from source ─────────────────────────
echo "[3/5] Dropping '${TEMPLATE_DB}' (if exists) and cloning from '${SOURCE_DB}'..."
docker exec "${POSTGRES_CONTAINER}" psql \
    -U "${POSTGRES_USER}" \
    -d postgres \
    -v ON_ERROR_STOP=1 \
    -c "DROP DATABASE IF EXISTS ${TEMPLATE_DB};"

docker exec "${POSTGRES_CONTAINER}" psql \
    -U "${POSTGRES_USER}" \
    -d postgres \
    -v ON_ERROR_STOP=1 \
    -c "CREATE DATABASE ${TEMPLATE_DB} TEMPLATE ${SOURCE_DB};"

# Clone done — unlock the source NOW (augment touches only the template below),
# so the dev backend can reconnect. The EXIT trap also covers failure paths.
restore_source_connections
trap - EXIT

echo "    Clone complete."

# ─── Step 4: Apply augment.sql into the template ─────────────────────────────
echo "[4/5] Applying fixture seed (augment.sql) into '${TEMPLATE_DB}'..."
docker exec -i "${POSTGRES_CONTAINER}" psql \
    -U "${POSTGRES_USER}" \
    -d "${TEMPLATE_DB}" \
    -v ON_ERROR_STOP=1 \
    < "${AUGMENT_SQL}"

echo "    Fixtures applied."

# ─── Step 5: Done ────────────────────────────────────────────────────────────
echo "[5/5] Template ready."
echo ""
echo "Next steps (run by operator):"
echo "  1. Create shadow DBs from template:"
echo "       docker exec ${POSTGRES_CONTAINER} psql -U ${POSTGRES_USER} -d postgres \\"
echo "         -c \"CREATE DATABASE draftright_shadow_node TEMPLATE ${TEMPLATE_DB};\""
echo "       docker exec ${POSTGRES_CONTAINER} psql -U ${POSTGRES_USER} -d postgres \\"
echo "         -c \"CREATE DATABASE draftright_shadow_go   TEMPLATE ${TEMPLATE_DB};\""
echo "  2. Start shadow backends:"
echo "       docker compose -f deploy/shadow/docker-compose.shadow.yml \\"
echo "         --env-file ../../.env.shadow up -d"
