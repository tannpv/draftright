#!/usr/bin/env python3
"""Golden-vector parity guard (RULE #1): the pinyin syllable set exists as two
copies — Kotlin `PinyinSyllables.ALL` and Swift `PinyinSyllables.all` — because
Kotlin and Swift can't share source. Segmentation results diverge between
platforms if they disagree. This asserts the two sets are identical.

Fails (exit 1) with the symmetric difference. Run in CI on both files.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KOTLIN = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/PinyinSyllables.kt"
SWIFT = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/PinyinSyllables.swift"


def parse(path, open_token, close_char):
    text = path.read_text(encoding="utf-8")
    i = text.index(open_token) + len(open_token)
    depth = 1
    # scan to the matching close for the set literal
    body = []
    while i < len(text) and depth > 0:
        ch = text[i]
        if ch in "([":
            depth += 1
        elif ch in ")]":
            depth -= 1
            if depth == 0:
                break
        body.append(ch)
        i += 1
    # syllables are the only bare lowercase-letter quoted tokens in the block
    return set(re.findall(r'"([a-z]+)"', "".join(body)))


def main():
    if not KOTLIN.exists() or not SWIFT.exists():
        sys.exit(f"ERROR: missing input\n  kotlin: {KOTLIN} ({KOTLIN.exists()})\n  swift: {SWIFT} ({SWIFT.exists()})")
    kt = parse(KOTLIN, "setOf(", ")")
    sw = parse(SWIFT, "all: Set<String> = [", "]")
    if kt == sw:
        print(f"✓ pinyin syllable parity OK — {len(kt)} syllables agree")
        return 0
    print("✗ pinyin syllable parity FAILED:")
    only_kt = sorted(kt - sw)
    only_sw = sorted(sw - kt)
    if only_kt:
        print(f"  only in Kotlin ({len(only_kt)}): {only_kt}")
    if only_sw:
        print(f"  only in Swift ({len(only_sw)}): {only_sw}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
