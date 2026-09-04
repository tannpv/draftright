#!/usr/bin/env python3
"""Parity guard (RULE #1): the pinned Flutter version exists in several CI
surfaces that can't share config — GitHub workflow yml, the mirror's deploy
workflow, and Xcode Cloud's shell hook. A bump applied to one but not the
others builds different engines per pipeline. This asserts every pin agrees.

Fails (exit 1) naming each divergent file.
"""
import re
import sys
import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]

# File -> regex capturing the version. Add every new pin location here.
PINS = {
    ".github/workflows/mobile-test-ci.yml": r"flutter-version:\s*([\d.]+)",
    "DraftRightMobile/.github/workflows/play-deploy.yml": r"flutter-version:\s*([\d.]+)",
    "DraftRightMobile/ios/ci_scripts/ci_post_clone.sh": r"FLUTTER_VERSION=([\d.]+)",
}


def main():
    versions = {}
    for rel, pattern in PINS.items():
        path = ROOT / rel
        if not path.exists():
            sys.exit(f"ERROR: missing input: {path}")
        m = re.search(pattern, path.read_text(encoding="utf-8"))
        if not m:
            sys.exit(f"ERROR: no Flutter version pin found in {rel}")
        versions[rel] = m.group(1)
    unique = set(versions.values())
    if len(unique) == 1:
        print(f"✓ Flutter version parity OK — {len(PINS)} pins agree on {unique.pop()}")
        return 0
    print("✗ Flutter version parity FAILED — pins disagree:")
    for rel, v in versions.items():
        print(f"  {v}  {rel}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
