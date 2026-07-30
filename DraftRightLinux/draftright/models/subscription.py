"""Subscription lifecycle state.

Mirrors the backend ``subscription.status`` values.  Replaces the raw
``"active"`` / ``"expired"`` / ``"cancelled"`` string comparisons in the
Subscription UI.  Rule #1: enums, not literals.  Pairs with
:class:`draftright.models.payment.BillingPeriod`.
"""

from __future__ import annotations

from enum import Enum


class SubscriptionStatus(Enum):
    """Status of the current user's subscription."""

    ACTIVE = "active"
    EXPIRED = "expired"
    CANCELLED = "cancelled"
    PAST_DUE = "past_due"

    @property
    def display_name(self) -> str:
        """User-facing English label."""
        return {
            SubscriptionStatus.ACTIVE: "Active",
            SubscriptionStatus.EXPIRED: "Expired",
            SubscriptionStatus.CANCELLED: "Cancelled",
            SubscriptionStatus.PAST_DUE: "Past due",
        }[self]

    @classmethod
    def from_wire(cls, value: str) -> "SubscriptionStatus":
        """Parse a wire value (case-insensitive); unknown → ACTIVE (backend
        default for a live plan)."""
        norm = (value or "").lower()
        for s in cls:
            if s.value == norm:
                return s
        return cls.ACTIVE
