#!/usr/bin/env python3
"""Regression guard for #27 (iOS bootstrap crash).

`path_provider_foundation` 2.5.0/2.6.0 switched its iOS implementation to the
Dart-FFI `objective_c` package, whose `objective_c.framework` fails to load at
bootstrap → the app crashes before it starts (`DOBJC_initializeApi` /
"Failed to load dynamic library 'objective_c.framework'"). The fix is a
`dependency_overrides` pin to 2.4.4 (Swift-only, no objective_c).

A future `flutter pub upgrade` could silently re-resolve to >=2.5.0 and drag
`objective_c` back in, reintroducing the crash. This asserts the pin holds:
  1. `objective_c` is NOT in the resolved dependency graph.
  2. `path_provider_foundation` stays below the objective_c line (2.5.0).
Run on every mobile change (mobile-parity-ci.yml). Reads the single source of
truth — the resolved pubspec.lock — not the pubspec pin, so an override that
fails to take effect is still caught.
"""
import re
import sys
import pathlib

LOCK = pathlib.Path(__file__).resolve().parents[1] / "DraftRightMobile/pubspec.lock"
FIRST_OBJC_VERSION = (2, 5, 0)  # path_provider_foundation that pulls in objective_c
PKG = "path_provider_foundation"


def resolved_version(text, pkg):
    # a package block: "  <pkg>:\n    ...\n    version: \"x.y.z\""
    m = re.search(rf'^  {re.escape(pkg)}:\n(?:    .*\n)*?    version: "([^"]+)"', text, re.M)
    return m.group(1) if m else None


def as_tuple(v):
    return tuple(int(x) for x in re.findall(r"\d+", v)[:3])


def main():
    if not LOCK.exists():
        sys.exit(f"ERROR: missing {LOCK}")
    text = LOCK.read_text(encoding="utf-8")

    problems = []
    if re.search(r"^  objective_c:", text, re.M):
        problems.append("`objective_c` is back in pubspec.lock — the FFI framework "
                        "that crashes iOS bootstrap (#27). Keep path_provider_foundation < 2.5.0.")

    ver = resolved_version(text, PKG)
    if ver is None:
        problems.append(f"could not find {PKG} version in pubspec.lock")
    elif as_tuple(ver) >= FIRST_OBJC_VERSION:
        problems.append(f"{PKG} resolved to {ver} (>= 2.5.0) — pulls in objective_c "
                        f"and reintroduces the #27 bootstrap crash. Hold the 2.4.4 override.")

    if problems:
        print("✗ #27 regression guard FAILED:")
        for p in problems:
            print(f"  - {p}")
        return 1
    print(f"✓ #27 guard OK — {PKG} {ver}, no objective_c")
    return 0


if __name__ == "__main__":
    sys.exit(main())
