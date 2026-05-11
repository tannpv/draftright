#!/usr/bin/env bash
# Publish a new build to the public download URL + update the website manifest.
#
# Usage:
#   release-publish.sh <platform> <version> <local-file> [--meta "size · runtime"]
#
# Examples:
#   release-publish.sh android 2.1.8 /path/to/app-release.apk
#   release-publish.sh macos 2.1.7 /opt/openAi/DraftRight/dist/DraftRight-2.1.7.dmg
#   release-publish.sh windows 2.1.7 /path/to/Setup.exe --meta "Installer · 132 MB · Win 10/11 x64"
#
# What it does:
#   1. Uploads the file to the droplet at /var/www/draftright/downloads/
#   2. Updates /var/www/draftright/downloads/versions.json so the website's
#      download cards immediately reflect the new version.
#   3. Updates the `app_releases` row in the prod database so that
#      /updates/latest (consumed by every desktop app's "Check for Updates")
#      reflects the new version. No backend rebuild, no container restart.
#
# All three changes propagate within seconds of this script finishing.
#
# Filename conventions on the public URL:
#   android  →  DraftRight-Android-<version>.apk
#   ios-sim  →  DraftRight-iOS-Simulator-<version>.zip
#   macos    →  DraftRight-macOS-<version>.dmg
#   windows  →  DraftRight-Setup-Windows-<version>-x64.exe
#   linux    →  DraftRight-Linux-<version>.tar.gz
#
# Requires: ssh alias `draftright`, jq on the droplet (apt-get install jq).
set -euo pipefail

if [ $# -lt 3 ]; then
  echo "Usage: $0 <platform> <version> <local-file> [--meta \"<size · runtime>\"]" >&2
  echo "  platforms: android | ios-sim | macos | windows | linux" >&2
  exit 1
fi

PLATFORM="$1"
VERSION="$2"
LOCAL="$3"
shift 3
META=""
while [ $# -gt 0 ]; do
  case "$1" in
    --meta) META="$2"; shift 2 ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

if [ ! -f "$LOCAL" ]; then
  echo "File not found: $LOCAL" >&2
  exit 1
fi

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

# ── Auto-meta if not provided ──────────────────────────────────────────────
if [ -z "$META" ]; then
  size_bytes=$(stat -f%z "$LOCAL" 2>/dev/null || stat -c%s "$LOCAL" 2>/dev/null)
  size_mb=$(awk -v b="$size_bytes" 'BEGIN{printf "%.1f", b/1048576}')
  case "$PLATFORM" in
    android)  META="APK · ${size_mb} MB · Android 7.0+" ;;
    ios-sim)  META="${size_mb} MB · Mac + Xcode required" ;;
    macos)    META="Universal · ${size_mb} MB · macOS 13+" ;;
    windows)  META="Installer · ${size_mb} MB · Win 10/11 x64" ;;
    linux)    META="Source · ${size_mb} MB · GTK 4 · Python 3.10+" ;;
  esac
fi

echo "==> Publishing"
echo "    platform: $PLATFORM"
echo "    version:  $VERSION"
echo "    file:     $LOCAL"
echo "    remote:   $REMOTE_NAME"
echo "    meta:     $META"
echo

# ── 1. Upload artifact ─────────────────────────────────────────────────────
echo "==> Uploading to droplet..."
scp "$LOCAL" "draftright:/tmp/${REMOTE_NAME}"
ssh draftright "
  set -e
  sudo mv /tmp/${REMOTE_NAME} /var/www/draftright/downloads/${REMOTE_NAME}
  sudo chown www-data:www-data /var/www/draftright/downloads/${REMOTE_NAME}
  sudo chmod 644 /var/www/draftright/downloads/${REMOTE_NAME}
"

# ── 2. Update manifest on droplet via jq ───────────────────────────────────
echo "==> Updating versions.json..."
TODAY="$(date +%Y-%m-%d)"
ssh draftright bash <<EOF
set -e
MAN=/var/www/draftright/downloads/versions.json
TMP=\$(mktemp)
sudo jq --arg p "$PLATFORM" --arg v "$VERSION" --arg u "$URL" --arg m "$META" --arg d "$TODAY" '
  ._updated = \$d
  | (.mobile, .desktop) |= map(
      if .platform == \$p then
        .version = \$v | .url = \$u | .meta = \$m
      else . end
    )
' "\$MAN" > "\$TMP"
sudo mv "\$TMP" "\$MAN"
sudo chown www-data:www-data "\$MAN"
sudo chmod 644 "\$MAN"
EOF

# ── 3. Update app_releases row in the prod DB (drives /updates/latest) ─────
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
  SQL_URL="https://draftright.info$URL"
  ssh draftright "sudo docker exec -i draftright-postgres-1 psql -U draftright -d draftright -v ON_ERROR_STOP=1 <<EOF
-- Always writes the 'direct' channel row. Store-channel rows are managed
-- via the admin Versions page (POST /admin/releases with channel=store).
INSERT INTO app_releases (platform, channel, version, download_url, release_notes, required, enabled)
VALUES ('$DB_PLATFORM', 'direct', '$VERSION', '$SQL_URL', '', false, true)
ON CONFLICT (platform, channel) DO UPDATE SET
  version = EXCLUDED.version,
  download_url = EXCLUDED.download_url,
  updated_at = now();
EOF" 2>&1 | tail -2
fi

# ── 4. Verify ──────────────────────────────────────────────────────────────
echo "==> Verifying..."
HTTP_CODE=$(curl -sS -o /dev/null -w '%{http_code}' "https://draftright.info${URL}")
JSON=$(curl -sS "https://draftright.info/downloads/versions.json")
echo "    binary:    HTTP $HTTP_CODE  →  https://draftright.info${URL}"
echo "$JSON" | python3 -c "
import json, sys
d = json.load(sys.stdin)
for cat in ('mobile','desktop'):
    for p in d.get(cat, []):
        if p.get('platform') == '$PLATFORM':
            print(f'    manifest:  {p[\"label\"]} v{p[\"version\"]} → {p[\"url\"]}')
            print(f'    meta:      {p[\"meta\"]}')
"
if [ -n "$DB_PLATFORM" ]; then
  DB_JSON=$(curl -sS "https://api.draftright.info/updates/latest")
  echo "$DB_JSON" | python3 -c "
import json, sys
d = json.load(sys.stdin)
p = d.get('platforms', {}).get('$DB_PLATFORM')
if p:
    print(f'    /updates/: $DB_PLATFORM v{p[\"version\"]} → {p[\"url\"]}')
"
fi
echo
echo "==> Done. Website + in-app updater both reflect $VERSION."
