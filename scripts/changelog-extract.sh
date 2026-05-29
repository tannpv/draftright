#!/usr/bin/env bash
# Print the release-note body for a single version + (optionally) one platform
# from CHANGELOG.md.
#
# Usage:
#   changelog-extract.sh <version> [platform] [changelog-path]
#
# Without a platform: prints every bullet under "## <version>" up to the next
# "## " heading (legacy behavior).
#
# With a platform (macos|windows|android|ios|linux): prints only bullets under
# the matching "### <Platform-Label>" sub-section PLUS bullets under
# "### All platforms". Sub-sections for other platforms are skipped, so
# e.g. Windows-only notes never leak into the macOS publish.
#
# The version is matched against the heading's first token, so
# "## 2.3.6 — 2026-05-20" is found by "2.3.6". Exits non-zero (and prints
# nothing) when the version has no notes — the release pipeline treats that as
# "notes are required, fail the release".
set -euo pipefail

VERSION="${1:?usage: changelog-extract.sh <version> [platform] [changelog-path]}"
PLATFORM=""
FILE="CHANGELOG.md"

# Args 2 and 3 are positional and either may be the platform or the path.
# Distinguish by whether the value is a known platform code.
for arg in "${2:-}" "${3:-}"; do
  [ -z "$arg" ] && continue
  case "$arg" in
    macos|windows|android|ios|linux) PLATFORM="$arg" ;;
    *) FILE="$arg" ;;
  esac
done

if [ ! -f "$FILE" ]; then
  echo "changelog-extract: $FILE not found" >&2
  exit 2
fi

# Map the publish-script's platform code to the CHANGELOG sub-heading label.
case "$PLATFORM" in
  macos)   PLATFORM_LABEL="macOS" ;;
  windows) PLATFORM_LABEL="Windows" ;;
  android) PLATFORM_LABEL="Android" ;;
  ios)     PLATFORM_LABEL="iOS" ;;
  linux)   PLATFORM_LABEL="Linux" ;;
  *)       PLATFORM_LABEL="" ;;
esac

OUT="$(awk \
  -v ver="$VERSION" \
  -v platform_label="$PLATFORM_LABEL" '
  function trim(s) { sub(/^[[:space:]]+/, "", s); sub(/[[:space:]]+$/, "", s); return s }

  # Top-level version heading: enter/exit capture mode.
  /^## / {
    if (capture) exit
    if ($2 == ver) {
      capture = 1
      include_section = (platform_label == "")  # no filter → include everything
    }
    next
  }

  !capture { next }

  # Platform sub-heading inside the version section.
  /^### / {
    section = trim(substr($0, 5))   # strip "### "
    if (platform_label == "") {
      include_section = 1
    } else {
      include_section = (section == platform_label || section == "All platforms")
    }
    next
  }

  # Body lines (bullets, blank lines).
  include_section {
    if ($0 ~ /[^[:space:]]/) {
      for (; blanks > 0; blanks--) print ""
      print
      started = 1
    } else if (started) {
      blanks++
    }
  }
' "$FILE")"

if [ -z "$OUT" ]; then
  echo "changelog-extract: no notes for ${VERSION}${PLATFORM:+ ($PLATFORM)} in $FILE" >&2
  exit 1
fi

printf '%s\n' "$OUT"
