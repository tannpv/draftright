#!/usr/bin/env bash
# Build the Chinese pinyin→hanzi dictionary pack from permissive open data and
# optionally publish to the pack CDN. Mirrors build-jp-dict-pack.sh 1:1 — same
# TSV format, same loader (DictPackLoader), same engine (DictionaryCandidateEngine).
#
# Usage:
#   build-zh-dict-pack.sh <version> [--publish]
#
#   <version>   Integer — bumped each release (matches manifest version).
#   --publish   rsync to draftright.info/ime-packs/ and print sha256+size to
#               paste into ImePacksService.
#
# Output (always written to dist/):
#   dist/draftright-ime-zh-v<version>.pack    — reading<TAB>word1,word2,...
#   dist/draftright-ime-zh-v<version>.meta    — sha256<newline>sizeBytes
#
# Format: one UTF-8 line per toneless pinyin reading:
#   reading<TAB>candidate1,candidate2,...
# Candidates ordered by descending frequency (best conversion first).
# Comment lines (#) and blanks are skipped by DictPackLoader on iOS + Android.
#
# Sources (all permissive — no GPL, matching the JP pack's policy):
#   - phrase-pinyin-data  (MIT)  word→toned pinyin for ~common phrases
#       https://github.com/mozillazg/phrase-pinyin-data
#   - pinyin-data         (MIT)  char→pinyin (derived from Unicode Unihan)
#       https://github.com/mozillazg/pinyin-data
#   - FrequencyWords zh_cn (MIT) word→count (OpenSubtitles) — ranks candidates
#       https://github.com/hermitdave/FrequencyWords
# Tones are stripped (input is toneless full syllables); ü→v (standard IME).

set -euo pipefail

VERSION="${1:?usage: build-zh-dict-pack.sh <version> [--publish]}"
PUBLISH="${2:-}"

cd "$(dirname "$0")/.."
DIST="dist"
mkdir -p "$DIST"

PACK_NAME="draftright-ime-zh-v${VERSION}.pack"
PACK_PATH="$DIST/$PACK_NAME"
META_PATH="$DIST/draftright-ime-zh-v${VERSION}.meta"

PHRASE_URL="https://raw.githubusercontent.com/mozillazg/phrase-pinyin-data/master/large_pinyin.txt"
CHAR_URL="https://raw.githubusercontent.com/mozillazg/pinyin-data/master/pinyin.txt"
FREQ_URL="https://raw.githubusercontent.com/hermitdave/FrequencyWords/master/content/2018/zh_cn/zh_cn_50k.txt"

TMP_PHRASE=$(mktemp)
TMP_CHAR=$(mktemp)
TMP_FREQ=$(mktemp)
cleanup() { rm -f "$TMP_PHRASE" "$TMP_CHAR" "$TMP_FREQ"; }
trap cleanup EXIT

echo "[1/4] Downloading sources..."
echo "  phrase-pinyin-data"; curl -fsSL "$PHRASE_URL" > "$TMP_PHRASE"
echo "  pinyin-data";        curl -fsSL "$CHAR_URL"   > "$TMP_CHAR"
echo "  FrequencyWords zh";  curl -fsSL "$FREQ_URL"   > "$TMP_FREQ"

echo "[2/4] Building reading→word map, ranking by frequency..."
python3 - "$TMP_PHRASE" "$TMP_CHAR" "$TMP_FREQ" "$PACK_PATH" << 'PYEOF'
import sys, re, unicodedata
from collections import defaultdict

phrase_path, char_path, freq_path, dst = sys.argv[1:5]

# --- tone strip: any accented latin vowel → base; ü/ǖ.. → v (IME convention) ---
UMLAUT_U = {'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ', 'Ü'}
def toneless(syllable: str) -> str:
    out = []
    for ch in syllable:
        if ch in UMLAUT_U:
            out.append('v'); continue
        # decompose accent (ā → a + combining macron), drop combining marks
        base = ''.join(c for c in unicodedata.normalize('NFD', ch)
                       if not unicodedata.combining(c))
        out.append(base.lower())
    s = ''.join(out)
    # keep only a-z and v (already lowercase); drop stray punctuation
    return re.sub(r'[^a-z]', '', s)

# --- frequency: word → count (ranks candidates within a reading) ---
freq = {}
with open(freq_path, encoding='utf-8') as f:
    for line in f:
        parts = line.split()
        if len(parts) == 2 and parts[1].isdigit():
            freq[parts[0]] = int(parts[1])

reading_words = defaultdict(set)   # reading → {word, ...}

# --- phrases: "你好: nǐ hǎo" → reading "nihao" → 你好 ---
with open(phrase_path, encoding='utf-8') as f:
    for line in f:
        line = line.rstrip('\n')
        if not line or line.startswith('#') or ':' not in line:
            continue
        word, pinyin = line.split(':', 1)
        word = word.strip()
        reading = ''.join(toneless(s) for s in pinyin.split())
        if word and reading:
            reading_words[reading].add(word)

# --- single chars: "U+4E00: yī,yì  # 一" → 一 under "yi" (all readings) ---
char_re = re.compile(r'^U\+[0-9A-Fa-f]+:\s*([^#]+)#\s*(\S+)')
with open(char_path, encoding='utf-8') as f:
    for line in f:
        m = char_re.match(line)
        if not m:
            continue
        pinyins, char = m.group(1).strip(), m.group(2).strip()
        if len(char) != 1:
            continue
        for syl in pinyins.split(','):
            reading = toneless(syl)
            if reading:
                reading_words[reading].add(char)

# Rank candidates within each reading by descending frequency; unseen → 0.
# Rank readings by their most-frequent candidate so the cap keeps common words.
def wfreq(w): return freq.get(w, 0)
ranked = {}
for reading, words in reading_words.items():
    ordered = sorted(words, key=lambda w: (-wfreq(w), len(w), w))
    ranked[reading] = (ordered, wfreq(ordered[0]))

# Cap to the most common readings — a keyboard needs frequent words, not the
# full 400k+ long tail; keeps the pack small so it loads fast on language-switch.
MAX_READINGS = 60000
kept = sorted(ranked.keys(), key=lambda r: -ranked[r][1])[:MAX_READINGS]

with open(dst, 'w', encoding='utf-8') as out:
    for reading in sorted(kept):
        cands = ','.join(ranked[reading][0][:9])   # cap 9 candidates per reading
        out.write(f'{reading}\t{cands}\n')

print(f'  {len(reading_words)} readings → kept top {len(kept)} by frequency')
PYEOF

echo "[3/4] Writing meta for $PACK_PATH..."
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
