#!/bin/bash
# CI gate: catches schema drift between NestJS migrations and our sqlc
# bindings. Runs in PR checks; any change to backend/sql/*.sql that
# touches a table we read here will fail this step until we regenerate
# + commit the new bindings.
#
# Local usage:
#   ./scripts/sqlc-check.sh
#
# To regenerate after a schema change:
#   1. scripts/dump-prod-schema.sh   # refresh schema.sql from prod
#   2. sqlc generate                 # re-emit bindings
#   3. git add internal/adapter/pg/sqlc/ internal/platform/db/schema.sql
#   4. commit + push
set -euo pipefail
cd "$(dirname "$0")/.."
if ! command -v sqlc >/dev/null 2>&1; then
    echo "ERROR: sqlc not installed. brew install sqlc / https://docs.sqlc.dev/en/latest/overview/install.html"
    exit 2
fi
sqlc generate
if ! git diff --exit-code -- internal/adapter/pg/sqlc/ internal/platform/db/schema.sql; then
    echo
    echo "ERROR: sqlc-generated files are out of sync with queries.sql/schema.sql."
    echo "       Run \`sqlc generate\` locally + commit the diff."
    exit 1
fi
echo "sqlc-check: bindings match queries.sql + schema.sql ✓"
