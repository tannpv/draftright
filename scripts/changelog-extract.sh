#!/usr/bin/env bash
# Print the release-note body for a single version from CHANGELOG.md.
#
# Usage: changelog-extract.sh <version> [changelog-path]
#
# Captures the lines under the "## <version>" heading up to the next "## "
# heading, trimming surrounding blank lines. The version is matched against the
# heading's first token, so "## 2.3.6 — 2026-05-20" is found by "2.3.6".
# Exits non-zero (and prints nothing) if the version has no section — the
# release pipeline treats that as "notes are required, fail the release".
set -euo pipefail

VERSION="${1:?usage: changelog-extract.sh <version> [changelog-path]}"
FILE="${2:-CHANGELOG.md}"

if [ ! -f "$FILE" ]; then
  echo "changelog-extract: $FILE not found" >&2
  exit 2
fi

OUT="$(awk -v ver="$VERSION" '
  /^## / { if (capture) exit; if ($2 == ver) capture = 1; next }
  capture {
    if ($0 ~ /[^[:space:]]/) { for (; blanks > 0; blanks--) print ""; print; started = 1 }
    else if (started) { blanks++ }
  }
' "$FILE")"

if [ -z "$OUT" ]; then
  echo "changelog-extract: no '## $VERSION' section in $FILE" >&2
  exit 1
fi

printf '%s\n' "$OUT"
