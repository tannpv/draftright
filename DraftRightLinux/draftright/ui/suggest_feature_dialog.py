"""Modal dialog letting the user suggest a feature → POST /feedback."""
from __future__ import annotations

import threading
from typing import Optional

import gi

gi.require_version("Gtk", "4.0")
gi.require_version("Adw", "1")
from gi.repository import Adw, GLib, Gtk  # noqa: E402

from ..services.feedback_service import submit_feature_request

_FEEDBACK_BOARD_URL = "https://draftright.info/feedback"

# (label, value) — value is what the backend expects.
_PLATFORMS: list[tuple[str, str]] = [
    ("Playground (web)", "playground"),
    ("Mobile (iOS / Android)", "mobile"),
    ("Windows", "windows"),
    ("macOS", "mac"),
    ("Linux", "linux"),
]


class SuggestFeatureDialog(Adw.Window):
    """libadwaita modal for submitting a feature request.

    Pass the optional ``bearer_token`` to authenticate the submission via
    the user's current session JWT.  When omitted the user's email field
    is used instead.
    """

    def __init__(
        self,
        parent: Optional[Gtk.Window] = None,
        bearer_token: Optional[str] = None,
    ) -> None:
        super().__init__(
            title="Suggest a Feature",
            modal=True,
            default_width=480,
            default_height=420,
        )
        if parent is not None:
            self.set_transient_for(parent)

        self._bearer_token = bearer_token

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
                label="<b>Suggest a feature</b>",
                use_markup=True,
                xalign=0,
                margin_bottom=4,
            )
        )

        # Title
        outer.append(self._field_label("Title"))
        self._title_entry = Gtk.Entry(
            placeholder_text="One line — what should we build?",
            max_length=80,
        )
        self._title_entry.connect("changed", self._refresh_submit_enabled)
        outer.append(self._title_entry)

        # Platform
        outer.append(self._field_label("Which platform is this for?"))
        labels = Gtk.StringList.new([lbl for lbl, _ in _PLATFORMS])
        self._platform_dropdown = Gtk.DropDown(model=labels)
        self._platform_dropdown.set_selected(4)  # linux selected by default
        outer.append(self._platform_dropdown)

        # Details
        outer.append(self._field_label("Details"))
        details_scroll = Gtk.ScrolledWindow(min_content_height=100, vexpand=True)
        self._details_buf = Gtk.TextBuffer()
        self._details_buf.connect("changed", self._refresh_submit_enabled)
        details_view = Gtk.TextView.new_with_buffer(self._details_buf)
        details_view.set_wrap_mode(Gtk.WrapMode.WORD_CHAR)
        details_scroll.set_child(details_view)
        outer.append(details_scroll)

        # Email — always shown; users with a valid session can leave it blank.
        outer.append(self._field_label("Email (optional — to follow up)"))
        self._email_entry = Gtk.Entry(placeholder_text="you@example.com")
        outer.append(self._email_entry)

        # Status label (errors)
        self._status_label = Gtk.Label(xalign=0, wrap=True, visible=False)
        self._status_label.add_css_class("error")
        outer.append(self._status_label)

        # Bottom bar: link on the left, Cancel + Submit on the right.
        bottom = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        outer.append(bottom)

        link = Gtk.LinkButton.new_with_label(_FEEDBACK_BOARD_URL, "See all requests →")
        link.set_halign(Gtk.Align.START)
        link.set_hexpand(True)
        bottom.append(link)

        cancel_btn = Gtk.Button(label="Cancel")
        cancel_btn.connect("clicked", lambda *_: self.close())
        bottom.append(cancel_btn)

        self._submit_btn = Gtk.Button(label="Submit request", sensitive=False)
        self._submit_btn.add_css_class("suggested-action")
        self._submit_btn.connect("clicked", self._on_submit)
        bottom.append(self._submit_btn)

    @staticmethod
    def _field_label(text: str) -> Gtk.Label:
        lbl = Gtk.Label(label=text, xalign=0)
        lbl.add_css_class("caption")
        return lbl

    def _refresh_submit_enabled(self, *_: object) -> None:
        title = self._title_entry.get_text().strip()
        start, end = self._details_buf.get_bounds()
        details = self._details_buf.get_text(start, end, False).strip()
        self._submit_btn.set_sensitive(bool(title) and bool(details))

    def _platform_value(self) -> str:
        return _PLATFORMS[self._platform_dropdown.get_selected()][1]

    def _on_submit(self, _btn: Gtk.Button) -> None:
        title = self._title_entry.get_text().strip()
        start, end = self._details_buf.get_bounds()
        details = self._details_buf.get_text(start, end, False).strip()
        email = self._email_entry.get_text().strip() or None
        platform = self._platform_value()

        self._submit_btn.set_sensitive(False)
        self._submit_btn.set_label("Submitting…")
        self._status_label.set_visible(False)

        def worker() -> None:
            try:
                submit_feature_request(
                    title=title,
                    target_platform=platform,
                    description=details,
                    user_email=email,
                    bearer_token=self._bearer_token,
                )
                GLib.idle_add(self._on_success)
            except Exception as exc:  # noqa: BLE001 — report whatever went wrong
                GLib.idle_add(self._on_error, str(exc))

        threading.Thread(target=worker, daemon=True).start()

    def _on_success(self) -> bool:
        self._toast_overlay.add_toast(
            Adw.Toast.new("Feature request submitted — thanks!")
        )
        # Give the toast a moment to be seen, then close.
        GLib.timeout_add(1200, self.close)
        return False  # don't repeat the idle callback

    def _on_error(self, msg: str) -> bool:
        self._status_label.set_text(f"Couldn't submit — {msg}")
        self._status_label.set_visible(True)
        self._submit_btn.set_label("Submit request")
        self._submit_btn.set_sensitive(True)
        return False


def open_suggest_feature_dialog(
    parent: Optional[Gtk.Window],
    bearer_token: Optional[str] = None,
) -> None:
    """Construct and present a ``SuggestFeatureDialog``.

    Args:
        parent: Transient parent window (may be ``None``).
        bearer_token: Optional JWT for authenticated submissions.
    """
    dlg = SuggestFeatureDialog(parent=parent, bearer_token=bearer_token)
    dlg.present()
