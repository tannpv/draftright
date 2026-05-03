#!/usr/bin/env python3
"""Generate DraftRightWindows/DraftRightWindows/Assets/DraftRight.ico from
store-assets/google-play/icon-512.png.

PIL's default ICO writer embeds PNGs inside the .ico container. That format
is fine for modern apps but the Windows system tray's NotifyIcon path on
some builds (especially with WinUI 3 / .NET 8 self-contained) renders such
icons as blank. The classic Win32 tray API expects BMP-encoded sub-icons.

This script forces the output to multi-size BMP-encoded sub-icons inside
a single .ico file: 16, 24, 32, 40, 48, 64, 96, 128, 256.

Re-run: python3 scripts/gen-windows-ico.py
"""

from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "store-assets" / "google-play" / "icon-512.png"
DST = ROOT / "DraftRightWindows" / "DraftRightWindows" / "Assets" / "DraftRight.ico"

img = Image.open(SRC).convert("RGBA")

# Sizes the tray picks from. Windows tray usually uses 16 or 32.
sizes = [16, 24, 32, 40, 48, 64, 96, 128, 256]

DST.parent.mkdir(parents=True, exist_ok=True)
img.save(DST, format="ICO", sizes=[(s, s) for s in sizes], bitmap_format="bmp")
print(f"Wrote {DST} ({DST.stat().st_size:,} bytes)")
