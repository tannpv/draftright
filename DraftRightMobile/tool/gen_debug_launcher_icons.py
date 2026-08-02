#!/usr/bin/env python3
"""Generate the debug-variant launcher icons from the release icons.

The debug build (applicationIdSuffix ".debug") installs alongside the release,
so it needs a visually distinct icon. This overlays a red "DEBUG" band on both
the legacy mipmap icons and the adaptive-icon foreground, writing the results
into android/app/src/debug/res/ where Gradle merges them over src/main for the
debug variant.

Reproducible + reusable (Rule #1): re-run whenever the base icon changes.

    python3 tool/gen_debug_launcher_icons.py
"""
from __future__ import annotations

import os

from PIL import Image, ImageDraw, ImageFont

# --- Tunables (no scattered literals) ---------------------------------------
DENSITIES = ["mdpi", "hdpi", "xhdpi", "xxhdpi", "xxxhdpi"]
BADGE_TEXT = "DEBUG"
BADGE_FILL = (211, 47, 47, 235)      # material red 700, slightly translucent
TEXT_FILL = (255, 255, 255, 255)     # white
# Band geometry as fractions of the icon's shorter edge / height, chosen to sit
# inside the adaptive-icon safe zone (central ~66%) so the mask never clips it.
BAND_CENTER_Y = 0.72                 # vertical center of the band (0=top,1=bottom)
BAND_HEIGHT = 0.20                   # band height
BAND_WIDTH = 0.64                    # band width
BAND_RADIUS = 0.06                   # corner radius
TEXT_HEIGHT = 0.12                   # target glyph height

RES = os.path.join(os.path.dirname(__file__), "..", "android", "app", "src")
MAIN = os.path.join(RES, "main", "res")
DEBUG = os.path.join(RES, "debug", "res")

# Which source assets to badge: (subdir, filename).
TARGETS = [
    ("mipmap-{d}", "ic_launcher.png"),
    ("drawable-{d}", "ic_launcher_foreground.png"),
]


def _font(px: int) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    for path in (
        "/System/Library/Fonts/Supplemental/Arial Bold.ttf",
        "/System/Library/Fonts/Helvetica.ttc",
        "/Library/Fonts/Arial.ttf",
    ):
        if os.path.exists(path):
            try:
                return ImageFont.truetype(path, px)
            except OSError:
                pass
    return ImageFont.load_default()


def badge(src_path: str, dst_path: str) -> None:
    img = Image.open(src_path).convert("RGBA")
    w, h = img.size
    draw = ImageDraw.Draw(img)

    band_w, band_h = int(w * BAND_WIDTH), int(h * BAND_HEIGHT)
    cx, cy = w // 2, int(h * BAND_CENTER_Y)
    x0, y0 = cx - band_w // 2, cy - band_h // 2
    x1, y1 = cx + band_w // 2, cy + band_h // 2
    draw.rounded_rectangle([x0, y0, x1, y1], radius=int(h * BAND_RADIUS), fill=BADGE_FILL)

    font = _font(int(h * TEXT_HEIGHT))
    tb = draw.textbbox((0, 0), BADGE_TEXT, font=font)
    tw, th = tb[2] - tb[0], tb[3] - tb[1]
    draw.text((cx - tw / 2 - tb[0], cy - th / 2 - tb[1]), BADGE_TEXT, font=font, fill=TEXT_FILL)

    os.makedirs(os.path.dirname(dst_path), exist_ok=True)
    img.save(dst_path)
    print(f"  {os.path.relpath(dst_path, RES)}")


def main() -> None:
    for subdir, name in TARGETS:
        for d in DENSITIES:
            src = os.path.join(MAIN, subdir.format(d=d), name)
            if not os.path.exists(src):
                print(f"  skip (missing): {subdir.format(d=d)}/{name}")
                continue
            badge(src, os.path.join(DEBUG, subdir.format(d=d), name))
    print("done — debug launcher icons written to src/debug/res")


if __name__ == "__main__":
    main()
