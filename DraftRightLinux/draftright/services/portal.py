"""Shared plumbing for xdg-desktop-portal request/response calls.

Every portal method that needs user interaction answers asynchronously: the
call returns a handle, and the real answer arrives later as a ``Response``
signal on an ``org.freedesktop.portal.Request`` object.  Getting that right
involves one subtlety — the reply can land *before* you subscribe — so the
handle path is computed up front from our unique bus name and subscribed to
before the call is issued, exactly as the portal spec recommends.

Both portal users share this: the GlobalShortcuts hotkey (#99) and the
RemoteDesktop text injector.  Rule #1 — one implementation of the handshake,
not one per feature.
"""

from __future__ import annotations

import logging
from enum import Enum
from typing import Callable, Optional

from gi.repository import GLib, Gio  # type: ignore[attr-defined]

from draftright import config

log = logging.getLogger(__name__)


class PortalResponse(Enum):
    """``response`` code carried by ``org.freedesktop.portal.Request::Response``."""

    SUCCESS = 0
    CANCELLED = 1
    ENDED = 2

    @property
    def display_name(self) -> str:
        return {
            PortalResponse.SUCCESS: "granted",
            PortalResponse.CANCELLED: "dismissed by the user",
            PortalResponse.ENDED: "ended by the portal",
        }[self]

    @classmethod
    def from_wire(cls, value: int) -> "PortalResponse":
        for response in cls:
            if response.value == value:
                return response
        return cls.ENDED


class PortalClient:
    """Issues request/response calls against one portal interface."""

    def __init__(self, interface: str, name_prefix: str) -> None:
        """
        Args:
            interface: Full portal interface name, e.g.
                ``org.freedesktop.portal.GlobalShortcuts``.
            name_prefix: Short tag used to build unique handle tokens, so two
                portal clients in one process cannot collide.
        """
        self._interface = interface
        self._name_prefix = name_prefix
        self._serial = 0
        self.bus: Optional[Gio.DBusConnection] = None

    # -- connection --------------------------------------------------------

    def connect(self) -> bool:
        """Acquire the session bus.  False (logged) when unavailable."""
        try:
            self.bus = Gio.bus_get_sync(Gio.BusType.SESSION, None)
            return True
        except GLib.Error as exc:
            log.error("No session bus for %s: %s", self._interface, exc.message)
            self.bus = None
            return False

    def close(self) -> None:
        self.bus = None

    # -- tokens / paths ----------------------------------------------------

    def next_token(self, purpose: str) -> str:
        self._serial += 1
        return f"draftright_{self._name_prefix}_{purpose}_{self._serial}"

    def request_path(self, handle_token: str) -> str:
        """Predict the Request object path the portal will use."""
        sender = self.bus.get_unique_name().lstrip(":").replace(".", "_")
        return f"{config.PORTAL_OBJECT_PATH}/request/{sender}/{handle_token}"

    # -- calls -------------------------------------------------------------

    def subscribe_signal(
        self, signal: str, handler: Callable, path: str | None = None
    ) -> int:
        """Subscribe to a signal on this portal interface."""
        return self.bus.signal_subscribe(
            config.PORTAL_BUS_NAME,
            self._interface,
            signal,
            path or config.PORTAL_OBJECT_PATH,
            None,
            Gio.DBusSignalFlags.NONE,
            handler,
        )

    def unsubscribe(self, subscription_id: int) -> None:
        if self.bus is not None:
            self.bus.signal_unsubscribe(subscription_id)

    def call_with_request(
        self,
        method: str,
        build_params: Callable[[str], GLib.Variant],
        on_response: Callable[[PortalResponse, GLib.Variant], None],
        purpose: str,
        on_error: Callable[[str], None] | None = None,
    ) -> None:
        """Invoke *method*, delivering the eventual Response to *on_response*.

        *on_error* is called when the method itself fails, because no Response
        signal will ever arrive in that case and a caller waiting only on
        *on_response* would hang forever.
        """
        handle_token = self.next_token(purpose)
        subscription: list[int] = []

        def on_signal(_conn, _sender, _path, _iface, _signal, params):
            response = PortalResponse.from_wire(params.get_child_value(0).get_uint32())
            results = params.get_child_value(1)
            if subscription:
                self.unsubscribe(subscription[0])
            on_response(response, results)

        subscription.append(
            self.bus.signal_subscribe(
                config.PORTAL_BUS_NAME,
                config.PORTAL_REQUEST_IFACE,
                "Response",
                self.request_path(handle_token),
                None,
                Gio.DBusSignalFlags.NONE,
                on_signal,
            )
        )

        def on_call_done(bus, result):
            try:
                bus.call_finish(result)
            except GLib.Error as exc:
                if subscription:
                    self.unsubscribe(subscription[0])
                message = f"Portal {method} failed: {exc.message}"
                log.warning(message)
                if on_error is not None:
                    on_error(message)

        self.bus.call(
            config.PORTAL_BUS_NAME,
            config.PORTAL_OBJECT_PATH,
            self._interface,
            method,
            build_params(handle_token),
            None,
            Gio.DBusCallFlags.NONE,
            config.PORTAL_CALL_TIMEOUT_MS,
            None,
            on_call_done,
        )

    def call_no_reply(self, method: str, params: GLib.Variant) -> None:
        """Fire-and-forget call (portal methods that answer with no Request)."""
        if self.bus is None:
            return
        self.bus.call(
            config.PORTAL_BUS_NAME,
            config.PORTAL_OBJECT_PATH,
            self._interface,
            method,
            params,
            None,
            Gio.DBusCallFlags.NONE,
            config.PORTAL_CALL_TIMEOUT_MS,
            None,
            None,
        )

    def close_session(self, session_handle: str) -> None:
        """Best-effort ``Session.Close`` — the portal also drops it on disconnect."""
        if self.bus is None or not session_handle:
            return
        self.bus.call(
            config.PORTAL_BUS_NAME,
            session_handle,
            config.PORTAL_SESSION_IFACE,
            "Close",
            None,
            None,
            Gio.DBusCallFlags.NONE,
            config.PORTAL_CALL_TIMEOUT_MS,
            None,
            None,
        )
