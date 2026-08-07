"""Shared GTK styling for the rewrite panel and its result views.

Every rule here is generated from the design tokens in :mod:`draftright.config`
— there are no colour literals in this file, so retinting the app is a config
edit rather than a grep. The provider is registered once per display: the panel
used to do that itself, and a second styled widget would otherwise have had to
duplicate both the sheet and the bookkeeping.
"""

from __future__ import annotations

from importlib import resources

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")

from gi.repository import Gdk, Gtk

from draftright import config

PANEL_CSS = f"""
.rewrite-panel {{
    background-color: {config.COLOR_BACKGROUND};
    border: 1px solid {config.COLOR_BORDER};
    border-radius: 12px;
}}
.rewrite-header {{
    color: {config.COLOR_BRAND_BLUE};
    font-size: 16px;
    font-weight: bold;
}}
.input-preview {{
    color: {config.COLOR_MUTED};
    font-size: 13px;
}}
.tone-button {{
    background-color: {config.COLOR_CARD};
    color: {config.COLOR_TEXT};
    border: 1px solid {config.COLOR_BORDER};
    border-radius: 8px;
    padding: 8px 12px;
    font-size: 13px;
    min-height: 40px;
}}
.tone-button:hover {{
    background-color: {config.COLOR_BORDER};
}}
.tone-button-selected {{
    background-color: {config.COLOR_BRAND_BLUE};
    color: {config.COLOR_WHITE};
    border: 1px solid {config.COLOR_BRAND_BLUE};
    border-radius: 8px;
    padding: 8px 12px;
    font-size: 13px;
    min-height: 40px;
}}
.result-area {{
    background-color: {config.COLOR_CARD};
    color: {config.COLOR_TEXT};
    border: 1px solid {config.COLOR_BORDER};
    border-radius: 8px;
    padding: 8px;
    font-size: 14px;
}}
.btn-primary {{
    background-color: {config.COLOR_BRAND_BLUE};
    color: {config.COLOR_WHITE};
    border-radius: 8px;
    padding: 8px 16px;
    font-weight: bold;
}}
.btn-primary:hover {{
    background-color: {config.COLOR_BRAND_BLUE_HOVER};
}}
.btn-outlined {{
    background-color: transparent;
    color: {config.COLOR_BRAND_BLUE};
    border: 1px solid {config.COLOR_BRAND_BLUE};
    border-radius: 8px;
    padding: 8px 16px;
}}
.btn-outlined:hover {{
    background-color: {config.COLOR_CARD};
}}
.btn-ghost {{
    background-color: transparent;
    color: {config.COLOR_MUTED};
    border: none;
    border-radius: 8px;
    padding: 8px 16px;
}}
.btn-ghost:hover {{
    background-color: {config.COLOR_CARD};
}}
.error-label {{
    color: {config.COLOR_ERROR};
    font-size: 13px;
}}
.copied-label {{
    color: {config.COLOR_SUCCESS};
    font-size: 13px;
}}
"""

# Diff view + grammar view (#107). Section headings share one class so the two
# result pages read as the same surface.
RESULT_VIEW_CSS = f"""
.section-heading {{
    color: {config.COLOR_MUTED};
    font-size: 10px;
    font-weight: bold;
}}
.diff-notice {{
    color: {config.COLOR_WARNING};
    font-size: 11px;
}}
.grammar-score {{
    font-size: 13px;
    font-weight: bold;
}}
.grammar-meta {{
    color: {config.COLOR_MUTED};
    font-size: 11px;
}}
.grammar-card {{
    background-color: {config.COLOR_CARD};
    border: 1px solid {config.COLOR_BORDER};
    border-radius: 6px;
    padding: 8px;
}}
.grammar-clean {{
    color: {config.COLOR_SUCCESS};
    font-size: 12px;
    font-weight: bold;
}}
.btn-compact {{
    padding: 2px 10px;
    font-size: 11px;
    min-height: 0;
}}
"""

_STYLESHEET = PANEL_CSS + RESULT_VIEW_CSS

# Keyed by (display, sheet) so a multi-seat / multi-display session still gets
# styled, while code that runs more than once — a panel rebuilt on every hotkey
# press, do_activate() on every tray "Show" — does not stack one provider per
# call for the life of the process.
_loaded_displays: set = set()


def _ensure(key: str, load) -> None:
    """Build and register a provider for *key*, at most once per display.

    *load* receives the fresh provider and may raise; a sheet that fails to
    load is simply not registered, rather than leaving an empty provider on
    the display and marking the job done.
    """
    display = Gdk.Display.get_default()
    if display is None or (display, key) in _loaded_displays:
        return
    provider = Gtk.CssProvider()
    load(provider)
    Gtk.StyleContext.add_provider_for_display(
        display,
        provider,
        Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
    )
    _loaded_displays.add((display, key))


def ensure_loaded() -> None:
    """Register the token-generated panel + result-view stylesheet."""
    _ensure("tokens", lambda provider: provider.load_from_string(_STYLESHEET))


def ensure_resource_css_loaded() -> None:
    """Register the shipped ``resources/style.css`` app-wide overrides.

    Missing or unreadable resources are ignored: the token stylesheet above
    carries everything the panel needs, so a packaging slip degrades the look
    rather than failing the launch.
    """
    try:
        path = resources.files("draftright.resources").joinpath("style.css")
        _ensure("resources", lambda provider: provider.load_from_path(str(path)))
    except Exception:
        pass


def rgba(color: str, alpha: float = 1.0) -> Gdk.RGBA:
    """Turn a design-token hex string into a (optionally translucent) RGBA.

    ``Gtk.TextTag`` colours cannot come from CSS, so text-level highlighting
    needs the tokens as real colour objects.
    """
    value = Gdk.RGBA()
    value.parse(color)
    value.alpha = alpha
    return value
