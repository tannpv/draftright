"""Adw.PreferencesPage that renders the Subscription tab.

Behaviour mirrors the Flutter / macOS / Windows clients:
- shows plan + status + usage for the current user,
- for free users renders a method-picker that drives the upgrade
  flow via :class:`PaymentService`,
- for paid users shows a "Manage subscription" button that opens
  the Lemon Squeezy Customer Portal.
"""

from __future__ import annotations

import threading
from typing import Optional

import gi
gi.require_version("Adw", "1")
gi.require_version("Gtk", "4.0")
from gi.repository import Adw, GLib, Gtk

from draftright.models.payment import (
    BankTransferCheckout,
    PaymentMethodDescriptor,
    PaymentMethodKind,
    QrCheckout,
)
from draftright.services.api_client import APIError
from draftright.services.payment_service import (
    PaymentService,
    PaymentSheetPresenter,
    PaymentStatusStream,
)
from draftright.ui.checkout_dialogs import BankTransferDialog, QrCheckoutDialog


class SubscriptionPage(Adw.PreferencesPage, PaymentSheetPresenter):
    """The Subscription preferences page."""

    def __init__(self, app, api_client):
        super().__init__(title="Subscription", icon_name="credit-card-symbolic")
        self._app = app
        self._api = api_client
        # PaymentService accepts an APIClient; we only need the
        # orchestrator when api_client exists.  Build a placeholder
        # when None so refresh() can short-circuit without instance
        # checks scattered through the rest of the class.
        if api_client is not None:
            self._payments = PaymentService(api_client)
        else:
            self._payments = None  # type: ignore[assignment]

        self._info_group = Adw.PreferencesGroup(title="Current plan")
        self.add(self._info_group)
        self._info_status = Gtk.Label(label="Loading…", xalign=0)
        self._info_group.add(self._info_status)

        self._actions_group = Adw.PreferencesGroup()
        self.add(self._actions_group)
        self._actions_placeholder = Gtk.Label(label="", xalign=0)
        self._actions_group.add(self._actions_placeholder)

        self.connect("realize", lambda _w: self.refresh())

    # ------------------------------------------------------------------
    # Refresh
    # ------------------------------------------------------------------

    def refresh(self) -> None:
        """Load /subscription + /payment/methods on a worker thread."""
        if self._api is None:
            # The application bootstraps its services lazily; on first
            # show the api_client may not be ready yet.  Show a hint
            # instead of crashing so the user can sign in on the
            # Account tab and come back.
            self._set_info_text("Not signed in.  Use the Account tab first.")
            return
        self._set_info_text("Loading…")
        self._clear_actions()

        def worker():
            try:
                sub = self._api.get_subscription()
                methods = self._payments.list_available_methods()
                GLib.idle_add(self._apply, sub, methods)
            except Exception as e:  # noqa: BLE001
                GLib.idle_add(self._set_info_text, f"Error: {e}")

        threading.Thread(target=worker, daemon=True).start()

    def _apply(self, sub: dict, methods: list[PaymentMethodKind]) -> bool:
        plan = sub.get("plan") if isinstance(sub.get("plan"), dict) else {}
        plan_name = plan.get("name", "Free")
        billing = (plan.get("billing_period") or "none").lower()
        is_free = billing in ("", "none")
        status = sub.get("status", "active")
        usage = sub.get("usage_today", 0)
        daily_limit = plan.get("daily_limit", 10)

        text = (
            f"Plan: {plan_name}\n"
            f"Billing: {_billing_label(billing)}\n"
            f"Status: {_status_label(status)}\n"
            f"Usage today: {usage} / {daily_limit}"
        )
        self._set_info_text(text)

        self._clear_actions()
        if is_free:
            self._render_method_picker(methods)
        else:
            self._render_manage_button()
        return False

    def _set_info_text(self, text: str) -> None:
        self._info_status.set_label(text)

    def _clear_actions(self) -> None:
        # Adw.PreferencesGroup doesn't support remove_all() before
        # libadwaita 1.4, so rebuild the group on every render.
        # Cheaper than maintaining diffs, runs once per page activation.
        child = self._actions_group.get_first_child()
        while child is not None:
            next_child = child.get_next_sibling()
            self._actions_group.remove(child)
            child = next_child

    # ------------------------------------------------------------------
    # Upgrade — method picker
    # ------------------------------------------------------------------

    def _render_method_picker(self, methods: list[PaymentMethodKind]) -> None:
        self._actions_group.set_title("Upgrade to Pro")
        self._actions_group.set_description(
            "Pick a payment method.  Your plan activates automatically once payment completes."
        )

        if not methods:
            self._actions_group.add(Gtk.Label(
                label="No payment methods are enabled yet. Please check back later.",
                xalign=0,
            ))
            return

        for kind in methods:
            self._actions_group.add(self._build_method_row(kind))

    def _build_method_row(self, kind: PaymentMethodKind) -> Gtk.Widget:
        d = PaymentMethodDescriptor.for_kind(kind)
        row = Adw.ActionRow(title=d.display_name, subtitle=d.description)
        row.set_activatable(True)
        row.add_prefix(Gtk.Image(icon_name=d.icon_name))
        chevron = Gtk.Image(icon_name="go-next-symbolic")
        row.add_suffix(chevron)
        row.connect("activated", lambda _r, k=kind: self._on_method_selected(k))
        return row

    def _on_method_selected(self, kind: PaymentMethodKind) -> None:
        def worker():
            try:
                plan_id = self._payments.resolve_pro_plan_id()
                self._payments.upgrade(kind, plan_id, self)
            except APIError as e:
                GLib.idle_add(self._show_error, str(e))

        threading.Thread(target=worker, daemon=True).start()

    # ------------------------------------------------------------------
    # Manage subscription
    # ------------------------------------------------------------------

    def _render_manage_button(self) -> None:
        self._actions_group.set_title("Manage subscription")
        self._actions_group.set_description(
            "Cancel, change plan, or update your payment method."
        )
        btn = Gtk.Button(label="Manage subscription")
        btn.add_css_class("pill")
        btn.add_css_class("suggested-action")
        btn.set_halign(Gtk.Align.START)
        btn.connect("clicked", self._on_manage_clicked)
        self._actions_group.add(btn)

    def _on_manage_clicked(self, _btn) -> None:
        def worker():
            try:
                self._payments.open_customer_portal()
            except APIError as e:
                GLib.idle_add(self._show_error, str(e))

        threading.Thread(target=worker, daemon=True).start()

    # ------------------------------------------------------------------
    # PaymentSheetPresenter — called from worker threads after the
    # backend returns a typed CheckoutResult.
    # ------------------------------------------------------------------

    def present_qr_dialog(
        self,
        checkout: QrCheckout,
        status_stream: Optional[PaymentStatusStream],
    ) -> None:
        GLib.idle_add(self._present_qr, checkout, status_stream)

    def present_bank_transfer_dialog(
        self,
        checkout: BankTransferCheckout,
        status_stream: Optional[PaymentStatusStream],
    ) -> None:
        GLib.idle_add(self._present_bank, checkout, status_stream)

    def _present_qr(self, checkout: QrCheckout,
                    status_stream: Optional[PaymentStatusStream]) -> bool:
        parent = self.get_root() if hasattr(self, "get_root") else None
        dlg = QrCheckoutDialog(parent, checkout, status_stream)
        dlg.connect("close-request", lambda *_: self.refresh() or False)
        dlg.present()
        return False

    def _present_bank(self, checkout: BankTransferCheckout,
                       status_stream: Optional[PaymentStatusStream]) -> bool:
        parent = self.get_root() if hasattr(self, "get_root") else None
        dlg = BankTransferDialog(parent, checkout, status_stream)
        dlg.connect("close-request", lambda *_: self.refresh() or False)
        dlg.present()
        return False

    # ------------------------------------------------------------------
    # Error surfacing
    # ------------------------------------------------------------------

    def _show_error(self, message: str) -> bool:
        # Re-use the info label since we don't have a Toast container here.
        self._set_info_text(f"Error: {message}")
        return False


def _billing_label(b: str) -> str:
    if b in ("", "none"):
        return "Free"
    if b == "monthly":
        return "Monthly"
    if b == "yearly":
        return "Yearly"
    return b


def _status_label(s: str) -> str:
    if s == "active":
        return "Active"
    if s == "expired":
        return "Expired"
    if s == "cancelled":
        return "Cancelled"
    return s
