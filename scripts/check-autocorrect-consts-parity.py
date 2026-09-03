#!/usr/bin/env python3
"""Parity guard (RULE #1): the auto-correct thresholds (#207) exist as two
copies — Kotlin `AutoCorrector` and Swift `AutoCorrector` — because the two
clients can't share source. Tuning one without the other would make the same
typo correct on Android and stand on iOS. This asserts the values agree.

The shared behaviour cases live in parity/autocorrect-vectors.json, which both
test suites run; this guard covers the numbers those cases don't pin.

Fails (exit 1) naming each divergent constant.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]
KOTLIN = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ime/AutoCorrector.kt"
SWIFT = ROOT / "DraftRightMobile/ios/DraftRightKeyboardCore/Sources/DraftRightKeyboardCore/IME/AutoCorrector.swift"

# Logical name -> (Kotlin identifier, Swift identifier)
CONSTANTS = {
    "max edits": ("MAX_EDITS", "maxEdits"),
    "min confidence freq": ("MIN_CONFIDENCE_FREQ", "minConfidenceFreq"),
    "min confidence margin": ("MIN_CONFIDENCE_MARGIN", "minConfidenceMargin"),
}


def read_const(path, identifier):
    """Value of `... <identifier> ... = <int>` in a Kotlin/Swift source file."""
    m = re.search(rf"\b{re.escape(identifier)}\b\s*(?::\s*Int\s*)?=\s*(-?\d+)",
                  path.read_text(encoding="utf-8"))
    if not m:
        sys.exit(f"ERROR: could not find constant `{identifier}` in {path.name}")
    return int(m.group(1))


def main():
    for path in (KOTLIN, SWIFT):
        if not path.exists():
            sys.exit(f"ERROR: missing input: {path}")
    diffs = []
    for name, (kt_id, sw_id) in CONSTANTS.items():
        kt, sw = read_const(KOTLIN, kt_id), read_const(SWIFT, sw_id)
        if kt != sw:
            diffs.append(f"  [{name}]  kotlin {kt_id}={kt}  swift {sw_id}={sw}")
    if not diffs:
        print(f"✓ auto-correct consts parity OK — {len(CONSTANTS)} thresholds agree")
        return 0
    print("✗ auto-correct consts parity FAILED — Kotlin and Swift disagree:")
    print("\n".join(diffs))
    return 1


if __name__ == "__main__":
    sys.exit(main())
