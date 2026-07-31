"""
Global hotkey listener supporting both X11 and Wayland.

X11     — python-xlib ``XGrabKey`` on the root window (background thread).
Wayland — ``org.freedesktop.portal.GlobalShortcuts`` over D-Bus, driven on the
          GLib main loop (#99).  There is no fallback: nothing else can observe
          a global key press under Wayland, so when the portal declines, the
          service reports that plainly instead of pretending to be bound.

Key strings are parsed once into :class:`draftright.models.hotkey.Hotkey` and
rendered per backend — see that module.
"""

from __future__ import annotations

import logging
import threading
from typing import Callable

from gi.repository import GLib  # type: ignore[attr-defined]

from draftright import config
from draftright.helpers.display_server import is_wayland
from draftright.models.hotkey import Hotkey
from draftright.services.portal import PortalClient, PortalResponse

__all__ = ["HotkeyService", "PortalResponse"]

log = logging.getLogger(__name__)


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

        try:
            hotkey = Hotkey.parse(keystring)
        except ValueError as exc:
            log.error("Cannot parse hotkey %r: %s", keystring, exc)
            return
        mods, keysym_name = hotkey.x11_modifiers, hotkey.key

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
# Wayland listener — xdg-desktop-portal GlobalShortcuts (#99)
# ---------------------------------------------------------------------------


class _PortalListener:
    """Global shortcut on Wayland via ``org.freedesktop.portal.GlobalShortcuts``.

    The handshake is asynchronous and runs entirely on the GLib main loop —
    there is no worker thread, because every step is driven by a D-Bus reply:

        CreateSession → Response(session_handle)
          → BindShortcuts → Response(shortcuts)
            → ListShortcuts → Response(shortcuts)   [read back the real trigger]
              → Activated signals invoke the callback

    The compositor, not the app, owns the final binding: it may hand back a
    different trigger than we asked for, or none at all.  Whatever it reports
    is exposed via :attr:`active_trigger` so the UI can show what actually
    works instead of what we requested.
    """

    def __init__(self) -> None:
        self._client = PortalClient(
            config.PORTAL_GLOBAL_SHORTCUTS_IFACE, name_prefix="globalshortcuts"
        )
        self._callback: Callable[[], None] | None = None
        self._hotkey: Hotkey | None = None
        self._session_handle: str | None = None
        self._subscriptions: list[int] = []
        self.active_trigger: str | None = None
        self.on_trigger_changed: Callable[[str | None], None] | None = None

    # -- lifecycle ---------------------------------------------------------

    def start(self, keystring: str, callback: Callable[[], None]) -> None:
        self._callback = callback
        try:
            self._hotkey = Hotkey.parse(keystring)
        except ValueError as exc:
            log.error("Cannot parse hotkey %r: %s", keystring, exc)
            return

        if not self._client.connect():
            self._report_unavailable("No session bus")
            return

        # Listen for activations before the session exists; the filter is by
        # interface, and we match the session handle when one arrives.
        self._subscriptions.append(
            self._client.subscribe_signal("Activated", self._on_activated)
        )
        # The compositor can rebind our shortcut at any time (system settings).
        self._subscriptions.append(
            self._client.subscribe_signal("ShortcutsChanged", self._on_shortcuts_changed)
        )
        self._create_session()

    def stop(self) -> None:
        for subscription in self._subscriptions:
            self._client.unsubscribe(subscription)
        self._subscriptions.clear()
        if self._session_handle:
            self._client.close_session(self._session_handle)
        self._session_handle = None
        self._client.close()
        self._callback = None
        # Clear without notifying: shutdown is not "the compositor refused",
        # and firing the callback here would flip the UI to "declined" while
        # the app is quitting.
        self.active_trigger = None

    # -- handshake ---------------------------------------------------------

    def _create_session(self) -> None:
        def build(handle_token: str) -> GLib.Variant:
            options = {
                "handle_token": GLib.Variant("s", handle_token),
                "session_handle_token": GLib.Variant(
                    "s", self._client.next_token("session")
                ),
            }
            return GLib.Variant("(a{sv})", (options,))

        self._client.call_with_request(
            "CreateSession", build, self._on_session_created, "createsession",
            on_error=self._report_unavailable,
        )

    def _on_session_created(
        self, response: PortalResponse, results: GLib.Variant
    ) -> None:
        if response is not PortalResponse.SUCCESS:
            self._report_unavailable(
                f"GlobalShortcuts session not created ({response.display_name})"
            )
            return
        self._session_handle = results.unpack().get("session_handle")
        if not self._session_handle:
            self._report_unavailable("Portal returned no session_handle")
            return
        log.debug("GlobalShortcuts session: %s", self._session_handle)
        self._bind_shortcuts()

    def _bind_shortcuts(self) -> None:
        trigger = self._hotkey.to_portal_trigger()

        def build(handle_token: str) -> GLib.Variant:
            shortcut_options = {
                "description": GLib.Variant("s", config.PORTAL_SHORTCUT_DESCRIPTION),
                "preferred_trigger": GLib.Variant("s", trigger),
            }
            shortcuts = [(config.PORTAL_SHORTCUT_ID, shortcut_options)]
            options = {"handle_token": GLib.Variant("s", handle_token)}
            return GLib.Variant(
                "(oa(sa{sv})sa{sv})",
                (self._session_handle, shortcuts, "", options),
            )

        log.info("Requesting Wayland global shortcut: %s", trigger)
        self._client.call_with_request(
            "BindShortcuts", build, self._on_shortcuts_bound, "bind",
            on_error=self._report_unavailable,
        )

    def _on_shortcuts_bound(
        self, response: PortalResponse, results: GLib.Variant
    ) -> None:
        if response is not PortalResponse.SUCCESS:
            self._report_unavailable(f"Shortcut request was {response.display_name}")
            return
        self._adopt_reported_shortcuts(results, source="BindShortcuts")
        # Ask the portal what it actually bound; some compositors return an
        # empty list from BindShortcuts and only populate ListShortcuts.
        if self.active_trigger is None:
            self._list_shortcuts()

    def _list_shortcuts(self) -> None:
        def build(handle_token: str) -> GLib.Variant:
            options = {"handle_token": GLib.Variant("s", handle_token)}
            return GLib.Variant("(oa{sv})", (self._session_handle, options))

        self._client.call_with_request(
            "ListShortcuts",
            build,
            lambda response, results: self._adopt_reported_shortcuts(
                results, source="ListShortcuts"
            )
            if response is PortalResponse.SUCCESS
            else None,
            "list",
        )

    # -- signals -----------------------------------------------------------

    def _adopt_reported_shortcuts(self, results: GLib.Variant, source: str) -> None:
        """Record the trigger the compositor says it bound."""
        shortcuts = results.unpack().get("shortcuts") or []
        for shortcut_id, options in shortcuts:
            if shortcut_id != config.PORTAL_SHORTCUT_ID:
                continue
            trigger = options.get("trigger_description") or options.get("trigger")
            self._set_active_trigger(trigger)
            log.info("Wayland global shortcut active (%s): %s", source, trigger)
            return

    def _set_active_trigger(self, trigger: str | None, force: bool = False) -> None:
        """Record the live trigger and notify.

        *force* re-notifies even when the value is unchanged — needed to report
        failure, where the trigger stays None and the UI would otherwise sit on
        "requesting…" forever.
        """
        if trigger == self.active_trigger and not force:
            return
        self.active_trigger = trigger
        if self.on_trigger_changed is not None:
            self.on_trigger_changed(trigger)

    def _report_unavailable(self, reason: str) -> None:
        """Tell the app no shortcut is bound, and why."""
        log.warning("%s — the Wayland global hotkey will not fire.", reason)
        self._set_active_trigger(None, force=True)

    def _on_activated(self, _conn, _sender, _path, _iface, _signal, params) -> None:
        session_handle = params.get_child_value(0).get_string()
        shortcut_id = params.get_child_value(1).get_string()
        if session_handle != self._session_handle:
            return  # another app's session on the same bus
        if shortcut_id != config.PORTAL_SHORTCUT_ID:
            return
        if self._callback is not None:
            # Already on the main loop, but idle_add keeps the contract
            # identical to the X11 listener (which fires from a thread).
            GLib.idle_add(self._callback)

    def _on_shortcuts_changed(
        self, _conn, _sender, _path, _iface, _signal, params
    ) -> None:
        """The user rebound our shortcut in system settings."""
        if params.get_child_value(0).get_string() != self._session_handle:
            return
        for shortcut_id, options in params.get_child_value(1).unpack():
            if shortcut_id == config.PORTAL_SHORTCUT_ID:
                trigger = options.get("trigger_description") or options.get("trigger")
                self._set_active_trigger(trigger)
                log.info("Wayland global shortcut rebound to: %s", trigger)


# ---------------------------------------------------------------------------
# ---------------------------------------------------------------------------
# Public facade
# ---------------------------------------------------------------------------

class HotkeyService:
    """Register and listen for a global hotkey across X11 / Wayland."""

    def __init__(self) -> None:
        self._listener: _X11Listener | _PortalListener | None = None
        # Called with the trigger the compositor actually bound, or None when
        # no global shortcut is available.  Wayland only: on X11 the grab
        # either succeeds with the requested combination or fails outright.
        self.on_trigger_changed: Callable[[str | None], None] | None = None

    def start(self, keystring: str, callback: Callable[[], None]) -> None:
        """Start listening for *keystring* (e.g. ``Ctrl+Shift+R``).

        When the hotkey is detected, *callback* is invoked on the GTK main
        thread via ``GLib.idle_add``.

        Binding is **asynchronous on Wayland** — the portal may prompt the
        user, and the compositor may substitute a different trigger — so a
        successful return does not mean the shortcut is live.  Subscribe to
        :attr:`on_trigger_changed` for the authoritative answer.
        """
        self.stop()

        if is_wayland():
            log.info("Wayland detected — requesting a portal global shortcut.")
            listener = _PortalListener()
            listener.on_trigger_changed = self._on_trigger_changed
            self._listener = listener
        else:
            log.info("X11 detected — grabbing the key on the root window.")
            self._listener = _X11Listener()

        self._listener.start(keystring, callback)

    def stop(self) -> None:
        """Stop listening and release the binding."""
        if self._listener is not None:
            self._listener.stop()
            self._listener = None

    @property
    def active_trigger(self) -> str | None:
        """The trigger currently bound, or None if none is (Wayland only)."""
        return getattr(self._listener, "active_trigger", None)

    def _on_trigger_changed(self, trigger: str | None) -> None:
        if self.on_trigger_changed is not None:
            self.on_trigger_changed(trigger)
