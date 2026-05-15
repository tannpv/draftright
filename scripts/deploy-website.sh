#!/usr/bin/env bash
# Deploy the marketing website to prod.
#
# CRITICAL: must NOT delete /var/www/draftright/downloads/ — those binary
# artifacts (DraftRight-Android-X.Y.Z.apk, .dmg, .exe, .tar.gz) live alongside
# the static site but are not built by `npm run build`. The 2026-05-01 deploy
# wiped them by accident; --exclude=downloads guards against that. A second
# wipe happened around 2026-05-13 (cause unknown — likely an ad-hoc rsync
# without this script). To make recurrence loud + recoverable, this script
# now (a) refuses to deploy if downloads/ is empty on the droplet,
# (b) re-verifies a known binary after the deploy, and (c) hard-fails if
# the verification trips Caddy's HTML fallback.
set -euo pipefail

REMOTE_HOST="draftright"
REMOTE_PATH="/var/www/draftright"
DOWNLOADS_PATH="$REMOTE_PATH/downloads"
# Known-good sentinel binary that must keep serving a real DMG after deploy.
# Update this when the macOS line moves; the file just needs to exist in
# /var/www/draftright/downloads/ so the post-check has something to verify.
SENTINEL_URL="https://draftright.info/downloads/DraftRight-macOS-2.2.5.dmg"

cd "$(dirname "$0")/../website"

echo "==> Pre-check: downloads/ on droplet"
DOWNLOADS_COUNT=$(ssh "$REMOTE_HOST" "ls $DOWNLOADS_PATH 2>/dev/null | wc -l" || echo "0")
if [ "$DOWNLOADS_COUNT" -lt 1 ]; then
  echo "❌ ABORTED — $DOWNLOADS_PATH is empty or missing on droplet." >&2
  echo "   Restore from backup before deploying:" >&2
  echo "   ssh $REMOTE_HOST 'sudo ls /var/www | grep draftright.bak'" >&2
  echo "   ssh $REMOTE_HOST 'sudo cp -r /var/www/draftright.bak.YYYYMMDD-*/downloads $DOWNLOADS_PATH'" >&2
  exit 1
fi
echo "    downloads/ has $DOWNLOADS_COUNT entries — OK"

echo "==> Building Astro site"
npm run build

echo "==> Rsyncing dist/ → droplet ($REMOTE_PATH/) — downloads/ excluded (triple-belt)"
rsync -av --delete \
  --exclude='downloads' \
  --exclude='downloads/' \
  --exclude='downloads/**' \
  dist/ "$REMOTE_HOST:$REMOTE_PATH/"

echo "==> Post-check: downloads/ still populated"
POST_COUNT=$(ssh "$REMOTE_HOST" "ls $DOWNLOADS_PATH 2>/dev/null | wc -l" || echo "0")
if [ "$POST_COUNT" -lt "$DOWNLOADS_COUNT" ]; then
  echo "❌ DOWNLOADS WIPE DETECTED — was $DOWNLOADS_COUNT, now $POST_COUNT" >&2
  echo "   Restore IMMEDIATELY from /var/www/draftright.bak.* on droplet." >&2
  exit 1
fi
echo "    downloads/ retained $POST_COUNT entries — OK"

echo "==> Verifying sentinel binary serves real bytes (not Caddy HTML fallback)"
CT=$(curl -sSI -m 5 "$SENTINEL_URL" | awk -F': ' '/^content-type:/ { print $2 }' | tr -d '\r\n' || echo "unknown")
CL=$(curl -sSI -m 5 "$SENTINEL_URL" | awk -F': ' '/^content-length:/ { print $2 }' | tr -d '\r\n' || echo "0")
if [[ "$CT" == text/html* ]]; then
  echo "❌ SENTINEL FAILED — $SENTINEL_URL returned content-type=$CT (Caddy HTML fallback)." >&2
  echo "   This means the file is missing under $DOWNLOADS_PATH. Restore from backup." >&2
  exit 1
fi
if [ "${CL:-0}" -lt 100000 ]; then
  echo "❌ SENTINEL FAILED — $SENTINEL_URL content-length=$CL is too small (<100 KB)." >&2
  exit 1
fi
echo "    sentinel OK — $CT, $CL bytes"

echo "==> Deployed. Smoke-testing key paths:"
for path in / /signup /verify-email /pricing /download /account /feedback; do
  printf "    https://draftright.info%-15s -> HTTP %s\n" "$path" \
    "$(curl -sS -o /dev/null -w '%{http_code}' "https://draftright.info$path")"
done
