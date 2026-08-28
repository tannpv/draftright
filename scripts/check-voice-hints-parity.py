#!/usr/bin/env python3
"""Parity guard (RULE #1) for the voice-outcome hint strings (#65).

When a dictation is salvaged (polish failed, or the recognizer errored
mid-sentence), the keyboard commits the raw words with a user-facing hint. The
two hint strings are duplicated: Kotlin `VoiceSessionController` and Swift
`VoiceSessionController`. If they drift, iOS and Android show different text for
the same event. Assert both agree.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/VoiceSessionController.kt"
SW = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/Voice/VoiceSessionController.swift"

# logical name -> (Kotlin const, Swift const)
HINTS = {
    "raw_fallback": ("RAW_FALLBACK_HINT", "rawFallbackHint"),
    "partial_salvage": ("PARTIAL_SALVAGE_HINT", "partialSalvageHint"),
}


def value(text, name):
    m = re.search(rf'{name}\s*=\s*"([^"]*)"', text)
    return m.group(1) if m else None


def main():
    if not KT.exists() or not SW.exists():
        sys.exit(f"ERROR: missing input\n  kt: {KT} ({KT.exists()})\n  sw: {SW} ({SW.exists()})")
    kt_text, sw_text = KT.read_text(encoding="utf-8"), SW.read_text(encoding="utf-8")
    problems = []
    for key, (kt_name, sw_name) in HINTS.items():
        kt_v, sw_v = value(kt_text, kt_name), value(sw_text, sw_name)
        if kt_v is None or sw_v is None:
            problems.append(f"{key}: could not parse (kt={kt_v!r}, sw={sw_v!r})")
        elif kt_v != sw_v:
            problems.append(f"{key} differs:\n    kt: {kt_v!r}\n    sw: {sw_v!r}")
    if problems:
        print("✗ voice hints parity FAILED:")
        for p in problems:
            print(f"  - {p}")
        return 1
    print(f"✓ voice hints parity OK — {len(HINTS)} hints agree")
    return 0


if __name__ == "__main__":
    sys.exit(main())
