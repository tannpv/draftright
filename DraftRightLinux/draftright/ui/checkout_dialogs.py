"""Modal dialogs shown for the VietQR + bank-transfer payment flows.

Both dialogs embed a :class:`PaymentStatusBanner` that auto-dismisses
the dialog the moment the server-side webhook flips the payment to
``completed``.
"""

from __future__ import annotations

import io
import threading
from typing import Optional

import gi
import requests
gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")
gi.require_version("GdkPixbuf", "2.0")
from gi.repository import Gdk, GdkPixbuf, GLib, Gtk

from draftright.models.payment import BankInfo, BankTransferCheckout, QrCheckout
from draftright.services.payment_service import PaymentStatusStream
from draftright.ui.payment_status_banner import PaymentStatusBanner


class QrCheckoutDialog(Gtk.Window):
    """Dialog rendering the VietQR image + manual fallback fields."""

    def __init__(self, parent: Gtk.Window, checkout: QrCheckout,
                 status_stream: Optional[PaymentStatusStream]):
        super().__init__(modal=True, transient_for=parent, title="Scan to pay")
        self.set_default_size(420, 620)

        outer = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        outer.set_margin_top(16)
        outer.set_margin_bottom(16)
        outer.set_margin_start(20)
        outer.set_margin_end(20)
        self.set_child(outer)

        self._banner = PaymentStatusBanner(status_stream, on_confirmed=self._close_on_success)
        outer.append(self._banner)

        outer.append(_make_heading("Scan to pay"))
        outer.append(_make_blurb(
            "Open your banking app and scan this QR code.  "
            "Your plan activates automatically after payment."
        ))

        self._qr_image = Gtk.Picture()
        self._qr_image.set_size_request(260, 260)
        self._qr_image.set_can_shrink(False)
        outer.append(self._qr_image)
        threading.Thread(target=self._load_qr_async, args=(checkout.image_url,),
                          daemon=True).start()

        if checkout.bank_info is not None:
            outer.append(_make_divider("Or transfer manually"))
            _append_bank_table(outer, checkout.bank_info)

        close_btn = Gtk.Button(label="Close")
        close_btn.set_halign(Gtk.Align.END)
        close_btn.connect("clicked", lambda _: self.close())
        outer.append(close_btn)

        self.connect("close-request", self._on_close_request)

    def _load_qr_async(self, url: str) -> None:
        try:
            data = requests.get(url, timeout=15).content
            loader = GdkPixbuf.PixbufLoader.new()
            loader.write(data)
            loader.close()
            pixbuf = loader.get_pixbuf()
            if pixbuf is None:
                return
            texture = Gdk.Texture.new_for_pixbuf(pixbuf)
            GLib.idle_add(self._apply_qr_texture, texture)
        except Exception:  # noqa: BLE001 — fallback handled in UI
            GLib.idle_add(self._show_qr_error)

    def _apply_qr_texture(self, texture: Gdk.Texture) -> bool:
        self._qr_image.set_paintable(texture)
        return False

    def _show_qr_error(self) -> bool:
        label = Gtk.Label(label="Could not load QR.\nUse manual transfer below.")
        self._qr_image.set_paintable(None)
        return False

    def _close_on_success(self) -> None:
        self.close()

    def _on_close_request(self, *_args) -> bool:
        self._banner.cancel()
        return False


class BankTransferDialog(Gtk.Window):
    """Dialog rendering bank-transfer account fields."""

    def __init__(self, parent: Gtk.Window, checkout: BankTransferCheckout,
                 status_stream: Optional[PaymentStatusStream]):
        super().__init__(modal=True, transient_for=parent, title="Bank transfer")
        self.set_default_size(420, 380)

        outer = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        outer.set_margin_top(16)
        outer.set_margin_bottom(16)
        outer.set_margin_start(20)
        outer.set_margin_end(20)
        self.set_child(outer)

        self._banner = PaymentStatusBanner(status_stream, on_confirmed=self.close)
        outer.append(self._banner)

        outer.append(_make_heading("Bank transfer"))
        outer.append(_make_blurb(
            "Transfer this exact amount from any Vietnamese bank.  "
            "The reference code links the payment to your account; "
            "your plan activates automatically once received."
        ))

        _append_bank_table(outer, checkout.info)

        close_btn = Gtk.Button(label="Close")
        close_btn.set_halign(Gtk.Align.END)
        close_btn.connect("clicked", lambda _: self.close())
        outer.append(close_btn)

        self.connect("close-request", self._on_close_request)

    def _on_close_request(self, *_args) -> bool:
        self._banner.cancel()
        return False


# ----------------------------------------------------------------------
# Shared widget helpers
# ----------------------------------------------------------------------

def _make_heading(text: str) -> Gtk.Widget:
    label = Gtk.Label(label=text, xalign=0)
    label.add_css_class("title-2")
    return label


def _make_blurb(text: str) -> Gtk.Widget:
    label = Gtk.Label(label=text, xalign=0)
    label.set_wrap(True)
    label.add_css_class("dim-label")
    return label


def _make_divider(text: str) -> Gtk.Widget:
    box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
    box.append(Gtk.Separator(orientation=Gtk.Orientation.HORIZONTAL, hexpand=True))
    box.append(Gtk.Label(label=text))
    box.append(Gtk.Separator(orientation=Gtk.Orientation.HORIZONTAL, hexpand=True))
    return box


def _append_bank_table(outer: Gtk.Box, info: BankInfo) -> None:
    grid = Gtk.Grid(row_spacing=6, column_spacing=12)
    grid.attach(_field_label("Bank"),      0, 0, 1, 1)
    grid.attach(_value_label(info.bank_name), 1, 0, 1, 1)
    _add_copy_row(grid, 1, "Account #", info.account_number)
    grid.attach(_field_label("Name"),      0, 2, 1, 1)
    grid.attach(_value_label(info.account_name), 1, 2, 1, 1)
    _add_copy_row(grid, 3, "Amount", f"{_amount(info.amount)} {info.currency}")
    _add_copy_row(grid, 4, "Reference", info.reference,
                  hint="Must include this in the transfer description.")
    outer.append(grid)


def _add_copy_row(grid: Gtk.Grid, row: int, label: str, value: str,
                  hint: Optional[str] = None) -> None:
    grid.attach(_field_label(label), 0, row, 1, 1)
    val = _value_label(value)
    grid.attach(val, 1, row, 1, 1)
    btn = Gtk.Button(label="Copy")
    btn.add_css_class("flat")

    def _on_copy(_b):
        display = Gdk.Display.get_default()
        if display is not None:
            display.get_clipboard().set(value)

    btn.connect("clicked", _on_copy)
    grid.attach(btn, 2, row, 1, 1)
    if hint:
        hint_lbl = Gtk.Label(label=hint, xalign=0)
        hint_lbl.add_css_class("dim-label")
        hint_lbl.add_css_class("caption")
        grid.attach(hint_lbl, 1, row + 1, 2, 1)


def _field_label(text: str) -> Gtk.Widget:
    label = Gtk.Label(label=text, xalign=0)
    label.add_css_class("dim-label")
    return label


def _value_label(text: str) -> Gtk.Widget:
    label = Gtk.Label(label=text, xalign=0, selectable=True)
    label.add_css_class("monospace")
    return label


def _amount(value: float) -> str:
    return str(int(value)) if value == int(value) else f"{value:.2f}"
