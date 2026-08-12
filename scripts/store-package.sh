#!/usr/bin/env bash
# Get the Windows Store MSIX for the current (or a given) build and stage it to
# the public /downloads dir, ready to reupload in Partner Center.
#
#   scripts/store-package.sh [ref]                 # ref = branch or v-tag; default: v<csproj version>
#   FORCE_BUILD=1 scripts/store-package.sh <ref>   # dispatch a fresh CI build first
#
# The MSIX is built by CI (build-windows.yml msix job); this fetches that
# artifact — it cannot be built on macOS. Package version is stamped from the
# csproj <Version> by CI (#170), so it is always higher than the last submission.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/windows-artifacts.lib.sh

VERSION="$(wa_version)"
REF="${1:-v$VERSION}"
echo "==> Store MSIX for '$REF' (app version $VERSION)"

RUN=$(wa_resolve_run "$REF")
MSIXUPLOAD=$(wa_download "$RUN" "DraftRight-MSIX-Store")

# Confirm the package version inside matches the app (guards the #170 stamp).
TMP=$(mktemp -d); unzip -o -q "$MSIXUPLOAD" -d "$TMP"
INNER=$(basename "$(ls "$TMP"/*.msix | head -1)")
echo "    package: $INNER"
case "$INNER" in
  *_"$VERSION".0_*) echo "    version OK ($VERSION.0)";;
  *) echo "WARNING: MSIX version does not match csproj $VERSION — check the #170 stamp" >&2;;
esac

NAME="DraftRight-Store-$VERSION.msixupload"
URL=$(wa_stage "$MSIXUPLOAD" "$NAME")
wa_verify "$URL"

cat <<EOF

✅ Store package staged (version $VERSION.0):
   $URL

Reupload in Partner Center:
  1. partner.microsoft.com → DraftRight → Start new submission
  2. Packages → drag the downloaded .msixupload
  3. Submit → certification (~1-3 days)

Remove it from /downloads after uploading:
  ssh $DROPLET "sudo rm -f $DOWNLOADS_PATH/$NAME"
EOF
