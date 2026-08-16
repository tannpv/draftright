"""Pencil-bubble placement arithmetic (#188). No display needed.

    python3 test/test_x11_placement.py

The bubble is the one window this app positions itself, so the offset/flip rule
is the part worth pinning: it decides whether the button lands under the click
that spawned it, or off the edge of the screen.
"""

import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from draftright import config
from draftright.helpers.x11_placement import Point, clamp_to_screen

SIZE = Point(config.BUBBLE_SIZE, config.BUBBLE_SIZE)
SCREEN = Point(1920, 1080)
OFFSET = Point(config.BUBBLE_OFFSET_X, config.BUBBLE_OFFSET_Y)


class ClampToScreenTest(unittest.TestCase):
    def test_sits_beside_the_pointer_not_under_it(self):
        # Under the pointer, the bubble would eat the click that raised it.
        placed = clamp_to_screen(Point(500, 400), SIZE, SCREEN, OFFSET)
        self.assertEqual(placed, Point(500 + OFFSET.x, 400 + OFFSET.y))
        self.assertGreater(placed.x, 500)
        self.assertGreater(placed.y, 400)

    def test_flips_left_at_the_right_edge(self):
        anchor = Point(SCREEN.x - 5, 400)
        placed = clamp_to_screen(anchor, SIZE, SCREEN, OFFSET)
        self.assertLess(placed.x, anchor.x, "should flip to the pointer's left")
        self.assertLessEqual(placed.x + SIZE.x, SCREEN.x, "must stay on screen")

    def test_flips_above_at_the_bottom_edge(self):
        anchor = Point(500, SCREEN.y - 5)
        placed = clamp_to_screen(anchor, SIZE, SCREEN, OFFSET)
        self.assertLess(placed.y, anchor.y, "should flip above the pointer")
        self.assertLessEqual(placed.y + SIZE.y, SCREEN.y, "must stay on screen")

    def test_corner_flips_both_axes(self):
        placed = clamp_to_screen(Point(SCREEN.x - 1, SCREEN.y - 1), SIZE, SCREEN, OFFSET)
        self.assertLessEqual(placed.x + SIZE.x, SCREEN.x)
        self.assertLessEqual(placed.y + SIZE.y, SCREEN.y)

    def test_never_goes_negative(self):
        # A tiny screen can't satisfy both rules; off the top-left is the worse
        # failure, so clamping wins over the flip.
        placed = clamp_to_screen(Point(0, 0), SIZE, Point(10, 10), OFFSET)
        self.assertGreaterEqual(placed.x, 0)
        self.assertGreaterEqual(placed.y, 0)

    def test_unknown_screen_size_still_offsets(self):
        # Better to place it beside the pointer than to guess a resolution.
        placed = clamp_to_screen(Point(500, 400), SIZE, None, OFFSET)
        self.assertEqual(placed, Point(500 + OFFSET.x, 400 + OFFSET.y))

    def test_hidpi_size_is_what_keeps_it_on_screen(self):
        # The dev box runs at 2x: the window covers twice its logical size, so
        # clamping against the logical size lets it hang off the edge. This is
        # the case that was wrong before the scale factor was applied.
        scale = 2
        physical = Point(SIZE.x * scale, SIZE.y * scale)
        anchor = Point(SCREEN.x - 5, SCREEN.y - 5)
        placed = clamp_to_screen(anchor, physical, SCREEN, OFFSET)
        self.assertLessEqual(placed.x + physical.x, SCREEN.x)
        self.assertLessEqual(placed.y + physical.y, SCREEN.y)

        # Same anchor with the logical size would have overflowed — that is the
        # bug this scaling prevents, asserted rather than described.
        wrong = clamp_to_screen(anchor, SIZE, SCREEN, OFFSET)
        self.assertGreater(wrong.x + physical.x, SCREEN.x)


if __name__ == "__main__":
    unittest.main()
