"""Place a window at the pointer on X11 (#188 pencil bubble).

GTK4 dropped client-side window positioning, and under Wayland a client may not
place its own surface at all — that is why the rewrite panel lets the compositor
decide (#103). **X11 is different**: the window manager honours a move on the
underlying X window, so a feature that already runs only on X11 can put a widget
where the user is looking.

Everything here goes through ``xdotool``, which the app already requires for
keystroke injection — no new dependency, and no second error policy (the shared
``run_tool``/``read_tool`` own that).

The arithmetic is kept in :func:`clamp_to_screen`, a pure function, so the part
that is easy to get wrong is testable without a display.
"""

from __future__ import annotations

import logging
import re
from typing import NamedTuple, Optional

from draftright.helpers.system_input import has_command, read_tool, run_tool

log = logging.getLogger(__name__)

_TOOL = "xdotool"

# `xdotool getmouselocation` → "x:1461 y:418 screen:0 window:4194314"
_MOUSE_RE = re.compile(r"x:(?P<x>-?\d+)\s+y:(?P<y>-?\d+)")
# `xdotool getdisplaygeometry` → "1920 1080"
_GEOMETRY_RE = re.compile(r"(?P<w>\d+)\s+(?P<h>\d+)")


class Point(NamedTuple):
    x: int
    y: int


def cursor_position() -> Optional[Point]:
    """Where the pointer is, or None if X11/xdotool cannot say."""
    if not has_command(_TOOL):
        log.warning("Cannot locate the pointer — install %s.", _TOOL)
        return None
    match = _MOUSE_RE.search(read_tool([_TOOL, "getmouselocation"]))
    if match is None:
        return None
    return Point(int(match["x"]), int(match["y"]))


def screen_size() -> Optional[Point]:
    """The display's pixel size, or None if it cannot be read."""
    match = _GEOMETRY_RE.search(read_tool([_TOOL, "getdisplaygeometry"]))
    if match is None:
        return None
    return Point(int(match["w"]), int(match["h"]))


def move_window(xid: int, position: Point) -> bool:
    """Move the X window *xid* so its top-left sits at *position*."""
    return run_tool([_TOOL, "windowmove", str(xid), str(position.x), str(position.y)])


def clamp_to_screen(
    anchor: Point,
    size: Point,
    screen: Optional[Point],
    offset: Point,
) -> Point:
    """Where to put a *size* window anchored near *anchor*, kept on screen.

    Every argument is in **physical** pixels, because that is what xdotool
    reports and consumes. A caller holding GTK's logical sizes must multiply by
    the scale factor first — on a 2x display a 40px window really occupies 80px,
    and clamping against the logical size lets it hang off the screen edge.
    *offset* is required rather than defaulted for that reason: there is no
    correct default without knowing the caller's scale.

    Offset so the widget sits beside the pointer rather than under it — a window
    spawned directly beneath the cursor swallows the click that spawned it. When
    that would overflow the screen edge, it flips to the other side of the
    pointer instead of being pushed back over it, so the anchor stays visible.

    *screen* of None means the size is unknown; the offset is applied without
    clamping rather than guessing at a resolution.
    """
    x = anchor.x + offset.x
    y = anchor.y + offset.y
    if screen is None:
        return Point(x, y)

    if x + size.x > screen.x:
        x = anchor.x - offset.x - size.x   # flip to the pointer's left
    if y + size.y > screen.y:
        y = anchor.y - offset.y - size.y   # flip above the pointer
    # A window pushed off the top/left edge is worse than one under the pointer.
    return Point(max(0, x), max(0, y))
