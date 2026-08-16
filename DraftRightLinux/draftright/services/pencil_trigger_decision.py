"""Pure rule: should an X11 PRIMARY-selection change fire the pencil rewrite?

Kept free of GTK/X11 so it is unit-testable without a display, mirroring the
macOS/Windows ``PencilTriggerDecision`` (DraftRight #188). The engine
(``pencil_trigger.py``) owns the polling and GTK; this owns only the rule, so
the two cannot drift and the behaviour is tested in isolation.
"""

from __future__ import annotations

from typing import Optional


def should_trigger(previous: Optional[str], current: Optional[str]) -> bool:
    """Fire when the PRIMARY selection becomes a NEW, non-blank value.

    Two guards, both needed to stop the rewrite panel re-opening on every poll:

    - blank / whitespace-only ``current`` never fires — a plain click clears or
      empties PRIMARY, and an empty selection is nothing to rewrite;
    - a ``current`` equal to the last observed value never fires — that is the
      same still selection seen on the next poll tick, not a new highlight.
    """
    if current is None or not current.strip():
        return False
    if current == previous:
        return False
    return True
