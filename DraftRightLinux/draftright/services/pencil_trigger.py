"""X11-only on-selection trigger (the "pencil") for Linux (#188).

Wayland forbids global input monitoring, so this runs on X11 only — the caller
gates on ``display_server.is_x11()``. Unlike macOS/Windows the Linux pencil
needs no synthetic copy:

- X11 auto-populates the PRIMARY selection whenever text is highlighted, so a
  drag-select is observable by polling PRIMARY — no global mouse hook;
- a highlight raises the pencil bubble at the pointer (``ui/pencil_bubble``) and
  only a click on it routes the text onward, so a stray selection costs nothing.
  #103 said GTK4 cannot place a window at the cursor — that holds for Wayland,
  which forbids self-placement, but **not for X11**, where the WM honours a move
  on the underlying X window. This trigger is X11-only, so it can.
- ``on_selection`` is what the bubble's click ultimately reaches: the app's
  shared routing chokepoint, the same one the hotkey funnels into. RULE #1: one
  routing path, both triggers share it.

``read_selection`` must be **side-effect free**: this runs on a timer, so it may
only *observe* the selection. ``ClipboardService.get_primary_selection`` is that
read. Its sibling ``get_selected_text`` is not — when PRIMARY is empty it
synthesises Ctrl+C and rewrites CLIPBOARD, which on a poll loop means a
keystroke injected into the focused window every tick (SIGINT in a terminal) and
a rewrite fired from stale clipboard text.

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
