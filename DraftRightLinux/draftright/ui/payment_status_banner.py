"""Compact live-status banner shown inside QR / bank-transfer dialogs.

Subscribes to a :class:`PaymentStatusStream` (polling
``/payment/status/:ref`` in a background thread) and updates a
single-line GTK widget with the current state.  On success it auto-
dismisses the parent dialog via the ``on_confirmed`` callback after
``auto_dismiss_seconds``.
"""

from __future__ import annotations

from typing import Callable, Optional

import gi
gi.require_version("Gtk", "4.0")
from gi.repository import GLib, Gtk

from draftright.models.payment import PaymentStatus, PaymentStatusUpdate
from draftright.services.payment_service import PaymentStatusStream


class PaymentStatusBanner(Gtk.Box):
    """Status banner wired to a :class:`PaymentStatusStream`."""

    def __init__(
        self,
        stream: Optional[PaymentStatusStream],
        on_confirmed: Callable[[], None],
        auto_dismiss_seconds: float = 2.0,
    ):
        super().__init__(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self._stream = stream
        self._on_confirmed = on_confirmed
        self._auto_dismiss_seconds = auto_dismiss_seconds
        self._dismiss_source: Optional[int] = None

        self.set_margin_top(6)
        self.set_margin_bottom(6)
        self.set_margin_start(10)
        self.set_margin_end(10)
        self.add_css_class("card")

        self._icon = Gtk.Label(label="⏳")
        self._text = Gtk.Label(label="Waiting for payment…", xalign=0)
        self._text.set_hexpand(True)
        self._text.set_wrap(True)
        self.append(self._icon)
        self.append(self._text)

        if stream is None:
            self.set_visible(False)
        else:
            self._apply_status(PaymentStatus.PENDING)
            stream.subscribe(self._on_update_from_thread)

    def cancel(self) -> None:
        """Stop polling; safe to call multiple times."""
        if self._stream is not None:
            self._stream.cancel()
            self._stream = None

    # ------------------------------------------------------------------
    # Background-thread callback → marshal to GTK main loop
    # ------------------------------------------------------------------

    def _on_update_from_thread(self, update: PaymentStatusUpdate) -> None:
        GLib.idle_add(self._on_update_on_main, update)

    def _on_update_on_main(self, update: PaymentStatusUpdate) -> bool:
        self._apply_status(update.status)
        if update.status.is_success() and self._dismiss_source is None:
            self._dismiss_source = GLib.timeout_add(
                int(self._auto_dismiss_seconds * 1000),
                self._fire_confirmed,
            )
        return False  # one-shot

    def _fire_confirmed(self) -> bool:
        self._dismiss_source = None
        try:
            self._on_confirmed()
        finally:
            self.cancel()
        return False

    # ------------------------------------------------------------------
    # Visual state
    # ------------------------------------------------------------------

    def _apply_status(self, status: PaymentStatus) -> None:
        if status in (PaymentStatus.PENDING, PaymentStatus.NOT_FOUND, PaymentStatus.UNKNOWN):
            self._icon.set_label("⏳")
            self._text.set_label("Waiting for payment…")
        elif status == PaymentStatus.COMPLETED:
            self._icon.set_label("✅")
            self._text.set_label("Payment confirmed!")
        elif status == PaymentStatus.FAILED:
            self._icon.set_label("❌")
            self._text.set_label("Payment failed. Please try again.")
        elif status == PaymentStatus.EXPIRED:
            self._icon.set_label("⌛")
            self._text.set_label(
                "Took too long to confirm. If you already paid, check Subscription in a minute."
            )
        elif status == PaymentStatus.REFUNDED:
            self._icon.set_label("↩")
            self._text.set_label("Refunded.")
