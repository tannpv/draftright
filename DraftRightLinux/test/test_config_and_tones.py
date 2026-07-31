"""GTK-free unit tests for the config + Rule #1 refactor.

Runnable without a display / GTK:  python3 -m unittest discover tests
Covers the invariants the refactor introduced — enum single-source, env
override, and the clipboard inject_text delegation.
"""

import inspect
import json
import re
import os
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from draftright import config
from draftright.models.tone import Tone
from draftright.models.payment import BillingPeriod
from draftright.models.subscription import SubscriptionStatus
from draftright.models.app_mode import AppMode
from draftright.models.health import HealthStatus
from draftright.models.rewrite import RewriteResult
from draftright.models.hotkey import Hotkey, Modifier
from draftright.models.tray import TrayAction, TrayCommand
from draftright.services.hotkey_service import PortalResponse
from draftright.services.input_portal import InjectorState, RemoteDesktopInjector
from draftright.services.portal import PortalClient
from draftright.services import settings_service as settings_service_mod
from draftright.services.auth_service import AuthService
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


class RuntimeContractTest(unittest.TestCase):
    """Guards the service-side contracts the GTK UI calls into.

    Every one of these shipped broken: the code imports and the rest of the
    suite passes, because the mismatch only fires when a window is actually
    opened.  Assert the contracts directly so a display-less CI catches them.
    """

    def test_auth_service_exposes_methods_settings_window_calls(self):
        # settings_window._refresh_account_ui() calls both as methods.
        for name in ("is_authenticated", "get_user"):
            self.assertTrue(
                callable(getattr(AuthService, name, None)),
                f"AuthService.{name}() is called by settings_window",
            )

    def test_get_user_always_returns_a_mapping(self):
        auth = AuthService(mock.Mock())
        # Signed out: callers still do user.get("email", "") — must not be None.
        self.assertEqual(auth.get_user(), {})
        self.assertFalse(auth.is_authenticated())

    def test_register_signature_matches_settings_window_call_order(self):
        # The call site passes (email, password, name) positionally; a
        # transposition here silently registers the name as the email.
        params = list(inspect.signature(AuthService.register).parameters)
        self.assertEqual(params, ["self", "email", "password", "name"])

    def test_settings_service_key_access_round_trips(self):
        with tempfile.TemporaryDirectory() as tmp:
            with mock.patch.object(
                settings_service_mod, "_settings_file",
                return_value=Path(tmp) / "settings.json",
            ):
                s = SettingsService()
                s.load()
                s.set("translate_language", "Japanese")
                # A second instance must see the persisted value.
                other = SettingsService()
                other.load()
                self.assertEqual(other.get("translate_language"), "Japanese")
                # Unknown key falls back to the caller's default.
                self.assertEqual(other.get("nope", "fallback"), "fallback")
                # Known key with no default falls back to _DEFAULTS.
                self.assertEqual(other.get("hotkey"), "Ctrl+Shift+R")

    def test_set_does_not_clobber_unrelated_settings(self):
        # SettingsService.__init__ only seeds defaults; if a caller sets a key
        # without load()ing first, save() would persist defaults over the
        # user's file.  Writing one key must preserve the others.
        with tempfile.TemporaryDirectory() as tmp:
            with mock.patch.object(
                settings_service_mod, "_settings_file",
                return_value=Path(tmp) / "settings.json",
            ):
                seed = SettingsService()
                seed.load()
                seed.set("backend_url", "https://example.test")
                seed.set("translate_language", "Japanese")

                reloaded = SettingsService()
                reloaded.load()
                self.assertEqual(reloaded.get("backend_url"), "https://example.test")
                self.assertEqual(reloaded.get("translate_language"), "Japanese")

    def test_feedback_service_imports_without_a_singleton(self):
        # feedback_service previously imported a module-level `settings_service`
        # that does not exist → ImportError killed "Suggest a feature".
        self.assertFalse(
            hasattr(settings_service_mod, "settings_service"),
            "no module-level singleton should exist; feedback_service must "
            "construct SettingsService itself",
        )
        import draftright.services.feedback_service as feedback
        self.assertTrue(callable(feedback.submit_feature_request))


class TrayContractTest(unittest.TestCase):
    """The tray runs in a second process, so its contract must be single-sourced.

    A drifted action name fails silently — the menu click is just dropped —
    so assert the two halves agree here rather than discovering it by hand.
    """

    def test_object_path_derives_from_app_id(self):
        self.assertEqual(config.APP_OBJECT_PATH, "/com/draftright/app")
        self.assertEqual(
            config.APP_OBJECT_PATH, "/" + config.APP_ID.replace(".", "/")
        )

    def test_every_action_has_a_label_and_parses_back(self):
        for action in TrayAction:
            self.assertTrue(action.display_name.strip(), f"{action} has no label")
            self.assertIs(TrayAction.from_wire(action.value), action)
        self.assertIsNone(TrayAction.from_wire("no-such-action"))

    def test_action_names_are_valid_gaction_names(self):
        # Gio.SimpleAction rejects names outside [a-z0-9-]; an invalid member
        # would raise at construction and take the whole app down.
        for action in TrayAction:
            self.assertRegex(action.value, r"^[a-z][a-z0-9-]*$")

    def test_command_encode_parse_round_trip(self):
        line = TrayCommand.STATUS.encode(HealthStatus.CONNECTED.value)
        self.assertTrue(line.endswith("\n"))
        command, payload = TrayCommand.parse(line)
        self.assertIs(command, TrayCommand.STATUS)
        self.assertIs(HealthStatus.from_wire(payload), HealthStatus.CONNECTED)

        command, _ = TrayCommand.parse(TrayCommand.QUIT.encode())
        self.assertIs(command, TrayCommand.QUIT)

    def test_malformed_command_is_ignored_not_fatal(self):
        for junk in ("", "garbage", "status", ":", "status:not-a-status"):
            command, payload = TrayCommand.parse(junk)
            self.assertIn(command, (None, TrayCommand.STATUS))
            if command is TrayCommand.STATUS:
                # Unknown wire value must degrade to OFFLINE, never raise.
                self.assertIs(HealthStatus.from_wire(payload), HealthStatus.OFFLINE)

    def test_application_registers_a_handler_for_every_action(self):
        # application.py raises if a TrayAction has no handler; assert the
        # mapping is exhaustive without importing GTK by reading the source.
        source = Path(__file__).resolve().parent.parent / "draftright" / "application.py"
        text = source.read_text(encoding="utf-8")
        for action in TrayAction:
            self.assertIn(
                f"TrayAction.{action.name}:", text,
                f"application.py has no handler entry for TrayAction.{action.name}",
            )


class HotkeyModelTest(unittest.TestCase):
    """#99: one parser, two renderings (X11 mask names vs portal trigger)."""

    def test_parses_and_renders_for_both_backends(self):
        hotkey = Hotkey.parse("Ctrl+Shift+R")
        self.assertEqual(hotkey.modifiers, (Modifier.CTRL, Modifier.SHIFT))
        self.assertEqual(hotkey.key, "R")
        # X11 wants mask attribute names; the portal wants its own spelling
        # and a lower-case key.
        self.assertEqual(hotkey.x11_modifiers, ["control", "shift"])
        self.assertEqual(hotkey.to_portal_trigger(), "CTRL+SHIFT+r")
        self.assertEqual(hotkey.display_name, "Ctrl+Shift+R")

    def test_modifier_aliases_from_stored_settings(self):
        # Settings and portal triggers use several spellings for the same key.
        for alias in ("super", "mod4", "logo", "meta", "win", "cmd"):
            self.assertIs(Modifier.from_wire(alias), Modifier.SUPER)
        self.assertIs(Modifier.from_wire("control"), Modifier.CTRL)
        self.assertIs(Modifier.from_wire("mod1"), Modifier.ALT)
        self.assertIsNone(Modifier.from_wire("hyper"))

    def test_every_modifier_renders_for_every_backend(self):
        for modifier in Modifier:
            self.assertTrue(modifier.x11_name)
            self.assertTrue(modifier.portal_name)
            self.assertTrue(modifier.display_name)
            self.assertIs(Modifier.from_wire(modifier.value), modifier)

    def test_unknown_modifier_is_dropped_not_fatal(self):
        # A malformed stored hotkey must not stop the app from starting.
        hotkey = Hotkey.parse("Ctrl+Bogus+K")
        self.assertEqual(hotkey.modifiers, (Modifier.CTRL,))
        self.assertEqual(hotkey.key, "K")

    def test_duplicate_modifiers_collapse(self):
        self.assertEqual(
            Hotkey.parse("Ctrl+Control+R").modifiers, (Modifier.CTRL,)
        )

    def test_empty_keystring_raises(self):
        with self.assertRaises(ValueError):
            Hotkey.parse("   ")

    def test_multichar_key_keeps_its_case(self):
        # Named keys (space, F5) are not single letters and must not be
        # lower-cased into something XKB cannot resolve.
        self.assertEqual(Hotkey.parse("Ctrl+F5").to_portal_trigger(), "CTRL+F5")
        self.assertEqual(
            Hotkey.parse("Super+Alt+space").to_portal_trigger(), "LOGO+ALT+space"
        )

    def test_default_hotkey_is_parseable(self):
        hotkey = Hotkey.parse(config.DEFAULT_HOTKEY)
        self.assertTrue(hotkey.to_portal_trigger())
        self.assertTrue(hotkey.x11_modifiers)

    def test_portal_response_from_wire(self):
        self.assertIs(PortalResponse.from_wire(0), PortalResponse.SUCCESS)
        self.assertIs(PortalResponse.from_wire(1), PortalResponse.CANCELLED)
        # Unknown codes must not raise — degrade to ENDED.
        self.assertIs(PortalResponse.from_wire(99), PortalResponse.ENDED)
        for response in PortalResponse:
            self.assertTrue(response.display_name)


class InjectorStateTest(unittest.TestCase):
    """Wayland text injection (RemoteDesktop portal) — state machine only.

    The D-Bus handshake needs a live portal and user consent, so these cover
    the parts that must behave without one: never block the caller, never
    lose the callback, and never claim success when denied.
    """

    def test_states_have_labels(self):
        for state in InjectorState:
            self.assertTrue(state.display_name.strip())

    def test_denied_reports_failure_without_a_session(self):
        injector = RemoteDesktopInjector()
        injector.state = InjectorState.DENIED
        seen = []
        injector.paste(seen.append)
        # Must answer immediately and negatively — the caller is a UI button.
        self.assertEqual(seen, [False])

    def test_pending_callbacks_all_drain_on_failure(self):
        injector = RemoteDesktopInjector()
        injector.state = InjectorState.STARTING  # handshake already in flight
        seen = []
        injector.paste(seen.append)
        injector.paste(seen.append)
        injector._fail("test")
        self.assertEqual(seen, [False, False])
        self.assertIs(injector.state, InjectorState.DENIED)

    def test_paste_with_no_callback_is_safe(self):
        injector = RemoteDesktopInjector()
        injector.state = InjectorState.STARTING
        injector.paste(None)
        injector._fail("test")  # must not raise on a None callback

    def test_state_change_notifies_once_per_transition(self):
        injector = RemoteDesktopInjector()
        seen = []
        injector.on_state_changed = seen.append
        injector._set_state(InjectorState.STARTING)
        injector._set_state(InjectorState.STARTING)  # no-op
        injector._set_state(InjectorState.DENIED)
        self.assertEqual(seen, [InjectorState.STARTING, InjectorState.DENIED])

    def test_clipboard_service_without_injector_never_calls_portal(self):
        # X11 path, and the safety net if the injector was not wired up.
        service = ClipboardService(injector=None)
        self.assertIsNone(service._injector)


class LegacySettingsKeyTest(unittest.TestCase):
    """The Settings UI wrote hyphenated keys nothing read back.

    A user could pick a translate language and have the app keep using the old
    one — both spellings coexisted in settings.json.
    """

    def _service(self, tmp, stored):
        path = Path(tmp) / "settings.json"
        path.write_text(json.dumps(stored), encoding="utf-8")
        with mock.patch.object(
            settings_service_mod, "_settings_file", return_value=path
        ):
            service = SettingsService()
            service.load()
            return service, path

    def test_hyphen_key_wins_and_is_removed(self):
        with tempfile.TemporaryDirectory() as tmp:
            service, path = self._service(
                tmp,
                {"translate_language": "Vietnamese", "translate-language": "English"},
            )
            # The hyphenated value is the user's most recent explicit choice.
            self.assertEqual(service.translate_language, "English")
            self.assertNotIn("translate-language", service._data)
            # And it must be persisted, not just fixed in memory.
            self.assertNotIn("translate-language", json.loads(path.read_text()))

    def test_all_aliases_migrate(self):
        with tempfile.TemporaryDirectory() as tmp:
            stored = {legacy: "x" for legacy in settings_service_mod._LEGACY_KEY_ALIASES}
            service, _ = self._service(tmp, stored)
            for legacy, canonical in settings_service_mod._LEGACY_KEY_ALIASES.items():
                self.assertNotIn(legacy, service._data)
                self.assertEqual(service.get(canonical), "x")

    def test_no_legacy_keys_leaves_data_untouched(self):
        with tempfile.TemporaryDirectory() as tmp:
            service, _ = self._service(tmp, {"translate_language": "Vietnamese"})
            self.assertEqual(service.translate_language, "Vietnamese")

    def test_settings_ui_uses_canonical_keys_only(self):
        # Guards the regression directly: any hyphenated key in the Settings
        # window is a key the service will never read.
        source = (
            Path(__file__).resolve().parent.parent
            / "draftright" / "ui" / "settings_window.py"
        )
        used = set(re.findall(r'_(?:get|save)_setting\("([^"]+)"', source.read_text()))
        self.assertTrue(used, "expected the Settings window to address settings by key")
        for key in used:
            self.assertNotIn("-", key, f"{key!r} uses a hyphen; defaults use underscores")
            self.assertIn(key, settings_service_mod._DEFAULTS, f"{key!r} has no default")


class RewriteResultTest(unittest.TestCase):
    """The /rewrite response is a dict; the panel treated it as a string.

    Every *successful* rewrite raised "TypeError: Must be string, not dict".
    It survived because the app had never been signed in, so the success path
    had never executed.
    """

    def test_extracts_text_from_the_wire_dict(self):
        result = RewriteResult.from_wire(
            {"rewritten_text": "Hello.", "usage_today": 3, "daily_limit": 50}
        )
        self.assertEqual(result.text, "Hello.")
        self.assertEqual(result.usage_today, 3)
        self.assertEqual(result.daily_limit, 50)
        self.assertEqual(result.usage_display, "3 / 50 today")

    def test_text_is_always_a_str_for_gtk(self):
        # The actual defect: whatever reaches set_text() must be a str.
        self.assertIsInstance(
            RewriteResult.from_wire({"rewritten_text": "x"}).text, str
        )

    def test_accepts_a_bare_string(self):
        # Cached values and plain-text backends must not crash the panel.
        self.assertEqual(RewriteResult.from_wire("cached").text, "cached")

    def test_missing_text_raises_rather_than_returning_a_dict(self):
        for payload in ({}, {"rewritten_text": None}, {"other": "x"}):
            with self.assertRaises(ValueError):
                RewriteResult.from_wire(payload)

    def test_counters_tolerate_strings_and_nulls(self):
        result = RewriteResult.from_wire(
            {"rewritten_text": "x", "usage_today": "7", "daily_limit": None}
        )
        self.assertEqual(result.usage_today, 7)
        self.assertIsNone(result.daily_limit)
        self.assertEqual(result.usage_display, "")


class AppModeTest(unittest.TestCase):
    """#96 One-Click — wire values must match macOS/Windows exactly."""

    def test_wire_values_match_other_platforms(self):
        self.assertEqual(AppMode.ADVANCED.value, "advanced")
        # camelCase on purpose: it is what macOS and Windows already persist.
        self.assertEqual(AppMode.ONE_CLICK.value, "oneClick")

    def test_display_names_match_other_platforms(self):
        self.assertEqual(AppMode.ADVANCED.display_name, "Advanced")
        self.assertEqual(AppMode.ONE_CLICK.display_name, "Simple")

    def test_unknown_falls_back_to_advanced(self):
        # Advanced always shows the panel, so a bad value cannot silently
        # rewrite and replace the user's text.
        for raw in (None, "", "garbage", "ONECLICK"):
            self.assertIs(AppMode.from_wire(raw), AppMode.ADVANCED)

    def test_round_trips(self):
        for mode in AppMode:
            self.assertIs(AppMode.from_wire(mode.value), mode)
            self.assertTrue(mode.description.strip())

    def test_settings_defaults_to_advanced_with_a_valid_tone(self):
        service = SettingsService()
        self.assertIs(service.app_mode, AppMode.ADVANCED)
        self.assertIn(service.one_click_tone, {t.api_value for t in Tone})

    def test_settings_guards_a_stale_one_click_tone(self):
        service = SettingsService()
        service._data["one_click_tone"] = "a_tone_that_was_removed"
        # Must not hand the hotkey a tone the backend will reject.
        self.assertIn(service.one_click_tone, {t.api_value for t in Tone})


class PortalClientTest(unittest.TestCase):
    """Shared portal plumbing (#99 hotkey + text injection use one copy)."""

    def test_request_tokens_are_unique_per_call(self):
        client = PortalClient("org.example.Iface", name_prefix="probe")
        tokens = {client.next_token("x") for _ in range(5)}
        self.assertEqual(len(tokens), 5, "handle tokens must not collide")

    def test_token_carries_the_prefix(self):
        client = PortalClient("org.example.Iface", name_prefix="probe")
        self.assertIn("probe", client.next_token("createsession"))

    def test_two_clients_do_not_collide(self):
        a = PortalClient("org.example.A", name_prefix="alpha")
        b = PortalClient("org.example.B", name_prefix="beta")
        self.assertNotEqual(a.next_token("x"), b.next_token("x"))

    def test_close_session_without_bus_is_a_noop(self):
        PortalClient("org.example.Iface", name_prefix="probe").close_session("/x")


if __name__ == "__main__":
    unittest.main()
