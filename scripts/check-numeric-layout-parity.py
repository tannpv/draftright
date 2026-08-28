#!/usr/bin/env python3
"""Parity guard (RULE #1) for the OTP numeric keypad (#209/#208).

The numeric layout (1-9 grid + ABC/0/←/↵ row) is duplicated: Kotlin
`QwertyLayout.numericRows` and Swift `QwertyLayout.numericRows`. If the two
drift, iOS and Android show different keypads on numeric fields. The special-key
CODES are per-platform enums (SpecialKeys.ALPHA vs .alpha), so this compares the
visible LABEL sequence — the actual layout — which must be identical.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/lang/QwertyLayout.kt"
SW = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/Lang/QwertyLayout.swift"

OPENERS = {"(": ")", "[": "]"}


def numeric_block(text):
    """The numericRows initializer, extracted by bracket-depth from its first
    opener so we never bleed into the next declaration."""
    i = text.find("numericRows")
    if i < 0:
        return None
    # start at the '=' so a Swift type annotation ([[KeyDef]]) isn't mistaken
    # for the value's opener, then advance to the first real opener.
    i = text.find("=", i)
    if i < 0:
        return None
    while i < len(text) and text[i] not in "([":
        i += 1
    if i >= len(text):
        return None
    close = OPENERS[text[i]]
    depth, start = 0, i
    while i < len(text):
        c = text[i]
        if c in OPENERS:
            depth += 1
        elif c in (")", "]"):
            depth -= 1
            if depth == 0:
                return text[start:i + 1]
        i += 1
    return None


def labels(text):
    block = numeric_block(text)
    if block is None:
        return None
    # Drop keyCode("x") helper calls so the quoted arg isn't counted as a label
    # (Android uses the char literal '0'.code, which isn't double-quoted).
    block = re.sub(r'keyCode\([^)]*\)', "", block)
    return re.findall(r'"([^"]*)"', block)


def main():
    if not KT.exists() or not SW.exists():
        sys.exit(f"ERROR: missing input\n  kt: {KT} ({KT.exists()})\n  sw: {SW} ({SW.exists()})")
    kt = labels(KT.read_text(encoding="utf-8"))
    sw = labels(SW.read_text(encoding="utf-8"))
    if kt is None or sw is None:
        print(f"✗ numeric layout parity FAILED — could not extract block (kt={kt}, sw={sw})")
        return 1
    if kt == sw and kt:
        print(f"✓ numeric layout parity OK — {len(kt)} keys agree: {kt}")
        return 0
    print("✗ numeric layout parity FAILED:")
    print(f"  kt: {kt}")
    print(f"  sw: {sw}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
