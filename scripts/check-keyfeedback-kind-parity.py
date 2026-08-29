#!/usr/bin/env python3
"""Parity guard (RULE #1) for the key-feedback kinds (#209).

Every key press on both platforms is classified into a `KeyFeedbackKind`, the
single vocabulary the feedback chokepoint routes through. The Kotlin enum
(`KeyFeedback.kt`) and the Swift enum (`KeyFeedbackKind.swift`) are two copies;
if a kind is added on one platform but not the other, a key would fall through
without feedback there. Assert the case sets agree (case-insensitively —
Kotlin SCREAMING_CASE vs Swift lowerCamel).
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/KeyFeedback.kt"
SW = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/KeyFeedbackKind.swift"


def kotlin_cases(text):
    m = re.search(r"enum class KeyFeedbackKind\s*\{([^}]*)\}", text)
    if not m:
        return set()
    return {tok.strip().lower() for tok in m.group(1).split(",") if tok.strip()}


def swift_cases(text):
    # only the KeyFeedbackKind enum body, not KeyFeedbackImpact
    m = re.search(r"enum KeyFeedbackKind\s*\{(.*?)\n\}", text, re.S)
    body = m.group(1) if m else text
    return {c.lower() for c in re.findall(r"^\s*case\s+(\w+)", body, re.M)}


def main():
    if not KT.exists() or not SW.exists():
        sys.exit(f"ERROR: missing input\n  kt: {KT} ({KT.exists()})\n  sw: {SW} ({SW.exists()})")
    kt = kotlin_cases(KT.read_text(encoding="utf-8"))
    sw = swift_cases(SW.read_text(encoding="utf-8"))
    if kt == sw and kt:
        print(f"✓ key-feedback kind parity OK — {len(kt)} kinds agree: {sorted(kt)}")
        return 0
    print("✗ key-feedback kind parity FAILED:")
    print(f"  only in Kotlin: {sorted(kt - sw)}")
    print(f"  only in Swift:  {sorted(sw - kt)}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
