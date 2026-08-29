#!/usr/bin/env python3
"""Parity guard (RULE #1): the hiragana→katakana transliteration exists as two
copies — Kotlin `Katakana` and Swift `Katakana`. The mapping is a fixed Unicode
shift defined by three constants (block start, block end, katakana offset); if
the two copies disagree on any of them the katakana candidate differs per
platform. Extract the constants from both and assert they match.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/Katakana.kt"
SW = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/Katakana.swift"

# name (as it appears in each language) -> the shared meaning
FIELDS = {
    "start": ("HIRA_START", "hiraStart"),
    "end": ("HIRA_END", "hiraEnd"),
    "offset": ("TO_KATAKANA", "toKatakana"),
}


def grab(text, name):
    m = re.search(rf"{name}\b[^=]*=\s*(0x[0-9A-Fa-f]+|\d+)", text)
    return int(m.group(1), 0) if m else None


def parse(path, idx):
    text = path.read_text(encoding="utf-8")
    return {key: grab(text, names[idx]) for key, names in FIELDS.items()}


def main():
    if not KT.exists() or not SW.exists():
        sys.exit(f"ERROR: missing input\n  kt: {KT} ({KT.exists()})\n  sw: {SW} ({SW.exists()})")
    kt, sw = parse(KT, 0), parse(SW, 1)
    if None in kt.values() or None in sw.values():
        print(f"✗ katakana parity FAILED — could not parse constants\n  kt={kt}\n  sw={sw}")
        return 1
    if kt == sw:
        print(f"✓ katakana parity OK — block {hex(kt['start'])}..{hex(kt['end'])} +{hex(kt['offset'])} agree")
        return 0
    print(f"✗ katakana parity FAILED:\n  kt={kt}\n  sw={sw}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
