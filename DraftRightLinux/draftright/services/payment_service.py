"""
Payment subsystem orchestrator for the DraftRight Linux app.

Mirrors the strategy pattern shipped on Flutter, macOS, and Windows:
the UI calls :meth:`PaymentService.upgrade` with a
:class:`PaymentMethodKind` and a presenter, and the right handler runs
the post-checkout UX (open browser, show QR dialog, show bank dialog).
"""

from __future__ import annotations

import subprocess
import threading
import time
from typing import Callable, Optional, Protocol

from draftright.models.payment import (
    BankInfo,
    BankTransferCheckout,
    BillingPeriod,
    CheckoutResult,
    PaymentMethodKind,
    PaymentStatus,
    PaymentStatusUpdate,
    QrCheckout,
    RedirectCheckout,
)
from draftright.services.api_client import APIClient, APIError


class PaymentSheetPresenter(Protocol):
    """Lightweight binding the orchestrator uses to ask the UI to
    show a QR / bank-transfer dialog without depending on Gtk types."""

    def present_qr_dialog(
        self,
        checkout: QrCheckout,
        status_stream: "PaymentStatusStream",
    ) -> None: ...

    def present_bank_transfer_dialog(
        self,
        checkout: BankTransferCheckout,
        status_stream: "PaymentStatusStream",
    ) -> None: ...


class PaymentHandler(Protocol):
    """Post-checkout UX for one :class:`PaymentMethodKind`."""

    @property
    def kind(self) -> PaymentMethodKind: ...

    def handle(self, result: CheckoutResult, presenter: PaymentSheetPresenter) -> None: ...


class RedirectPaymentHandler:
    """Opens ``result.url`` in the default browser via ``xdg-open``.

    Works under both X11 and Wayland; xdg-open delegates to the
    user's chosen browser, so saved cards / cookies carry over.
    """

    def __init__(self, kind: PaymentMethodKind):
        self._kind = kind

    @property
    def kind(self) -> PaymentMethodKind:
        return self._kind

    def handle(self, result: CheckoutResult, presenter: PaymentSheetPresenter) -> None:
        if not isinstance(result, RedirectCheckout):
            raise RuntimeError(
                f"RedirectPaymentHandler received non-redirect result: {type(result).__name__}"
            )
        # subprocess.Popen so we don't block the main loop on xdg-open.
        subprocess.Popen(
            ["xdg-open", result.url],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )


class QrPaymentHandler:
    def __init__(self, status_watcher: Callable[[str], "PaymentStatusStream"]):
        self._watcher = status_watcher

    @property
    def kind(self) -> PaymentMethodKind:
        return PaymentMethodKind.VIETQR

    def handle(self, result: CheckoutResult, presenter: PaymentSheetPresenter) -> None:
        if not isinstance(result, QrCheckout):
            raise RuntimeError(
                f"QrPaymentHandler received non-qr result: {type(result).__name__}"
            )
        presenter.present_qr_dialog(result, self._watcher(result.reference_code))


class BankTransferPaymentHandler:
    def __init__(self, status_watcher: Callable[[str], "PaymentStatusStream"]):
        self._watcher = status_watcher

    @property
    def kind(self) -> PaymentMethodKind:
        return PaymentMethodKind.BANK_TRANSFER

    def handle(self, result: CheckoutResult, presenter: PaymentSheetPresenter) -> None:
        if not isinstance(result, BankTransferCheckout):
            raise RuntimeError(
                f"BankTransferPaymentHandler received non-bank result: {type(result).__name__}"
            )
        presenter.present_bank_transfer_dialog(result, self._watcher(result.reference_code))


class PaymentStatusStream:
    """Foreground poller for /payment/status/:ref.

    Runs a daemon thread that calls the backend every ``interval``
    seconds until a terminal status arrives, ``timeout`` elapses, or
    :meth:`cancel` is called.  Consumers register a callback via
    :meth:`subscribe`; callbacks are invoked on the polling thread —
    GTK consumers must marshal updates back to the main loop via
    ``GLib.idle_add``.
    """

    def __init__(
        self,
        api: APIClient,
        reference_code: str,
        interval: float = 3.0,
        timeout: float = 60 * 15.0,
    ):
        self._api = api
        self._ref = reference_code
        self._interval = interval
        self._timeout = timeout
        self._cancelled = threading.Event()
        self._callback: Optional[Callable[[PaymentStatusUpdate], None]] = None
        self._thread: Optional[threading.Thread] = None

    def subscribe(self, callback: Callable[[PaymentStatusUpdate], None]) -> None:
        """Start polling and route every update to ``callback``."""
        if self._thread is not None:
            return  # Already started; one subscriber per stream.
        self._callback = callback
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def cancel(self) -> None:
        """Stop polling.  Idempotent."""
        self._cancelled.set()

    def _run(self) -> None:
        deadline = time.monotonic() + self._timeout
        cb = self._callback
        if cb is None:
            return
        while not self._cancelled.is_set() and time.monotonic() < deadline:
            try:
                raw = self._api.get_payment_status(self._ref)
                update = PaymentStatusUpdate.from_json(raw)
                cb(update)
                if update.status.is_terminal():
                    return
            except APIError:
                # Transient error — keep polling; log levels handled
                # by api_client.
                pass
            # Interruptible sleep so cancel() takes effect fast.
            self._cancelled.wait(timeout=self._interval)

        if not self._cancelled.is_set():
            # Deadline reached without a terminal status — emit a
            # synthetic expired update so the UI can show
            # "took too long, try again".
            cb(PaymentStatusUpdate(
                reference_code=self._ref,
                status=PaymentStatus.EXPIRED,
            ))


class PaymentService:
    """Top-level orchestrator the UI calls."""

    def __init__(self, api: APIClient):
        self.api = api
        self._handlers: dict[PaymentMethodKind, PaymentHandler] = {}
        self._register_default_handlers()

    def _register_default_handlers(self) -> None:
        watch = self._watch_payment
        for handler in [
            RedirectPaymentHandler(PaymentMethodKind.LEMONSQUEEZY),
            RedirectPaymentHandler(PaymentMethodKind.STRIPE),
            RedirectPaymentHandler(PaymentMethodKind.PAYPAL),
            QrPaymentHandler(watch),
            BankTransferPaymentHandler(watch),
        ]:
            self._handlers[handler.kind] = handler

    def register(self, handler: PaymentHandler) -> None:
        """Override the handler for one kind (tests + future overrides)."""
        self._handlers[handler.kind] = handler

    # --- Public API ---

    def list_available_methods(self) -> list[PaymentMethodKind]:
        """Backend-enabled methods this client understands.  No Apple
        policy gate on Linux — show everything."""
        raw = self.api.list_payment_methods()
        kinds: list[PaymentMethodKind] = []
        for value in raw:
            k = PaymentMethodKind.from_wire(value)
            if k is not None:
                kinds.append(k)
        return kinds

    def resolve_pro_plan_id(
        self,
        method: Optional[PaymentMethodKind] = None,
        billing_period: Optional[BillingPeriod] = None,
    ) -> str:
        """Pick the Pro-tier plan id from /plans for the requested
        ``method`` + ``billing_period``.

          - Currency-aware so VietQR doesn't pick a USD plan (the QR
            would bake ``"$4.99 đồng"`` — useless).
          - Cadence-aware so the Monthly / Yearly toggle on the
            Subscription page charges the matching variant.

        Mirrors ``resolveProPlanId`` on Flutter / macOS / Windows.
        """
        plans = self.api.list_plans()
        want_currency = self.currency_for(method) if method is not None else None

        def matches(p: dict) -> bool:
            bp = str(p.get("billing_period", "")).lower()
            active = p.get("is_active") if "is_active" in p else True
            if not bp or bp == "none" or not active:
                return False
            if want_currency is not None:
                cur = str(p.get("currency", "")).upper()
                if cur != want_currency.upper():
                    return False
            return True

        paid = [p for p in plans if isinstance(p, dict) and matches(p)]
        if not paid:
            raise APIError(
                f"Could not find a Pro plan in {want_currency} for {method}"
                if want_currency is not None
                else "Could not find a Pro plan in the catalog"
            )
        if billing_period is not None:
            for p in paid:
                if BillingPeriod.from_wire(p.get("billing_period")) is billing_period:
                    pid = str(p.get("id", ""))
                    if pid:
                        return pid
        # No exact cadence match (or none requested) — fall back to
        # monthly, then the first paid plan.
        monthly = next(
            (p for p in paid if BillingPeriod.from_wire(p.get("billing_period")) is BillingPeriod.MONTHLY),
            paid[0],
        )
        pid = str(monthly.get("id", ""))
        if not pid:
            raise APIError("Pro plan row is missing an id")
        return pid

    @staticmethod
    def currency_for(method: PaymentMethodKind) -> str:
        """Currency the strategy expects to charge the plan in.  VietQR
        + bank-transfer settle in VND (Vietnamese-bank-only spec); all
        others default to USD.  Mirrors ``_currencyFor`` on Flutter."""
        if method in (PaymentMethodKind.VIETQR, PaymentMethodKind.BANK_TRANSFER):
            return "VND"
        return "USD"

    def upgrade(
        self,
        method: PaymentMethodKind,
        plan_id: str,
        presenter: PaymentSheetPresenter,
    ) -> None:
        """Run the full upgrade flow for ``method``."""
        handler = self._handlers.get(method)
        if handler is None:
            raise APIError(f"No handler registered for {method}")
        raw = self.api.create_checkout(plan_id, method.value)
        result = CheckoutResult.from_json(raw)
        handler.handle(result, presenter)

    def open_customer_portal(self) -> None:
        """Open the LS Customer Portal in the default browser."""
        url = self.api.get_customer_portal_url()
        subprocess.Popen(
            ["xdg-open", url],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

    # --- Status poller factory ---

    def _watch_payment(self, reference_code: str) -> PaymentStatusStream:
        return PaymentStatusStream(self.api, reference_code)
