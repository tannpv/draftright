#!/usr/bin/env python3
"""Generate a 1024x500 Google Play feature graphic for DraftRight.

Layout: dark slate vertical gradient background, app icon on the left
with a soft drop shadow, app name + tagline on the right. Output:
store-assets/google-play/feature-graphic.png

Re-run any time to refresh: python3 scripts/gen-feature-graphic.py
"""

from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont

W, H = 1024, 500
ROOT = Path(__file__).resolve().parents[1]
ICON_SRC = ROOT / "store-assets" / "google-play" / "icon-512.png"
OUT = ROOT / "store-assets" / "google-play" / "feature-graphic.png"

# Vertical gradient: slate-900 -> slate-800 (Tailwind palette)
TOP = (15, 23, 42)
BOTTOM = (30, 41, 59)
out = Image.new("RGB", (W, H), TOP)
draw = ImageDraw.Draw(out)
for y in range(H):
    t = y / H
    r = int(TOP[0] + (BOTTOM[0] - TOP[0]) * t)
    g = int(TOP[1] + (BOTTOM[1] - TOP[1]) * t)
    b = int(TOP[2] + (BOTTOM[2] - TOP[2]) * t)
    draw.line([(0, y), (W, y)], fill=(r, g, b))

# Subtle accent — soft circular glow on the right (cyan-300, blurred)
glow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
gd = ImageDraw.Draw(glow)
gd.ellipse([W - 360, H // 2 - 240, W + 120, H // 2 + 240], fill=(103, 232, 249, 38))
glow = glow.filter(ImageFilter.GaussianBlur(radius=80))
out.paste(glow, (0, 0), glow)

# Icon on left with a soft drop shadow
icon = Image.open(ICON_SRC).convert("RGBA")
ICON_SIZE = 280
icon = icon.resize((ICON_SIZE, ICON_SIZE), Image.LANCZOS)

shadow = Image.new("RGBA", (ICON_SIZE + 80, ICON_SIZE + 80), (0, 0, 0, 0))
sd = ImageDraw.Draw(shadow)
sd.rounded_rectangle(
    [40, 40, 40 + ICON_SIZE, 40 + ICON_SIZE],
    radius=60,
    fill=(0, 0, 0, 110),
)
shadow = shadow.filter(ImageFilter.GaussianBlur(radius=22))

icon_x = 100
icon_y = (H - ICON_SIZE) // 2
out.paste(shadow, (icon_x - 40, icon_y - 30), shadow)
out.paste(icon, (icon_x, icon_y), icon)

# Try to use a nice system font; fall back to default if missing.
def load_font(size: int) -> ImageFont.ImageFont:
    candidates = [
        "/System/Library/Fonts/Helvetica.ttc",
        "/System/Library/Fonts/HelveticaNeue.ttc",
        "/System/Library/Fonts/SFNS.ttf",
        "/Library/Fonts/Arial.ttf",
        "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
    ]
    for path in candidates:
        if Path(path).exists():
            try:
                return ImageFont.truetype(path, size)
            except (OSError, ValueError):
                continue
    return ImageFont.load_default()

font_brand = load_font(76)
font_tag = load_font(30)

# Text on right
text_x = icon_x + ICON_SIZE + 70

# Compute vertical center of the headline + tagline block
brand_bbox = draw.textbbox((0, 0), "DraftRight", font=font_brand)
brand_h = brand_bbox[3] - brand_bbox[1]

tag1 = "Rewrite text in any tone."
tag2 = "AI keyboard. Any app. Any device."
tag_bbox = draw.textbbox((0, 0), tag2, font=font_tag)
tag_h = tag_bbox[3] - tag_bbox[1]

block_h = brand_h + 30 + tag_h + 8 + tag_h
top = (H - block_h) // 2

draw.text((text_x, top), "DraftRight", fill=(255, 255, 255), font=font_brand)
draw.text((text_x, top + brand_h + 30), tag1, fill=(203, 213, 225), font=font_tag)
draw.text((text_x, top + brand_h + 30 + tag_h + 8), tag2, fill=(148, 163, 184), font=font_tag)

OUT.parent.mkdir(parents=True, exist_ok=True)
out.save(OUT, "PNG", optimize=True)
print(f"Wrote {OUT}")
print(f"Size: {OUT.stat().st_size:,} bytes ({OUT.stat().st_size / 1024:.1f} KB)")
