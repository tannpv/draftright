"""Build the tray icon for a given state: status tint + "update ready" dot.

Mirrors ``TrayIconBadge.WithDot`` on Windows and ``MenuBarIcon.image`` on
macOS — same red-500 dot, same ~45% diameter, same contrasting ring — so the
three trays convey identical information.

The plain connected state deliberately produces **no** file: the caller uses
the named symbolic icon instead, letting the shell recolour it for light and
dark panels. Only a tint or a badge needs a composited PNG, and once we paint
our own colours the shell must not repaint them.
"""

from __future__ import annotations

import logging
import tempfile
from pathlib import Path

import gi

gi.require_version("GdkPixbuf", "2.0")
from gi.repository import GdkPixbuf, GLib

from draftright import config
from draftright.models.health import HealthStatus

log = logging.getLogger(__name__)

# The symbolic SVG ships with the Adwaita placeholder fill; swapping that
# string is how we tint without a vector library.
_SYMBOLIC_PLACEHOLDER_FILL = "#2e3436"
# Foreground for a composited icon. GNOME's top bar is dark by default, so
# white reads correctly; a tinted state overrides this anyway.
_DEFAULT_FOREGROUND = "#ffffff"


def symbolic_source() -> Path | None:
    """Locate the installed symbolic SVG, or None if it isn't installed."""
    candidates = [
        Path.home() / ".local/share/icons/hicolor/scalable/apps"
        / f"{config.APP_ID}-symbolic.svg",
        Path("/usr/share/icons/hicolor/scalable/apps") / f"{config.APP_ID}-symbolic.svg",
        # Source checkout, so a dev run behaves like an installed one.
        Path(__file__).resolve().parent.parent.parent
        / "data/icons/hicolor/scalable/apps" / f"{config.APP_ID}-symbolic.svg",
    ]
    for path in candidates:
        if path.exists():
            return path
    return None


def _render_svg(svg_bytes: bytes, size: int) -> GdkPixbuf.Pixbuf:
    loader = GdkPixbuf.PixbufLoader.new_with_type("svg")
    loader.set_size(size, size)
    loader.write(svg_bytes)
    loader.close()
    return loader.get_pixbuf()


def _draw_badge(pixbuf: GdkPixbuf.Pixbuf) -> GdkPixbuf.Pixbuf:
    """Composite the update dot into the bottom-right corner.

    Drawn by filling pixels inside two circles rather than with cairo, which
    keeps this dependency-free and is cheap at this size.
    """
    size = pixbuf.get_width()
    pixbuf = pixbuf.copy()
    if not pixbuf.get_has_alpha():
        pixbuf = pixbuf.add_alpha(False, 0, 0, 0)

    diameter = max(6, int(size * config.TRAY_BADGE_SIZE_RATIO))
    radius = diameter / 2
    # The offline state paints the mark red, so a bare red dot would vanish
    # into it; the ring is what keeps the badge readable there.
    ring_width = max(1.0, size * config.TRAY_BADGE_RING_RATIO)
    cx = size - radius - 1
    cy = size - radius - 1

    dot = _hex_to_rgb(config.TRAY_BADGE_COLOR)
    ring = _hex_to_rgb(config.TRAY_BADGE_RING_COLOR)

    pixels = bytearray(pixbuf.get_pixels())
    stride = pixbuf.get_rowstride()
    channels = pixbuf.get_n_channels()

    reach = int(radius + ring_width) + 2
    for y in range(max(0, int(cy - reach)), size):
        for x in range(max(0, int(cx - reach)), size):
            d = ((x - cx) ** 2 + (y - cy) ** 2) ** 0.5
            if d > radius + ring_width:
                continue
            colour = dot if d <= radius else ring
            offset = y * stride + x * channels
            pixels[offset] = colour[0]
            pixels[offset + 1] = colour[1]
            pixels[offset + 2] = colour[2]
            if channels == 4:
                pixels[offset + 3] = 255

    return GdkPixbuf.Pixbuf.new_from_bytes(
        GLib.Bytes.new(bytes(pixels)),
        pixbuf.get_colorspace(),
        pixbuf.get_has_alpha(),
        pixbuf.get_bits_per_sample(),
        pixbuf.get_width(),
        pixbuf.get_height(),
        stride,
    )


def _hex_to_rgb(value: str) -> tuple[int, int, int]:
    value = value.lstrip("#")
    return tuple(int(value[i:i + 2], 16) for i in (0, 2, 4))  # type: ignore[return-value]


def build(
    status: HealthStatus,
    update_available: bool,
    directory: str | None = None,
) -> tuple[str, str] | None:
    """Render the icon for *status* / *update_available*.

    Returns ``(theme_directory, icon_name)`` for AppIndicator, or None when
    the plain named symbolic should be used instead (nothing to composite).
    """
    if status.tint_color is None and not update_available:
        return None  # let the shell recolour the named symbolic

    source = symbolic_source()
    if source is None:
        log.warning("Symbolic icon not installed; cannot render a tray state.")
        return None

    try:
        svg = source.read_text(encoding="utf-8")
        svg = svg.replace(
            _SYMBOLIC_PLACEHOLDER_FILL, status.tint_color or _DEFAULT_FOREGROUND
        )
        pixbuf = _render_svg(svg.encode("utf-8"), config.TRAY_ICON_RENDER_SIZE)
        if update_available:
            pixbuf = _draw_badge(pixbuf)

        target_dir = directory or tempfile.mkdtemp(prefix="draftright-tray-")
        # AppIndicator caches by name, so vary it per state or the icon will
        # not visibly change.
        name = f"draftright-tray-{status.value}{'-update' if update_available else ''}"
        pixbuf.savev(str(Path(target_dir) / f"{name}.png"), "png", [], [])
        return target_dir, name
    except (OSError, GLib.Error) as exc:
        log.warning("Could not render the tray icon: %s", exc)
        return None
