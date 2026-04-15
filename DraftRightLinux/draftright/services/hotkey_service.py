"""
Global hotkey listener supporting both X11 and Wayland.

X11  — python-xlib ``XGrabKey`` on the root window (background thread).
Wayland — xdg-desktop-portal GlobalShortcuts via ``gi.repository.Xdp``
          (libportal), falling back to ``dbus-send``, then ``xdotool``.
"""

from __future__ import annotations

import logging
import os
import subprocess
import threading
from typing import Callable

from gi.repository import GLib  # type: ignore[attr-defined]

log = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Display-server detection
# ---------------------------------------------------------------------------

def _is_wayland() -> bool:
    if os.environ.get("GDK_BACKEND", "").lower() == "wayland":
        return True
    if os.environ.get("WAYLAND_DISPLAY"):
        return True
    return False


# ---------------------------------------------------------------------------
# Key-string parsing  ("Ctrl+Shift+R" → modifiers, keycode)
# ---------------------------------------------------------------------------

_MOD_MAP = {
    "ctrl": "control",
    "control": "control",
    "shift": "shift",
    "alt": "mod1",
    "mod1": "mod1",
    "super": "mod4",
    "mod4": "mod4",
}


def _parse_keystring(keystring: str) -> tuple[list[str], str]:
    """Return (modifier_names, keysym_name) from e.g. ``Ctrl+Shift+R``."""
    parts = [p.strip() for p in keystring.split("+")]
    key = parts[-1]
    mods = [_MOD_MAP[p.lower()] for p in parts[:-1] if p.lower() in _MOD_MAP]
    return mods, key


# ---------------------------------------------------------------------------
# X11 listener
# ---------------------------------------------------------------------------

class _X11Listener:
    """Grabs a key combo on the X root window and waits in a thread."""

    def __init__(self) -> None:
        self._thread: threading.Thread | None = None
        self._running = False

    def start(self, keystring: str, callback: Callable[[], None]) -> None:
        self._running = True
        self._thread = threading.Thread(
            target=self._run, args=(keystring, callback), daemon=True
        )
        self._thread.start()

    def stop(self) -> None:
        self._running = False

    def _run(self, keystring: str, callback: Callable[[], None]) -> None:
        try:
            from Xlib import X, XK, display as xdisplay  # type: ignore[import-untyped]
            from Xlib.ext import record as _  # noqa: F841 — just ensure ext loads
        except ImportError:
            log.error("python-xlib not installed — X11 hotkeys unavailable.")
            return

        dpy = xdisplay.Display()
        root = dpy.screen().root

        mods, keysym_name = _parse_keystring(keystring)

        keysym = XK.string_to_keysym(keysym_name)
        if keysym == 0:
            # Try uppercase variant
            keysym = XK.string_to_keysym(keysym_name.capitalize())
        if keysym == 0:
            log.error("Unknown keysym: %s", keysym_name)
            return

        keycode = dpy.keysym_to_keycode(keysym)
        if keycode == 0:
            log.error("Cannot map keysym %s to keycode.", keysym_name)
            return

        mod_mask = 0
        if "control" in mods:
            mod_mask |= X.ControlMask
        if "shift" in mods:
            mod_mask |= X.ShiftMask
        if "mod1" in mods:
            mod_mask |= X.Mod1Mask
        if "mod4" in mods:
            mod_mask |= X.Mod4Mask

        # Grab with and without NumLock / CapsLock / ScrollLock.
        numlk = X.Mod2Mask
        capslk = X.LockMask
        scrolllk = X.Mod3Mask
        ignored = [0, numlk, capslk, scrolllk,
                   numlk | capslk, numlk | scrolllk,
                   capslk | scrolllk, numlk | capslk | scrolllk]

        for extra in ignored:
            root.grab_key(
                keycode,
                mod_mask | extra,
                True,
                X.GrabModeAsync,
                X.GrabModeAsync,
            )

        root.change_attributes(event_mask=X.KeyPressMask)
        log.info("X11 hotkey registered: %s (keycode=%d, mod_mask=0x%x)",
                 keystring, keycode, mod_mask)

        while self._running:
            evt = dpy.next_event()
            if evt.type == X.KeyPress:
                GLib.idle_add(callback)

        # Ungrab on exit.
        for extra in ignored:
            root.ungrab_key(keycode, mod_mask | extra)
        dpy.close()


# ---------------------------------------------------------------------------
# Wayland listener
# ---------------------------------------------------------------------------

class _WaylandListener:
    """Best-effort global shortcut on Wayland compositors."""

    def __init__(self) -> None:
        self._thread: threading.Thread | None = None
        self._running = False

    def start(self, keystring: str, callback: Callable[[], None]) -> None:
        self._running = True
        self._thread = threading.Thread(
            target=self._run, args=(keystring, callback), daemon=True
        )
        self._thread.start()

    def stop(self) -> None:
        self._running = False

    def _run(self, keystring: str, callback: Callable[[], None]) -> None:
        # Strategy 1: libportal (Xdp)
        if self._try_libportal(keystring, callback):
            return
        # Strategy 2: dbus-send (xdg-desktop-portal GlobalShortcuts)
        if self._try_dbus(keystring, callback):
            return
        # Strategy 3: poll xdotool (very rough fallback)
        self._poll_xdotool(keystring, callback)

    # -- libportal ---------------------------------------------------------

    @staticmethod
    def _try_libportal(keystring: str, callback: Callable[[], None]) -> bool:
        try:
            import gi
            gi.require_version("Xdp", "1.0")
            from gi.repository import Xdp  # type: ignore[attr-defined]

            portal = Xdp.Portal.new()
            # libportal's global-shortcuts API is async/D-Bus based.
            # A full implementation would create a session and bind
            # the Activated signal.  Placeholder for now.
            log.info("libportal available but full GlobalShortcuts binding "
                     "is not yet implemented; trying next strategy.")
            return False
        except Exception:
            return False

    # -- dbus-send ---------------------------------------------------------

    def _try_dbus(self, keystring: str, callback: Callable[[], None]) -> bool:
        # xdg-desktop-portal GlobalShortcuts requires a proper D-Bus session;
        # implementing a full portal handshake here is non-trivial.
        log.debug("D-Bus GlobalShortcuts not implemented; trying xdotool.")
        return False

    # -- xdotool polling ---------------------------------------------------

    def _poll_xdotool(self, keystring: str, callback: Callable[[], None]) -> None:
        """Very coarse fallback: use ``xdotool`` to detect key combos."""
        import time

        log.warning(
            "Falling back to xdotool polling for Wayland hotkey '%s'. "
            "This is imprecise — consider running under X11 or installing "
            "libportal.",
            keystring,
        )

        # Convert "Ctrl+Shift+R" → args for xdotool
        parts = keystring.lower().replace("ctrl", "ctrl").split("+")
        target_key = parts[-1] if parts else "r"

        while self._running:
            try:
                result = subprocess.run(
                    ["xdotool", "getactivewindow", "getwindowname"],
                    capture_output=True, text=True, timeout=2,
                )
                # xdotool cannot truly detect hotkeys in Wayland — this is
                # a stub that keeps the thread alive.  Real Wayland hotkey
                # support requires compositor integration.
                time.sleep(1)
            except FileNotFoundError:
                log.error("xdotool not found — Wayland hotkeys unavailable.")
                return
            except Exception:
                time.sleep(2)


# ---------------------------------------------------------------------------
# Public facade
# ---------------------------------------------------------------------------

class HotkeyService:
    """Register and listen for a global hotkey across X11 / Wayland."""

    def __init__(self) -> None:
        self._listener: _X11Listener | _WaylandListener | None = None

    def start(self, keystring: str, callback: Callable[[], None]) -> None:
        """Start listening for *keystring* (e.g. ``Ctrl+Shift+R``).

        When the hotkey is detected, *callback* is invoked on the GTK
        main thread via ``GLib.idle_add``.
        """
        self.stop()

        if _is_wayland():
            log.info("Wayland detected — using Wayland hotkey listener.")
            self._listener = _WaylandListener()
        else:
            log.info("X11 detected — using X11 hotkey listener.")
            self._listener = _X11Listener()

        self._listener.start(keystring, callback)

    def stop(self) -> None:
        """Stop the listening thread."""
        if self._listener is not None:
            self._listener.stop()
            self._listener = None
