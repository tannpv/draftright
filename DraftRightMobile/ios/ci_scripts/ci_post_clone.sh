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

cd "$CI_PRIMARY_REPOSITORY_PATH/DraftRightMobile"
flutter pub get

cd ios
pod install
