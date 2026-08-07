"""Side-by-side before/after diff for a rewrite (#107).

The macOS counterpart is ``DraftRight/UI/DiffView.swift`` and the Windows one
``DiffView`` in the rewrite panel: original on the left with removed words
struck through in red, rewrite on the right with added words highlighted in
green. All the reasoning lives in :mod:`draftright.models.diff`; this widget
only paints the tokens it is handed.
"""

from __future__ import annotations

from dataclasses import dataclass

import gi

gi.require_version("Gtk", "4.0")

from gi.repository import Gtk

from draftright import config
from draftright.models.diff import (
    DiffKind,
    DiffToken,
    exceeds_diff_cap,
    word_diff,
)
from draftright.ui import styles


@dataclass(frozen=True)
class _Column:
    """One side of the diff and the tag it highlights its own changes with."""

    buffer: Gtk.TextBuffer
    tag: Gtk.TextTag
    kind: DiffKind


class DiffView(Gtk.Box):
    """Renders a word diff as two scrollable, selectable columns."""

    def __init__(self):
        super().__init__(orientation=Gtk.Orientation.VERTICAL, spacing=6)
        styles.ensure_loaded()

        self._notice = Gtk.Label()
        self._notice.add_css_class("diff-notice")
        self._notice.set_halign(Gtk.Align.START)
        self._notice.set_wrap(True)
        self._notice.set_visible(False)
        self.append(self._notice)

        columns = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL, spacing=8)
        columns.set_homogeneous(True)
        columns.set_vexpand(True)
        self.append(columns)

        self._old = self._build_column(columns, "Original", DiffKind.DELETED)
        self._new = self._build_column(columns, "Rewritten", DiffKind.INSERTED)

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    def set_texts(self, original: str, rewritten: str) -> None:
        """Diff the pair and render it."""
        old_tokens, new_tokens = word_diff(original, rewritten)
        self._render(self._old, old_tokens)
        self._render(self._new, new_tokens)

        # Say so when the text was too long to diff word-by-word, rather than
        # letting a wholesale red/green wall read as "everything changed".
        truncated = exceeds_diff_cap(original, rewritten)
        self._notice.set_visible(truncated)
        if truncated:
            self._notice.set_text(
                "Text too long for a word-by-word comparison — showing the "
                "whole rewrite as changed."
            )

    def clear(self) -> None:
        """Drop both sides, so a stale diff never flashes on the next result."""
        for column in (self._old, self._new):
            column.buffer.set_text("")
        self._notice.set_visible(False)

    # ------------------------------------------------------------------
    # Internals
    # ------------------------------------------------------------------

    def _build_column(
        self, parent: Gtk.Box, title: str, kind: DiffKind
    ) -> _Column:
        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=4)

        heading = Gtk.Label(label=title)
        heading.add_css_class("section-heading")
        heading.set_halign(Gtk.Align.START)
        box.append(heading)

        scrolled = Gtk.ScrolledWindow()
        scrolled.set_vexpand(True)
        scrolled.set_min_content_width(config.PANEL_DIFF_COLUMN_MIN_WIDTH)
        scrolled.set_min_content_height(config.PANEL_RESULT_MIN_HEIGHT)

        view = Gtk.TextView()
        view.set_editable(False)
        view.set_cursor_visible(False)
        view.set_wrap_mode(Gtk.WrapMode.WORD_CHAR)
        view.add_css_class("result-area")
        scrolled.set_child(view)

        box.append(scrolled)
        parent.append(box)

        buffer = view.get_buffer()
        return _Column(buffer=buffer, tag=self._build_tag(buffer, kind), kind=kind)

    @staticmethod
    def _build_tag(buffer: Gtk.TextBuffer, kind: DiffKind) -> Gtk.TextTag:
        """One reusable tag per column, coloured from the kind's own token."""
        return buffer.create_tag(
            kind.wire_value,
            background_rgba=styles.rgba(
                kind.tint_color, config.DIFF_HIGHLIGHT_ALPHA
            ),
            foreground_rgba=styles.rgba(config.COLOR_WHITE),
            strikethrough=(kind is DiffKind.DELETED),
        )

    @staticmethod
    def _render(column: _Column, tokens: list[DiffToken]) -> None:
        buffer = column.buffer
        buffer.set_text("")
        for token in tokens:
            if token.kind is column.kind:
                buffer.insert_with_tags(
                    buffer.get_end_iter(), token.text, column.tag
                )
            elif token.kind is DiffKind.EQUAL:
                buffer.insert(buffer.get_end_iter(), token.text)
            # A token belonging to the *other* side never appears here; by
            # construction each side carries only EQUAL plus its own kind.
