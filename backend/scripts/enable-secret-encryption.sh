#!/usr/bin/env bash
#
# enable-secret-encryption.sh — guided enable of #50 secret-at-rest encryption.
#
# Turns the key-gated cipher (already deployed) ON for one environment:
#   backup → widen columns → encrypt existing rows → verify.
# It does NOT set the key on running backends or restart them — that is
# environment-specific and printed at the end for you to run.
#
# PREREQUISITES (the script refuses to run without them):
#   1. #49 done — every secret ROTATED at source. Encryption protects the DB,
#      not values that were historically admin-readable.
#   2. Phases 1-3 code deployed to the target backends (read-both, still no-op
#      until the key is set).
#   3. psql + pg_dump on PATH, and node/npx for the encrypt migration.
#
# Usage:
#   DATABASE_URL=postgres://…  ./enable-secret-encryption.sh [--gen-key] [--dry-run]
#   SECRETS_ENCRYPTION_KEY=<base64-32b> DATABASE_URL=… ./enable-secret-encryption.sh
#
#   --gen-key   generate a fresh 32-byte base64 key, print it, and use it
#   --dry-run   show every step without touching the database
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WIDEN_SQL="$BACKEND_DIR/sql/2026-07-30-widen-secret-columns.sql"
ENCRYPT_TS="$SCRIPT_DIR/encrypt-secrets.ts"

DRY_RUN=false
GEN_KEY=false
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    --gen-key) GEN_KEY=true ;;
    *) echo "unknown arg: $arg" >&2; exit 2 ;;
  esac
done

die() { echo "ERROR: $*" >&2; exit 1; }
run() { echo "+ $*"; $DRY_RUN || "$@"; }

# ── Preflight ────────────────────────────────────────────────────────────────
: "${DATABASE_URL:?set DATABASE_URL to the target database}"
command -v psql    >/dev/null || die "psql not found on PATH"
command -v pg_dump >/dev/null || die "pg_dump not found on PATH"
command -v npx     >/dev/null || die "npx (node) not found on PATH"
[ -f "$WIDEN_SQL" ]  || die "missing $WIDEN_SQL"
[ -f "$ENCRYPT_TS" ] || die "missing $ENCRYPT_TS"

if $GEN_KEY; then
  SECRETS_ENCRYPTION_KEY="$(openssl rand -base64 32)"
  echo "==> Generated SECRETS_ENCRYPTION_KEY (STORE THIS SECURELY — losing it makes"
  echo "    every encrypted secret unrecoverable):"
  echo "    $SECRETS_ENCRYPTION_KEY"
fi
: "${SECRETS_ENCRYPTION_KEY:?set SECRETS_ENCRYPTION_KEY (base64 of 32 bytes) or pass --gen-key}"

# Validate the key decodes to exactly 32 bytes.
KEY_BYTES="$(printf '%s' "$SECRETS_ENCRYPTION_KEY" | base64 --decode 2>/dev/null | wc -c | tr -d ' ')"
[ "$KEY_BYTES" = "32" ] || die "SECRETS_ENCRYPTION_KEY must decode to 32 bytes (got $KEY_BYTES)"

echo
echo "Target DB : $(printf '%s' "$DATABASE_URL" | sed -E 's#(://[^:]+):[^@]*@#\1:***@#')"
echo "Dry run   : $DRY_RUN"
echo
echo "This will WIDEN secret columns to text and ENCRYPT existing secrets in place."
echo "Confirm you have (1) rotated all secrets (#49) and (2) a way to restore this DB."
read -r -p "Type 'ENABLE' to proceed: " confirm
[ "$confirm" = "ENABLE" ] || die "aborted"

# ── 1. Backup the two affected tables ────────────────────────────────────────
STAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP="/tmp/draftright-secrets-backup-$STAMP.sql"
echo "==> Backing up ai_providers + app_settings → $BACKUP"
run pg_dump "$DATABASE_URL" -t ai_providers -t app_settings --no-owner --file "$BACKUP"
$DRY_RUN || echo "    backup written ($(wc -c <"$BACKUP" | tr -d ' ') bytes)"

# ── 2. Widen the secret columns (varchar → text) ─────────────────────────────
echo "==> Widening secret columns (idempotent)"
run psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$WIDEN_SQL"

# ── 3. Encrypt existing rows (idempotent — skips enc:v1:) ─────────────────────
echo "==> Encrypting existing secrets"
if $DRY_RUN; then
  echo "+ SECRETS_ENCRYPTION_KEY=*** DATABASE_URL=*** npx ts-node $ENCRYPT_TS"
else
  ( cd "$BACKEND_DIR" && SECRETS_ENCRYPTION_KEY="$SECRETS_ENCRYPTION_KEY" DATABASE_URL="$DATABASE_URL" \
      npx ts-node scripts/encrypt-secrets.ts )
fi

# ── 4. Verify ────────────────────────────────────────────────────────────────
echo "==> Verifying (expect enc:v1: on non-empty secrets)"
if ! $DRY_RUN; then
  psql "$DATABASE_URL" -tAc \
    "SELECT 'ai_providers.api_key encrypted=' || count(*) FILTER (WHERE api_key LIKE 'enc:v1:%') || '/' || count(*) FILTER (WHERE api_key <> '') FROM ai_providers;"
  psql "$DATABASE_URL" -tAc \
    "SELECT 'stripe_secret_key encrypted=' || (stripe_secret_key LIKE 'enc:v1:%')::text FROM app_settings LIMIT 1;"
fi

# ── 5. Next steps (manual, per environment) ──────────────────────────────────
cat <<NEXT

✅ Database is widened + encrypted. Now set the key on the running backends and
   recreate them (encryption stays inert until the key is present):

  # --- TESTING (dev stack) ---
  ssh draftright
  # add to /home/deploy/deploys/draftright-dev/.env.dev:
  #   SECRETS_ENCRYPTION_KEY=$SECRETS_ENCRYPTION_KEY
  cd /home/deploy/deploys/draftright-dev
  DC="docker compose -f docker-compose.yml -f docker-compose.dev.yml"
  \$DC --env-file .env.dev up -d --force-recreate backend-dev backend-go-dev

  # --- PRODUCTION ---
  ssh draftright
  # add SECRETS_ENCRYPTION_KEY=… to BOTH:
  #   /opt/draftright/.env                 (Go rewrite backend)
  #   the Node backend's env-file          (if a live Node backend serves admin)
  cd /opt/draftright && docker compose -f docker-compose.prod.yml up -d --force-recreate backend-go

THEN VERIFY end-to-end (do NOT skip):
  - one AI rewrite succeeds,
  - EACH enabled payment provider (checkout + webhook),
  - a test email sends.

ROLLBACK: restore $BACKUP and unset SECRETS_ENCRYPTION_KEY, then recreate.
NEXT
