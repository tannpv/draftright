"""X11-only on-selection trigger (the "pencil") for Linux (#188).

Wayland forbids global input monitoring, so this runs on X11 only — the caller
gates on ``display_server.is_x11()``. Unlike macOS/Windows the Linux pencil
needs no synthetic copy and no floating overlay button:

- X11 auto-populates the PRIMARY selection whenever text is highlighted, so a
  drag-select is observable by polling PRIMARY — no global mouse hook;
- GTK4 cannot position a window at the cursor and Wayland forbids self-placement
  (#103), so there is no floating pencil button. A highlight instead polls
  straight through to the existing ``RewritePanel`` via the same capture +
  routing the hotkey uses (``read_selection`` = ``ClipboardService`` PRIMARY
  read, ``on_selection`` = the app's shared capture chokepoint). RULE #1: one
  capture path, one routing path, both triggers share them.

UNVERIFIED: built without a Linux X11 host (gi/PyGObject is not on the dev Mac).
The pure decision (``pencil_trigger_decision.should_trigger``) is unit-tested;
the polling loop needs a real X11 session, debugged via the app log.
"""

from __future__ import annotations

import logging
from typing import Callable, Optional

from gi.repository import GLib

from draftright.services.pencil_trigger_decision import should_trigger

log = logging.getLogger(__name__)

# Poll PRIMARY this often. Fast enough to feel instant after a drag-select,
# slow enough to stay idle-cheap (each read is a short `xsel`/`xclip` call).
DEFAULT_POLL_INTERVAL_MS = 400


class PencilTrigger:
    """Poll the X11 PRIMARY selection; open a rewrite on each new highlight."""

    def __init__(
        self,
        read_selection: Callable[[], str],
        on_selection: Callable[[str], None],
        poll_interval_ms: int = DEFAULT_POLL_INTERVAL_MS,
    ) -> None:
        self._read_selection = read_selection
        self._on_selection = on_selection
        self._poll_interval_ms = poll_interval_ms
        self._timeout_id: Optional[int] = None
        self._last_selection: Optional[str] = None

    def start(self) -> None:
        """Begin polling. Idempotent — a second call while running is a no-op."""
        if self._timeout_id is not None:
            return
        # Seed with the selection already present so an existing highlight at
        # start-up (or when the user switches to Pencil mode mid-selection)
        # doesn't immediately pop the panel.
        self._last_selection = self._safe_read()
        self._timeout_id = GLib.timeout_add(self._poll_interval_ms, self._poll)
        log.info(
            "Pencil trigger started (X11 PRIMARY polling every %dms)",
            self._poll_interval_ms,
        )

    def stop(self) -> None:
        """Stop polling. Idempotent."""
        if self._timeout_id is None:
            return
        GLib.source_remove(self._timeout_id)
        self._timeout_id = None
        log.info("Pencil trigger stopped")

    @property
    def running(self) -> bool:
        return self._timeout_id is not None

    def _safe_read(self) -> Optional[str]:
        try:
            return self._read_selection()
        except Exception as exc:  # noqa: BLE001 — a clipboard-tool hiccup
            log.debug("Pencil selection read failed: %s", exc)
            return None

    def _poll(self) -> bool:
        """GLib timeout callback — returns True to stay armed."""
        current = self._safe_read()
        if should_trigger(self._last_selection, current):
            self._on_selection(current)
        # Track every observed value (including blanks/clears) so the *next*
        # distinct highlight is what fires, never a re-poll of the same one.
        self._last_selection = current
        return True
