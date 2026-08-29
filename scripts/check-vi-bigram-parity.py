#!/usr/bin/env python3
"""Golden-vector parity guard (RULE #1): the Vietnamese bigram data exists as two
copies — Android's TSV resource and the iOS Swift literal — because Kotlin and
Swift can't share the source. They MUST agree or next-word prediction diverges
between platforms. This asserts they are byte-for-byte equivalent as maps.

Fails (exit 1) with a readable diff on any divergence. Run in CI on both files.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
TSV = ROOT / "DraftRightMobile/android/app/src/main/res/raw/wordlist_vi_bigrams.tsv"
SWIFT = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/VietnameseBootstrapWordList.swift"


def parse_tsv(path):
    d = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("\t")
        if len(parts) != 3:
            continue
        prev, nxt, cnt = parts[0].strip(), parts[1].strip(), parts[2].strip()
        d.setdefault(prev, {})[nxt] = int(cnt)
    return d


def parse_swift(path):
    text = path.read_text(encoding="utf-8")
    m = re.search(r"static let bigrams[^=]*=\s*\[\n(.*?)\n    \]", text, re.S)
    if not m:
        sys.exit("ERROR: could not find the `bigrams` literal in the Swift file")
    body = m.group(1)
    d = {}
    for entry in re.finditer(r'"([^"]+)":\s*\[([^\]]*)\]', body):
        prev, inner = entry.group(1), entry.group(2)
        d[prev] = {p.group(1): int(p.group(2)) for p in re.finditer(r'"([^"]+)":\s*(\d+)', inner)}
    return d


def main():
    if not TSV.exists() or not SWIFT.exists():
        sys.exit(f"ERROR: missing input\n  tsv:   {TSV} ({TSV.exists()})\n  swift: {SWIFT} ({SWIFT.exists()})")
    tsv, sw = parse_tsv(TSV), parse_swift(SWIFT)
    if tsv == sw:
        print(f"✓ VI bigram parity OK — {len(tsv)} heads, {sum(len(v) for v in tsv.values())} pairs agree")
        return 0
    # readable diff
    print("✗ VI bigram parity FAILED — TSV and Swift literal disagree:")
    for prev in sorted(set(tsv) | set(sw)):
        a, b = tsv.get(prev), sw.get(prev)
        if a != b:
            print(f"  [{prev}]  tsv={a}  swift={b}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
