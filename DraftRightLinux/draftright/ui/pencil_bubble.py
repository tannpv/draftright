"""The pencil bubble — a small button shown at the pointer after a highlight.

Pencil mode (#188) offers the rewrite rather than imposing it: a highlight puts
this button beside the pointer, and only a click on it opens the rewrite panel.
Without that opt-in step every selection anywhere on the desktop would raise a
full-size panel and take focus.

X11 only, like the rest of Pencil mode — placement at the pointer is exactly
what Wayland forbids and X11 allows (see ``helpers/x11_placement``).
"""

from __future__ import annotations

import logging
from typing import Callable

import gi

gi.require_version("Gtk", "4.0")
from gi.repository import GLib, Gtk

try:
    gi.require_version("GdkX11", "4.0")
    from gi.repository import GdkX11
except (ValueError, ImportError):  # pragma: no cover — non-X11 build of GTK
    GdkX11 = None

from draftright import config
from draftright.helpers import x11_placement
from draftright.helpers.x11_placement import Point
from draftright.ui import styles

log = logging.getLogger(__name__)

BUBBLE_LABEL = "✎"


class PencilBubble(Gtk.Window):
    """A one-button window that offers to rewrite the current selection."""

    def __init__(self, on_click: Callable[[], None]) -> None:
        super().__init__()
        self._on_click = on_click
        self._dismiss_source = None

        self.set_decorated(False)
        self.set_resizable(False)
        self.set_default_size(config.BUBBLE_SIZE, config.BUBBLE_SIZE)
        styles.ensure_loaded()

        button = Gtk.Button(label=BUBBLE_LABEL)
        button.add_css_class("pencil-bubble")
        button.connect("clicked", self._on_button_clicked)
        self.set_child(button)

        # Undecorated windows get no close button from the compositor.
        keys = Gtk.EventControllerKey()
        keys.connect("key-pressed", self._on_key_pressed)
        self.add_controller(keys)

    # ------------------------------------------------------------------
    # Showing / hiding
    # ------------------------------------------------------------------

    def show_at_pointer(self) -> None:
        """Present the bubble beside the pointer and arm its auto-dismiss.

        The move must happen after ``present()``: the X window does not exist
        until the surface is realised, so there is nothing to move before then.
        """
        anchor = x11_placement.cursor_position()
        self.present()
        self._arm_dismiss()
        if anchor is None:
            # No pointer reading — better to show it wherever the WM put it
            # than to swallow the user's only affordance.
            log.debug("Pencil bubble shown without placement (no pointer read)")
            return
        GLib.idle_add(self._place_at, anchor)

    def dismiss(self) -> None:
        """Hide the bubble and disarm the timer. Idempotent."""
        self._cancel_dismiss()
        self.set_visible(False)

    # ------------------------------------------------------------------
    # Internals
    # ------------------------------------------------------------------

    def _place_at(self, anchor: Point) -> bool:
        xid = self._xid()
        if xid is None:
            log.debug("Pencil bubble has no X window; leaving placement to the WM")
            return False
        # GTK sizes are logical; xdotool speaks physical pixels. On a 2x display
        # this window really covers 2 * BUBBLE_SIZE, and clamping against the
        # logical size would let it hang off the screen edge.
        scale = self.get_scale_factor() or 1
        target = x11_placement.clamp_to_screen(
            anchor,
            Point(config.BUBBLE_SIZE * scale, config.BUBBLE_SIZE * scale),
            x11_placement.screen_size(),
            Point(config.BUBBLE_OFFSET_X * scale, config.BUBBLE_OFFSET_Y * scale),
        )
        x11_placement.move_window(xid, target)
        return False  # one-shot

    def _xid(self):
        surface = self.get_surface()
        if GdkX11 is None or not isinstance(surface, GdkX11.X11Surface):
            return None
        return surface.get_xid()

    def _arm_dismiss(self) -> None:
        self._cancel_dismiss()
        self._dismiss_source = GLib.timeout_add_seconds(
            config.BUBBLE_TIMEOUT_SECONDS, self._on_timeout
        )

    def _cancel_dismiss(self) -> None:
        if self._dismiss_source is not None:
            GLib.source_remove(self._dismiss_source)
            self._dismiss_source = None

    def _on_timeout(self) -> bool:
        self.dismiss()
        return False  # one-shot

    def _on_button_clicked(self, _button) -> None:
        self.dismiss()
        self._on_click()

    def _on_key_pressed(self, _controller, keyval, _keycode, _state) -> bool:
        from gi.repository import Gdk

        if keyval == Gdk.KEY_Escape:
            self.dismiss()
            return True
        return False
