#!/usr/bin/env python3
"""Parity guard (RULE #1): the ~8.5k VI unigram dictionary exists as two copies —
Android's TSV resource and the iOS generated Swift array — because Kotlin and
Swift can't share the source. Both are emitted by
`DraftRightMobile/tools/gen_vi_wordlist.py` from one file; this asserts nobody
hand-edited either copy so suggestions and auto-correct can't diverge between
platforms.

Fails (exit 1) naming the first divergent row. Run in CI on both files.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
TSV = ROOT / "DraftRightMobile/android/app/src/main/res/raw/wordlist_vi.tsv"
SWIFT = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/VietnameseWordList.swift"


def parse_tsv(path):
    rows = []
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split("\t")
        if len(parts) != 2:
            sys.exit(f"ERROR: malformed TSV row (want word<TAB>freq): {raw!r}")
        rows.append((parts[0].strip(), int(parts[1].strip())))
    return rows


def parse_swift(path):
    text = path.read_text(encoding="utf-8")
    m = re.search(r"static let entries[^=]*=\s*\[\n(.*?)\n    \]", text, re.S)
    if not m:
        sys.exit("ERROR: could not find the `entries` literal in the Swift file")
    return [(w, int(f)) for w, f in re.findall(r'\(\s*"([^"]+)"\s*,\s*(\d+)\s*\)', m.group(1))]


def main():
    if not TSV.exists() or not SWIFT.exists():
        sys.exit(f"ERROR: missing input\n  tsv:   {TSV} ({TSV.exists()})\n  swift: {SWIFT} ({SWIFT.exists()})")
    tsv, sw = parse_tsv(TSV), parse_swift(SWIFT)
    if tsv == sw:
        print(f"✓ VI wordlist parity OK — {len(tsv)} entries agree")
        return 0
    print(f"✗ VI wordlist parity FAILED — tsv={len(tsv)} rows, swift={len(sw)} rows")
    for i, (a, b) in enumerate(zip(tsv, sw)):
        if a != b:
            print(f"  first diff at row {i}: tsv={a}  swift={b}")
            break
    else:
        longer, name = (tsv, "tsv") if len(tsv) > len(sw) else (sw, "swift")
        print(f"  common rows agree; {name} has extra rows from {min(len(tsv), len(sw))}: {longer[min(len(tsv), len(sw)):][:3]}")
    print("  regenerate both: cd DraftRightMobile && python3 tools/gen_vi_wordlist.py")
    return 1


if __name__ == "__main__":
    sys.exit(main())
