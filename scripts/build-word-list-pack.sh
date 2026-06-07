#!/usr/bin/env bash
# Build a downloadable word-list pack for the suggestion engine and (optionally)
# upload it to the public pack URL.
#
# Usage:
#   build-word-list-pack.sh <lang> <version> <source-tsv> [--publish]
#
#   <lang>        ISO-ish code: "vi" | "en" | "fr" | "es" | "de" | "it" | "pt"
#   <version>     Integer; bumped any time you ship a new pack (matches
#                 LanguagePack.version in the manifest).
#   <source-tsv>  TSV with `word<TAB>frequency` lines. Optional bigram TSV
#                 (`prev<TAB>next<TAB>count`) at `<source-tsv>.bigrams.tsv`
#                 gets bundled when present.
#   --publish     Upload to draftright.info/ime-packs/ and print the
#                 sha256/size to paste into ImePacksService.
#
# Output (always written to dist/):
#   dist/draftright-wordlist-<lang>-v<version>.tsv          — the pack itself
#   dist/draftright-wordlist-<lang>-v<version>.sha256       — hash
#   dist/draftright-wordlist-<lang>-v<version>.meta         — sha256 + size,
#                                                              one per line
#
# Why TSV instead of a binary blob:
#   The current loader (Android + iOS WordListLoader) consumes TSV directly.
#   A 5k-entry pack is ~80 KB uncompressed, well under any latency budget.
#   When we cross 50k entries / second prediction starts costing >1 ms we'll
#   switch to a binary CSR format and update this script — until then YAGNI.

set -euo pipefail

LANG_CODE="${1:?usage: build-word-list-pack.sh <lang> <version> <source-tsv> [--publish]}"
VERSION="${2:?missing version}"
SOURCE="${3:?missing source TSV path}"
PUBLISH="${4:-}"

if [ ! -f "$SOURCE" ]; then
  echo "build-word-list-pack: $SOURCE not found" >&2
  exit 2
fi

cd "$(dirname "$0")/.."
DIST="$(pwd)/dist"
mkdir -p "$DIST"

PACK_NAME="draftright-wordlist-${LANG_CODE}-v${VERSION}.tsv"
PACK_PATH="$DIST/$PACK_NAME"
META_PATH="$DIST/draftright-wordlist-${LANG_CODE}-v${VERSION}.meta"

# Copy + dedupe + sort by descending frequency so the on-device loader
# can short-circuit prefix scans at the user's limit.
awk -F '\t' '
  !/^#/ && NF==2 && $1 != "" && $2 ~ /^[0-9]+$/ { if (!seen[$1]++) print $1 "\t" $2 }
' "$SOURCE" | sort -t $'\t' -k2,2nr > "$PACK_PATH"

# Optional bigram sidecar.
BIGRAM_SOURCE="${SOURCE%.tsv}.bigrams.tsv"
if [ -f "$BIGRAM_SOURCE" ]; then
  BIGRAM_NAME="draftright-wordlist-${LANG_CODE}-v${VERSION}.bigrams.tsv"
  cp "$BIGRAM_SOURCE" "$DIST/$BIGRAM_NAME"
fi

SHA="$(shasum -a 256 "$PACK_PATH" | awk '{print $1}')"
SIZE="$(wc -c < "$PACK_PATH" | tr -d ' ')"

# Write a tiny machine-readable meta file. The release pipeline reads this
# to patch sha256 + sizeBytes in ime-packs.service.ts at deploy time.
{
  echo "sha256=$SHA"
  echo "sizeBytes=$SIZE"
  echo "file=$PACK_NAME"
} > "$META_PATH"

echo "==> Built $PACK_NAME"
echo "    sha256:    $SHA"
echo "    sizeBytes: $SIZE"

if [ "$PUBLISH" = "--publish" ]; then
  echo "==> Uploading to draftright:/var/www/draftright/ime-packs/"
  rsync -az --rsync-path="sudo rsync" "$PACK_PATH" draftright:/var/www/draftright/ime-packs/
  if [ -f "${PACK_PATH%.tsv}.bigrams.tsv" ]; then
    rsync -az --rsync-path="sudo rsync" "${PACK_PATH%.tsv}.bigrams.tsv" draftright:/var/www/draftright/ime-packs/
  fi
  echo "==> Done. Paste sha256 + sizeBytes into ime-packs.service.ts."
fi
