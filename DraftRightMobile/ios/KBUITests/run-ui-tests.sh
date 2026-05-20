#!/usr/bin/env bash
# Run the DraftRight keyboard end-to-end UI tests on a simulator.
#
# These tests drive the REAL keyboard extension typing into a native host
# field (KBTestHost). Several environment preconditions are easy to get
# wrong and produce confusing failures, so this script sets them all up.
#
# Usage: ios/KBUITests/run-ui-tests.sh [SIMULATOR_UDID]
set -euo pipefail

UDID="${1:-}"
TEAM="Y8NWQK9BZJ"
HERE="$(cd "$(dirname "$0")/.." && pwd)"   # ios/
cd "$HERE"

if [[ -z "$UDID" ]]; then
  UDID="$(xcrun simctl list devices booted | grep -oE '[0-9A-F-]{36}' | head -1)"
fi
if [[ -z "$UDID" ]]; then
  echo "No booted simulator. Boot one or pass a UDID." >&2
  exit 1
fi
echo "Simulator: $UDID"

# 1. The software keyboard never presents while the Mac hardware keyboard
#    is "connected" to the sim — every key query then returns nothing.
defaults write com.apple.iphonesimulator ConnectHardwareKeyboard -bool false

# 2. Pin exactly two keyboards so a single globe tap reaches DraftRight.
xcrun simctl spawn "$UDID" defaults write .GlobalPreferences AppleKeyboards -array \
  "en_US@sw=QWERTY;hw=Automatic" \
  "com.draftright.draftrightMobile.v2.DraftRightKeyboard"
xcrun simctl spawn "$UDID" launchctl stop com.apple.SpringBoard || true
sleep 3

# 3. Build + test WITH signing — KBTestHost needs its App Group entitlement
#    applied or the keyboard reads no enabled languages (CODE_SIGNING_ALLOWED=NO
#    silently strips entitlements). The host (KBTestHost) carries the
#    com.draftright.v2 App Group and seeds enabled/active languages on launch;
#    Runner.app (with DraftRightKeyboard.appex) must already be installed.
xcodebuild test \
  -project Runner.xcodeproj \
  -scheme KBUITests \
  -destination "platform=iOS Simulator,id=${UDID}" \
  DEVELOPMENT_TEAM="$TEAM" \
  CODE_SIGN_STYLE=Automatic
