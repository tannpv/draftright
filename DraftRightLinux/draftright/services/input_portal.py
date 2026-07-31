"""Keystroke injection on Wayland via ``org.freedesktop.portal.RemoteDesktop``.

Why this exists
---------------
Replacing the user's selection means synthesising Ctrl+V into whatever app has
focus.  Under X11 ``xdotool`` does that.  Under Wayland it cannot: XTEST only
reaches XWayland clients, so a paste aimed at a native Wayland window silently
does nothing.  ``wtype`` is no help on GNOME either — it needs
``zwp_virtual_keyboard_manager_v1``, which Mutter does not implement.

The RemoteDesktop portal is the sanctioned path.  It costs one consent prompt,
so the session is created once and — via ``persist_mode`` plus a stored
``restore_token`` — reused across restarts, meaning the user approves once
rather than once per rewrite.

Because the handshake is asynchronous and the paste is not, :meth:`paste`
queues the keystroke and replays it as soon as the session is live.  The text
is already on the clipboard by then, so even a denied session leaves the user
able to paste manually.
"""

from __future__ import annotations

import logging
from enum import Enum
from typing import Callable, Optional

from gi.repository import GLib  # type: ignore[attr-defined]

from draftright import config
from draftright.services.portal import PortalClient, PortalResponse

log = logging.getLogger(__name__)

# X11 keysyms (the portal speaks keysyms, not scancodes).
_KEYSYM_CTRL_L = 0xFFE3
_KEYSYM_V = 0x76
_KEY_RELEASED = 0
_KEY_PRESSED = 1

# RemoteDesktop device bitmask.
_DEVICE_KEYBOARD = 1

# Start() persist_mode: 2 = persistent, so the grant survives app restarts.
_PERSIST_PERSISTENT = 2


class InjectorState(Enum):
    """Lifecycle of the RemoteDesktop session."""

    IDLE = "idle"
    STARTING = "starting"
    READY = "ready"
    DENIED = "denied"

    @property
    def display_name(self) -> str:
        return {
            InjectorState.IDLE: "not requested",
            InjectorState.STARTING: "waiting for permission",
            InjectorState.READY: "ready",
            InjectorState.DENIED: "permission denied",
        }[self]


class RemoteDesktopInjector:
    """Synthesises Ctrl+V through the RemoteDesktop portal."""

    def __init__(self, settings_service=None) -> None:
        """
        Args:
            settings_service: Optional SettingsService used to persist the
                portal's restore token, so consent survives restarts.
        """
        self._client = PortalClient(
            config.PORTAL_REMOTE_DESKTOP_IFACE, name_prefix="remotedesktop"
        )
        self._settings = settings_service
        self._session_handle: Optional[str] = None
        self._pending: list[Callable[[bool], None] | None] = []
        self.state = InjectorState.IDLE
        # Notified whenever `state` changes, so the UI can explain itself.
        self.on_state_changed: Callable[[InjectorState], None] | None = None

    # -- public API --------------------------------------------------------

    def paste(self, on_done: Callable[[bool], None] | None = None) -> None:
        """Send Ctrl+V to the focused window, starting a session if needed.

        *on_done* receives True once the keystroke was sent, False when no
        session could be established.  It may be called synchronously (session
        already live) or later (after the consent prompt).
        """
        if self.state is InjectorState.READY:
            self._send_paste()
            if on_done is not None:
                on_done(True)
            return

        if self.state is InjectorState.DENIED:
            if on_done is not None:
                on_done(False)
            return

        self._pending.append(on_done)
        if self.state is InjectorState.IDLE:
            self._start_session()

    def stop(self) -> None:
        if self._session_handle:
            self._client.close_session(self._session_handle)
        self._session_handle = None
        self._client.close()
        self._pending.clear()
        self.state = InjectorState.IDLE

    # -- session handshake -------------------------------------------------

    def _set_state(self, state: InjectorState) -> None:
        if state is self.state:
            return
        self.state = state
        if self.on_state_changed is not None:
            self.on_state_changed(state)

    def _fail(self, reason: str) -> None:
        log.warning("Text injection unavailable: %s", reason)
        self._set_state(InjectorState.DENIED)
        self._drain(False)

    def _drain(self, ok: bool) -> None:
        pending, self._pending = self._pending, []
        for callback in pending:
            if callback is not None:
                callback(ok)

    def _start_session(self) -> None:
        if not self._client.connect():
            self._fail("no session bus")
            return
        self._set_state(InjectorState.STARTING)

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
            on_error=self._fail,
        )

    def _on_session_created(self, response: PortalResponse, results: GLib.Variant) -> None:
        if response is not PortalResponse.SUCCESS:
            self._fail(f"session not created ({response.display_name})")
            return
        self._session_handle = results.unpack().get("session_handle")
        if not self._session_handle:
            self._fail("portal returned no session_handle")
            return
        self._select_devices()

    def _select_devices(self) -> None:
        def build(handle_token: str) -> GLib.Variant:
            options = {
                "handle_token": GLib.Variant("s", handle_token),
                "types": GLib.Variant("u", _DEVICE_KEYBOARD),
            }
            return GLib.Variant("(oa{sv})", (self._session_handle, options))

        self._client.call_with_request(
            "SelectDevices", build, self._on_devices_selected, "selectdevices",
            on_error=self._fail,
        )

    def _on_devices_selected(self, response: PortalResponse, _results: GLib.Variant) -> None:
        if response is not PortalResponse.SUCCESS:
            self._fail(f"keyboard access {response.display_name}")
            return
        self._start()

    def _start(self) -> None:
        restore_token = ""
        if self._settings is not None:
            restore_token = self._settings.get(config.SETTING_INPUT_RESTORE_TOKEN, "")

        def build(handle_token: str) -> GLib.Variant:
            options = {
                "handle_token": GLib.Variant("s", handle_token),
                # Persist the grant so the consent prompt appears once, not on
                # every launch.
                "persist_mode": GLib.Variant("u", _PERSIST_PERSISTENT),
            }
            if restore_token:
                options["restore_token"] = GLib.Variant("s", restore_token)
            return GLib.Variant(
                "(osa{sv})", (self._session_handle, "", options)
            )

        log.info("Requesting permission to replace text (RemoteDesktop portal)…")
        self._client.call_with_request(
            "Start", build, self._on_started, "start", on_error=self._fail
        )

    def _on_started(self, response: PortalResponse, results: GLib.Variant) -> None:
        if response is not PortalResponse.SUCCESS:
            self._fail(f"permission {response.display_name}")
            return

        unpacked = results.unpack()
        token = unpacked.get("restore_token")
        if token and self._settings is not None:
            # Store it so the next launch skips the prompt.
            self._settings.set(config.SETTING_INPUT_RESTORE_TOKEN, token)

        self._set_state(InjectorState.READY)
        log.info("Text injection granted — Replace will work in Wayland apps.")
        # Replay whatever was waiting on the prompt.
        self._send_paste()
        self._drain(True)

    # -- keystroke ---------------------------------------------------------

    def _send_paste(self) -> None:
        """Press Ctrl, press V, release V, release Ctrl."""
        for keysym, state in (
            (_KEYSYM_CTRL_L, _KEY_PRESSED),
            (_KEYSYM_V, _KEY_PRESSED),
            (_KEYSYM_V, _KEY_RELEASED),
            (_KEYSYM_CTRL_L, _KEY_RELEASED),
        ):
            self._client.call_no_reply(
                "NotifyKeyboardKeysym",
                GLib.Variant(
                    "(oa{sv}iu)", (self._session_handle, {}, keysym, state)
                ),
            )
