"""Take a screenshot via ``org.freedesktop.portal.Screenshot`` (#85).

Wayland forbids a client from grabbing the screen itself, so the portal is
the only sanctioned route — and it is the better one anyway: the compositor
runs its own picker, so the user chooses area / window / whole screen and
explicitly consents to what is captured.

Reuses :class:`draftright.services.portal.PortalClient`, the same
request/response plumbing behind the global shortcut and text injection.
"""

from __future__ import annotations

import logging
from typing import Callable
from urllib.parse import unquote, urlparse

from gi.repository import GLib  # type: ignore[attr-defined]

from draftright import config
from draftright.services.portal import PortalClient, PortalResponse

log = logging.getLogger(__name__)


class ScreenshotService:
    """One-shot screen capture through the desktop portal."""

    def __init__(self) -> None:
        self._client = PortalClient(
            config.PORTAL_SCREENSHOT_IFACE, name_prefix="screenshot"
        )

    def capture(
        self,
        on_done: Callable[[str | None, str | None], None],
        interactive: bool = True,
    ) -> None:
        """Capture a screenshot.

        Args:
            on_done: Called as ``(path, error)`` — exactly one is non-None.
                May run well after this returns; the portal shows a picker and
                waits for the user.
            interactive: Let the user choose area/window in the compositor's
                own UI. False grabs the whole screen immediately, which some
                compositors refuse without a prior grant.
        """
        if not self._client.connect():
            on_done(None, "No session bus — cannot reach the screenshot portal.")
            return

        def build(handle_token: str) -> GLib.Variant:
            options = {
                "handle_token": GLib.Variant("s", handle_token),
                "interactive": GLib.Variant("b", interactive),
            }
            # parent_window is a window identifier; "" is valid and lets the
            # portal parent to the active window itself.
            return GLib.Variant("(sa{sv})", ("", options))

        def on_response(response: PortalResponse, results: GLib.Variant) -> None:
            if response is not PortalResponse.SUCCESS:
                # Cancelling is the common case and is not an error worth
                # shouting about.
                on_done(None, None if response is PortalResponse.CANCELLED
                        else f"Screenshot {response.display_name}.")
                return
            uri = results.unpack().get("uri")
            if not uri:
                on_done(None, "The portal returned no screenshot.")
                return
            on_done(uri_to_path(uri), None)

        self._client.call_with_request(
            "Screenshot", build, on_response, "capture",
            on_error=lambda message: on_done(None, message),
        )


def uri_to_path(uri: str) -> str:
    """Convert a ``file://`` URI to a local path, decoding percent-escapes."""
    parsed = urlparse(uri)
    if parsed.scheme and parsed.scheme != "file":
        return uri  # not a local file; hand it back untouched
    return unquote(parsed.path or uri)
