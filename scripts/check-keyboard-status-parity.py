#!/usr/bin/env python3
"""Parity guard (RULE #1) for the keyboard-status vocabulary (#272).

Android never auto-enables a newly installed input method, so after any fresh
install our keyboard exists but is dead. The app detects that and prompts, and
the status crosses a platform channel as a bare string: Kotlin decides it
(`KeyboardStatus` enum) and Dart maps the name back
(`KeyboardStatusService._statusByNativeName`).

Those are two copies of one vocabulary. If Kotlin gains or renames a case and
Dart is not updated, the name falls through to `unknown` and the banner
silently stops warning — the exact failure the banner exists to prevent, and
one no unit test on either side can catch alone.

Dart carries one extra case, `unknown`, which is deliberately Dart-only: it
covers iOS and older builds where the platform cannot answer at all. It is
excluded here rather than mirrored into Kotlin.
"""
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
KT = ROOT / "DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/KeyboardStatus.kt"
DART = ROOT / "DraftRightMobile/lib/services/keyboard_status_service.dart"

# Dart-only: "the platform cannot say", which Kotlin never needs to express.
DART_ONLY = {"unknown"}


def kotlin_cases(text):
    """Enum constant names, i.e. the strings `.name` puts on the channel."""
    body = re.search(r"enum class KeyboardStatus\s*\{(.*?)\n\s*companion object",
                     text, re.S)
    if not body:
        return set()
    return {m.group(1).lower()
            for m in re.finditer(r"^\s{4}([A-Z][A-Z_]*)\s*[,;]", body.group(1), re.M)}


def dart_cases(text):
    """Keys of the name->enum map, which is what Dart accepts off the channel."""
    body = re.search(r"_statusByNativeName\s*=\s*<String,\s*KeyboardStatus>\{(.*?)\};",
                     text, re.S)
    if not body:
        return set()
    return {m.group(1).lower() for m in re.finditer(r"'([A-Z_]+)'", body.group(1))}


def main():
    for path in (KT, DART):
        if not path.exists():
            sys.exit(f"ERROR: missing input: {path}")

    kt = kotlin_cases(KT.read_text(encoding="utf-8"))
    dart = dart_cases(DART.read_text(encoding="utf-8"))

    if not kt or not dart:
        sys.exit("ERROR: parsed zero cases — the guard itself is broken, not the code")

    if kt == dart:
        print(f"✓ keyboard-status parity OK — {len(kt)} statuses agree: {sorted(kt)}")
        print(f"  (Dart additionally carries {sorted(DART_ONLY)}, platform-can't-say)")
        return 0

    print("✗ keyboard-status vocabulary diverged", file=sys.stderr)
    for name, missing in (("Dart", kt - dart), ("Kotlin", dart - kt)):
        if missing:
            print(f"  missing in {name}: {sorted(missing)}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
