"""GTK-free unit tests for the config + Rule #1 refactor.

Runnable without a display / GTK:  python3 -m unittest discover tests
Covers the invariants the refactor introduced — enum single-source, env
override, and the clipboard inject_text delegation.
"""

import os
import unittest
from unittest import mock

from draftright import config
from draftright.models.tone import Tone
from draftright.models.payment import BillingPeriod
from draftright.models.subscription import SubscriptionStatus
from draftright.models.health import HealthStatus
from draftright.services.settings_service import SettingsService
from draftright.services.clipboard_service import ClipboardService
from draftright.services.rewrite_cache import RewriteCache


class ConfigBackendUrl(unittest.TestCase):
    def test_default_when_env_unset(self):
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop(config.BACKEND_ENV_VAR, None)
            self.assertEqual(config.default_backend_url(), config.DEFAULT_BACKEND_URL)

    def test_env_override_wins(self):
        with mock.patch.dict(os.environ, {config.BACKEND_ENV_VAR: "http://dev.local"}):
            self.assertEqual(config.default_backend_url(), "http://dev.local")

    def test_blank_env_ignored(self):
        with mock.patch.dict(os.environ, {config.BACKEND_ENV_VAR: "   "}):
            self.assertEqual(config.default_backend_url(), config.DEFAULT_BACKEND_URL)


class ToneSingleSource(unittest.TestCase):
    def test_settings_default_tones_match_enum(self):
        # Rule #1: enabled_tones must derive from the Tone enum, not a
        # hand-maintained literal list.
        s = SettingsService()
        self.assertEqual(s.enabled_tones, [t.api_value for t in Tone])

    def test_settings_backend_env_override(self):
        with mock.patch.dict(os.environ, {config.BACKEND_ENV_VAR: "http://dev.local"}):
            self.assertEqual(SettingsService().backend_url, "http://dev.local")


class ClipboardInjectText(unittest.TestCase):
    def test_inject_sets_clipboard_then_pastes(self):
        c = ClipboardService()
        c.set_clipboard = mock.Mock()
        c._sim = mock.Mock()
        c._sim.paste.return_value = True
        c.inject_text("hello world")
        c.set_clipboard.assert_called_once_with("hello world")
        c._sim.paste.assert_called_once()
        c._sim.type_text.assert_not_called()

    def test_inject_falls_back_to_type_when_paste_unavailable(self):
        c = ClipboardService()
        c.set_clipboard = mock.Mock()
        c._sim = mock.Mock()
        c._sim.paste.return_value = False  # no paste tool
        c.inject_text("hi")
        c._sim.type_text.assert_called_once_with("hi")


class EnumDispatch(unittest.TestCase):
    """Rule #1 round 2 (#106): status/billing/health derive from enums,
    not raw wire-string if/elif chains."""

    def test_billing_period_from_wire(self):
        self.assertIs(BillingPeriod.from_wire("monthly"), BillingPeriod.MONTHLY)
        self.assertIs(BillingPeriod.from_wire("YEARLY"), BillingPeriod.YEARLY)
        self.assertEqual(BillingPeriod.MONTHLY.display_name, "Monthly")
        # Free / unknown / absent → None (drives is_free + paid-plan filter)
        for v in ("", "none", None, "weekly"):
            self.assertIsNone(BillingPeriod.from_wire(v))

    def test_subscription_status_from_wire(self):
        self.assertIs(SubscriptionStatus.from_wire("active"), SubscriptionStatus.ACTIVE)
        self.assertIs(SubscriptionStatus.from_wire("Cancelled"), SubscriptionStatus.CANCELLED)
        self.assertEqual(SubscriptionStatus.EXPIRED.display_name, "Expired")
        # Unknown / blank → ACTIVE (backend default for a live plan)
        self.assertIs(SubscriptionStatus.from_wire(""), SubscriptionStatus.ACTIVE)
        self.assertIs(SubscriptionStatus.from_wire("bogus"), SubscriptionStatus.ACTIVE)

    def test_health_status_from_wire(self):
        self.assertIs(HealthStatus.from_wire("connected"), HealthStatus.CONNECTED)
        self.assertIs(HealthStatus.from_wire("not_logged_in"), HealthStatus.NOT_LOGGED_IN)
        self.assertIs(HealthStatus.from_wire("wrong_server"), HealthStatus.WRONG_SERVER)
        self.assertEqual(HealthStatus.CONNECTED.display_name, "Connected")
        # Unknown → OFFLINE (safe default)
        self.assertIs(HealthStatus.from_wire("garbage"), HealthStatus.OFFLINE)


class RewriteCacheTest(unittest.TestCase):
    """#108: client-side rewrite cache (pure logic, mirrors macOS)."""

    def test_hit_and_miss_keyed_by_text_and_tone(self):
        c = RewriteCache(max_entries=10)
        self.assertIsNone(c.get("hello", "polished"))
        c.set("hello", "polished", "Hello.")
        self.assertEqual(c.get("hello", "polished"), "Hello.")
        # Same text, different tone → separate entry (miss).
        self.assertIsNone(c.get("hello", "concise"))

    def test_bounded_eviction_drops_oldest(self):
        c = RewriteCache(max_entries=4, evict_fraction=4)  # evict 1 when full
        for i in range(4):
            c.set(f"t{i}", "polished", f"r{i}")
        self.assertEqual(len(c), 4)
        c.set("t4", "polished", "r4")  # over cap → evict oldest (t0)
        self.assertIsNone(c.get("t0", "polished"))
        self.assertEqual(c.get("t4", "polished"), "r4")
        self.assertLessEqual(len(c), 4)

    def test_clear(self):
        c = RewriteCache()
        c.set("a", "natural", "A")
        c.clear()
        self.assertIsNone(c.get("a", "natural"))


if __name__ == "__main__":
    unittest.main()
