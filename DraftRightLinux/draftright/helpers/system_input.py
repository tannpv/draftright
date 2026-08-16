"""Shared low-level system-input helpers for X11 / Wayland.

Centralizes the two things that were copy-pasted across
``clipboard_service`` and ``text_injector`` (Rule #1: no duplication):

  * :func:`has_command` — is a CLI tool on PATH?
  * :func:`run_tool` / :func:`read_tool` — run one, ignoring or capturing its
    output, with the app's single error policy.
  * :class:`TextInputSimulator` — simulate copy/paste/type keystrokes via
    ``wtype`` (Wayland) or ``xdotool`` (X11), so callers never re-implement
    the tool-selection ladder.

All subprocess calls share :data:`config.SUBPROCESS_TIMEOUT` and never raise —
a missing tool degrades to a logged warning, matching the rest of the app.
"""

from __future__ import annotations

import logging
import shutil
import subprocess

from draftright import config
from draftright.helpers.display_server import is_wayland

log = logging.getLogger(__name__)

# Keep short strings pastable via `xdotool type` as a last resort; longer text
# is only injected via a real clipboard paste to avoid per-keystroke latency.
_TYPE_FALLBACK_MAX_CHARS = 500


def has_command(cmd: str) -> bool:
    """True if *cmd* is resolvable on PATH."""
    return shutil.which(cmd) is not None


def run_tool(cmd: list[str], stdin_text: str | None = None) -> bool:
    """Run *cmd*, optionally feeding it *stdin_text*. True if it ran.

    The one place the app decides what a failing CLI tool means: a missing tool
    is a warning (the user can install it), anything else is debug noise, and
    nothing raises — a clipboard or keystroke helper must never crash a rewrite.
    """
    try:
        subprocess.run(
            cmd,
            input=stdin_text,
            text=stdin_text is not None,
            timeout=config.SUBPROCESS_TIMEOUT,
            check=False,
        )
        return True
    except FileNotFoundError:
        log.warning("Tool not found: %s", cmd[0])
    except Exception as exc:  # noqa: BLE001 — see docstring
        log.debug("Tool %s failed: %s", cmd[0], exc)
    return False


def read_tool(cmd: list[str]) -> str:
    """Run *cmd* and return its stdout, or "" if it failed in any way.

    Same policy as :func:`run_tool`; the empty string is the universal "no
    answer", so callers branch on content rather than on exceptions.
    """
    try:
        result = subprocess.run(
            cmd, capture_output=True, text=True, timeout=config.SUBPROCESS_TIMEOUT,
        )
        return result.stdout if result.returncode == 0 else ""
    except FileNotFoundError:
        log.warning("Tool not found: %s", cmd[0])
    except Exception as exc:  # noqa: BLE001 — see run_tool
        log.debug("Tool %s failed: %s", cmd[0], exc)
    return ""


class TextInputSimulator:
    """Simulate copy/paste/type keystrokes on the active window.

    Resolves the display server once at construction; callers just ask for the
    action they want.
    """

    def __init__(self) -> None:
        self._wayland = is_wayland()

    def copy(self) -> bool:
        """Send Ctrl+C to the focused window. Returns True if a tool ran."""
        return self._send_combo("c")

    def paste(self) -> bool:
        """Send Ctrl+V to the focused window. Returns True if a tool ran."""
        return self._send_combo("v")

    def type_text(self, text: str) -> bool:
        """Type *text* character-by-character (X11 only; short strings).

        Practical only for short strings — used as the last-resort injection
        path when no clipboard paste is possible.
        """
        if has_command("xdotool") and len(text) < _TYPE_FALLBACK_MAX_CHARS:
            return run_tool(["xdotool", "type", "--clearmodifiers", text])
        return False

    # ------------------------------------------------------------------
    # Internals
    # ------------------------------------------------------------------

    def _send_combo(self, key: str) -> bool:
        """Send Ctrl+*key* via the best available tool for the display server."""
        if self._wayland and has_command("wtype"):
            if run_tool(["wtype", "-M", "ctrl", "-P", key, "-m", "ctrl"]):
                return True
        # X11, or Wayland without wtype → xdotool (works under XWayland too).
        if has_command("xdotool"):
            return run_tool(["xdotool", "key", "--clearmodifiers", f"ctrl+{key}"])
        log.error(
            "Cannot simulate Ctrl+%s — install xdotool (X11) or wtype (Wayland).",
            key,
        )
        return False
