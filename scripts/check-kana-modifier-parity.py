#!/usr/bin/env python3
"""Golden-vector parity guard (RULE #1): the kana dakuten/small variant cycles
exist as two copies — Kotlin `KanaModifier.cycles` and Swift `KanaModifier.cycles`.
They must agree or the 小゛゜ key produces different kana per platform. Extracts
every variant list from both and asserts the sets are identical.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/KanaModifier.kt"
SW = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/KanaModifier.swift"

# Kotlin: listOf("あ", "ぁ")   Swift: ["あ", "ぁ"]
LIST = re.compile(r'(?:listOf\(|\[)((?:\s*"[^"]+"\s*,?)+)\s*(?:\)|\])')
ITEM = re.compile(r'"([^"]+)"')


def parse(path):
    # limit to the cycles block so we don't catch unrelated literals
    text = path.read_text(encoding="utf-8")
    m = re.search(r"cycles[^=]*=\s*(?:listOf\(|\[)(.*?)\n\s*(?:\)|\])\s*\n", text, re.S)
    body = m.group(1) if m else text
    lists = set()
    for lm in LIST.finditer(body):
        lists.add(tuple(ITEM.findall(lm.group(1))))
    return lists


def main():
    if not KT.exists() or not SW.exists():
        sys.exit(f"ERROR: missing input\n  kt: {KT} ({KT.exists()})\n  sw: {SW} ({SW.exists()})")
    kt, sw = parse(KT), parse(SW)
    if kt == sw and kt:
        print(f"✓ kana modifier parity OK — {len(kt)} variant cycles agree")
        return 0
    print("✗ kana modifier parity FAILED:")
    print(f"  only in Kotlin: {sorted(kt - sw)}")
    print(f"  only in Swift:  {sorted(sw - kt)}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
