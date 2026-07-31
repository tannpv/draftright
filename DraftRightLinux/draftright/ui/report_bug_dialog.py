"""libadwaita modal for reporting a bug (#98).

Deliberately shaped like :mod:`draftright.ui.suggest_feature_dialog` — same
layout, threading and toast conventions — so the two feedback paths look and
behave alike.  Adds an optional screenshot attachment, which the feature
dialog has no use for.
"""

from __future__ import annotations

import os
import tempfile
import threading
import time
from typing import Optional

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Adw", "1")
from gi.repository import Adw, Gdk, GLib, Gtk, Pango

from draftright import config
from pathlib import Path
from draftright.services.bug_report_service import (
    MAX_SCREENSHOT_BYTES,
    MIN_DESCRIPTION_CHARS,
    submit_bug_report,
)

# Kept beside the service's _MIME_BY_SUFFIX map: the picker offers exactly
# what the uploader knows how to label.
_IMAGE_MIME_TYPES = ("image/png", "image/jpeg", "image/gif", "image/webp")


class ReportBugDialog(Adw.Window):
    """Modal for submitting a bug report.

    Pass ``bearer_token`` to attribute the report to the signed-in user; when
    omitted the email field is used instead, and the backend accepts the
    report anonymously.
    """

    def __init__(
        self,
        parent: Optional[Gtk.Window] = None,
        bearer_token: Optional[str] = None,
        user_email: Optional[str] = None,
    ) -> None:
        super().__init__(
            title="Report a Bug",
            modal=True,
            default_width=480,
            default_height=460,
        )
        if parent is not None:
            self.set_transient_for(parent)

        self._bearer_token = bearer_token
        self._screenshot_path: Optional[str] = None

        toast_overlay = Adw.ToastOverlay()
        self.set_content(toast_overlay)
        self._toast_overlay = toast_overlay

        outer = Gtk.Box(
            orientation=Gtk.Orientation.VERTICAL,
            spacing=12,
            margin_start=18,
            margin_end=18,
            margin_top=18,
            margin_bottom=18,
        )
        toast_overlay.set_child(outer)

        outer.append(
            Gtk.Label(
                label="<b>Report a bug</b>",
                use_markup=True,
                xalign=0,
                margin_bottom=4,
            )
        )

        # Description
        outer.append(
            self._field_label("What went wrong? Steps to reproduce help most.")
        )
        details_scroll = Gtk.ScrolledWindow(min_content_height=140, vexpand=True)
        self._details_buf = Gtk.TextBuffer()
        self._details_buf.connect("changed", self._refresh_submit_enabled)
        details_view = Gtk.TextView.new_with_buffer(self._details_buf)
        details_view.set_wrap_mode(Gtk.WrapMode.WORD_CHAR)
        details_scroll.set_child(details_view)
        outer.append(details_scroll)

        # Screenshot (optional)
        outer.append(self._field_label("Screenshot (optional)"))
        shot_row = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        self._shot_label = Gtk.Label(label="No file selected", xalign=0, hexpand=True)
        self._shot_label.add_css_class("dim-label")
        self._shot_label.set_ellipsize(Pango.EllipsizeMode.END)
        shot_row.append(self._shot_label)

        # Capture without leaving the app — no external screenshot tool.
        self._capture_btn = Gtk.Button(label="Take screenshot")
        self._capture_btn.set_tooltip_text(
            "Pick an area, window or the whole screen; DraftRight hides itself first"
        )
        self._capture_btn.connect("clicked", self._on_capture)
        shot_row.append(self._capture_btn)

        self._paste_btn = Gtk.Button(label="Paste")
        self._paste_btn.set_tooltip_text(
            "Attach an image from the clipboard (Ctrl+V) — e.g. after Print Screen"
        )
        self._paste_btn.connect("clicked", lambda _b: self._paste_from_clipboard())
        shot_row.append(self._paste_btn)

        attach_btn = Gtk.Button(label="Attach…")
        attach_btn.connect("clicked", self._on_attach)
        shot_row.append(attach_btn)

        self._clear_shot_btn = Gtk.Button(label="Remove", sensitive=False)
        self._clear_shot_btn.connect("clicked", self._on_clear_screenshot)
        shot_row.append(self._clear_shot_btn)
        outer.append(shot_row)

        # Preview — the whole point of attaching is knowing what you attached.
        # Windows and macOS both show one; a filename alone tells the user
        # nothing about whether they captured the right thing.
        self._preview_frame = Gtk.Frame()
        self._preview_frame.set_visible(False)
        self._preview = Gtk.Picture()
        self._preview.set_can_shrink(True)
        self._preview.set_content_fit(Gtk.ContentFit.CONTAIN)
        self._preview.set_size_request(-1, config.BUG_REPORT_PREVIEW_HEIGHT)
        self._preview_frame.set_child(self._preview)
        outer.append(self._preview_frame)

        # Email — always shown; signed-in users can leave it blank.
        outer.append(self._field_label("Email (optional — to follow up)"))
        self._email_entry = Gtk.Entry(placeholder_text="you@example.com")
        if user_email:
            self._email_entry.set_text(user_email)
        outer.append(self._email_entry)

        self._status_label = Gtk.Label(xalign=0, wrap=True, visible=False)
        self._status_label.add_css_class("error")
        outer.append(self._status_label)

        bottom = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        outer.append(bottom)

        link = Gtk.LinkButton.new_with_label(
            config.FEEDBACK_BOARD_URL, "See known issues →"
        )
        link.set_halign(Gtk.Align.START)
        link.set_hexpand(True)
        bottom.append(link)

        cancel_btn = Gtk.Button(label="Cancel")
        cancel_btn.connect("clicked", lambda *_: self.close())
        bottom.append(cancel_btn)

        # Ctrl+V attaches a clipboard image. Print Screen on GNOME puts the
        # shot straight on the clipboard, so this is the path most people
        # already have muscle memory for.
        paste_keys = Gtk.EventControllerKey()
        paste_keys.connect("key-pressed", self._on_key_pressed)
        self.add_controller(paste_keys)

        self._submit_btn = Gtk.Button(label="Send report", sensitive=False)
        self._submit_btn.add_css_class("suggested-action")
        self._submit_btn.connect("clicked", self._on_submit)
        bottom.append(self._submit_btn)

    # -- helpers -----------------------------------------------------------

    @staticmethod
    def _field_label(text: str) -> Gtk.Label:
        lbl = Gtk.Label(label=text, xalign=0)
        lbl.add_css_class("caption")
        return lbl

    def _description(self) -> str:
        start, end = self._details_buf.get_bounds()
        return self._details_buf.get_text(start, end, False).strip()

    def _refresh_submit_enabled(self, *_: object) -> None:
        # Mirror the service's own guard so the button explains itself rather
        # than letting the user submit into a ValueError.
        self._submit_btn.set_sensitive(
            len(self._description()) >= MIN_DESCRIPTION_CHARS
        )

    # -- screenshot --------------------------------------------------------

    def _on_key_pressed(self, _controller, keyval, _keycode, state) -> bool:
        """Ctrl+V pastes a clipboard image as the attachment."""
        ctrl = bool(state & Gdk.ModifierType.CONTROL_MASK)
        if ctrl and keyval in (Gdk.KEY_v, Gdk.KEY_V):
            # Only claim the keystroke if the clipboard actually holds an
            # image; otherwise let Ctrl+V paste text into the description.
            if self._clipboard_has_image():
                self._paste_from_clipboard()
                return True
        return False

    def _clipboard_has_image(self) -> bool:
        clipboard = Gdk.Display.get_default().get_clipboard()
        formats = clipboard.get_formats()
        return any(formats.contain_mime_type(m) for m in _IMAGE_MIME_TYPES)

    def _paste_from_clipboard(self) -> None:
        """Read an image off the clipboard and attach it."""
        clipboard = Gdk.Display.get_default().get_clipboard()

        def on_read(cb, result):
            try:
                texture = cb.read_texture_finish(result)
            except GLib.Error as exc:
                self._show_error(f"No image on the clipboard ({exc.message}).")
                return
            if texture is None:
                self._show_error("No image on the clipboard.")
                return
            try:
                path = Path(tempfile.gettempdir()) / (
                    f"draftright-pasted-{os.getpid()}-{int(time.time())}.png"
                )
                texture.save_to_png(str(path))
                if path.stat().st_size > MAX_SCREENSHOT_BYTES:
                    self._show_error("Pasted image is larger than 5 MB.")
                    return
            except (OSError, GLib.Error) as exc:
                self._show_error(f"Couldn't save the pasted image: {exc}")
                return
            self._set_screenshot(str(path))

        clipboard.read_texture_async(None, on_read)

    def _on_capture(self, _btn: Gtk.Button) -> None:
        """Screenshot via the desktop portal, hiding this dialog first."""
        from draftright.services.screenshot_service import ScreenshotService

        self._capture_btn.set_sensitive(False)
        self._capture_btn.set_label("Choose an area…")
        self._status_label.set_visible(False)

        # Otherwise this window sits in the middle of the user's own bug.
        self.set_visible(False)

        def finish(path, error):
            # Portal replies arrive on the main loop already, but idle_add
            # keeps this correct regardless of where it is dispatched from.
            GLib.idle_add(self._on_capture_done, path, error)

        # Give the compositor a moment to actually unmap us before the
        # portal starts capturing.
        GLib.timeout_add(
            config.SCREENSHOT_HIDE_DELAY_MS,
            lambda: (ScreenshotService().capture(finish), False)[1],
        )

    def _on_capture_done(self, path, error) -> bool:
        self.set_visible(True)
        self.present()
        self._capture_btn.set_sensitive(True)
        self._capture_btn.set_label("Take screenshot")

        if error:
            self._show_error(error)
            return False
        if not path:
            return False  # user cancelled — nothing to say

        try:
            size = Path(path).stat().st_size
        except OSError as exc:
            self._show_error(f"Couldn't read the screenshot: {exc}")
            return False
        if size > MAX_SCREENSHOT_BYTES:
            self._show_error("Screenshot is larger than 5 MB.")
            return False

        self._set_screenshot(path)
        return False

    def _set_screenshot(self, path: str) -> None:
        """Adopt *path* as the attachment and reflect it in the UI."""
        self._screenshot_path = path
        self._shot_label.set_text(self._describe(path))
        self._shot_label.remove_css_class("dim-label")
        self._clear_shot_btn.set_sensitive(True)
        self._status_label.set_visible(False)

        self._preview.set_filename(path)
        self._preview_frame.set_visible(True)

    @staticmethod
    def _describe(path: str) -> str:
        """"name.png — 1920x1080, 240 KB", falling back to just the name."""
        name = Path(path).name
        try:
            size_kb = max(1, Path(path).stat().st_size // 1024)
        except OSError:
            return name
        try:
            texture = Gdk.Texture.new_from_filename(path)
            return f"{name} — {texture.get_width()}x{texture.get_height()}, {size_kb} KB"
        except GLib.Error:
            # Not decodable as an image; the size is still worth showing.
            return f"{name} — {size_kb} KB"

    def _on_attach(self, _btn: Gtk.Button) -> None:
        image_filter = Gtk.FileFilter()
        image_filter.set_name("Images")
        for mime in _IMAGE_MIME_TYPES:
            image_filter.add_mime_type(mime)

        dialog = Gtk.FileDialog(title="Choose a screenshot")
        dialog.set_default_filter(image_filter)
        dialog.open(self, None, self._on_attach_chosen)

    def _on_attach_chosen(self, dialog: Gtk.FileDialog, result) -> None:
        try:
            gfile = dialog.open_finish(result)
        except GLib.Error:
            return  # user cancelled
        if gfile is None:
            return
        path = gfile.get_path()
        if path is None:
            self._show_error("That file isn't available locally.")
            return
        try:
            size = gfile.query_info("standard::size", 0, None).get_size()
        except GLib.Error:
            size = 0
        if size > MAX_SCREENSHOT_BYTES:
            self._show_error("Screenshot is larger than 5 MB.")
            return
        self._set_screenshot(path)

    def _on_clear_screenshot(self, _btn: Gtk.Button) -> None:
        self._screenshot_path = None
        self._shot_label.set_text("No file selected")
        self._shot_label.add_css_class("dim-label")
        self._clear_shot_btn.set_sensitive(False)
        self._preview.set_filename(None)
        self._preview_frame.set_visible(False)

    # -- submit ------------------------------------------------------------

    def _on_submit(self, _btn: Gtk.Button) -> None:
        description = self._description()
        email = self._email_entry.get_text().strip() or None
        screenshot = self._screenshot_path

        self._submit_btn.set_sensitive(False)
        self._submit_btn.set_label("Sending…")
        self._status_label.set_visible(False)

        def worker() -> None:
            try:
                submit_bug_report(
                    description=description,
                    screenshot_path=screenshot,
                    user_email=email,
                    bearer_token=self._bearer_token,
                )
                GLib.idle_add(self._on_success)
            except Exception as exc:  # noqa: BLE001 — surface whatever failed
                GLib.idle_add(self._on_error, str(exc))

        threading.Thread(target=worker, daemon=True).start()

    def _on_success(self) -> bool:
        self._toast_overlay.add_toast(Adw.Toast.new("Bug report sent — thanks!"))
        GLib.timeout_add(1200, self.close)
        return False

    def _on_error(self, msg: str) -> bool:
        self._show_error(f"Couldn't send — {msg}")
        self._submit_btn.set_label("Send report")
        self._submit_btn.set_sensitive(True)
        return False

    def _show_error(self, msg: str) -> None:
        self._status_label.set_text(msg)
        self._status_label.set_visible(True)


def open_report_bug_dialog(
    parent: Optional[Gtk.Window],
    bearer_token: Optional[str] = None,
    user_email: Optional[str] = None,
) -> None:
    """Construct and present a :class:`ReportBugDialog`."""
    ReportBugDialog(
        parent=parent, bearer_token=bearer_token, user_email=user_email
    ).present()
