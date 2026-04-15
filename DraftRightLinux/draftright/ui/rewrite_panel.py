"""Floating rewrite panel window."""

import threading

import gi
gi.require_version("Gtk", "4.0")
gi.require_version("Gdk", "4.0")

from gi.repository import Gdk, Gio, GLib, Gtk, Pango

# Tone definitions: (id, emoji, label)
TONES = [
    ("simple", "\u270e", "Simple"),
    ("natural", "\U0001F4AC", "Natural"),
    ("polished", "\u2728", "Polished"),
    ("concise", "\u2296", "Concise"),
    ("technical", "\U0001F527", "Technical"),
    ("claude", "\U0001F916", "Claude Style"),
    ("grammar_check", "\u2713", "Grammar Check"),
    ("translate", "\U0001F30D", "Translate"),
]

# CSS for the rewrite panel
PANEL_CSS = """
.rewrite-panel {
    background-color: #0f172a;
    border: 1px solid #334155;
    border-radius: 12px;
}
.rewrite-header {
    color: #5d87ff;
    font-size: 16px;
    font-weight: bold;
}
.input-preview {
    color: #94a3b8;
    font-size: 13px;
}
.tone-button {
    background-color: #1e293b;
    color: #e2e8f0;
    border: 1px solid #334155;
    border-radius: 8px;
    padding: 8px 12px;
    font-size: 13px;
    min-height: 40px;
}
.tone-button:hover {
    background-color: #334155;
}
.tone-button-selected {
    background-color: #5d87ff;
    color: #ffffff;
    border: 1px solid #5d87ff;
    border-radius: 8px;
    padding: 8px 12px;
    font-size: 13px;
    min-height: 40px;
}
.result-area {
    background-color: #1e293b;
    color: #e2e8f0;
    border: 1px solid #334155;
    border-radius: 8px;
    padding: 8px;
    font-size: 14px;
}
.btn-primary {
    background-color: #5d87ff;
    color: #ffffff;
    border-radius: 8px;
    padding: 8px 16px;
    font-weight: bold;
}
.btn-primary:hover {
    background-color: #4a6fe0;
}
.btn-outlined {
    background-color: transparent;
    color: #5d87ff;
    border: 1px solid #5d87ff;
    border-radius: 8px;
    padding: 8px 16px;
}
.btn-outlined:hover {
    background-color: #1e293b;
}
.btn-ghost {
    background-color: transparent;
    color: #94a3b8;
    border: none;
    border-radius: 8px;
    padding: 8px 16px;
}
.btn-ghost:hover {
    background-color: #1e293b;
}
.error-label {
    color: #ef4444;
    font-size: 13px;
}
.copied-label {
    color: #10b981;
    font-size: 13px;
}
"""


class RewritePanel(Gtk.Window):
    """Floating panel for tone selection and text rewriting.

    Appears near the mouse cursor when invoked, shows tone buttons,
    calls the backend API, and displays the rewritten result.
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

        # Window properties
        self.set_default_size(420, 520)
        self.set_decorated(False)
        self.set_resizable(False)

        # Keep on top
        self.set_transient_for(None)

        # Load CSS
        self._load_css()

        # Build UI
        self._build_ui()

    def _load_css(self):
        """Load panel-specific CSS."""
        css_provider = Gtk.CssProvider()
        css_provider.load_from_string(PANEL_CSS)
        Gtk.StyleContext.add_provider_for_display(
            Gdk.Display.get_default(),
            css_provider,
            Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
        )

    def _build_ui(self):
        """Build the panel layout."""
        # Main container
        main_box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        main_box.set_margin_top(16)
        main_box.set_margin_bottom(16)
        main_box.set_margin_start(16)
        main_box.set_margin_end(16)
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

        for i, (tone_id, emoji, label) in enumerate(TONES):
            row = i // 3
            col = i % 3
            btn = Gtk.Button(label=f"{emoji} {label}")
            btn.add_css_class("tone-button")
            btn.set_hexpand(True)
            btn.connect("clicked", self._on_tone_clicked, tone_id)
            tone_grid.attach(btn, col, row, 1, 1)
            self._tone_buttons[tone_id] = btn

        main_box.append(tone_grid)

        # --- Loading spinner ---
        self._spinner = Gtk.Spinner()
        self._spinner.set_visible(False)
        main_box.append(self._spinner)

        # --- Result area ---
        scrolled = Gtk.ScrolledWindow()
        scrolled.set_vexpand(True)
        scrolled.set_min_content_height(120)

        self._result_view = Gtk.TextView()
        self._result_view.set_editable(False)
        self._result_view.set_cursor_visible(False)
        self._result_view.set_wrap_mode(Gtk.WrapMode.WORD_CHAR)
        self._result_view.add_css_class("result-area")
        scrolled.set_child(self._result_view)
        main_box.append(scrolled)

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
        action_box.set_halign(Gtk.Align.END)

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
        self._result_text = ""
        self._selected_tone = None

        # Update input preview
        preview = text[:200].replace("\n", " ")
        self._input_label.set_text(preview)

        # Clear previous result
        self._result_view.get_buffer().set_text("")
        self._error_box.set_visible(False)
        self._replace_btn.set_sensitive(False)
        self._copy_btn.set_sensitive(False)

        # Reset tone button styles
        for btn in self._tone_buttons.values():
            btn.remove_css_class("tone-button-selected")
            btn.add_css_class("tone-button")

        # Position near cursor (best effort -- Wayland may not expose pointer)
        self._position_near_cursor()

        self.set_visible(True)
        self.present()

    def on_replace(self):
        """Replace the original text with the rewritten result."""
        if self._result_text and self.app.clipboard_service:
            self.app.clipboard_service.inject_text(self._result_text)
        self._close()

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

            # Hide after 1.5 seconds
            GLib.timeout_add(1500, self._hide_copied_feedback)

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
        self._spinner.set_visible(True)
        self._spinner.start()
        self._error_box.set_visible(False)
        self._replace_btn.set_sensitive(False)
        self._copy_btn.set_sensitive(False)
        self._result_view.get_buffer().set_text("")

        def _do_request():
            try:
                if self.app.api_client is None:
                    raise RuntimeError("API client not initialized. Please sign in first.")

                result = self.app.api_client.rewrite(self._input_text, tone)
                GLib.idle_add(self._on_api_success, result)
            except Exception as exc:
                GLib.idle_add(self._on_api_error, str(exc))

        thread = threading.Thread(target=_do_request, daemon=True)
        thread.start()

    def _on_api_success(self, result: str):
        """Handle successful API response on the main thread.

        Args:
            result: The rewritten text.
        """
        self._spinner.stop()
        self._spinner.set_visible(False)
        self._result_text = result
        self._result_view.get_buffer().set_text(result)
        self._replace_btn.set_sensitive(True)
        self._copy_btn.set_sensitive(True)

    def _on_api_error(self, message: str):
        """Handle API error on the main thread.

        Args:
            message: The error message to display.
        """
        self._spinner.stop()
        self._spinner.set_visible(False)
        self._error_label.remove_css_class("copied-label")
        self._error_label.add_css_class("error-label")
        self._error_label.set_text(message)
        self._error_box.set_visible(True)

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

    def _position_near_cursor(self):
        """Position the window near the current mouse cursor."""
        # On X11 we can query pointer position; on Wayland this may not work
        # and the window manager decides placement.
        try:
            seat = Gdk.Display.get_default().get_default_seat()
            if seat:
                pointer = seat.get_pointer()
                if pointer:
                    # GTK4 does not expose pointer coords directly on the window.
                    # We rely on the window manager for placement in most cases.
                    pass
        except Exception:
            pass

    def _close(self):
        """Hide the panel."""
        self.set_visible(False)
