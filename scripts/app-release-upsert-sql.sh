#!/usr/bin/env bash
# Emit the `app_releases` upsert SQL for one published artifact, to stdout.
#
# Usage:
#   RELEASE_NOTES="…" app-release-upsert-sql.sh <db-platform> <channel> <version> <download-url> <sha256>
#
# Example:
#   RELEASE_NOTES="- Fixed the thing" \
#     app-release-upsert-sql.sh windows direct 2.3.31 https://draftright.info/downloads/x.exe abc…64hex
#
# Why this exists (Rule #1 — single source of truth):
#   Two release paths write this same row — scripts/release-publish.sh (manual,
#   over the `draftright` ssh alias) and .github/workflows/build-windows.yml
#   (CI, over a deploy key + DROPLET_HOST). They share the SQL but NOT the SSH
#   transport, so the seam is here: this script owns the statement, each caller
#   owns how it reaches psql.
#
#   Keeping the statement in one place is not cosmetic. The two copies drifted
#   once already: `sha256` was added to the script for issue #22 but never to
#   the workflow, so every CI-published Windows release carried an empty hash
#   and the desktop updater's integrity check silently fell back to installing
#   unverified binaries. Adding a column in one place now changes both callers.
#
# Callers are responsible for piping the output to psql, e.g.
#   … | ssh <host> "sudo docker exec -i draftright-postgres-1 \
#         psql -U draftright -d draftright -v ON_ERROR_STOP=1"
set -euo pipefail

if [ $# -ne 5 ]; then
  echo "Usage: RELEASE_NOTES=… $0 <db-platform> <channel> <version> <download-url> <sha256>" >&2
  echo "  db-platform: android | ios | mac | windows | linux" >&2
  echo "  sha256:      64-hex artifact digest, or empty string when unavailable" >&2
  exit 1
fi

DB_PLATFORM="$1"
CHANNEL="$2"
VERSION="$3"
DOWNLOAD_URL="$4"
SHA256="$5"
NOTES="${RELEASE_NOTES-}"

# Reject anything that could break out of the single-quoted SQL literals below.
# These values come from git tags and build metadata, which are attacker-
# influenceable in a fork/PR context — validate rather than trust.
if ! [[ "$DB_PLATFORM" =~ ^(android|ios|mac|windows|linux)$ ]]; then
  echo "::error::Unknown db-platform '$DB_PLATFORM' — expected android|ios|mac|windows|linux" >&2
  exit 1
fi
if ! [[ "$CHANNEL" =~ ^[a-z]+$ ]]; then
  echo "::error::Refusing unsafe channel: $CHANNEL" >&2
  exit 1
fi
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
  echo "::error::Refusing unsafe version string: $VERSION" >&2
  exit 1
fi
if ! [[ "$DOWNLOAD_URL" =~ ^https://[A-Za-z0-9./_-]+$ ]]; then
  echo "::error::Refusing unsafe download URL: $DOWNLOAD_URL" >&2
  exit 1
fi
# Empty is allowed (back-compat: clients warn + install unverified), but a
# non-empty value must be a real digest — a truncated or malformed hash would
# make every client abort the install.
if [ -n "$SHA256" ] && ! [[ "$SHA256" =~ ^[0-9a-f]{64}$ ]]; then
  echo "::error::sha256 must be 64 lowercase hex chars (or empty), got: $SHA256" >&2
  exit 1
fi

# Notes are dollar-quoted so multi-line CHANGELOG text needs no escaping.
#
# The tag must NOT appear inside the notes themselves. A fixed `$drnote$` is an
# injection hole: notes containing that literal close the quote early and the
# rest of the line is executed as SQL, against prod. Every other field here is
# validated; this one cannot be, because release notes are free text. So pick a
# tag and confirm it is absent from the body instead.
TAG="drnote"
i=0
while printf '%s' "$NOTES" | grep -qF "\$${TAG}\$"; do
  i=$((i + 1))
  TAG="drnote${i}"
  if [ "$i" -gt 100 ]; then
    echo "::error::could not find a dollar-quote tag absent from the release notes" >&2
    exit 1
  fi
done

# Only the 'direct' channel is written here; store-channel rows are managed via
# the admin Versions page (POST /admin/releases with channel=store).
cat <<SQL_EOF
INSERT INTO app_releases (platform, channel, version, download_url, sha256, release_notes, required, enabled)
VALUES ('$DB_PLATFORM', '$CHANNEL', '$VERSION', '$DOWNLOAD_URL', '$SHA256', \$${TAG}\$
$NOTES
\$${TAG}\$, false, true)
ON CONFLICT (platform, channel) DO UPDATE SET
  version = EXCLUDED.version,
  download_url = EXCLUDED.download_url,
  sha256 = EXCLUDED.sha256,
  release_notes = EXCLUDED.release_notes,
  updated_at = now();
SQL_EOF
