"""The UI layer may only use Application's public API. GTK-free.

    python3 test/test_ui_uses_public_app_api.py

Reaching into ``app._something`` from ``draftright/ui/`` hides real API in a
name that reads as internal: the next person refactors what looks private,
and the UI breaks at runtime — nothing imports these modules together, because
they need GTK. So this walks the source with ``ast`` instead.

Two ways the reach hides, both checked:
  * attribute access — ``app._show_update_dialog(...)``;
  * ``getattr(app, "_update_service", None)``, which the attribute walk misses
    and which additionally invites a hand-rolled fallback when it comes back
    None — exactly the duplicate-construction drift RULE #1 exists to stop.
"""

import ast
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

PACKAGE = Path(__file__).resolve().parent.parent / "draftright"
UI_DIR = PACKAGE / "ui"

# How the UI refers to the Application instance.
APP_REFS = ("self.app", "app")


def _is_app_ref(node: ast.expr) -> bool:
    return ast.unparse(node) in APP_REFS


def private_app_reaches(tree: ast.AST) -> list[str]:
    """Every private Application member *tree* touches, unparsed.

    The one definition of the rule — both the scan of the real UI and the test
    that proves the guard can fail run through here, so they cannot disagree
    about what counts as a reach.
    """
    found = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Attribute):
            if node.attr.startswith("_") and _is_app_ref(node.value):
                found.append(ast.unparse(node))
        elif isinstance(node, ast.Call) and getattr(node.func, "id", None) == "getattr":
            # getattr(app, "_private", default)
            if (
                len(node.args) >= 2
                and _is_app_ref(node.args[0])
                and isinstance(node.args[1], ast.Constant)
                and isinstance(node.args[1].value, str)
                and node.args[1].value.startswith("_")
            ):
                found.append(ast.unparse(node))
    return found


class UiUsesPublicAppApiTest(unittest.TestCase):
    def test_ui_never_reaches_into_private_app_members(self):
        reaches = [
            f"{path.name}: {reach}"
            for path in sorted(UI_DIR.rglob("*.py"))
            for reach in private_app_reaches(ast.parse(path.read_text()))
        ]
        self.assertEqual(
            reaches,
            [],
            "UI reached into private Application members — make them public on "
            "Application (with a docstring line saying why) rather than widening "
            "access here:\n  " + "\n  ".join(reaches),
        )

    def test_the_guard_actually_detects_a_reach(self):
        # A guard that cannot fail is not a guard. These are the three reaches
        # that were really in the tree, in both shapes it has to catch.
        sample = ast.parse(
            "app._show_update_dialog(result)\n"
            "svc = getattr(app, '_update_service', None)\n"
            "self.app._apply_trigger_mode()\n"
        )
        # Sorted: which reaches were found, not the order ast.walk happens to
        # visit them in.
        self.assertEqual(
            sorted(private_app_reaches(sample)),
            sorted(
                [
                    "app._show_update_dialog",
                    "getattr(app, '_update_service', None)",
                    "self.app._apply_trigger_mode",
                ]
            ),
        )

    def test_public_app_calls_are_not_flagged(self):
        # The replacements for those three must pass cleanly, or the guard
        # would just push the UI toward some other workaround.
        sample = ast.parse(
            "app.show_update_dialog(result)\n"
            "svc = app.update_service\n"
            "self.app.apply_trigger_mode()\n"
        )
        self.assertEqual(private_app_reaches(sample), [])


if __name__ == "__main__":
    unittest.main()
