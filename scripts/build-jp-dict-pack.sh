#!/usr/bin/env bash
# Build the Japanese reading→kanji dictionary pack from the Mozc open-source
# dictionary (BSD-licensed) and optionally publish to the pack CDN.
#
# Usage:
#   build-jp-dict-pack.sh <version> [--publish]
#
#   <version>   Integer — bumped each release (matches manifest version).
#   --publish   rsync to draftright.info/ime-packs/ and print sha256+size to
#               paste into ImePacksService.
#
# Output (always written to dist/):
#   dist/draftright-ime-ja-v<version>.pack    — reading<TAB>kanji1,kanji2,...
#   dist/draftright-ime-ja-v<version>.meta    — sha256<newline>sizeBytes
#
# Format: one UTF-8 line per reading:
#   reading<TAB>candidate1,candidate2,...
# Candidates are ordered by descending frequency (best conversion first).
# Comment lines (#) and blanks are skipped by JapaneseDictLoader on both
# iOS and Android — no code changes needed for format evolution.
#
# Source: Mozc project dictionary (BSD-3-Clause)
#   https://github.com/google/mozc/blob/master/src/data/dictionary_oss/dictionary00.txt
# The dictionary files are text tables with fields:
#   reading<TAB>left_id<TAB>right_id<TAB>cost<TAB>surface<TAB>...
# We extract (reading, surface, cost) and rank by ascending cost (lower=better).
# No GPL material; SKK-JISYO is explicitly excluded.

set -euo pipefail

VERSION="${1:?usage: build-jp-dict-pack.sh <version> [--publish]}"
PUBLISH="${2:-}"

cd "$(dirname "$0")/.."
DIST="dist"
mkdir -p "$DIST"

PACK_NAME="draftright-ime-ja-v${VERSION}.pack"
PACK_PATH="$DIST/$PACK_NAME"
META_PATH="$DIST/draftright-ime-ja-v${VERSION}.meta"

MOZC_BASE="https://raw.githubusercontent.com/google/mozc/master/src/data/dictionary_oss"
# Mozc dictionary is split into multiple shards — fetch the first 4 for a
# compact but useful pack (~60-80k entries after dedup).
SHARDS=(dictionary00.txt dictionary01.txt dictionary02.txt dictionary03.txt)

TMP_RAW=$(mktemp)
TMP_SORTED=$(mktemp)
cleanup() { rm -f "$TMP_RAW" "$TMP_SORTED"; }
trap cleanup EXIT

echo "[1/4] Downloading Mozc dictionary shards..."
for shard in "${SHARDS[@]}"; do
  echo "  fetching $shard"
  curl -fsSL "$MOZC_BASE/$shard" >> "$TMP_RAW"
done

echo "[2/4] Extracting reading→kanji pairs (reading col 0, surface col 4, cost col 3)..."
# Mozc columns: reading  left_id  right_id  cost  surface  ...
# Filter out:
#   - entries where surface == reading (kana-only → no value as kanji)
#   - entries where surface contains ASCII/numbers (romanized / numeric)
#   - special tokens starting with '#'
python3 - "$TMP_RAW" "$TMP_SORTED" << 'PYEOF'
import sys, re

src = sys.argv[1]
dst = sys.argv[2]
pairs = []   # (reading, surface, cost)
not_kanji = re.compile(r'[a-zA-Z0-9\x00-\x1F\x7F]')

with open(src, encoding='utf-8') as f:
    for line in f:
        line = line.rstrip('\n')
        if not line or line.startswith('#'):
            continue
        parts = line.split('\t')
        if len(parts) < 5:
            continue
        reading, cost_s, surface = parts[0], parts[3], parts[4]
        if surface == reading:      # pure kana — skip
            continue
        if not_kanji.search(surface):  # ASCII/numeric in surface — skip
            continue
        try:
            cost = int(cost_s)
        except ValueError:
            continue
        pairs.append((reading, surface, cost))

# Sort by reading then ascending cost (lower cost = better conversion).
pairs.sort(key=lambda x: (x[0], x[2]))

# Dedupe: for each reading keep candidates in cost order, no duplicates.
from collections import defaultdict
grouped = defaultdict(list)
seen = defaultdict(set)
for reading, surface, cost in pairs:
    if surface not in seen[reading]:
        seen[reading].add(surface)
        grouped[reading].append(surface)

with open(dst, 'w', encoding='utf-8') as out:
    for reading in sorted(grouped.keys()):
        candidates = ','.join(grouped[reading][:8])   # cap at 8 per reading
        out.write(f'{reading}\t{candidates}\n')

print(f'  {len(grouped)} unique readings → {sum(len(v) for v in grouped.values())} candidates')
PYEOF

echo "[3/4] Writing pack to $PACK_PATH..."
cp "$TMP_SORTED" "$PACK_PATH"
LINES=$(wc -l < "$PACK_PATH" | tr -d ' ')
SIZE=$(wc -c < "$PACK_PATH" | tr -d ' ')
SHA=$(sha256sum "$PACK_PATH" | awk '{print $1}')
printf '%s\n%s\n' "$SHA" "$SIZE" > "$META_PATH"

echo ""
echo "✅  Pack: $PACK_PATH"
echo "    Lines  : $LINES readings"
echo "    Size   : $SIZE bytes ($(echo "$SIZE / 1024" | bc) KB)"
echo "    SHA256 : $SHA"
echo ""
echo "Paste into ImePacksService (backend/src/ime-packs/ime-packs.service.ts):"
echo "  sizeBytes: $SIZE, sha256: '$SHA'"

if [ "$PUBLISH" = "--publish" ]; then
  echo "[4/4] Publishing to draftright.info/ime-packs/ ..."
  SSH_HOST="${DEPLOY_HOST:-draftright}"
  REMOTE_DIR="/var/www/draftright/ime-packs"
  ssh "$SSH_HOST" "mkdir -p $REMOTE_DIR"
  rsync -avz "$PACK_PATH" "$META_PATH" "$SSH_HOST:$REMOTE_DIR/"
  echo "✅  Published: https://draftright.info/ime-packs/$PACK_NAME"
fi
