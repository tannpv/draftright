"""Behaviour tests for the Linux grammar/diff UI and the fixes around it (#107).

Runnable without a display: python3 test/test_grammar_diff_ui.py

These drive the real objects. An earlier version of the panel tests asserted
on the *source text* of rewrite_panel.py, and a later one hand-copied
show_with_text's body instead of calling it — both stayed green while the
feature was broken. Every fix here is mutation-tested: removing the fix must
make a test fail.
"""

import json
import re
import threading
import unittest
from pathlib import Path
from unittest import mock

from draftright import config
from draftright.models.diff import DiffKind, exceeds_diff_cap, word_diff
from draftright.models.grammar import (
    GrammarIssue,
    GrammarIssueType,
    GrammarResult,
    ScoreBand,
)
from draftright.models.health import HealthStatus, RefreshOutcome
from draftright.models.rewrite import RewriteResult
from draftright.models.tone import Tone
from draftright.services import api_client as api_client_mod
from draftright.helpers import tray_icon_render
from draftright.services import grammar_fixer
from draftright.services import hotkey_service as hotkey_service_mod
from draftright.services.api_client import APIClient, APIError
from draftright.services.auth_service import AuthService
from draftright.services.hotkey_service import HotkeyService
from draftright.ui import rewrite_panel as rewrite_panel_mod
from draftright.ui.rewrite_panel import RewritePanel


# ----------------------------------------------------------------------
# Headless panel scaffolding
# ----------------------------------------------------------------------


class _FakeStack:
    def __init__(self):
        self.page = None

    def set_visible_child_name(self, name):
        self.page = name


class _FakeWidget:
    """Stand-in for the buttons/views the panel drives.

    start/stop are RECORDED: no-ops here hid a spinner that never stopped.
    """

    def __init__(self):
        self.visible = False
        self.sensitive = False
        self.active = False
        self.text = ""
        self.cleared = 0
        self.diffed = None
        self.grammar = None
        self.spinning = False

    def set_visible(self, v): self.visible = v
    def get_visible(self): return self.visible
    def set_sensitive(self, v): self.sensitive = v
    def get_sensitive(self): return self.sensitive
    def get_active(self): return self.active
    def set_active(self, v): self.active = v
    def add_css_class(self, _c): pass
    def remove_css_class(self, _c): pass
    def set_text(self, t): self.text = t
    def get_text(self): return self.text
    def start(self): self.spinning = True
    def stop(self): self.spinning = False
    def get_buffer(self): return self
    def clear(self): self.cleared += 1
    def set_texts(self, a, b): self.diffed = (a, b)

    def set_result(self, text, result):
        self.grammar = (text, result)

    @property
    def corrected_text(self):
        return self.grammar[0] if self.grammar else ""


class _HeadlessPanel(RewritePanel):
    """RewritePanel with only the toplevel-window calls neutralised.

    show_with_text, _call_api and the response handlers are the REAL
    implementations — that is the whole point, so a missing request-sequence
    bump cannot pass a test named for it.
    """

    def set_visible(self, visible):
        self._fake_visible = visible

    def present(self):
        self._fake_presented = True

    def _close(self):
        self._fake_visible = False

    @classmethod
    def build(cls, app=None, input_text=""):
        panel = cls.__new__(cls)
        panel.app = app
        panel._input_text = input_text
        panel._result_text = ""
        panel._selected_tone = None
        panel._request_seq = 0
        panel._diff_rendered = False
        panel._fake_visible = False
        panel._fake_presented = False
        panel._result_stack = _FakeStack()
        panel._input_label = _FakeWidget()
        panel._tone_buttons = {}
        for name in ("_result_view", "_diff_view", "_grammar_view",
                     "_diff_toggle", "_replace_btn", "_copy_btn", "_spinner",
                     "_error_box", "_error_label"):
            setattr(panel, name, _FakeWidget())
        return panel


class PanelResultRoutingTest(unittest.TestCase):
    """Response handling: staleness, tone routing, lazy diff."""

    def _panel(self, input_text="original text"):
        return _HeadlessPanel.build(input_text=input_text)

    def test_a_superseded_response_is_dropped(self):
        panel = self._panel()
        panel._request_seq = 1
        stale = 1
        panel._request_seq = 2
        panel._on_api_success("late", stale, Tone.POLISHED)
        self.assertEqual(panel._result_text, "")
        self.assertFalse(panel._replace_btn.get_sensitive())

    def test_reopening_the_panel_supersedes_an_in_flight_request(self):
        # show_with_text is CALLED, not re-implemented.
        panel = self._panel("text A")
        panel._request_seq += 1
        in_flight = panel._request_seq
        panel.show_with_text("text B")
        panel._on_api_success("rewrite of A", in_flight, Tone.POLISHED)
        self.assertEqual(panel._result_text, "",
                         "the previous selection's rewrite leaked into the new one")
        self.assertEqual(panel._input_text, "text B")

    def test_reopening_stops_a_spinner_left_by_the_old_request(self):
        panel = self._panel("text A")
        panel._request_seq += 1
        in_flight = panel._request_seq
        panel._spinner.start()
        panel.show_with_text("text B")
        panel._on_api_success("rewrite of A", in_flight, Tone.POLISHED)
        self.assertFalse(panel._spinner.spinning, "spinner never stopped")

    def test_a_stale_error_also_leaves_no_spinner(self):
        panel = self._panel("text A")
        panel._request_seq += 1
        in_flight = panel._request_seq
        panel._spinner.start()
        panel.show_with_text("text B")
        panel._on_api_error("boom", in_flight)
        self.assertFalse(panel._spinner.spinning)
        self.assertFalse(panel._error_box.get_visible())

    def test_a_grammar_payload_never_reaches_the_plain_text_view(self):
        panel = self._panel()
        panel._request_seq += 1
        payload = json.dumps({"score": 80, "issues": []})
        panel._on_api_success(payload, panel._request_seq, Tone.GRAMMAR_CHECK)
        self.assertEqual(panel._result_stack.page, "grammar")
        self.assertNotIn("score", panel._result_text)

    def test_the_tone_comes_from_the_request_not_the_current_selection(self):
        panel = self._panel()
        panel._request_seq += 1
        seq = panel._request_seq
        panel._selected_tone = Tone.POLISHED.api_value   # changed since
        payload = json.dumps({"score": 90, "issues": []})
        panel._on_api_success(payload, seq, Tone.GRAMMAR_CHECK)
        self.assertEqual(panel._result_stack.page, "grammar")

    def test_a_backend_reported_grammar_error_is_shown_not_scored(self):
        panel = self._panel()
        panel._request_seq += 1
        payload = json.dumps({"score": 0, "issues": [],
                              "error": "Failed to parse grammar analysis"})
        panel._on_api_success(payload, panel._request_seq, Tone.GRAMMAR_CHECK)
        self.assertTrue(panel._error_box.get_visible())
        self.assertEqual(panel._error_label.get_text(),
                         "Failed to parse grammar analysis")

    def test_an_unreadable_grammar_payload_surfaces_as_an_error(self):
        panel = self._panel()
        panel._request_seq += 1
        panel._on_api_success("not json", panel._request_seq, Tone.GRAMMAR_CHECK)
        self.assertTrue(panel._error_box.get_visible())
        self.assertFalse(panel._replace_btn.get_sensitive())

    def test_the_diff_is_not_computed_until_it_is_shown(self):
        panel = self._panel()
        panel._request_seq += 1
        panel._on_api_success("rewritten", panel._request_seq, Tone.POLISHED)
        self.assertIsNone(panel._diff_view.diffed,
                          "word_diff ran on the main loop for a hidden page")

    def test_toggling_diff_computes_it_once(self):
        panel = self._panel()
        panel._request_seq += 1
        panel._on_api_success("rewritten", panel._request_seq, Tone.POLISHED)
        panel._diff_toggle.set_active(True)
        panel._on_diff_toggled(panel._diff_toggle)
        self.assertEqual(panel._diff_view.diffed, ("original text", "rewritten"))
        panel._diff_view.diffed = None
        panel._diff_toggle.set_active(False)
        panel._on_diff_toggled(panel._diff_toggle)
        panel._diff_toggle.set_active(True)
        panel._on_diff_toggled(panel._diff_toggle)
        self.assertIsNone(panel._diff_view.diffed, "diff recomputed needlessly")


class PanelRequestTest(unittest.TestCase):
    """Drives _call_api: cache keys, target language, input snapshot."""

    class _Cache:
        def __init__(self, seeded=None):
            self.gets, self.sets = [], []
            self._data = seeded or {}

        def get(self, text, tone, language=None):
            self.gets.append((text, tone, language))
            return self._data.get((text, tone, language))

        def set(self, text, tone, result, language=None):
            self.sets.append((text, tone, result, language))

    def _run(self, tone, cache=None, payload=None, seeded=None):
        cache = cache if cache is not None else self._Cache(seeded)
        api = mock.Mock()
        api.rewrite.return_value = payload or {"rewritten_text": "done"}
        app = mock.Mock(api_client=api, rewrite_cache=cache)
        app.settings_service.translate_language = "Vietnamese"
        panel = _HeadlessPanel.build(app=app, input_text="hello")
        with mock.patch.object(rewrite_panel_mod.GLib, "idle_add",
                               lambda fn, *a: fn(*a)):
            panel._call_api(tone)
            for t in threading.enumerate():
                if t is not threading.current_thread() and t.daemon:
                    t.join(timeout=5)
        return panel, api, cache

    def test_translate_sends_and_keys_on_the_language(self):
        _, api, cache = self._run(Tone.TRANSLATE.api_value)
        self.assertEqual(api.rewrite.call_args[0][2], "Vietnamese")
        self.assertEqual(cache.sets[0][3], "Vietnamese")

    def test_the_cache_is_written_under_the_key_it_is_read_with(self):
        for tone in (Tone.TRANSLATE.api_value, Tone.POLISHED.api_value):
            with self.subTest(tone=tone):
                _, _, cache = self._run(tone)
                wrote = cache.sets[0]
                self.assertEqual((wrote[0], wrote[1], wrote[3]), cache.gets[0])

    def test_other_tones_do_not_send_a_target_language(self):
        _, api, cache = self._run(Tone.POLISHED.api_value)
        self.assertIsNone(api.rewrite.call_args[0][2])
        self.assertIsNone(cache.sets[0][3])

    def test_a_cache_hit_skips_the_backend(self):
        seeded = {("hello", Tone.POLISHED.api_value, None): "from cache"}
        panel, api, _ = self._run(Tone.POLISHED.api_value, seeded=seeded)
        api.rewrite.assert_not_called()
        self.assertEqual(panel._result_text, "from cache")

    def test_the_worker_uses_a_snapshot_of_the_input(self):
        api = mock.Mock()
        cache = self._Cache()
        app = mock.Mock(api_client=api, rewrite_cache=cache)
        app.settings_service.translate_language = None
        panel = _HeadlessPanel.build(app=app, input_text="first")

        def slow(text, tone, language):
            panel._input_text = "second"        # user reopened mid-flight
            return {"rewritten_text": "done"}

        api.rewrite.side_effect = slow
        with mock.patch.object(rewrite_panel_mod.GLib, "idle_add",
                               lambda fn, *a: fn(*a)):
            panel._call_api(Tone.POLISHED.api_value)
            for t in threading.enumerate():
                if t is not threading.current_thread() and t.daemon:
                    t.join(timeout=5)
        self.assertEqual(api.rewrite.call_args[0][0], "first")
        self.assertEqual(cache.sets[0][0], "first")


class GrammarFixReportingTest(unittest.TestCase):
    """apply_all must say what it could NOT do.

    remaining_issues()/fix_all() drop stale issues silently, so a UI built on
    them announces "All issues fixed!" over text that is still wrong — and
    that text is what Replace pastes.
    """

    def _issue(self, original, suggestion, kind=GrammarIssueType.SPELLING):
        return GrammarIssue(original=original, suggestion=suggestion,
                            issue_type=kind)

    def test_an_overlapping_suggestion_is_reported_skipped(self):
        issues = [self._issue("their", "there"),
                  self._issue("their going", "they're going")]
        out = grammar_fixer.apply_all("their going to the store", issues)
        self.assertEqual(out.text, "there going to the store")
        self.assertEqual(len(out.applied), 1)
        self.assertEqual(len(out.skipped), 1)
        self.assertFalse(out.is_complete)

    def test_everything_applicable_is_complete(self):
        out = grammar_fixer.apply_all("teh cat", [self._issue("teh", "the")])
        self.assertEqual(out.text, "the cat")
        self.assertTrue(out.is_complete)

    def test_fix_all_still_agrees_with_apply_all(self):
        # fix_all delegates, so the loop exists once.
        issues = [self._issue("teh", "the"), self._issue("absent", "x")]
        self.assertEqual(grammar_fixer.fix_all("teh cat", issues),
                         grammar_fixer.apply_all("teh cat", issues).text)

    def test_no_issues_is_a_no_op(self):
        out = grammar_fixer.apply_all("untouched", [])
        self.assertEqual(out.text, "untouched")
        self.assertTrue(out.is_complete)


class GrammarResultTest(unittest.TestCase):
    """Score bands, the backend error field, and JSON carriage."""

    def test_score_bands_follow_the_config_thresholds(self):
        self.assertIs(ScoreBand.for_score(config.GRAMMAR_SCORE_GOOD), ScoreBand.GOOD)
        self.assertIs(ScoreBand.for_score(config.GRAMMAR_SCORE_GOOD - 1), ScoreBand.FAIR)
        self.assertIs(ScoreBand.for_score(config.GRAMMAR_SCORE_FAIR), ScoreBand.FAIR)
        self.assertIs(ScoreBand.for_score(config.GRAMMAR_SCORE_FAIR - 1), ScoreBand.POOR)

    def test_the_backend_parse_failure_is_carried(self):
        result = GrammarResult.from_wire(
            {"score": 0, "issues": [], "error": "Failed to parse grammar analysis"})
        self.assertEqual(result.error, "Failed to parse grammar analysis")

    def test_the_score_is_clamped(self):
        self.assertEqual(GrammarResult.from_wire({"score": 500}).score,
                         config.GRAMMAR_SCORE_MAX)
        self.assertEqual(GrammarResult.from_wire({"score": -5}).score, 0)

    def test_invalid_json_is_rejected_with_a_readable_error(self):
        with self.assertRaises(ValueError):
            GrammarResult.from_json_text("definitely not json")

    def test_issue_types_take_their_colour_from_config(self):
        self.assertEqual(GrammarIssueType.SPELLING.tint_color,
                         config.COLOR_GRAMMAR_SPELLING)
        self.assertEqual(GrammarIssueType.GRAMMAR.tint_color,
                         config.COLOR_GRAMMAR_GRAMMAR)


class CachedGrammarResponseTest(unittest.TestCase):
    """The backend serves a cached grammar_check as `rewritten_text`.

    rewrite.service.ts writes the Redis entry before the grammar branch, so a
    strict `grammar`-key check broke Grammar Check for the 5 minutes after
    each successful run, and on any second device.
    """

    ANALYSIS = {"score": 74, "issues": []}

    def test_the_live_shape_is_accepted(self):
        r = RewriteResult.from_wire({"grammar": self.ANALYSIS}, expects_grammar=True)
        self.assertEqual(GrammarResult.from_json_text(r.text).score, 74)

    def test_the_cached_shape_is_accepted(self):
        r = RewriteResult.from_wire(
            {"rewritten_text": json.dumps(self.ANALYSIS), "usage_today": 2},
            expects_grammar=True)
        self.assertEqual(GrammarResult.from_json_text(r.text).score, 74)
        self.assertEqual(r.usage_today, 2)

    def test_an_ordinary_rewrite_is_still_a_contract_error(self):
        with self.assertRaises(ValueError):
            RewriteResult.from_wire({"rewritten_text": "The cat is fast."},
                                    expects_grammar=True)

    def test_an_unrelated_json_object_is_not_an_analysis(self):
        # "Any JSON object" was too loose: GrammarResult defaults score to 0
        # and issues to empty, so the view rendered "0/100 · Needs work"
        # beside "Your writing looks great!" over unanalysed text.
        for body in ('{"corrected_text": "x"}', '{"error": "other"}', '{}'):
            with self.subTest(body=body):
                with self.assertRaises(ValueError):
                    RewriteResult.from_wire({"rewritten_text": body},
                                            expects_grammar=True)

    def test_text_tones_still_require_rewritten_text(self):
        with self.assertRaises(ValueError):
            RewriteResult.from_wire({"grammar": self.ANALYSIS})


class ToneCapabilityTest(unittest.TestCase):
    """Which tones return text, and which return an analysis."""

    def test_only_grammar_check_returns_an_analysis(self):
        self.assertTrue(Tone.GRAMMAR_CHECK.returns_grammar_analysis)
        for tone in Tone:
            if tone is not Tone.GRAMMAR_CHECK:
                self.assertFalse(tone.returns_grammar_analysis, tone)

    def test_replacement_text_is_the_complement(self):
        for tone in Tone:
            self.assertEqual(tone.produces_replacement_text,
                             not tone.returns_grammar_analysis, tone)

    def test_the_settings_chooser_offers_only_pasteable_tones(self):
        src = (Path(__file__).resolve().parent.parent / "draftright" / "ui"
               / "settings_window.py").read_text(encoding="utf-8")
        self.assertIn("produces_replacement_text", src)


class DiffCapTest(unittest.TestCase):
    """A document-sized selection must not stall the GTK main loop."""

    def test_oversized_input_degrades_instead_of_stalling(self):
        old_tokens, new_tokens = word_diff("a " * 40, "b " * 40, max_tokens=10)
        self.assertTrue(all(t.kind is DiffKind.DELETED for t in old_tokens))
        self.assertTrue(all(t.kind is DiffKind.INSERTED for t in new_tokens))

    def test_the_cap_is_reported_so_the_view_can_say_so(self):
        self.assertTrue(exceeds_diff_cap("a " * 40, "b " * 40, max_tokens=10))
        self.assertFalse(exceeds_diff_cap("a b c", "a b d"))

    def test_normal_input_still_diffs_word_by_word(self):
        old_tokens, new_tokens = word_diff("the quick brown fox",
                                           "the slow brown fox")
        self.assertEqual([t.text for t in old_tokens
                          if t.kind is DiffKind.DELETED], ["quick"])
        self.assertEqual([t.text for t in new_tokens
                          if t.kind is DiffKind.INSERTED], ["slow"])

    def test_the_cap_default_comes_from_config(self):
        self.assertGreater(config.DIFF_MAX_TOKENS_PER_SIDE, 0)

    def test_tints_come_from_config(self):
        self.assertEqual(DiffKind.DELETED.tint_color, config.COLOR_DIFF_DELETED)
        self.assertEqual(DiffKind.INSERTED.tint_color, config.COLOR_DIFF_INSERTED)
        self.assertIsNone(DiffKind.EQUAL.tint_color)


class RefreshFailureIsNotASignOutTest(unittest.TestCase):
    """Only a REJECTED refresh token may clear the session (#149).

    refresh_session() logged out on any exception. Once check_health() called
    it from the 30s background poll, one transient failure would erase the
    refresh token from the keyring unattended.
    """

    def _service(self, raises):
        api = mock.Mock()
        api.refresh.side_effect = raises
        service = AuthService(api)
        service._refresh_token = "stored-refresh"
        service._access_token = "stored-access"
        service.logout = mock.Mock()
        return service

    def test_a_network_failure_keeps_the_session(self):
        service = self._service(ConnectionError("reset"))
        self.assertIs(service.refresh_session(), RefreshOutcome.UNAVAILABLE)
        service.logout.assert_not_called()
        self.assertEqual(service._refresh_token, "stored-refresh")

    def test_a_server_error_keeps_the_session(self):
        service = self._service(APIError("[502]", status_code=502))
        self.assertIs(service.refresh_session(), RefreshOutcome.UNAVAILABLE)
        service.logout.assert_not_called()

    def test_a_rejected_token_signs_out(self):
        service = self._service(APIError("[401]", status_code=401))
        self.assertIs(service.refresh_session(), RefreshOutcome.REJECTED)
        service.logout.assert_called_once()

    def test_only_a_refresh_permits_the_retry(self):
        self.assertTrue(RefreshOutcome.REFRESHED.may_retry)
        self.assertFalse(RefreshOutcome.REJECTED.may_retry)
        self.assertFalse(RefreshOutcome.UNAVAILABLE.may_retry)


class HealthProbeTest(unittest.TestCase):
    """An expired access token is not a sign-out (#149)."""

    def _client(self, outcome):
        client = APIClient("https://example.invalid")
        client.set_token("expired")
        client.on_unauthorized = lambda: outcome
        self.calls = []

        def fake_get(url, **kwargs):
            self.calls.append(url)
            code = 200 if len(self.calls) == 1 else 401
            return mock.Mock(status_code=code, ok=(code == 200), reason="",
                             json=lambda: {"app": "draftright"})

        return client, fake_get

    def test_a_refreshable_session_reports_connected(self):
        client = APIClient("https://example.invalid")
        client.set_token("expired")
        client.on_unauthorized = lambda: RefreshOutcome.REFRESHED
        calls = []

        def fake_get(url, **kwargs):
            calls.append(url)
            code = 401 if len(calls) == 2 else 200
            return mock.Mock(status_code=code, ok=(code == 200), reason="",
                             json=lambda: {"app": "draftright"})

        with mock.patch.object(api_client_mod.requests, "get", fake_get):
            self.assertIs(client.check_health(), HealthStatus.CONNECTED)

    def test_an_unreachable_refresh_is_offline_not_signed_out(self):
        client, fake_get = self._client(RefreshOutcome.UNAVAILABLE)
        with mock.patch.object(api_client_mod.requests, "get", fake_get):
            self.assertIs(client.check_health(), HealthStatus.OFFLINE)

    def test_a_rejected_refresh_is_a_real_sign_out(self):
        client, fake_get = self._client(RefreshOutcome.REJECTED)
        with mock.patch.object(api_client_mod.requests, "get", fake_get):
            self.assertIs(client.check_health(), HealthStatus.NOT_LOGGED_IN)


class X11HotkeyPackagingTest(unittest.TestCase):
    """python-xlib is what makes the X11 hotkey exist at all (#150)."""

    ROOT = Path(__file__).resolve().parent.parent

    def test_pip_metadata_declares_it(self):
        for rel in ("requirements.txt", "setup.py"):
            self.assertIn("python-xlib",
                          (self.ROOT / rel).read_text(encoding="utf-8"), rel)

    def test_the_flatpak_ships_it_with_its_runtime_dep(self):
        src = (self.ROOT / "packaging" / "flatpak"
               / "com.draftright.app.yml").read_text(encoding="utf-8")
        self.assertIn("name: python3-xlib", src)
        self.assertIn("name: python3-six", src)

    def test_every_flatpak_archive_is_pinned_by_hash(self):
        src = (self.ROOT / "packaging" / "flatpak"
               / "com.draftright.app.yml").read_text(encoding="utf-8")
        self.assertEqual(src.count("type: archive"),
                         len(re.findall(r"sha256: [0-9a-f]{64}\b", src)))


class _StubX:
    """The handful of Xlib constants grab_hotkey touches."""

    GrabModeAsync = 1


class _StubDisplay:
    """Records every call into one shared log, so ordering is checkable."""

    def __init__(self, errors_on_sync=None):
        self.calls: list[str] = []
        self.handler = None
        self._errors = errors_on_sync or []

    def set_error_handler(self, handler):
        self.handler = handler

    def sync(self):
        self.calls.append("sync")
        # X reports grab failures asynchronously; they surface on the round trip.
        for err in self._errors:
            self.handler(err)


class _StubRoot:
    def __init__(self, display=None, raise_on_grab=False):
        self.display = display
        self.grabs: list[tuple] = []
        self._raise = raise_on_grab

    def grab_key(self, keycode, mask, owner_events, ptr_mode, kbd_mode):
        if self._raise:
            raise RuntimeError("grab exploded")
        self.grabs.append((keycode, mask))
        if self.display is not None:
            self.display.calls.append("grab")


class X11ListenerTest(unittest.TestCase):
    """The listener must send its grab, and stop() must actually stop (#151)."""

    def _run_body(self):
        src = (Path(__file__).resolve().parent.parent / "draftright"
               / "services" / "hotkey_service.py").read_text(encoding="utf-8")
        return src.split("def _run")[1].split("\nclass ")[0]

    def test_the_grabs_are_flushed_after_being_queued(self):
        # python-xlib buffers void requests; the old next_event() loop flushed
        # as a side effect. select() never writes, so without a round trip the
        # grab is never sent and the hotkey silently never fires while the app
        # logs "hotkey registered". Driven against a stub display so the
        # ordering is a fact, not a substring match.
        dpy = _StubDisplay()
        root = _StubRoot(display=dpy)
        errors = hotkey_service_mod.grab_hotkey(dpy, root, 27, 0x5, [0, 1], _StubX)
        self.assertEqual(errors, [])
        self.assertEqual(len(root.grabs), 2)
        self.assertEqual(dpy.calls, ["grab", "grab", "sync"],
                         "the grabs must be queued and then flushed, in order")

    def test_a_failed_grab_is_reported_not_swallowed(self):
        dpy, root = _StubDisplay(errors_on_sync=["BadAccess"]), _StubRoot()
        errors = hotkey_service_mod.grab_hotkey(dpy, root, 27, 0x5, [0], _StubX)
        self.assertEqual(errors, ["BadAccess"])

    def test_the_error_handler_is_restored(self):
        # Left installed it swallows every later error on the connection,
        # including a failing ungrab that leaves the key dead system-wide.
        dpy, root = _StubDisplay(), _StubRoot()
        hotkey_service_mod.grab_hotkey(dpy, root, 27, 0x5, [0], _StubX)
        self.assertIsNone(dpy.handler, "the grab handler outlived the grab")

    def test_the_handler_is_restored_even_when_the_grab_raises(self):
        dpy = _StubDisplay()
        root = _StubRoot(display=dpy, raise_on_grab=True)
        with self.assertRaises(RuntimeError):
            hotkey_service_mod.grab_hotkey(dpy, root, 27, 0x5, [0], _StubX)
        self.assertIsNone(dpy.handler)

    def test_the_queue_is_checked_before_blocking_on_the_socket(self):
        body = self._run_body()
        self.assertLess(body.index("pending_events()"), body.index("select.select"))

    def test_stop_wakes_the_thread_rather_than_waiting_it_out(self):
        src = (Path(__file__).resolve().parent.parent / "draftright"
               / "services" / "hotkey_service.py").read_text(encoding="utf-8")
        stop = src.split("    def stop")[1].split("\n    def ")[0]
        self.assertIn("os.write", stop, "stop() must wake the select()")
        self.assertIn("join(", stop)

    def test_the_join_timeout_outlives_the_wait_timeout(self):
        self.assertGreater(config.X11_STOP_JOIN_TIMEOUT,
                           config.X11_EVENT_WAIT_TIMEOUT)


class HotkeyRebindTest(unittest.TestCase):
    """Re-registering the same hotkey must not tear down a working binding."""

    def _service(self, active=True):
        service = HotkeyService()
        service._listener = mock.Mock()
        service._listener.is_active.return_value = active
        service._bound_keystring = "Ctrl+Shift+R"
        return service

    def test_rebinding_the_same_key_is_a_no_op(self):
        service = self._service()
        listener = service._listener
        service.start("Ctrl+Shift+R", lambda: None)
        listener.stop.assert_not_called()
        self.assertIs(service._listener, listener)

    def test_a_failed_binding_is_retried(self):
        # Re-activation is how the hotkey recovers once a conflicting app
        # quits or the portal comes up.
        service = self._service(active=False)
        listener = service._listener
        with mock.patch.object(hotkey_service_mod, "is_wayland", return_value=True):
            service.start("Ctrl+Shift+R", lambda: None)
        listener.stop.assert_called_once()
        self.assertIsNot(service._listener, listener)

    def test_changing_the_key_does_rebind(self):
        service = self._service()
        listener = service._listener
        with mock.patch.object(hotkey_service_mod, "is_wayland", return_value=True):
            service.start("Ctrl+Alt+P", lambda: None)
        listener.stop.assert_called_once()
        self.assertEqual(service._bound_keystring, "Ctrl+Alt+P")


class ResultViewStyleTest(unittest.TestCase):
    """Rule #1: the result views hold no colours of their own."""

    UI = Path(__file__).resolve().parent.parent / "draftright" / "ui"
    HEX = re.compile(r"#(?:[0-9a-fA-F]{6}(?:[0-9a-fA-F]{2})?"
                     r"|[0-9a-fA-F]*[a-fA-F][0-9a-fA-F]*)\b")

    def test_no_colour_literals_in_the_new_ui(self):
        for name in ("styles.py", "diff_view.py", "grammar_view.py"):
            self.assertEqual(
                self.HEX.findall((self.UI / name).read_text(encoding="utf-8")),
                [], f"{name} must take colours from config")

    def test_the_literal_detector_actually_detects(self):
        self.assertTrue(self.HEX.findall("color: #5d87ff;"))
        self.assertFalse(self.HEX.findall("see issue #107"))

    def test_the_models_take_their_tints_from_config(self):
        models = Path(__file__).resolve().parent.parent / "draftright" / "models"
        for name in ("diff.py", "grammar.py"):
            self.assertEqual(
                self.HEX.findall((models / name).read_text(encoding="utf-8")),
                [], f"{name} must take colours from config")

    def test_every_stylesheet_goes_through_the_once_per_display_guard(self):
        src = (self.UI / "styles.py").read_text(encoding="utf-8")
        self.assertIn("_loaded_displays", src.split("def _ensure")[1])
        for entry in ("def ensure_loaded", "def ensure_resource_css_loaded"):
            self.assertIn("_ensure(", src.split(entry)[1].split("\ndef ")[0])

    def test_the_application_no_longer_registers_css_itself(self):
        src = (self.UI.parent / "application.py").read_text(encoding="utf-8")
        self.assertNotIn("add_provider_for_display", src)
        self.assertIn("styles.ensure_resource_css_loaded()", src)

class TrayBusyPulseTest(unittest.TestCase):
    """One-Click has no window, so the tray is the only progress signal (#6).

    macOS and Windows show a spinner at the cursor; Wayland forbids a client
    placing its own surface, so the always-visible tray icon carries it.
    """

    def test_the_pulse_endpoints_are_design_tokens(self):
        self.assertEqual(tray_icon_render.busy_frame_color(0),
                         config.COLOR_BRAND_BLUE)
        self.assertEqual(
            tray_icon_render.busy_frame_color(config.TRAY_BUSY_FRAME_COUNT),
            config.COLOR_MUTED)

    def test_the_cycle_mirrors_so_it_breathes(self):
        cycle = config.TRAY_BUSY_FRAME_COUNT * 2
        self.assertEqual(tray_icon_render.busy_frame_color(1),
                         tray_icon_render.busy_frame_color(cycle - 1),
                         "the pulse snaps instead of easing back")

    def test_the_cycle_repeats_forever(self):
        cycle = config.TRAY_BUSY_FRAME_COUNT * 2
        self.assertEqual(tray_icon_render.busy_cycle_index(cycle * 7 + 3), 3)
        self.assertEqual(tray_icon_render.busy_frame_color(0),
                         tray_icon_render.busy_frame_color(cycle))

    def test_each_frame_gets_its_own_icon_name(self):
        # AppIndicator caches by name; a shared name would never repaint.
        names = {tray_icon_render.build_busy_frame(i, directory=self._dir())[1]
                 for i in range(config.TRAY_BUSY_FRAME_COUNT * 2)}
        self.assertEqual(len(names), config.TRAY_BUSY_FRAME_COUNT * 2)

    def _dir(self):
        if not hasattr(self, "_tmp"):
            import tempfile
            self._tmp = tempfile.mkdtemp()
        return self._tmp

    def test_the_busy_command_round_trips(self):
        from draftright.models.tray import TrayCommand
        line = TrayCommand.BUSY.encode("1")
        self.assertEqual(TrayCommand.parse(line), (TrayCommand.BUSY, "1"))

    def test_the_flag_and_the_pulse_move_together(self):
        # One place owns both, so the icon cannot keep spinning after the
        # rewrite has finished.
        from draftright.application import DraftRightApplication
        app = DraftRightApplication.__new__(DraftRightApplication)
        app._is_rewriting = False
        app._tray_icon = mock.Mock()

        DraftRightApplication._set_rewriting(app, True)
        self.assertTrue(app._is_rewriting)
        app._tray_icon.set_busy.assert_called_with(True)

        DraftRightApplication._set_rewriting(app, False)
        self.assertFalse(app._is_rewriting)
        app._tray_icon.set_busy.assert_called_with(False)

    def test_it_survives_a_missing_tray(self):
        # The tray is optional; One-Click must still run without it.
        from draftright.application import DraftRightApplication
        app = DraftRightApplication.__new__(DraftRightApplication)
        app._is_rewriting = False
        app._tray_icon = None
        DraftRightApplication._set_rewriting(app, True)
        self.assertTrue(app._is_rewriting)

if __name__ == "__main__":
    unittest.main()
