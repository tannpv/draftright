#!/usr/bin/env bash
# Get the direct Windows installer (.exe) for the current (or a given) build and
# stage it to the public /downloads dir, so you can download + install it on a
# Windows machine after a fix.
#
#   scripts/windows-install.sh [ref]               # ref = branch or v-tag; default: current branch
#   FORCE_BUILD=1 scripts/windows-install.sh <ref> # dispatch a fresh CI build first
#
# Fetches the installer CI built (build-windows.yml) — it cannot be built on
# macOS. The installer is unsigned, so Smart App Control may block it; the Store
# build is the signed path.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/windows-artifacts.lib.sh

VERSION="$(wa_version)"
REF="${1:-$(git branch --show-current)}"
echo "==> Windows installer for '$REF' (app version $VERSION)"

RUN=$(wa_resolve_run "$REF")
EXE=$(wa_download "$RUN" "DraftRight-Setup-win-x64")

NAME="DraftRight-Install-$VERSION-x64.exe"
URL=$(wa_stage "$EXE" "$NAME")
wa_verify "$URL"

cat <<EOF

✅ Windows installer staged (v$VERSION):
   $URL

On the Windows machine:
  1. Download the .exe from the URL above
  2. If Smart App Control blocks it: Windows Security → App & browser control →
     Smart App Control → Off, then run it (or install from the Microsoft Store)
  3. Run it; the app lands in the system tray (Ctrl+Shift+R to rewrite)

Remove it from /downloads when done:
  ssh $DROPLET "sudo rm -f $DOWNLOADS_PATH/$NAME"
EOF
