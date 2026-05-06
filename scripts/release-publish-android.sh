#!/usr/bin/env bash
# Build and upload an Android AAB to Play Console via fastlane supply.
# Closes today's "version code already used" / drag-drop loop.
#
# Usage:
#   scripts/release-publish-android.sh <track>           # use current pubspec version
#   scripts/release-publish-android.sh <track> --bump    # bump versionCode +1 first
#   scripts/release-publish-android.sh closed --bump --also-sideload   # publish APK to draftright.info too
#
# Tracks:
#   internal               — internal testing (up to 100 testers, no review)
#   closed                 — closed testing (alpha — your 12 testers, 14-day clock)
#   production_draft       — production track, draft state (manual promote required)
#   production_rollout_10  — production with 10% staged rollout
#
# Requirements (one-time setup, already done):
#   - DraftRightMobile/android/play-console-key.json (gitignored)
#   - DraftRightMobile/android/Gemfile.lock with fastlane installed
#   - Service account has Release Manager role in Play Console
#
# What this does:
#   1. Optionally bumps versionCode in pubspec.yaml
#   2. Builds AAB via `flutter build appbundle --release`
#   3. Stashes a copy at ~/Downloads/DraftRight-Android-<version>-<code>.aab
#   4. Uploads to the chosen track via fastlane supply
#   5. Optionally publishes the equivalent APK to draftright.info/downloads/
#      (and updates the DB-backed /updates/latest manifest)

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
MOBILE_DIR="$ROOT/DraftRightMobile"
ANDROID_DIR="$MOBILE_DIR/android"
PUBSPEC="$MOBILE_DIR/pubspec.yaml"
AAB_PATH="$MOBILE_DIR/build/app/outputs/bundle/release/app-release.aab"
APK_PATH="$MOBILE_DIR/build/app/outputs/flutter-apk/app-release.apk"

if [ $# -lt 1 ]; then
  echo "Usage: $0 <internal|closed|production_draft|production_rollout_10> [--bump] [--also-sideload]" >&2
  exit 1
fi

TRACK="$1"
shift
case "$TRACK" in
  internal|closed|production_draft|production_rollout_10) ;;
  *) echo "Invalid track '$TRACK'. Expected: internal, closed, production_draft, production_rollout_10" >&2; exit 1 ;;
esac

BUMP=0
SIDELOAD=0
while [ $# -gt 0 ]; do
  case "$1" in
    --bump)          BUMP=1; shift ;;
    --also-sideload) SIDELOAD=1; shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

# ── 1. Optionally bump versionCode ─────────────────────────────────────────
CURRENT=$(grep -E "^version:" "$PUBSPEC" | head -1 | sed -E 's/version: *//')
NAME="${CURRENT%%+*}"
CODE="${CURRENT##*+}"
echo "==> Current pubspec version: $NAME+$CODE"

if [ "$BUMP" = "1" ]; then
  NEW_CODE=$((CODE + 1))
  NEW_VERSION="${NAME}+${NEW_CODE}"
  echo "==> Bumping versionCode: $CODE → $NEW_CODE"
  /usr/bin/sed -i '' -E "s/^version: ${NAME}\+${CODE}\$/version: ${NEW_VERSION}/" "$PUBSPEC"
  CURRENT="$NEW_VERSION"
  CODE="$NEW_CODE"
  # Verify
  grep -E "^version:" "$PUBSPEC" | head -1
fi

# ── 2. Build AAB ───────────────────────────────────────────────────────────
echo "==> Building Android AAB..."
cd "$MOBILE_DIR"
flutter build appbundle --release

if [ ! -f "$AAB_PATH" ]; then
  echo "AAB not produced at $AAB_PATH" >&2
  exit 1
fi

DEST="$HOME/Downloads/DraftRight-Android-${NAME}-${CODE}.aab"
cp "$AAB_PATH" "$DEST"
SIZE=$(du -h "$DEST" | cut -f1)
echo "==> Stashed AAB: $DEST ($SIZE)"

# ── 3. Upload via fastlane ─────────────────────────────────────────────────
echo "==> Uploading to Play Console track: $TRACK"
cd "$ANDROID_DIR"

if [ ! -f "play-console-key.json" ] && [ -z "${PLAY_CONSOLE_JSON_KEY_PATH:-}" ]; then
  echo "Service account key not found at android/play-console-key.json" >&2
  echo "Set PLAY_CONSOLE_JSON_KEY_PATH env var or place the key file there." >&2
  exit 1
fi

export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
bundle exec fastlane "$TRACK"

# ── 4. Optionally also publish APK to public sideload URL ──────────────────
if [ "$SIDELOAD" = "1" ]; then
  echo "==> Also building APK + publishing to draftright.info/downloads/"
  cd "$MOBILE_DIR"
  flutter build apk --release
  if [ ! -f "$APK_PATH" ]; then
    echo "APK not produced at $APK_PATH" >&2
    exit 1
  fi
  cd "$ROOT"
  scripts/release-publish.sh android "$NAME" "$APK_PATH"
fi

echo ""
echo "==> Done"
echo "    Version:    $NAME+$CODE"
echo "    Track:      $TRACK"
echo "    AAB stash:  $DEST"
if [ "$SIDELOAD" = "1" ]; then
  echo "    Sideload:   https://draftright.info/downloads/DraftRight-Android-${NAME}.apk"
fi
