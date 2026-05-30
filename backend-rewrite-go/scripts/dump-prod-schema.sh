#!/bin/bash
# Refreshes the read-only mirror of the prod Postgres schema that sqlc
# compiles against. NestJS owns the schema; this script just snapshots
# the current state.
#
# Usage:
#   ./scripts/dump-prod-schema.sh
#
# Run after any new migration in backend/sql/*.sql that touches a table
# we read here, then run sqlc generate + commit the diff.
#
# Requires the `draftright` SSH alias (prod droplet).
set -euo pipefail
cd "$(dirname "$0")/.."
echo "==> dumping prod schema → internal/platform/db/schema.sql"
ssh draftright "sudo docker exec draftright-postgres-1 pg_dump --schema-only --no-owner --no-privileges -U draftright draftright" \
  > internal/platform/db/schema.sql
echo "==> $(wc -l < internal/platform/db/schema.sql) lines"
echo "==> regenerate sqlc bindings:"
echo "    sqlc generate"
