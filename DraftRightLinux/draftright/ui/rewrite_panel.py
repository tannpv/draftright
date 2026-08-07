"""Floating rewrite panel window."""

import threading

import gi
gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")

from gi.repository import Gdk, GLib, Gtk, Pango

from draftright import config
from draftright.models.grammar import GrammarResult
from draftright.models.rewrite import RewriteResult
from draftright.models.tone import Tone
from draftright.ui import styles
from draftright.ui.diff_view import DiffView
from draftright.ui.grammar_view import GrammarView

# Result pages. Which one is shown is decided by the tone, not by the caller.
PAGE_TEXT = "text"
PAGE_DIFF = "diff"
PAGE_GRAMMAR = "grammar"


class RewritePanel(Gtk.Window):
    """Floating panel for tone selection and text rewriting.

    Shows tone buttons, calls the backend API, and displays the rewritten
    result.  Placement is the compositor's decision: GTK4 dropped client-side
    window positioning and Wayland does not let a client place its own
    surface, so the panel cannot follow the mouse cursor (#103).
    """

    def __init__(self, app):
        """Initialize the rewrite panel.

        Args:
            app: The DraftRightApplication instance.
        """
        super().__init__()
        self.app = app
        self._selected_tone = None
        self._input_text = ""
        self._result_text = ""
        # Bumped per tone click; responses carrying an older number are dropped.
        self._request_seq = 0
        # Whether _diff_view holds a diff of the current result (computed on
        # first view, not on arrival).
        self._diff_rendered = False

        # Window properties
        self.set_default_size(config.PANEL_WIDTH, config.PANEL_HEIGHT)
        self.set_decorated(False)
        self.set_resizable(False)

        # Keep on top
        self.set_transient_for(None)

        # Load CSS (shared with the diff / grammar result views)
        styles.ensure_loaded()

        # Build UI
        self._build_ui()

        # The window is undecorated, so the compositor gives it no close
        # button; Escape is the expected way out of a floating panel.
        key_controller = Gtk.EventControllerKey()
        key_controller.connect("key-pressed", self._on_key_pressed)
        self.add_controller(key_controller)

    def _build_ui(self):
        """Build the panel layout."""
        # Main container
        main_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        main_box.set_margin_top(config.PANEL_MARGIN)
        main_box.set_margin_bottom(config.PANEL_MARGIN)
        main_box.set_margin_start(config.PANEL_MARGIN)
        main_box.set_margin_end(config.PANEL_MARGIN)
        main_box.add_css_class("rewrite-panel")
        self.set_child(main_box)

        # --- Header ---
        header_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=0)
        header_label = Gtk.Label(label="DraftRight")
        header_label.add_css_class("rewrite-header")
        header_label.set_hexpand(True)
        header_label.set_halign(Gtk.Align.START)
        header_box.append(header_label)

        close_btn = Gtk.Button(icon_name="window-close-symbolic")
        close_btn.add_css_class("btn-ghost")
        close_btn.connect("clicked", lambda _: self._close())
        header_box.append(close_btn)
        main_box.append(header_box)

        # --- Input preview ---
        self._input_label = Gtk.Label(label="")
        self._input_label.add_css_class("input-preview")
        self._input_label.set_halign(Gtk.Align.START)
        self._input_label.set_wrap(True)
        self._input_label.set_wrap_mode(Pango.WrapMode.WORD_CHAR)
        self._input_label.set_max_width_chars(50)
        self._input_label.set_lines(2)
        self._input_label.set_ellipsize(Pango.EllipsizeMode.END)
        main_box.append(self._input_label)

        # --- Tone buttons (3x2 grid) ---
        tone_grid = Gtk.Grid()
        tone_grid.set_column_spacing(8)
        tone_grid.set_row_spacing(8)
        tone_grid.set_column_homogeneous(True)
        self._tone_buttons = {}

        cols = config.PANEL_TONE_GRID_COLUMNS
        for i, tone in enumerate(Tone):
            row = i // cols
            col = i % cols
            btn = Gtk.Button(label=f"{tone.icon} {tone.display_name}")
            btn.add_css_class("tone-button")
            btn.set_hexpand(True)
            btn.connect("clicked", self._on_tone_clicked, tone.api_value)
            tone_grid.attach(btn, col, row, 1, 1)
            self._tone_buttons[tone.api_value] = btn

        main_box.append(tone_grid)

        # --- Loading spinner ---
        self._spinner = Gtk.Spinner()
        self._spinner.set_visible(False)
        main_box.append(self._spinner)

        # --- Result area ---
        # Three interchangeable presentations of one result: the plain text,
        # the same text diffed against the input, and — for Grammar Check,
        # which returns an analysis rather than a rewrite — the issue list.
        # Non-homogeneous so the panel only widens on the page that needs it.
        self._result_stack = Gtk.Stack()
        self._result_stack.set_hhomogeneous(False)
        self._result_stack.set_vhomogeneous(False)
        self._result_stack.set_vexpand(True)

        scrolled = Gtk.ScrolledWindow()
        scrolled.set_vexpand(True)
        scrolled.set_min_content_height(config.PANEL_RESULT_MIN_HEIGHT)

        self._result_view = Gtk.TextView()
        self._result_view.set_editable(False)
        self._result_view.set_cursor_visible(False)
        self._result_view.set_wrap_mode(Gtk.WrapMode.WORD_CHAR)
        self._result_view.add_css_class("result-area")
        scrolled.set_child(self._result_view)
        self._result_stack.add_named(scrolled, PAGE_TEXT)

        self._diff_view = DiffView()
        self._result_stack.add_named(self._diff_view, PAGE_DIFF)

        self._grammar_view = GrammarView(
            on_text_changed=self._on_grammar_text_changed
        )
        self._result_stack.add_named(self._grammar_view, PAGE_GRAMMAR)

        main_box.append(self._result_stack)

        # --- Error / feedback row ---
        self._error_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self._error_box.set_visible(False)

        self._error_label = Gtk.Label(label="")
        self._error_label.add_css_class("error-label")
        self._error_label.set_halign(Gtk.Align.START)
        self._error_label.set_hexpand(True)
        self._error_label.set_wrap(True)
        self._error_label.set_selectable(True)
        self._error_box.append(self._error_label)

        self._copy_error_btn = Gtk.Button(label="Copy")
        self._copy_error_btn.add_css_class("btn-ghost")
        self._copy_error_btn.set_valign(Gtk.Align.CENTER)
        self._copy_error_btn.connect("clicked", self._on_copy_error)
        self._error_box.append(self._copy_error_btn)

        main_box.append(self._error_box)

        # --- Action buttons ---
        action_box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        action_box.set_halign(Gtk.Align.FILL)

        # Toggles the result between plain text and the before/after diff.
        # Hidden until there is something to compare, and for Grammar Check,
        # which has no rewritten text to diff.
        self._diff_toggle = Gtk.ToggleButton(label="Diff")
        self._diff_toggle.add_css_class("btn-outlined")
        self._diff_toggle.set_visible(False)
        self._diff_toggle.connect("toggled", self._on_diff_toggled)
        action_box.append(self._diff_toggle)

        # Keeps the actions right-aligned whether or not Diff is showing;
        # halign END on the row would have left-packed them once it hid.
        spacer = Gtk.Box()
        spacer.set_hexpand(True)
        action_box.append(spacer)

        self._replace_btn = Gtk.Button(label="Replace")
        self._replace_btn.add_css_class("btn-primary")
        self._replace_btn.connect("clicked", lambda _: self.on_replace())
        self._replace_btn.set_sensitive(False)
        action_box.append(self._replace_btn)

        self._copy_btn = Gtk.Button(label="Copy")
        self._copy_btn.add_css_class("btn-outlined")
        self._copy_btn.connect("clicked", lambda _: self.on_copy())
        self._copy_btn.set_sensitive(False)
        action_box.append(self._copy_btn)

        close_action_btn = Gtk.Button(label="Close")
        close_action_btn.add_css_class("btn-ghost")
        close_action_btn.connect("clicked", lambda _: self._close())
        action_box.append(close_action_btn)

        main_box.append(action_box)

    # ------------------------------------------------------------------
    # Public methods
    # ------------------------------------------------------------------

    def show_with_text(self, text: str):
        """Show the panel with the given input text, positioned near the mouse.

        Args:
            text: The selected text to rewrite.
        """
        self._input_text = text
        self._selected_tone = None
        # Supersede anything still in flight. The panel is cached and reused,
        # so without this a rewrite requested for the *previous* selection is
        # not stale when it lands: it would be shown as this selection's
        # result, diffed against the wrong original, and — with Replace
        # enabled — pasted over the text the user has selected now.
        self._request_seq += 1

        # Update input preview
        preview = text[: config.PANEL_PREVIEW_CHARS].replace("\n", " ")
        self._input_label.set_text(preview)

        # Clear previous result — including the other two pages, or a stale
        # diff / analysis would flash under the next tone's result.
        self._reset_result()
        self._error_box.set_visible(False)

        # Reset tone button styles
        for btn in self._tone_buttons.values():
            btn.remove_css_class("tone-button-selected")
            btn.add_css_class("tone-button")

        # No cursor-relative placement: GTK4 removed client-side window
        # positioning, and Wayland forbids a client from placing its own
        # surface at all.  The compositor decides where this lands (#103).
        self.set_visible(True)
        self.present()

    def on_replace(self):
        """Replace the original text with the rewritten result.

        On Wayland this may need permission via the RemoteDesktop portal, so
        the panel stays open until the keystroke is delivered — closing early
        would leave the user staring at unchanged text with no explanation.
        The result is on the clipboard regardless, so a denial degrades to a
        manual paste rather than losing the rewrite.
        """
        if not (self._result_text and self.app.clipboard_service):
            self._close()
            return

        self._replace_btn.set_sensitive(False)
        self.app.clipboard_service.inject_text(
            self._result_text,
            on_done=lambda ok: GLib.idle_add(self._on_replace_done, ok),
        )

    def _on_replace_done(self, delivered: bool) -> bool:
        """Close on success; explain and fall back to Copy on failure."""
        if delivered:
            self._close()
            return False

        self._replace_btn.set_sensitive(True)
        self._error_label.set_text(
            "Couldn't paste automatically — the result is on your clipboard, "
            "press Ctrl+V to insert it."
        )
        self._error_label.remove_css_class("copied-label")
        self._error_label.add_css_class("error-label")
        self._error_box.set_visible(True)
        return False  # don't repeat this idle callback

    def on_copy(self):
        """Copy the rewritten result to the clipboard."""
        if self._result_text:
            clipboard = Gdk.Display.get_default().get_clipboard()
            clipboard.set(self._result_text)

            # Show brief feedback
            self._error_label.set_text("Copied!")
            self._error_label.remove_css_class("error-label")
            self._error_label.add_css_class("copied-label")
            self._error_box.set_visible(True)

            GLib.timeout_add(config.FEEDBACK_FLASH_MS, self._hide_copied_feedback)

    # ------------------------------------------------------------------
    # Private methods
    # ------------------------------------------------------------------

    def _on_tone_clicked(self, _button, tone_id: str):
        """Handle tone button click.

        Args:
            tone_id: The selected tone identifier.
        """
        # Update button styles
        for tid, btn in self._tone_buttons.items():
            if tid == tone_id:
                btn.remove_css_class("tone-button")
                btn.add_css_class("tone-button-selected")
            else:
                btn.remove_css_class("tone-button-selected")
                btn.add_css_class("tone-button")

        self._selected_tone = tone_id
        self._call_api(tone_id)

    def _call_api(self, tone: str):
        """Call the rewrite API in a background thread.

        Args:
            tone: The tone to apply to the rewrite.
        """
        selected = Tone.from_api_value(tone)
        settings = self.app.settings_service
        # Only Translate depends on the language; sending it for the others
        # would fragment the cache for no benefit.
        language = (
            settings.translate_language
            if settings is not None
            and selected is not None
            and selected.uses_target_language
            else None
        )
        expects_grammar = selected is not None and selected.returns_grammar_analysis

        # Every click supersedes the one before it. Without this, clicking a
        # second tone while the first is still in flight lets the slower,
        # older response land last and overwrite the newer result.
        self._request_seq += 1
        seq = self._request_seq

        # Clear before the cache lookup too, so a hit cannot leave the previous
        # tone's error or result page showing underneath it.
        self._error_box.set_visible(False)
        self._reset_result()

        # Snapshot the input: the panel is reused, so a request outliving a
        # reopen must not key its cache entry on the *next* selection's text.
        text = self._input_text

        # Serve an identical (text, tone) instantly from the client cache —
        # no spinner, no backend round-trip. Mirrors macOS.
        cache = getattr(self.app, "rewrite_cache", None)
        if cache is not None:
            cached = cache.get(text, tone, language)
            if cached is not None:
                self._on_api_success(cached, seq, selected)
                return

        self._spinner.set_visible(True)
        self._spinner.start()

        def _do_request():
            try:
                if self.app.api_client is None:
                    raise RuntimeError("API client not initialized. Please sign in first.")

                payload = self.app.api_client.rewrite(text, tone, language)
                # /rewrite returns a dict; the views need the payload out of
                # it — the rewritten text, or the grammar analysis as JSON.
                result = RewriteResult.from_wire(
                    payload, expects_grammar=expects_grammar
                ).text
                if cache is not None:
                    # Keyed exactly as the lookup above: without the language
                    # every Translate result was written under a key that
                    # ``get`` never reads, so its cache never hit.
                    cache.set(text, tone, result, language)
                GLib.idle_add(self._on_api_success, result, seq, selected)
            except Exception as exc:
                GLib.idle_add(self._on_api_error, str(exc), seq)

        thread = threading.Thread(target=_do_request, daemon=True)
        thread.start()

    def _is_stale(self, seq) -> bool:
        """True when a newer tone click has already superseded this response."""
        return seq is not None and seq != self._request_seq

    def _on_api_success(self, result: str, seq=None, tone: "Tone | None" = None):
        """Handle successful API response on the main thread.

        Args:
            result: The rewritten text, or the grammar analysis as JSON when
                the tone returns one.
            seq: Request generation; a superseded response is dropped.
            tone: The tone this response was requested for. Passed in rather
                than re-read from ``_selected_tone``, which is mutable and may
                already describe a different request by the time this runs —
                reading it back could route a grammar payload into the plain
                text view, pasting raw JSON over the user's selection.
        """
        if self._is_stale(seq):
            return False

        self._spinner.stop()
        self._spinner.set_visible(False)

        if tone is not None and tone.returns_grammar_analysis:
            self._show_grammar(result)
        else:
            self._show_rewrite(result)
        return False  # don't repeat this idle callback

    def _show_rewrite(self, result: str):
        """Present rewritten text: the plain result, and a diff on demand."""
        self._result_text = result
        self._result_view.get_buffer().set_text(result)
        self._diff_toggle.set_visible(True)
        # Not diffed yet — word_diff is an O(m×n) table on the main loop, and
        # most results are never toggled to the diff page at all.
        self._diff_rendered = False
        self._show_result_page(
            PAGE_DIFF if self._diff_toggle.get_active() else PAGE_TEXT
        )
        self._replace_btn.set_sensitive(True)
        self._copy_btn.set_sensitive(True)

    def _show_result_page(self, page: str):
        """Switch pages, computing the diff the first time it is actually shown."""
        if page == PAGE_DIFF and not self._diff_rendered:
            self._diff_view.set_texts(self._input_text, self._result_text)
            self._diff_rendered = True
        self._result_stack.set_visible_child_name(page)

    def _show_grammar(self, payload: str):
        """Present a grammar analysis: score, flagged text, per-issue fixes."""
        try:
            result = GrammarResult.from_json_text(payload)
        except ValueError as exc:
            self._on_api_error(f"Grammar check returned an unreadable result: {exc}")
            return

        # The backend answers with score 0 and an empty issue list when the
        # model's JSON did not parse; that is a failure, not a perfect zero.
        if result.error:
            self._on_api_error(result.error)
            return

        # Nothing to diff and nothing to toggle — this page is the result.
        # Only hidden, not un-toggled, so the user's diff preference survives
        # a detour through Grammar Check.
        self._diff_toggle.set_visible(False)

        self._grammar_view.set_result(self._input_text, result)
        self._show_result_page(PAGE_GRAMMAR)

        # Replace / Copy act on the corrected text, which starts out as the
        # input and gains each fix as it is applied.
        self._result_text = self._grammar_view.corrected_text
        self._replace_btn.set_sensitive(True)
        self._copy_btn.set_sensitive(True)

    def _on_grammar_text_changed(self, text: str):
        """Keep Replace / Copy pointed at the latest corrected text."""
        self._result_text = text

    def _on_diff_toggled(self, button: Gtk.ToggleButton):
        """Switch the result area between the plain text and the diff."""
        self._show_result_page(PAGE_DIFF if button.get_active() else PAGE_TEXT)

    def _reset_result(self):
        """Clear every result page and disable the actions that act on one."""
        # Stop the spinner here, not only when a response lands: a superseded
        # response returns at the staleness check before it would ever reach
        # _spinner.stop(), so a panel reopened while a request was in flight
        # spin forever with no result and no error.
        self._spinner.stop()
        self._spinner.set_visible(False)
        self._result_text = ""
        self._diff_rendered = False
        self._result_view.get_buffer().set_text("")
        self._diff_view.clear()
        self._grammar_view.clear()
        self._diff_toggle.set_visible(False)
        self._result_stack.set_visible_child_name(PAGE_TEXT)
        self._replace_btn.set_sensitive(False)
        self._copy_btn.set_sensitive(False)

    def _on_api_error(self, message: str, seq=None):
        """Handle API error on the main thread.

        Args:
            message: The error message to display.
            seq: Request generation; a superseded failure is dropped, so a
                slow error cannot bury a newer tone's successful result.
        """
        if self._is_stale(seq):
            return False

        self._spinner.stop()
        self._spinner.set_visible(False)
        self._error_label.remove_css_class("copied-label")
        self._error_label.add_css_class("error-label")
        self._error_label.set_text(message)
        self._error_box.set_visible(True)
        return False  # don't repeat this idle callback

    def _hide_copied_feedback(self):
        """Hide the 'Copied!' feedback label."""
        self._error_box.set_visible(False)
        return GLib.SOURCE_REMOVE

    def _on_copy_error(self, _button):
        """Copy the error message to clipboard."""
        error_text = self._error_label.get_text()
        if error_text:
            clipboard = Gdk.Display.get_default().get_clipboard()
            clipboard.set(error_text)

    def _on_key_pressed(self, _controller, keyval, _keycode, _state):
        """Escape closes the panel."""
        if keyval == Gdk.KEY_Escape:
            self._close()
            return True   # handled
        return False

    def _close(self):
        """Hide the panel."""
        self.set_visible(False)
