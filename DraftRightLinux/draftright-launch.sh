#!/bin/bash
# Dev launcher for DraftRight Linux.
#
# On Wayland the global shortcut goes through xdg-desktop-portal's
# GlobalShortcuts API, and the portal refuses any caller it cannot identify
# ("NotAllowed: An app id is required").  For an unsandboxed app the portal
# derives that id from the systemd scope the process runs in, so a bare
# `python3 -m draftright` from a terminal never gets a hotkey.
#
# Launching inside an app-<launcher>-<app-id>-<pid>.scope gives the portal what
# it needs, and the desktop entry must exist for it to resolve.  This mirrors
# how a desktop launcher (or Flatpak) starts the app, so dev runs behave like
# real ones.
set -euo pipefail

APP_ID="com.draftright.app"
DESKTOP_FILE="$HOME/.local/share/applications/${APP_ID}.desktop"
SOURCE_DESKTOP="$(dirname "$0")/data/${APP_ID}.desktop"

if [ ! -f "$DESKTOP_FILE" ] && [ -f "$SOURCE_DESKTOP" ]; then
    echo "note: installing ${DESKTOP_FILE} so the Wayland global shortcut" >&2
    echo "      can be granted." >&2
    mkdir -p "$(dirname "$DESKTOP_FILE")"
    cp "$SOURCE_DESKTOP" "$DESKTOP_FILE"
    update-desktop-database "$(dirname "$DESKTOP_FILE")" 2>/dev/null || true
fi

if [ "${XDG_SESSION_TYPE:-}" = "wayland" ] && command -v systemd-run >/dev/null 2>&1; then
    exec systemd-run --user --scope -q \
        --unit="app-gnome-${APP_ID}-$$" \
        python3 -m draftright "$@"
fi

exec python3 -m draftright "$@"
