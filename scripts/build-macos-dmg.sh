#!/usr/bin/env bash
# Build DraftRight.app + DraftRight-<version>.dmg for distribution.
#
# Output: dist/DraftRight-<version>.dmg
# Universal binary (arm64 + x86_64).

set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
INFO_PLIST="DraftRight/Info.plist"

VERSION=$(/usr/libexec/PlistBuddy -c "Print :CFBundleShortVersionString" "$INFO_PLIST")
BUNDLE_ID=$(/usr/libexec/PlistBuddy -c "Print :CFBundleIdentifier" "$INFO_PLIST")

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

echo "==> Ad-hoc signing (no notarization; DMG will require right-click → Open the first time)"
codesign --force --deep --sign - "$APP"

echo "==> Building DMG"
TMP_DMG_DIR="$DIST/dmg-staging"
mkdir -p "$TMP_DMG_DIR"
cp -R "$APP" "$TMP_DMG_DIR/"
ln -s /Applications "$TMP_DMG_DIR/Applications"

hdiutil create \
  -volname "DraftRight $VERSION" \
  -srcfolder "$TMP_DMG_DIR" \
  -ov -format UDZO \
  "$DMG"

rm -rf "$TMP_DMG_DIR"

SIZE=$(du -h "$DMG" | cut -f1)
SHA=$(shasum -a 256 "$DMG" | awk '{print $1}')

echo ""
echo "==> Done"
echo "    App:     $APP"
echo "    DMG:     $DMG ($SIZE)"
echo "    SHA256:  $SHA"
echo "    Bundle:  $BUNDLE_ID @ $VERSION"
