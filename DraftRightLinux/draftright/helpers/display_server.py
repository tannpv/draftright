"""Utility to detect X11 vs Wayland display server."""

import os


def is_wayland():
    """Return True if running under Wayland."""
    return bool(os.environ.get('WAYLAND_DISPLAY'))


def is_x11():
    """Return True if running under X11 (and not Wayland)."""
    return bool(os.environ.get('DISPLAY')) and not is_wayland()


def get_display_server():
    """Return the current display server name: 'wayland' or 'x11'."""
    if is_wayland():
        return 'wayland'
    return 'x11'
