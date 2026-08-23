#!/usr/bin/env bash
# Publish a new build to the public download URL + the in-app updater.
#
# Usage:
#   release-publish.sh <platform> <version> <local-file>
#
# Examples:
#   release-publish.sh android 2.1.8 /path/to/app-release.apk
#   release-publish.sh macos 2.1.7 /opt/openAi/DraftRight/dist/DraftRight-2.1.7.dmg
#   release-publish.sh windows 2.1.7 /path/to/Setup.exe
#
# What it does:
#   1. Uploads the file to the prod download dir ($DR_DOWNLOADS_DIR).
#   2. Updates the `app_releases` row in the prod database so that
#      /updates/latest (consumed by every desktop app's "Check for Updates"
#      AND the website download cards, at build time) reflects the new version.
#      No backend rebuild, no container restart.
#
# Both changes propagate within seconds of this script finishing.
#
# `app_releases` / /updates/latest is the SINGLE source of truth for versions
# (the Astro site's DownloadCTA reads it at build time). The old
# downloads/versions.json manifest was a second source that nobody reads any
# more — it was removed here rather than re-seeded on the migrated host.
#
# Filename conventions on the public URL:
#   android  →  DraftRight-Android-<version>.apk
#   ios-sim  →  DraftRight-iOS-Simulator-<version>.zip
#   macos    →  DraftRight-macOS-<version>.dmg
#   windows  →  DraftRight-Setup-Windows-<version>-x64.exe
#   linux    →  DraftRight-Linux-<version>.tar.gz
#
# Requires: ssh access to $DR_DEPLOY_SSH.
set -euo pipefail

# ── Deploy targets — single source of truth, env-overridable (Rule #1) ──────
# Defaults point at the Contabo prod box (prod migrated off the DigitalOcean
# droplet 2026-08-23; the old `ssh draftright` alias → 129.212.208.248 is dead).
# These were previously literals repeated across the file (host ×4, download dir
# ×4, public base ×3) — two copies drift, so they live here once. Override any
# via env to publish to a different host/layout (e.g. a future dev target).
DR_DEPLOY_SSH="${DR_DEPLOY_SSH:-deploy@169.58.214.18}"              # was ssh alias `draftright`
DR_DOWNLOADS_DIR="${DR_DOWNLOADS_DIR:-/srv/draftright/downloads}"   # Caddy serves /downloads/* from here (was /var/www/draftright/downloads on DO)
DR_DOWNLOADS_OWNER="${DR_DOWNLOADS_OWNER:-deploy:deploy}"           # dir is deploy-owned on Contabo (was www-data:www-data on DO)
DR_PUBLIC_BASE="${DR_PUBLIC_BASE:-https://draftright.info}"         # static /downloads host
DR_API_BASE="${DR_API_BASE:-https://api.draftright.info}"           # backend /updates/latest
DR_PG_CONTAINER="${DR_PG_CONTAINER:-draftright-postgres-1}"         # prod Postgres container (unchanged across the move)

if [ $# -lt 3 ]; then
  echo "Usage: $0 <platform> <version> <local-file>" >&2
  echo "  platforms: android | ios-sim | macos | windows | linux" >&2
  exit 1
fi

PLATFORM="$1"
VERSION="$2"
LOCAL="$3"
shift 3
if [ $# -gt 0 ]; then
  echo "Unknown argument: $1" >&2
  exit 1
fi

if [ ! -f "$LOCAL" ]; then
  echo "File not found: $LOCAL" >&2
  exit 1
fi

# Resolve once, at top level — the sibling helpers are called from several
# steps below, including ones outside the DB_PLATFORM branch.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Filename + URL by platform ─────────────────────────────────────────────
case "$PLATFORM" in
  android)  REMOTE_NAME="DraftRight-Android-${VERSION}.apk" ;;
  ios-sim)  REMOTE_NAME="DraftRight-iOS-Simulator-${VERSION}.zip" ;;
  macos)    REMOTE_NAME="DraftRight-macOS-${VERSION}.dmg" ;;
  windows)  REMOTE_NAME="DraftRight-Setup-Windows-${VERSION}-x64.exe" ;;
  linux)    REMOTE_NAME="DraftRight-Linux-${VERSION}.tar.gz" ;;
  *) echo "Unknown platform '$PLATFORM' — expected android|ios-sim|macos|windows|linux" >&2; exit 1 ;;
esac
URL="/downloads/${REMOTE_NAME}"

# ── Artifact size ──────────────────────────────────────────────────────────
# The verify step compares this against the served Content-Length to prove the
# upload actually landed (issue #144).
size_bytes=$(stat -f%z "$LOCAL" 2>/dev/null || stat -c%s "$LOCAL" 2>/dev/null)

# ── Artifact SHA-256 (integrity — desktop updaters verify against this) ────
# Portable: sha256sum on Linux, shasum on macOS.
if command -v sha256sum >/dev/null 2>&1; then
  SHA256=$(sha256sum "$LOCAL" | awk '{print $1}')
else
  SHA256=$(shasum -a 256 "$LOCAL" | awk '{print $1}')
fi

echo "==> Publishing"
echo "    platform: $PLATFORM"
echo "    version:  $VERSION"
echo "    file:     $LOCAL"
echo "    remote:   $REMOTE_NAME"
echo "    sha256:   $SHA256"
echo

# ── 1. Upload artifact ─────────────────────────────────────────────────────
echo "==> Uploading to $DR_DEPLOY_SSH..."
scp "$LOCAL" "$DR_DEPLOY_SSH:/tmp/${REMOTE_NAME}"
ssh "$DR_DEPLOY_SSH" "
  set -e
  sudo mv /tmp/${REMOTE_NAME} ${DR_DOWNLOADS_DIR}/${REMOTE_NAME}
  sudo chown ${DR_DOWNLOADS_OWNER} ${DR_DOWNLOADS_DIR}/${REMOTE_NAME}
  sudo chmod 644 ${DR_DOWNLOADS_DIR}/${REMOTE_NAME}
"

# ── 2. Update app_releases row in the prod DB (drives /updates/latest) ─────
case "$PLATFORM" in
  android)  DB_PLATFORM="android" ;;
  ios-sim)  DB_PLATFORM="ios" ;;
  macos)    DB_PLATFORM="mac" ;;
  windows)  DB_PLATFORM="windows" ;;
  linux)    DB_PLATFORM="linux" ;;
  *) DB_PLATFORM="" ;;
esac

if [ -n "$DB_PLATFORM" ]; then
  echo "==> Updating app_releases.$DB_PLATFORM in prod DB..."
  SQL_URL="${DR_PUBLIC_BASE}$URL"

  # Pull this version's user-facing note from CHANGELOG.md so clients can show
  # a "What's New" notice after updating. Best-effort here (manual path) — a
  # missing section just publishes empty notes with a warning; the CI release
  # path enforces it.
  # Pass the platform code so the extractor only returns notes scoped to this
  # release target (### macOS / ### Windows / ### All platforms). Otherwise
  # Windows-only bullets leak into the macOS publish and vice-versa. Pipeline
  # arg "ios-sim" maps to extractor code "ios"; others pass through.
  case "$PLATFORM" in
    ios-sim) NOTES_PLATFORM="ios" ;;
    *)       NOTES_PLATFORM="$PLATFORM" ;;
  esac
  if NOTES="$("$SCRIPT_DIR/changelog-extract.sh" "$VERSION" "$NOTES_PLATFORM" "$SCRIPT_DIR/../CHANGELOG.md" 2>/dev/null)"; then
    echo "    notes:    $(printf '%s' "$NOTES" | head -1) …"
  else
    echo "    notes:    (!) no CHANGELOG.md section for $VERSION — publishing empty notes" >&2
    NOTES=""
  fi

  # Build the SQL locally and pipe it to the remote psql over stdin. The
  # statement itself lives in app-release-upsert-sql.sh so this path and the
  # CI path (.github/workflows/build-windows.yml) can never drift — see the
  # header of that script for why that matters (issue #22).
  SQL=$(RELEASE_NOTES="$NOTES" \
    "$SCRIPT_DIR/app-release-upsert-sql.sh" "$DB_PLATFORM" direct "$VERSION" "$SQL_URL" "$SHA256")
  printf '%s\n' "$SQL" | ssh "$DR_DEPLOY_SSH" "sudo docker exec -i ${DR_PG_CONTAINER} psql -U draftright -d draftright -v ON_ERROR_STOP=1" 2>&1 | tail -2
fi

# ── 3. Verify ──────────────────────────────────────────────────────────────
echo "==> Verifying..."

# Assert the artifact is really there. This used to print the HTTP status and
# continue regardless, which is why a missing Linux artifact stayed advertised
# for months: /downloads falls through to the marketing site's SPA handler, so
# a missing file answers 200 with HTML (issue #144). The helper checks the
# status, the content type, and the byte count.
"$SCRIPT_DIR/verify-published-artifact.sh" \
  "${DR_PUBLIC_BASE}${URL}" "$size_bytes" --sha256 "$SHA256"

if [ -n "$DB_PLATFORM" ]; then
  DB_JSON=$(curl -sS "${DR_API_BASE}/updates/latest")
  echo "$DB_JSON" | DB_PLATFORM="$DB_PLATFORM" python3 -c "
import json, os, sys
d = json.load(sys.stdin)
want = os.environ['DB_PLATFORM']
p = d.get('platforms', {}).get(want)
if p:
    print(f'    /updates/: {want} v{p.get(\"version\", \"?\")} → {p.get(\"url\", \"?\")}')
    print(f'    sha256:    {p.get(\"sha256\") or \"(none — unverified)\"}')
"
fi
echo
echo "==> Done. Website + in-app updater both reflect $VERSION."
