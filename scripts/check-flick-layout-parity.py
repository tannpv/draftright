#!/usr/bin/env python3
"""Golden-vector parity guard (RULE #1): the JP flick kana map exists as two
copies — Kotlin `FlickLayout` and Swift `FlickLayout`. They must agree or flick
input produces different kana per platform. Reconstructs the full
(row, direction, kana) set from each (expanding the gojuon() helper + the
explicit や/わ rows) and asserts they're identical.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/FlickLayout.kt"
SW = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/FlickLayout.swift"

GOJUON = re.compile(
    r'"([^"]+)"\s*(?:to|:)\s*gojuon\("([^"]+)",\s*"([^"]+)",\s*"([^"]+)",\s*"([^"]+)",\s*"([^"]+)"\)'
)
EXPLICIT_ROW = re.compile(r'"([^"]+)"\s*(?:to\s*mapOf\(|:\s*\[)(.*?)(?:\)|\])', re.S)
DIR_KANA = re.compile(r'(?:FlickDirection\.)?\.?(TAP|LEFT|UP|RIGHT|DOWN|tap|left|up|right|down)\s*(?:to|:)\s*"([^"]+)"')


def parse(path):
    text = path.read_text(encoding="utf-8")
    triples = set()
    for m in GOJUON.finditer(text):
        head, a, i, u, e, o = m.groups()
        for d, k in (("TAP", a), ("LEFT", i), ("UP", u), ("RIGHT", e), ("DOWN", o)):
            triples.add((head, d, k))
    for m in EXPLICIT_ROW.finditer(text):
        head, body = m.group(1), m.group(2)
        if "gojuon" in body:
            continue
        for dm in DIR_KANA.finditer(body):
            triples.add((head, dm.group(1).upper(), dm.group(2)))
    return triples


def main():
    if not KT.exists() or not SW.exists():
        sys.exit(f"ERROR: missing input\n  kt: {KT} ({KT.exists()})\n  sw: {SW} ({SW.exists()})")
    kt, sw = parse(KT), parse(SW)
    if kt == sw and kt:
        print(f"✓ flick layout parity OK — {len(kt)} (row,dir,kana) entries agree")
        return 0
    print("✗ flick layout parity FAILED:")
    if not kt or not sw:
        print(f"  parsed kt={len(kt)} sw={len(sw)} — parser found nothing, check the files")
    print(f"  only in Kotlin: {sorted(kt - sw)}")
    print(f"  only in Swift:  {sorted(sw - kt)}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
