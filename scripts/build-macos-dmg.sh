#!/usr/bin/env bash
# Build DraftRight.app + DraftRight-<version>.dmg for distribution.
#
# Output: dist/DraftRight-<version>.dmg
# Universal binary (arm64 + x86_64), Developer ID signed, notarized,
# stapled. The resulting .dmg installs cleanly on any modern macOS — no
# "right-click → Open" workaround needed.
#
# Signing identity: Developer ID Application: tan nguyen (Y8NWQK9BZJ)
# Notarization: via xcrun notarytool with App Store Connect API key.
#
# Set NOTARIZE=0 to skip notarization (e.g. local dev builds).

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
INFO_PLIST="DraftRight/Info.plist"
ENTITLEMENTS="DraftRight/DraftRight.entitlements"

VERSION=$(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$INFO_PLIST")
BUNDLE_ID=$(/usr/libexec/PlistBuddy -c "Print :CFBundleIdentifier" "$INFO_PLIST")

# Developer ID Application identity for code signing (release builds).
SIGN_IDENTITY="${SIGN_IDENTITY:-Developer ID Application: tan nguyen (Y8NWQK9BZJ)}"

# App Store Connect API key for notarytool. The same key the iOS pipeline
# uses (admin-scoped, expires when the user revokes it).
ASC_API_KEY_ID="${ASC_API_KEY_ID:-W8U3VAS72T}"
ASC_API_ISSUER_ID="${ASC_API_ISSUER_ID:-88b3df44-6aa5-4157-9598-ef2aa366690b}"
ASC_API_KEY_PATH="${ASC_API_KEY_PATH:-$HOME/Downloads/AuthKey_${ASC_API_KEY_ID}.p8}"

NOTARIZE="${NOTARIZE:-1}"

DIST="$ROOT/dist"
APP="$DIST/DraftRight.app"
DMG="$DIST/DraftRight-${VERSION}.dmg"

echo "==> Cleaning $DIST"
rm -rf "$DIST"
mkdir -p "$DIST"

echo "==> Building arm64 (release)"
swift build -c release --arch arm64
ARM64_BIN="$ROOT/.build/arm64-apple-macosx/release/DraftRight"

echo "==> Building x86_64 (release)"
swift build -c release --arch x86_64
X86_BIN="$ROOT/.build/x86_64-apple-macosx/release/DraftRight"

echo "==> Assembling DraftRight.app bundle"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

lipo -create -output "$APP/Contents/MacOS/DraftRight" "$ARM64_BIN" "$X86_BIN"
chmod +x "$APP/Contents/MacOS/DraftRight"

cp "$INFO_PLIST" "$APP/Contents/Info.plist"
cp "DraftRight/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"

# SwiftPM-built resources bundle (claude-icon.png lives inside it)
SPM_BUNDLE="$ROOT/.build/arm64-apple-macosx/release/DraftRight_DraftRight.bundle"
if [ -d "$SPM_BUNDLE" ]; then
  cp -R "$SPM_BUNDLE" "$APP/Contents/Resources/"
fi

echo "==> Code-signing with Developer ID + Hardened Runtime + entitlements"
# --options runtime turns on Hardened Runtime (required for notarization).
# --timestamp embeds a secure RFC3161 timestamp (also required).
# --deep recurses into nested helpers/frameworks.
codesign --force --deep --options runtime --timestamp \
  --entitlements "$ENTITLEMENTS" \
  --sign "$SIGN_IDENTITY" \
  "$APP"

echo "==> Verifying signature"
codesign --verify --strict --verbose=2 "$APP"

echo "==> Building DMG"
TMP_DMG_DIR="$DIST/dmg-staging"
mkdir -p "$TMP_DMG_DIR"
cp -R "$APP" "$TMP_DMG_DIR/"
ln -s /Applications "$TMP_DMG_DIR/Applications"

# -fs HFS+ forces an HFS+ (not APFS) filesystem. Modern macOS otherwise images
# the source as APFS, whose `hdiutil attach` output puts the mount point on a
# non-last line — the shipped in-app updater (<= 2.3.28) parses the LAST line's
# last field and grabbed "Apple_APFS", failing with "The folder Apple_APFS
# doesn't exist". HFS+ keeps the mount point on the last line so every shipped
# updater can mount the DMG.
hdiutil create \
  -volname "DraftRight $VERSION" \
  -srcfolder "$TMP_DMG_DIR" \
  -fs HFS+ \
  -ov -format UDZO \
  "$DMG"

rm -rf "$TMP_DMG_DIR"

# Sign the DMG itself so Gatekeeper trusts the container too.
codesign --force --sign "$SIGN_IDENTITY" --timestamp "$DMG"

if [ "$NOTARIZE" = "1" ]; then
  echo "==> Submitting DMG to Apple notarization service (this takes 1-3 min)"
  xcrun notarytool submit "$DMG" \
    --key "$ASC_API_KEY_PATH" \
    --key-id "$ASC_API_KEY_ID" \
    --issuer "$ASC_API_ISSUER_ID" \
    --wait

  echo "==> Stapling notarization ticket to DMG (offline-verifiable)"
  xcrun stapler staple "$DMG"

  echo "==> Verifying staple"
  xcrun stapler validate "$DMG"
else
  echo "==> NOTARIZE=0 — skipping notarization (DMG will require right-click → Open)"
fi

SIZE=$(du -h "$DMG" | cut -f1)
SHA=$(shasum -a 256 "$DMG" | awk '{print $1}')

echo ""
echo "==> Done"
echo "    App:     $APP"
echo "    DMG:     $DMG ($SIZE)"
echo "    SHA256:  $SHA"
echo "    Bundle:  $BUNDLE_ID @ $VERSION"
if [ "$NOTARIZE" = "1" ]; then
  echo "    Status:  Developer ID signed + notarized + stapled"
else
  echo "    Status:  Developer ID signed (NOT notarized)"
fi
