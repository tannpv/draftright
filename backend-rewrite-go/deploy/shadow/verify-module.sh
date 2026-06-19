#!/usr/bin/env bash
# =============================================================================
# deploy/shadow/verify-module.sh
# Fast per-module fixture verification against ALREADY-RUNNING shadow backends.
#
#   ./deploy/shadow/verify-module.sh <module-dir-name> [/path/to/.env.shadow]
#   ./deploy/shadow/verify-module.sh auth
#
# Unlike run-gate.sh (which rebuilds images + rebuilds the template + recreates
# DBs every run — the authoritative full gate), this assumes node-shadow:3200 +
# go-shadow:3201 are already up (see run-gate.sh steps 1-4 / the local rig) and
# just runs shadowdiff over ONE fixture sub-directory with per-fixture reset +
# warmup. Use it in the authoring loop; use run-gate.sh for the final green run.
#
# No --routes flag here on purpose: coverage is asserted by the full gate, not
# per-module subsets.
# =============================================================================
set -euo pipefail

MODULE="${1:?usage: verify-module.sh <module-dir-name> [env-file]}"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
ENVFILE="${2:-$ROOT/.env.shadow}"

[ -f "$ENVFILE" ] || { echo "missing env file: $ENVFILE"; exit 2; }
set -a; . "$ENVFILE"; set +a
: "${MAINT_DSN:?set MAINT_DSN in $ENVFILE}"

FIXDIR="$ROOT/cmd/shadowdiff/fixtures/$MODULE"
[ -d "$FIXDIR" ] || { echo "no fixture dir: $FIXDIR"; exit 2; }

go run "$ROOT/cmd/shadowdiff" \
  --node="${NODE_BASE:-http://localhost:3200}" --go="${GO_BASE:-http://localhost:3201}" \
  --fixtures="$FIXDIR" \
  --maint-dsn="$MAINT_DSN" \
  --template="${SHADOW_TEMPLATE_DB:-draftright_shadow_tmpl}" \
  --db-node=draftright_shadow_node --db-go=draftright_shadow_go \
  --user-email="${SHADOW_USER_EMAIL:-shadow-user@draftright.info}"  --user-pass="${SHADOW_USER_PASS:-ShadowPass123}" \
  --admin-email="${SHADOW_ADMIN_EMAIL:-shadow-admin@draftright.info}" --admin-pass="${SHADOW_ADMIN_PASS:-ShadowPass123}"
