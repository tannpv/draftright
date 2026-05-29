#!/usr/bin/env python3
"""Generate MSIX visual assets for DraftRightWindows from the canonical
512x512 store icon. Microsoft Store / MSIX requires multiple square + wide
sizes at scale-100/-200/-400 variants.

Outputs:
  DraftRightWindows/DraftRightWindows/Assets/Square44x44Logo.scale-{100,200,400}.png
  DraftRightWindows/DraftRightWindows/Assets/Square71x71Logo.scale-{100,200,400}.png
  DraftRightWindows/DraftRightWindows/Assets/Square150x150Logo.scale-{100,200,400}.png
  DraftRightWindows/DraftRightWindows/Assets/Square310x310Logo.scale-{100,200,400}.png
  DraftRightWindows/DraftRightWindows/Assets/Wide310x150Logo.scale-{100,200,400}.png
  DraftRightWindows/DraftRightWindows/Assets/StoreLogo.scale-{100,200,400}.png
  DraftRightWindows/DraftRightWindows/Assets/SplashScreen.scale-{100,200,400}.png

Re-run: python3 scripts/gen-msix-assets.py
"""

from pathlib import Path
from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / "store-assets" / "google-play" / "icon-512.png"
OUT = ROOT / "DraftRightWindows" / "DraftRightWindows" / "Assets"

# DraftRight brand blue (matches the icon background)
BRAND = (30, 64, 175)

icon = Image.open(SRC).convert("RGBA")

# Square logos: simple resize, transparent or branded background
SQUARE_LOGOS = [
    ("Square44x44Logo", 44),
    ("Square71x71Logo", 71),
    ("Square150x150Logo", 150),
    ("Square310x310Logo", 310),
    ("StoreLogo", 50),
]

# scale-100 = base size, -200 = 2x, -400 = 4x
SCALES = [(100, 1.0), (200, 2.0), (400, 4.0)]

for base_name, base_size in SQUARE_LOGOS:
    for scale_label, scale_factor in SCALES:
        size = int(base_size * scale_factor)
        out = icon.resize((size, size), Image.LANCZOS)
        path = OUT / f"{base_name}.scale-{scale_label}.png"
        out.save(path, "PNG", optimize=True)

# Wide tile 310x150: brand background + centered icon
for scale_label, scale_factor in SCALES:
    w = int(310 * scale_factor)
    h = int(150 * scale_factor)
    canvas = Image.new("RGBA", (w, h), BRAND)
    icon_size = int(min(w, h) * 0.7)
    icon_resized = icon.resize((icon_size, icon_size), Image.LANCZOS)
    cx = (w - icon_size) // 2
    cy = (h - icon_size) // 2
    canvas.paste(icon_resized, (cx, cy), icon_resized)
    path = OUT / f"Wide310x150Logo.scale-{scale_label}.png"
    canvas.save(path, "PNG", optimize=True)

# Splash screen 620x300: same idea, larger canvas
for scale_label, scale_factor in SCALES:
    w = int(620 * scale_factor)
    h = int(300 * scale_factor)
    canvas = Image.new("RGBA", (w, h), BRAND)
    icon_size = int(min(w, h) * 0.7)
    icon_resized = icon.resize((icon_size, icon_size), Image.LANCZOS)
    cx = (w - icon_size) // 2
    cy = (h - icon_size) // 2
    canvas.paste(icon_resized, (cx, cy), icon_resized)
    path = OUT / f"SplashScreen.scale-{scale_label}.png"
    canvas.save(path, "PNG", optimize=True)

# Targeted Square44x44Logo for taskbar (MSIX requires these specific sizes)
TARGET_SIZES = [16, 24, 32, 48, 256]
for ts in TARGET_SIZES:
    out = icon.resize((ts, ts), Image.LANCZOS)
    path = OUT / f"Square44x44Logo.targetsize-{ts}.png"
    out.save(path, "PNG", optimize=True)
    # Unplated variant (transparent background fallback)
    path_unplated = OUT / f"Square44x44Logo.targetsize-{ts}_altform-unplated.png"
    out.save(path_unplated, "PNG", optimize=True)

print(f"Wrote MSIX assets to {OUT}")
print(f"Total files: {len(list(OUT.glob('*.png')))}")
