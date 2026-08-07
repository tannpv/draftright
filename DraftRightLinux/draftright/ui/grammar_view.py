"""Grammar-check result view: score, flagged text, and per-issue fixes (#107).

The macOS counterpart is ``DraftRight/UI/GrammarCheckView.swift``. Every range
comes from :mod:`draftright.services.grammar_fixer`, which resolves by content
rather than by the model's unreliable offsets; this widget only renders what
that returns and reports the corrected text back so the panel's Replace/Copy
act on the fixes rather than on the analysis JSON.
"""

from __future__ import annotations

from typing import Callable, Optional

import gi

gi.require_version("Gtk", "4.0")

from gi.repository import GLib, Gtk, Pango

from draftright import config
from draftright.models.grammar import GrammarIssue, GrammarIssueType, GrammarResult
from draftright.services import grammar_fixer
from draftright.ui import styles


class GrammarView(Gtk.Box):
    """Shows one :class:`GrammarResult` and applies its suggestions."""

    def __init__(self, on_text_changed: Optional[Callable[[str], None]] = None):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        styles.ensure_loaded()

        self._on_text_changed = on_text_changed
        self._result: Optional[GrammarResult] = None
        self._original = ""
        self._text = ""
        self._remaining: list[GrammarIssue] = []
        self._skipped: list[GrammarIssue] = []

        self.append(self._build_header())

        scrolled = Gtk.ScrolledWindow()
        scrolled.set_vexpand(True)
        scrolled.set_min_content_height(config.PANEL_GRAMMAR_MIN_HEIGHT)
        self._content = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=12)
        scrolled.set_child(self._content)
        self.append(scrolled)

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def set_result(self, original_text: str, result: GrammarResult) -> None:
        """Show *result* for *original_text*, discarding any previous state."""
        self._result = result
        self._original = original_text
        self._text = original_text
        self._remaining = list(result.issues)
        self._skipped = []
        self._refresh()

    @property
    def corrected_text(self) -> str:
        """The text with every applied fix folded in ("" before a result)."""
        return self._text

    def clear(self) -> None:
        """Drop the analysis so a stale one never outlives its input."""
        self._result = None
        self._original = self._text = ""
        self._remaining = []
        self._skipped = []
        self._refresh()

    # ------------------------------------------------------------------
    # Header
    # ------------------------------------------------------------------

    def _build_header(self) -> Gtk.Widget:
        header = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)

        self._score_label = Gtk.Label()
        self._score_label.add_css_class("grammar-score")
        self._score_label.set_halign(Gtk.Align.START)
        header.append(self._score_label)

        self._count_label = Gtk.Label()
        self._count_label.add_css_class("grammar-meta")
        self._count_label.set_hexpand(True)
        self._count_label.set_halign(Gtk.Align.END)
        header.append(self._count_label)

        self._fix_all_btn = Gtk.Button(label="Fix All")
        self._fix_all_btn.add_css_class("btn-outlined")
        self._fix_all_btn.add_css_class("btn-compact")
        self._fix_all_btn.set_valign(Gtk.Align.CENTER)
        self._fix_all_btn.connect("clicked", self._on_fix_all_clicked)
        header.append(self._fix_all_btn)

        return header

    # ------------------------------------------------------------------
    # Rendering
    # ------------------------------------------------------------------

    @property
    def _is_clean(self) -> bool:
        """True only when every issue was genuinely dealt with.

        Skipped issues count against this: reporting "All issues fixed!" while
        one could never be applied claimed a correction that never happened,
        over text Replace was about to paste.
        """
        return not self._remaining and not self._skipped

    def _refresh(self) -> None:
        """Rebuild everything from current state — the single render path."""
        _remove_children(self._content)

        if self._result is None:
            self._score_label.set_text("")
            self._count_label.set_text("")
            self._fix_all_btn.set_visible(False)
            return

        self._score_label.set_markup(
            _colored(self._result.score_display,
                     self._result.band.tint_color, bold=True)
        )
        # Only fall back to the original total while nothing has been acted
        # on; repeating it afterwards claimed fixed issues were outstanding,
        # with no card anywhere to act on them.
        if self._remaining:
            count_text = f"{len(self._remaining)} left"
        elif self._skipped:
            count_text = f"{len(self._skipped)} skipped"
        else:
            count_text = self._result.issue_count_display
        self._count_label.set_text(count_text)
        self._fix_all_btn.set_visible(bool(self._remaining))

        if self._is_clean:
            self._content.append(self._build_clean_section())
            return

        self._content.append(self._build_text_section())
        if self._skipped:
            self._content.append(self._build_skipped_notice())
        if self._remaining:
            self._content.append(
                Gtk.Separator(orientation=Gtk.Orientation.HORIZONTAL)
            )
            self._content.append(self._build_issues_section())

    def _build_clean_section(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=8)
        message = Gtk.Label(
            label=("All issues fixed!" if self._text != self._original
                   else "Your writing looks great!")
        )
        message.add_css_class("grammar-clean")
        message.set_halign(Gtk.Align.START)
        box.append(message)
        box.append(_text_view(self._text))
        return box

    def _build_skipped_notice(self) -> Gtk.Widget:
        count = len(self._skipped)
        label = Gtk.Label()
        label.add_css_class("grammar-meta")
        label.set_halign(Gtk.Align.START)
        label.set_xalign(0.0)
        label.set_wrap(True)
        label.set_markup(
            _colored(
                f"{count} suggestion{'' if count == 1 else 's'} no longer "
                f"matched the text and {'was' if count == 1 else 'were'} "
                f"skipped — an earlier fix had already changed that wording.",
                config.COLOR_WARNING,
            )
        )
        return label

    def _build_text_section(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        box.append(_heading("Your text"))

        view = _text_view(self._text)
        buffer = view.get_buffer()
        tags: dict[GrammarIssueType, Gtk.TextTag] = {}

        for issue in self._remaining:
            # Content-based, never by the model's offset — see grammar_fixer.
            found = grammar_fixer.resolve_range(issue, self._text)
            if found is None:
                continue
            start, length = found
            tag = tags.get(issue.issue_type)
            if tag is None:
                tag = _issue_tag(buffer, issue.issue_type)
                tags[issue.issue_type] = tag
            buffer.apply_tag(
                tag,
                buffer.get_iter_at_offset(start),
                buffer.get_iter_at_offset(start + length),
            )

        box.append(view)
        return box

    def _build_issues_section(self) -> Gtk.Widget:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        box.append(_heading("Issues found"))
        for issue in self._remaining:
            box.append(self._build_issue_card(issue))
        return box

    def _build_issue_card(self, issue: GrammarIssue) -> Gtk.Widget:
        card = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=10)
        card.add_css_class("grammar-card")

        details = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)
        details.set_hexpand(True)

        meta = Gtk.Label()
        meta.set_halign(Gtk.Align.START)
        meta.set_xalign(0.0)
        meta.set_ellipsize(Pango.EllipsizeMode.END)
        label = _colored(issue.issue_type.display_name,
                         issue.issue_type.tint_color, bold=True)
        if issue.reason:
            label += " · " + _colored(issue.reason, config.COLOR_MUTED)
        meta.set_markup(label)
        details.append(meta)

        change = Gtk.Label()
        change.set_halign(Gtk.Align.START)
        change.set_xalign(0.0)
        change.set_wrap(True)
        change.set_wrap_mode(Pango.WrapMode.WORD_CHAR)
        change.set_markup(
            "<s>" + _colored(issue.original, config.COLOR_MUTED) + "</s>  →  "
            + _colored(
                issue.suggestion or config.GRAMMAR_EMPTY_SUGGESTION_LABEL,
                issue.issue_type.tint_color, bold=True,
            )
        )
        details.append(change)
        card.append(details)

        fix_btn = Gtk.Button(label="Fix")
        fix_btn.add_css_class("btn-primary")
        fix_btn.add_css_class("btn-compact")
        fix_btn.set_valign(Gtk.Align.CENTER)
        fix_btn.connect("clicked", self._on_fix_clicked, issue)
        card.append(fix_btn)

        return card

    # ------------------------------------------------------------------
    # Actions
    # ------------------------------------------------------------------

    def _on_fix_clicked(self, _button, issue: GrammarIssue) -> None:
        if self._result is None:
            return
        if grammar_fixer.resolve_range(issue, self._text) is None:
            self._skipped.append(issue)
        else:
            self._text = grammar_fixer.apply_fix(self._text, issue)
        # Retire the clicked issue and any exact duplicate of it. Models repeat
        # an identical suggestion often, and leaving the twin behind gives it a
        # Fix button that can then only fail. Duplicates are matched by value,
        # never by offset+span alone — a spelling and a style fix over the same
        # words are different suggestions and must both survive.
        self._remaining = [
            i for i in self._remaining if i is not issue and i != issue
        ]
        self._refresh()
        self._notify_text_changed()

    def _on_fix_all_clicked(self, _button) -> None:
        if self._result is None:
            return
        outcome = grammar_fixer.apply_all(self._text, self._remaining)
        self._text = outcome.text
        self._skipped.extend(outcome.skipped)
        self._remaining = []
        self._refresh()
        self._notify_text_changed()

    def _notify_text_changed(self) -> None:
        if self._on_text_changed is not None:
            self._on_text_changed(self._text)


# ----------------------------------------------------------------------
# Small builders — shared by the sections above
# ----------------------------------------------------------------------


def _heading(title: str) -> Gtk.Label:
    label = Gtk.Label(label=title)
    label.add_css_class("section-heading")
    label.set_halign(Gtk.Align.START)
    return label


def _text_view(text: str) -> Gtk.TextView:
    view = Gtk.TextView()
    view.set_editable(False)
    view.set_cursor_visible(False)
    view.set_wrap_mode(Gtk.WrapMode.WORD_CHAR)
    view.add_css_class("result-area")
    # Nested in a scroller a TextView reports no natural height and collapses
    # to a few pixels — the text has to ask for room explicitly.
    view.set_size_request(-1, config.GRAMMAR_TEXT_MIN_HEIGHT)
    view.get_buffer().set_text(text)
    return view


def _issue_tag(buffer: Gtk.TextBuffer, kind: GrammarIssueType) -> Gtk.TextTag:
    """Squiggle + tint in the issue type's own colour."""
    return buffer.create_tag(
        kind.wire_value,
        underline=Pango.Underline.ERROR,
        underline_rgba=styles.rgba(kind.tint_color),
        background_rgba=styles.rgba(kind.tint_color,
                                    config.GRAMMAR_HIGHLIGHT_ALPHA),
    )


def _colored(text: str, color: str, bold: bool = False) -> str:
    """Pango markup in a design-token colour, with *text* escaped.

    Issue text comes from the model — unescaped, a stray ``<`` or ``&`` would
    break the markup and blank the label.
    """
    weight = ' weight="bold"' if bold else ""
    return (
        f'<span foreground="{color}"{weight}>'
        f"{GLib.markup_escape_text(text)}</span>"
    )


def _remove_children(box: Gtk.Box) -> None:
    """GTK4 has no ``remove_all`` on Gtk.Box."""
    child = box.get_first_child()
    while child is not None:
        following = child.get_next_sibling()
        box.remove(child)
        child = following
