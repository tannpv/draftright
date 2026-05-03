#!/usr/bin/env python3
"""Generate 4 marketing-style phone screenshots for Google Play.

Each is 1080x1920 (9:16, portrait), >= 1080 px each side so it
qualifies for Play Store promotion. Same dark slate palette and
font choices as the feature graphic for visual consistency.

Re-run any time: python3 scripts/gen-screenshots.py
"""

from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter, ImageFont

W, H = 1080, 1920
ROOT = Path(__file__).resolve().parents[1]
ICON_SRC = ROOT / "store-assets" / "google-play" / "icon-512.png"
OUT_DIR = ROOT / "store-assets" / "google-play"

TOP = (15, 23, 42)       # slate-900
BOTTOM = (30, 41, 59)    # slate-800
ACCENT = (103, 232, 249) # cyan-300
TEXT_PRIMARY = (255, 255, 255)
TEXT_SECONDARY = (203, 213, 225)  # slate-300
TEXT_TERTIARY = (148, 163, 184)   # slate-400


def load_font(size: int) -> ImageFont.ImageFont:
    candidates = [
        "/System/Library/Fonts/Helvetica.ttc",
        "/System/Library/Fonts/HelveticaNeue.ttc",
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


def base_canvas() -> tuple[Image.Image, ImageDraw.ImageDraw]:
    img = Image.new("RGB", (W, H), TOP)
    draw = ImageDraw.Draw(img)
    for y in range(H):
        t = y / H
        r = int(TOP[0] + (BOTTOM[0] - TOP[0]) * t)
        g = int(TOP[1] + (BOTTOM[1] - TOP[1]) * t)
        b = int(TOP[2] + (BOTTOM[2] - TOP[2]) * t)
        draw.line([(0, y), (W, y)], fill=(r, g, b))

    # Subtle accent glow at top-right
    glow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    gd = ImageDraw.Draw(glow)
    gd.ellipse([W - 700, -300, W + 200, 600], fill=(*ACCENT, 32))
    glow = glow.filter(ImageFilter.GaussianBlur(radius=120))
    img.paste(glow, (0, 0), glow)
    return img, ImageDraw.Draw(img)


def add_footer_brand(img: Image.Image, draw: ImageDraw.ImageDraw):
    """Small icon + 'DraftRight' wordmark anchored at the bottom."""
    icon = Image.open(ICON_SRC).convert("RGBA")
    s = 80
    icon = icon.resize((s, s), Image.LANCZOS)
    icon_x, icon_y = 80, H - 180
    img.paste(icon, (icon_x, icon_y), icon)

    f = load_font(40)
    draw.text((icon_x + s + 24, icon_y + 18), "DraftRight",
              fill=TEXT_PRIMARY, font=f)


def wrap_text(text: str, font: ImageFont.ImageFont,
              draw: ImageDraw.ImageDraw, max_w: int) -> list[str]:
    """Naive word-wrap."""
    words = text.split()
    lines, cur = [], []
    for w in words:
        trial = " ".join(cur + [w])
        bbox = draw.textbbox((0, 0), trial, font=font)
        if bbox[2] - bbox[0] <= max_w:
            cur.append(w)
        else:
            if cur:
                lines.append(" ".join(cur))
            cur = [w]
    if cur:
        lines.append(" ".join(cur))
    return lines


def screenshot_hero():
    img, draw = base_canvas()

    # Big icon centered
    icon = Image.open(ICON_SRC).convert("RGBA")
    s = 360
    icon = icon.resize((s, s), Image.LANCZOS)
    shadow = Image.new("RGBA", (s + 80, s + 80), (0, 0, 0, 0))
    sd = ImageDraw.Draw(shadow)
    sd.rounded_rectangle([40, 40, 40 + s, 40 + s], radius=80, fill=(0, 0, 0, 110))
    shadow = shadow.filter(ImageFilter.GaussianBlur(radius=24))
    icon_x = (W - s) // 2
    icon_y = 360
    img.paste(shadow, (icon_x - 40, icon_y - 30), shadow)
    img.paste(icon, (icon_x, icon_y), icon)

    # Headline
    headline_font = load_font(96)
    sub_font = load_font(48)
    headline = "Rewrite text"
    headline2 = "in any tone."
    hh_y = icon_y + s + 90
    bb1 = draw.textbbox((0, 0), headline, font=headline_font)
    draw.text(((W - (bb1[2] - bb1[0])) // 2, hh_y), headline,
              fill=TEXT_PRIMARY, font=headline_font)
    bb2 = draw.textbbox((0, 0), headline2, font=headline_font)
    draw.text(((W - (bb2[2] - bb2[0])) // 2, hh_y + 130), headline2,
              fill=TEXT_PRIMARY, font=headline_font)

    sub = "AI keyboard. Any app. Any device."
    bb3 = draw.textbbox((0, 0), sub, font=sub_font)
    draw.text(((W - (bb3[2] - bb3[0])) // 2, hh_y + 290), sub,
              fill=TEXT_SECONDARY, font=sub_font)

    img.save(OUT_DIR / "screenshot-1-hero.png", "PNG", optimize=True)


def screenshot_tones():
    img, draw = base_canvas()
    add_footer_brand(img, draw)

    headline_font = load_font(88)
    sub_font = load_font(40)
    list_font = load_font(46)

    headline = "9 tones."
    headline2 = "One tap."
    bb1 = draw.textbbox((0, 0), headline, font=headline_font)
    draw.text(((W - (bb1[2] - bb1[0])) // 2, 200), headline,
              fill=TEXT_PRIMARY, font=headline_font)
    bb2 = draw.textbbox((0, 0), headline2, font=headline_font)
    draw.text(((W - (bb2[2] - bb2[0])) // 2, 320), headline2,
              fill=TEXT_PRIMARY, font=headline_font)

    sub = "Pick how it should sound."
    bb3 = draw.textbbox((0, 0), sub, font=sub_font)
    draw.text(((W - (bb3[2] - bb3[0])) // 2, 470), sub,
              fill=TEXT_SECONDARY, font=sub_font)

    tones = [
        "Professional", "Casual", "Polished",
        "Concise", "Simple", "Technical",
        "Claude", "Grammar Check", "Translate",
    ]
    y = 620
    for tone in tones:
        bb = draw.textbbox((0, 0), tone, font=list_font)
        draw.text(((W - (bb[2] - bb[0])) // 2, y), tone,
                  fill=TEXT_PRIMARY, font=list_font)
        y += 90

    img.save(OUT_DIR / "screenshot-2-tones.png", "PNG", optimize=True)


def screenshot_anywhere():
    img, draw = base_canvas()
    add_footer_brand(img, draw)

    headline_font = load_font(96)
    sub_font = load_font(46)

    headline = "Works"
    headline2 = "in every app."
    bb1 = draw.textbbox((0, 0), headline, font=headline_font)
    draw.text(((W - (bb1[2] - bb1[0])) // 2, 360), headline,
              fill=TEXT_PRIMARY, font=headline_font)
    bb2 = draw.textbbox((0, 0), headline2, font=headline_font)
    draw.text(((W - (bb2[2] - bb2[0])) // 2, 480), headline2,
              fill=TEXT_PRIMARY, font=headline_font)

    examples = [
        "Messages", "Mail", "Slack",
        "WhatsApp", "Notes", "Twitter",
        "Reddit", "LinkedIn", "Anywhere you type",
    ]
    sf = load_font(40)
    y = 760
    for ex in examples:
        bb = draw.textbbox((0, 0), ex, font=sf)
        draw.text(((W - (bb[2] - bb[0])) // 2, y), ex,
                  fill=TEXT_TERTIARY, font=sf)
        y += 80

    sub = "Tap a tone in the keyboard toolbar."
    bb3 = draw.textbbox((0, 0), sub, font=sub_font)
    draw.text(((W - (bb3[2] - bb3[0])) // 2, 600), sub,
              fill=TEXT_SECONDARY, font=sub_font)

    img.save(OUT_DIR / "screenshot-3-anywhere.png", "PNG", optimize=True)


def screenshot_privacy():
    img, draw = base_canvas()
    add_footer_brand(img, draw)

    headline_font = load_font(96)
    sub_font = load_font(40)
    body_font = load_font(38)

    headline = "Your text,"
    headline2 = "never stored."
    bb1 = draw.textbbox((0, 0), headline, font=headline_font)
    draw.text(((W - (bb1[2] - bb1[0])) // 2, 360), headline,
              fill=TEXT_PRIMARY, font=headline_font)
    bb2 = draw.textbbox((0, 0), headline2, font=headline_font)
    draw.text(((W - (bb2[2] - bb2[0])) // 2, 480), headline2,
              fill=TEXT_PRIMARY, font=headline_font)

    body_lines = [
        "Text passes through to your AI",
        "provider and back to your device.",
        "",
        "DraftRight doesn't keep it.",
        "",
        "The keyboard never reads",
        "your contacts, passwords, or",
        "system clipboard.",
    ]
    y = 720
    for line in body_lines:
        if not line:
            y += 40
            continue
        bb = draw.textbbox((0, 0), line, font=body_font)
        draw.text(((W - (bb[2] - bb[0])) // 2, y), line,
                  fill=TEXT_SECONDARY, font=body_font)
        y += 60

    img.save(OUT_DIR / "screenshot-4-privacy.png", "PNG", optimize=True)


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    screenshot_hero()
    screenshot_tones()
    screenshot_anywhere()
    screenshot_privacy()

    for p in sorted(OUT_DIR.glob("screenshot-*.png")):
        size_kb = p.stat().st_size / 1024
        print(f"  {p.name:30} {size_kb:6.1f} KB")


if __name__ == "__main__":
    main()
