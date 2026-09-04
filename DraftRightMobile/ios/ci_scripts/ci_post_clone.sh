#!/bin/sh
# Xcode Cloud post-clone hook: provision Flutter before Runner.xcworkspace
# builds. Xcode Cloud machines have Xcode + CocoaPods but no Flutter, and the
# Runner build phases shell out to the Flutter tool.
#
# The version below is pinned to the same toolchain as the self-hosted CI —
# scripts/check-flutter-version-parity.py asserts all pins agree.
set -e

FLUTTER_VERSION=3.41.4

git clone https://github.com/flutter/flutter.git -b "$FLUTTER_VERSION" --depth 1 "$HOME/flutter"
export PATH="$HOME/flutter/bin:$PATH"

flutter precache --ios

# Xcode Cloud clones the MIRROR repo (tannpv/draftrightmobile), whose root IS
# the Flutter project; in the monorepo the project sits under DraftRightMobile/.
cd "$CI_PRIMARY_REPOSITORY_PATH"
[ -d DraftRightMobile ] && cd DraftRightMobile
flutter pub get

cd ios
pod install
