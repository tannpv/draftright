"""
Payment domain types for the DraftRight Linux app.

Mirrors the strategy pattern shipped on Flutter, macOS, and Windows
clients 1:1.  Same wire names → same backend handlers.  Adding a new
method = extend ``PaymentMethodKind`` + the helpers in this file.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Optional


class PaymentMethodKind(Enum):
    """Identity for one payment method advertised by GET /payment/methods."""

    LEMONSQUEEZY = "lemonsqueezy"
    STRIPE = "stripe"
    VIETQR = "vietqr"
    BANK_TRANSFER = "bank_transfer"
    PAYPAL = "paypal"
    APPLE_PAY = "apple_pay"
    GOOGLE_PAY = "google_pay"

    @classmethod
    def from_wire(cls, value: str) -> Optional["PaymentMethodKind"]:
        """Returns None for unknown strings (forward-compat with backend additions)."""
        for k in cls:
            if k.value == value:
                return k
        return None


class BillingPeriod(Enum):
    """Billing cadence supported by the backend ``plans.billing_period``
    column.  Mirrors ``BillingPeriod`` on Flutter / macOS / Windows
    (1:1, same wire names) so the UI never threads raw ``"monthly"`` /
    ``"yearly"`` strings through services.
    """

    MONTHLY = "monthly"
    YEARLY = "yearly"

    @property
    def display_name(self) -> str:
        """User-facing English label."""
        return "Monthly" if self == BillingPeriod.MONTHLY else "Yearly"

    @classmethod
    def from_wire(cls, value: Optional[str]) -> Optional["BillingPeriod"]:
        """Parse a wire value (case-insensitive).  None for unknown / Free."""
        if not value:
            return None
        norm = value.lower()
        for p in cls:
            if p.value == norm:
                return p
        return None


@dataclass(frozen=True)
class PaymentMethodDescriptor:
    """UI metadata for one :class:`PaymentMethodKind`."""

    kind: PaymentMethodKind
    display_name: str
    description: str
    icon_name: str  # GTK icon name (Adwaita / freedesktop)

    @classmethod
    def for_kind(cls, kind: PaymentMethodKind) -> "PaymentMethodDescriptor":
        if kind == PaymentMethodKind.LEMONSQUEEZY:
            return cls(kind, "Credit / Debit Card",
                       "Visa, Mastercard, and more via Lemon Squeezy",
                       "credit-card-symbolic")
        if kind == PaymentMethodKind.STRIPE:
            return cls(kind, "Stripe",
                       "Credit card via Stripe",
                       "credit-card-symbolic")
        if kind == PaymentMethodKind.VIETQR:
            return cls(kind, "VietQR (scan to pay)",
                       "Scan with any Vietnamese banking app — auto-confirms",
                       "qr-code-symbolic")
        if kind == PaymentMethodKind.BANK_TRANSFER:
            return cls(kind, "Bank Transfer",
                       "Manual transfer with reference code",
                       "money-symbolic")
        if kind == PaymentMethodKind.PAYPAL:
            return cls(kind, "PayPal",
                       "Pay with PayPal balance or card",
                       "wallet2-symbolic")
        if kind == PaymentMethodKind.APPLE_PAY:
            return cls(kind, "Apple Pay",
                       "Pay with Apple Pay (via Stripe)",
                       "wallet2-symbolic")
        if kind == PaymentMethodKind.GOOGLE_PAY:
            return cls(kind, "Google Pay",
                       "Pay with Google Pay (via Stripe)",
                       "wallet2-symbolic")
        raise ValueError(f"No descriptor for {kind!r}")


@dataclass(frozen=True)
class BankInfo:
    bank_name: str
    account_number: str
    account_name: str
    amount: float
    currency: str
    reference: str

    @classmethod
    def from_json(cls, data: dict) -> "BankInfo":
        return cls(
            bank_name=str(data.get("bank_name", "")),
            account_number=str(data.get("account_number", "")),
            account_name=str(data.get("account_name", "")),
            amount=float(data.get("amount", 0) or 0),
            currency=str(data.get("currency", "VND")),
            reference=str(data.get("reference", "")),
        )


class CheckoutResult:
    """Base class for the discriminated union returned by /payment/checkout.

    Subclasses: :class:`RedirectCheckout`, :class:`QrCheckout`,
    :class:`BankTransferCheckout`.  Decode with :meth:`from_json`.
    """

    def __init__(self, reference_code: str = ""):
        self.reference_code = reference_code

    @staticmethod
    def from_json(data: dict) -> "CheckoutResult":
        """Decode the backend envelope.

        Field priority matches the other clients:
        ``redirect_url`` > ``qr_data`` > ``bank_info``.
        """
        payment = data.get("payment") or {}
        ref = (payment.get("reference_code") if isinstance(payment, dict) else None) \
              or data.get("reference_code") or ""

        url = data.get("redirect_url")
        if isinstance(url, str) and url:
            return RedirectCheckout(reference_code=ref, url=url)

        bank_raw = data.get("bank_info")
        bank = BankInfo.from_json(bank_raw) if isinstance(bank_raw, dict) else None

        qr = data.get("qr_data")
        if isinstance(qr, str) and qr:
            return QrCheckout(reference_code=ref, image_url=qr, bank_info=bank)

        if bank is not None:
            return BankTransferCheckout(reference_code=ref, info=bank)

        raise ValueError(
            "Backend returned a checkout response with none of "
            "redirect_url / qr_data / bank_info"
        )


class RedirectCheckout(CheckoutResult):
    def __init__(self, reference_code: str, url: str):
        super().__init__(reference_code)
        self.url = url


class QrCheckout(CheckoutResult):
    def __init__(self, reference_code: str, image_url: str,
                 bank_info: Optional[BankInfo] = None):
        super().__init__(reference_code)
        self.image_url = image_url
        self.bank_info = bank_info


class BankTransferCheckout(CheckoutResult):
    def __init__(self, reference_code: str, info: BankInfo):
        super().__init__(reference_code)
        self.info = info


class PaymentStatus(Enum):
    """Lifecycle of a single payment.  Mirrors backend + the synthetic
    ``not_found`` envelope returned by /payment/status/:ref."""

    PENDING = "pending"
    COMPLETED = "completed"
    FAILED = "failed"
    EXPIRED = "expired"
    REFUNDED = "refunded"
    NOT_FOUND = "not_found"
    UNKNOWN = "unknown"

    @classmethod
    def from_wire(cls, value: str) -> "PaymentStatus":
        for s in cls:
            if s.value == value:
                return s
        return cls.UNKNOWN

    def is_terminal(self) -> bool:
        return self in {
            PaymentStatus.COMPLETED,
            PaymentStatus.FAILED,
            PaymentStatus.EXPIRED,
            PaymentStatus.REFUNDED,
        }

    def is_success(self) -> bool:
        return self == PaymentStatus.COMPLETED


@dataclass(frozen=True)
class PaymentStatusUpdate:
    reference_code: str
    status: PaymentStatus
    amount: Optional[float] = None
    currency: Optional[str] = None
    plan_name: Optional[str] = None

    @classmethod
    def from_json(cls, data: dict) -> "PaymentStatusUpdate":
        return cls(
            reference_code=str(data.get("reference_code", "")),
            status=PaymentStatus.from_wire(str(data.get("status", ""))),
            amount=data.get("amount"),
            currency=data.get("currency"),
            plan_name=data.get("plan_name"),
        )
