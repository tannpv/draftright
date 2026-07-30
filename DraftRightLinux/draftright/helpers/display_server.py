"""Utility to detect X11 vs Wayland display server."""

import os


def is_wayland():
    """Return True if running under Wayland.

    Honours an explicit ``GDK_BACKEND=wayland`` override in addition to the
    ``WAYLAND_DISPLAY`` socket, so the clipboard/hotkey/injection paths all
    agree on the display server from this one place (Rule #1: single source).
    """
    if os.environ.get('GDK_BACKEND', '').lower() == 'wayland':
        return True
    return bool(os.environ.get('WAYLAND_DISPLAY'))


def is_x11():
    """Return True if running under X11 (and not Wayland)."""
    return bool(os.environ.get('DISPLAY')) and not is_wayland()


def get_display_server():
    """Return the current display server name: 'wayland' or 'x11'."""
    if is_wayland():
        return 'wayland'
    return 'x11'
