"""GTK-free unit tests for the config + Rule #1 refactor.

Runnable without a display / GTK:  python3 -m unittest discover tests
Covers the invariants the refactor introduced — enum single-source, env
override, and the clipboard inject_text delegation.
"""

import base64
import hashlib
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
from draftright.services import google_oauth
from draftright.helpers import tray_icon_render
from draftright.services import keepalive_service
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


class GoogleOAuthTest(unittest.TestCase):
    """#97 — PKCE + loopback redirect (RFC 8252) for a native app."""

    def test_pkce_challenge_is_s256_of_the_verifier(self):
        verifier, challenge = google_oauth._pkce_pair()
        expected = (
            base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest())
            .rstrip(b"=")
            .decode()
        )
        self.assertEqual(challenge, expected)

    def test_pkce_values_are_base64url_without_padding(self):
        # Padding or '+'/'/' would be rejected by Google's parameter parsing.
        for value in google_oauth._pkce_pair():
            self.assertNotIn("=", value)
            self.assertNotIn("+", value)
            self.assertNotIn("/", value)

    def test_pkce_verifier_length_is_within_the_spec(self):
        # RFC 7636 requires 43-128 characters.
        verifier, _ = google_oauth._pkce_pair()
        self.assertGreaterEqual(len(verifier), 43)
        self.assertLessEqual(len(verifier), 128)

    def test_each_run_is_unique(self):
        # Reusing a verifier across sign-ins would defeat PKCE.
        pairs = {google_oauth._pkce_pair()[0] for _ in range(5)}
        self.assertEqual(len(pairs), 5)

    def test_sign_in_is_hidden_until_a_client_id_is_configured(self):
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop(config.GOOGLE_CLIENT_ID_ENV_VAR, None)
            with mock.patch.object(config, "DEFAULT_GOOGLE_CLIENT_ID", ""):
                self.assertFalse(config.google_sign_in_available())
            with mock.patch.object(config, "DEFAULT_GOOGLE_CLIENT_ID", "x.apps"):
                self.assertTrue(config.google_sign_in_available())

    def test_env_var_overrides_the_built_in_client_id(self):
        with mock.patch.dict(
            os.environ, {config.GOOGLE_CLIENT_ID_ENV_VAR: "env-client"}
        ):
            self.assertEqual(config.google_client_id(), "env-client")

    def test_scopes_request_identity_only(self):
        # An id_token needs openid; email/profile fill in the account. Nothing
        # broader should be requested.
        self.assertEqual(
            set(config.GOOGLE_OAUTH_SCOPES.split()), {"openid", "email", "profile"}
        )

    def test_endpoints_are_google_https(self):
        for url in (config.GOOGLE_AUTH_ENDPOINT, config.GOOGLE_TOKEN_ENDPOINT):
            self.assertTrue(url.startswith("https://"))
            self.assertIn("google", url)


class RewritePanelLifecycleTest(unittest.TestCase):
    """One panel, reused — and a way out of it.

    Each hotkey press used to build a fresh RewritePanel, so presses stacked
    identical windows; closing the top one revealed the next and the panel
    appeared impossible to close. Windows reuses a single panel and updates
    its text (App.cs).
    """

    def _source(self, rel: str) -> str:
        return (
            Path(__file__).resolve().parent.parent / "draftright" / rel
        ).read_text(encoding="utf-8")

    def test_panel_is_cached_not_rebuilt_per_press(self):
        body = self._source("application.py").split("def show_rewrite_panel")[1]
        body = body.split("\n    def ")[0]
        self.assertIn("self._rewrite_panel is None", body,
                      "a new panel per press stacks windows")
        self.assertIn("show_with_text", body)

    def test_cached_panel_is_dropped_when_destroyed(self):
        # Otherwise a destroyed window would be reused and never reappear.
        src = self._source("application.py")
        self.assertIn("_on_rewrite_panel_closed", src)
        handler = src.split("def _on_rewrite_panel_closed")[1].split("\n    def ")[0]
        self.assertIn("self._rewrite_panel = None", handler)

    def test_escape_is_bound(self):
        # The window is undecorated, so the compositor offers no close button.
        src = self._source("ui/rewrite_panel.py")
        self.assertIn("EventControllerKey", src)
        handler = src.split("def _on_key_pressed")[1].split("\n    def ")[0]
        self.assertIn("Gdk.KEY_Escape", handler)
        self.assertIn("self._close()", handler)

    def test_escape_handler_only_swallows_escape(self):
        # Returning True unconditionally would eat every keystroke.
        src = self._source("ui/rewrite_panel.py")
        handler = src.split("def _on_key_pressed")[1].split("\n    def ")[0]
        self.assertIn("return False", handler)

    def test_panel_stays_undecorated(self):
        # If this ever changes, the Escape binding is no longer the only exit
        # and the reasoning above should be revisited.
        self.assertIn("set_decorated(False)", self._source("ui/rewrite_panel.py"))


class SettingsLayoutTest(unittest.TestCase):
    """The mode switch has to be findable, and has to visibly do something.

    It was a row called "Hotkey mode" inside Preferences > Behavior — a page
    reached only by clicking past Account — and Panel Tones stayed visible in
    Simple mode, so the choice looked inert. Windows puts it first on its own
    Rewrite tab and hides the block that does not apply.
    """

    def _source(self) -> str:
        path = (
            Path(__file__).resolve().parent.parent
            / "draftright" / "ui" / "settings_window.py"
        )
        return path.read_text(encoding="utf-8")

    def test_rewrite_page_is_added_first(self):
        # Adw.PreferencesWindow lands on whichever page is added first.
        src = self._source()
        order = [
            line.strip() for line in src.splitlines()
            if line.strip().startswith("self._build_") and "_page()" in line
        ]
        self.assertTrue(order, "expected page builders in __init__")
        self.assertEqual(order[0], "self._build_rewrite_page()")

    def test_mode_row_lives_on_the_rewrite_page(self):
        src = self._source()
        page = src.split("def _build_rewrite_page")[1].split("\n    def ")[0]
        self.assertIn("_mode_row", page)
        self.assertIn("Interaction Mode", page)

    def test_mode_naming_matches_windows(self):
        # Windows: section header "Mode", field label "Interaction Mode".
        src = self._source()
        self.assertIn('title="Interaction Mode"', src)
        self.assertIn('title="Mode"', src)

    def test_blocks_are_mutually_exclusive(self):
        src = self._source()
        helper = src.split("def _apply_mode_visibility")[1].split("\n    def ")[0]
        self.assertIn("_simple_group.set_visible(mode is AppMode.ONE_CLICK)", helper)
        self.assertIn("_advanced_group.set_visible(mode is AppMode.ADVANCED)", helper)

    def test_changing_mode_reapplies_visibility(self):
        src = self._source()
        handler = src.split("def _on_mode_changed")[1].split("\n    def ")[0]
        self.assertIn("_apply_mode_visibility(mode)", handler)


class TranslateLanguageTest(unittest.TestCase):
    """The Translate tone must honour the user's chosen language.

    Windows sends Settings.TranslateLanguage on every rewrite; Linux sent
    nothing, so Translate ignored the setting entirely.
    """

    def test_only_translate_depends_on_the_language(self):
        self.assertTrue(Tone.TRANSLATE.uses_target_language)
        for tone in Tone:
            if tone is not Tone.TRANSLATE:
                self.assertFalse(tone.uses_target_language, tone)

    def test_from_api_value_round_trips(self):
        for tone in Tone:
            self.assertIs(Tone.from_api_value(tone.api_value), tone)
        self.assertIsNone(Tone.from_api_value("no_such_tone"))

    def test_cache_separates_languages_for_translate(self):
        # Without this, switching language serves the previous language's
        # translation from cache.
        cache = RewriteCache()
        cache.set("hello", "translate", "Xin chào", language="Vietnamese")
        self.assertEqual(cache.get("hello", "translate", "Vietnamese"), "Xin chào")
        self.assertIsNone(cache.get("hello", "translate", "Japanese"))

    def test_cache_key_unchanged_when_no_language_applies(self):
        # Non-translate tones must keep the macOS key shape so behaviour and
        # hit rates are unchanged.
        cache = RewriteCache()
        self.assertEqual(cache._key("hi", "polished"), "polished::hi")
        self.assertEqual(cache._key("hi", "polished", None), "polished::hi")

    def test_language_participates_in_the_key_when_given(self):
        cache = RewriteCache()
        self.assertNotEqual(
            cache._key("hi", "translate", "Vietnamese"),
            cache._key("hi", "translate", "Japanese"),
        )

    def test_both_rewrite_call_sites_pass_a_language(self):
        # The parameter existed and was simply never used at either site.
        root = Path(__file__).resolve().parent.parent / "draftright"
        for rel in ("application.py", "ui/rewrite_panel.py"):
            src = (root / rel).read_text(encoding="utf-8")
            call = src.split("api_client.rewrite(")[1].split(")")[0]
            self.assertIn("language", call, f"{rel} must send a target language")


class TrayIconStateTest(unittest.TestCase):
    """Tray conveys backend status by tint and an update by a red dot (#22).

    Parity with Windows' TrayIconBadge and macOS' MenuBarIcon.
    """

    def test_every_status_has_a_defined_tint(self):
        for status in HealthStatus:
            # None is a valid answer (leave it theme-coloured) — the point is
            # that the mapping is exhaustive and cannot KeyError at runtime.
            tint = status.tint_color
            self.assertTrue(tint is None or tint.startswith("#"), status)

    def test_connected_stays_theme_coloured(self):
        # The normal state should be recoloured by the shell so it sits well
        # in both light and dark panels.
        self.assertIsNone(HealthStatus.CONNECTED.tint_color)

    def test_offline_is_red(self):
        self.assertEqual(HealthStatus.OFFLINE.tint_color, "#ef4444")

    def test_badge_colour_matches_the_other_platforms(self):
        # Windows uses Color.FromArgb(239, 68, 68); macOS the same red-500.
        self.assertEqual(config.TRAY_BADGE_COLOR.lower(), "#ef4444")
        self.assertEqual(tuple(int("ef4444"[i:i+2], 16) for i in (0, 2, 4)),
                         (239, 68, 68))

    def test_plain_connected_needs_no_composited_file(self):
        # Returning None tells the caller to use the named symbolic.
        self.assertIsNone(
            tray_icon_render.build(HealthStatus.CONNECTED, False, directory=None)
        )

    def test_states_needing_paint_produce_distinct_icon_names(self):
        # AppIndicator caches by name; a shared name means the icon would not
        # visibly change between states.
        with tempfile.TemporaryDirectory() as tmp:
            names = set()
            for status in HealthStatus:
                for update in (False, True):
                    built = tray_icon_render.build(status, update, directory=tmp)
                    if built is not None:
                        names.add(built[1])
            # 4 statuses x 2, minus the one plain state that renders nothing.
            self.assertEqual(len(names), len(HealthStatus) * 2 - 1)

    def test_update_badge_renders_even_when_connected(self):
        with tempfile.TemporaryDirectory() as tmp:
            built = tray_icon_render.build(HealthStatus.CONNECTED, True, directory=tmp)
            self.assertIsNotNone(built, "an update must be visible when connected")
            self.assertTrue((Path(built[0]) / f"{built[1]}.png").exists())

    def test_update_command_round_trips(self):
        line = TrayCommand.UPDATE.encode("1")
        command, payload = TrayCommand.parse(line)
        self.assertIs(command, TrayCommand.UPDATE)
        self.assertEqual(payload, "1")


class TimerContractTest(unittest.TestCase):
    """GLib timers whose callback returns True must not use a 0 interval.

    `timeout_add_seconds(0, cb)` with `cb` returning True re-arms instantly:
    measured at ~1.3 million calls in 10 seconds. In the app that meant a
    network thread per call, a hammered backend, and a tray icon flickering
    between the app mark and the offline warning as results flapped.
    """

    def _application_source(self) -> str:
        path = (
            Path(__file__).resolve().parent.parent / "draftright" / "application.py"
        )
        return path.read_text(encoding="utf-8")

    def test_no_zero_interval_timers(self):
        source = self._application_source()
        # Ignore the comment that documents the old bug.
        code = "\n".join(
            line for line in source.splitlines()
            if not line.lstrip().startswith("#")
        )
        for bad in ("timeout_add_seconds(0,", "timeout_add(0,"):
            self.assertNotIn(
                bad, code,
                f"{bad} re-arms instantly when the callback returns True; "
                "call the function directly for an immediate run",
            )

    def test_repeating_health_timer_uses_the_configured_interval(self):
        source = self._application_source()
        self.assertIn(
            "timeout_add_seconds(config.HEALTH_CHECK_INTERVAL, self._trigger_health_check)",
            source,
        )
        self.assertGreaterEqual(config.HEALTH_CHECK_INTERVAL, 5)

    def test_one_shot_timers_do_not_repeat(self):
        # A callback returning True on a short timer is the same trap with a
        # bigger constant — the update check would run every 10 seconds.
        source = self._application_source()
        for name in ("_trigger_update_check", "_trigger_whats_new_check"):
            body = source.split(f"def {name}")[1].split("def ")[0]
            self.assertIn("return False", body, f"{name} must not repeat")


class KeepAliveServiceTest(unittest.TestCase):
    """#100 — systemd user unit that respawns the app after a crash."""

    def _unit_text(self) -> str:
        return keepalive_service._UNIT_TEMPLATE.format(
            exec_start="/usr/bin/draftright",
            restart_sec=config.KEEPALIVE_RESTART_SEC,
            burst=config.KEEPALIVE_START_LIMIT_BURST,
            interval=config.KEEPALIVE_START_LIMIT_INTERVAL,
        )

    def test_unit_name_lets_the_portal_derive_the_app_id(self):
        # xdg-desktop-portal refuses a global shortcut when it cannot identify
        # the caller (#99), and it identifies unsandboxed apps by systemd unit.
        self.assertTrue(keepalive_service.UNIT_NAME.startswith(f"app-{config.APP_ID}"))
        self.assertTrue(keepalive_service.UNIT_NAME.endswith(".service"))

    def test_restart_is_on_failure_not_always(self):
        # Restart=always would respawn a deliberate Quit, trapping the user.
        text = self._unit_text()
        self.assertIn("Restart=on-failure", text)
        self.assertNotIn("Restart=always", text)

    def test_rate_limit_keys_are_in_the_unit_section(self):
        # systemd silently ignores StartLimit* under [Service]; the rate limit
        # would not apply and a crash-loop would hammer the session.
        text = self._unit_text()
        # Anchor on the section header at line start — the comments mention
        # "[Service]" too.
        unit_section = text.split("\n[Service]")[0]
        self.assertIn("StartLimitBurst=", unit_section)
        self.assertIn("StartLimitIntervalSec=", unit_section)

    def test_unit_declares_an_install_target(self):
        # Without [Install] `systemctl enable` fails and nothing runs at login.
        self.assertIn("[Install]", self._unit_text())
        self.assertIn("WantedBy=", self._unit_text())

    def test_exec_start_is_absolute(self):
        # systemd does not resolve PATH for ExecStart.
        command = keepalive_service.executable_command()
        self.assertTrue(command.startswith("/"), command)

    def test_unit_path_is_under_the_user_unit_dir(self):
        path = keepalive_service.unit_path()
        self.assertEqual(path.name, keepalive_service.UNIT_NAME)
        self.assertTrue(str(path).endswith(".config/systemd/user/" + path.name))

    def test_install_is_a_noop_without_systemd(self):
        with mock.patch.object(keepalive_service, "systemd_available", return_value=False):
            self.assertFalse(keepalive_service.install())

    def test_uninstall_tolerates_nothing_installed(self):
        with mock.patch.object(keepalive_service, "unit_path") as unit_path:
            unit_path.return_value = Path(tempfile.gettempdir()) / "definitely-absent.service"
            with mock.patch.object(keepalive_service.shutil, "which", return_value=None):
                self.assertTrue(keepalive_service.uninstall())

    def test_auto_start_prefers_systemd_and_drops_the_xdg_entry(self):
        # Both mechanisms launching at login means one instance loses the
        # single-instance race and exits silently.
        with tempfile.TemporaryDirectory() as tmp:
            settings_path = Path(tmp) / "settings.json"
            autostart = Path(tmp) / "autostart.desktop"
            autostart.write_text("stale", encoding="utf-8")
            with mock.patch.object(
                settings_service_mod, "_settings_file", return_value=settings_path
            ), mock.patch.object(
                settings_service_mod, "_autostart_file", return_value=autostart
            ), mock.patch.object(keepalive_service, "install", return_value=True):
                service = SettingsService()
                service.set_auto_start(True)
            self.assertFalse(autostart.exists(), "XDG entry should have been removed")

    def test_auto_start_falls_back_to_xdg_without_systemd(self):
        with tempfile.TemporaryDirectory() as tmp:
            settings_path = Path(tmp) / "settings.json"
            autostart = Path(tmp) / "autostart.desktop"
            with mock.patch.object(
                settings_service_mod, "_settings_file", return_value=settings_path
            ), mock.patch.object(
                settings_service_mod, "_autostart_file", return_value=autostart
            ), mock.patch.object(
                keepalive_service, "install", return_value=False
            ), mock.patch.object(keepalive_service, "uninstall", return_value=True):
                service = SettingsService()
                service.set_auto_start(True)
            self.assertTrue(autostart.exists(), "should fall back to the XDG entry")

    def test_disabling_removes_both_mechanisms(self):
        with tempfile.TemporaryDirectory() as tmp:
            settings_path = Path(tmp) / "settings.json"
            autostart = Path(tmp) / "autostart.desktop"
            autostart.write_text("stale", encoding="utf-8")
            with mock.patch.object(
                settings_service_mod, "_settings_file", return_value=settings_path
            ), mock.patch.object(
                settings_service_mod, "_autostart_file", return_value=autostart
            ), mock.patch.object(keepalive_service, "uninstall") as uninstall:
                service = SettingsService()
                service.set_auto_start(False)
                uninstall.assert_called_once()
            self.assertFalse(autostart.exists())
            self.assertFalse(SettingsService().auto_start)


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
