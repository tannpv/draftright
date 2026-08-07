"""System tray icon, hosted in a GTK3 helper process.

The indicator itself lives in :mod:`draftright.tray_helper`, because Ayatana
AppIndicator3 is GTK3-only and cannot be loaded into this GTK4 process (the
import raises ``Requiring namespace 'Gtk' version '3.0', but '4.0' is already
loaded``).  This class is only the supervisor: it spawns the helper, pushes
status down its stdin, and shuts it down.

Menu clicks come back as GApplication action activations over the session bus,
not through this object — see the ``show`` / ``settings`` / ``suggest-feature``
/ ``sign-out`` / ``quit`` actions in :mod:`draftright.application`.
"""

import logging
import os
import subprocess
import sys
from pathlib import Path

from draftright import config
from draftright.models.health import HealthStatus
from draftright.models.tray import TrayCommand

logger = logging.getLogger(__name__)


class TrayIcon:
    """Supervises the GTK3 tray helper process."""

    def __init__(self, app):
        """Spawn the helper.

        Args:
            app: The DraftRightApplication instance.  Retained so callers can
                still reach the app, though menu actions travel over D-Bus.
        """
        self.app = app
        self._process = None

        try:
            self._process = subprocess.Popen(
                [sys.executable, "-m", config.TRAY_HELPER_MODULE],
                stdin=subprocess.PIPE,
                env=self._child_env(),
            )
        except OSError as exc:
            logger.warning("Could not start the tray helper: %s", exc)
            return

        logger.info("Tray helper started (pid %s)", self._process.pid)

    @staticmethod
    def _child_env() -> dict:
        """Environment for the helper, with our package made importable.

        The helper is launched as ``python -m draftright.tray_helper``, whose
        sys.path[0] is its own cwd — not necessarily where this package lives.
        Prepending the directory containing ``draftright/`` keeps the helper
        working when the app is run from a source tree or via PYTHONPATH, not
        just when it is installed.
        """
        env = os.environ.copy()
        package_root = str(Path(__file__).resolve().parent.parent.parent)
        existing = env.get("PYTHONPATH", "")
        env["PYTHONPATH"] = (
            f"{package_root}{os.pathsep}{existing}" if existing else package_root
        )
        return env

    @property
    def _alive(self) -> bool:
        return self._process is not None and self._process.poll() is None

    def _send(self, command: TrayCommand, payload: str = "") -> None:
        """Write one protocol line to the helper, tolerating its death."""
        if not self._alive or self._process.stdin is None:
            return
        try:
            self._process.stdin.write(command.encode(payload).encode("utf-8"))
            self._process.stdin.flush()
        except (BrokenPipeError, ValueError, OSError) as exc:
            # The helper exited (no tray host, user killed it, …).  The app
            # must keep running regardless — the tray is optional.
            logger.warning("Tray helper is gone, dropping update: %s", exc)

    def set_status(self, status: HealthStatus):
        """Update the status line shown in the tray menu.

        Args:
            status: The current :class:`HealthStatus`.
        """
        self._send(TrayCommand.STATUS, status.value)

    def set_update_available(self, available: bool):
        """Flag an available app update, drawn as a red dot on the tray icon."""
        self._send(TrayCommand.UPDATE, "1" if available else "0")

    def set_busy(self, busy: bool):
        """Pulse the tray icon while a One-Click rewrite runs (#6).

        One-Click has no window, so this is the only progress signal the user
        gets between pressing the hotkey and the text changing.
        """
        self._send(TrayCommand.BUSY, "1" if busy else "0")

    def stop(self):
        """Shut the helper down.

        Closing stdin is the normal path: the helper sees EOF and quits.  The
        terminate/kill escalation only covers a wedged process.
        """
        if self._process is None:
            return
        try:
            if self._process.stdin is not None:
                self._process.stdin.close()
        except (BrokenPipeError, OSError):
            pass

        try:
            self._process.wait(timeout=config.TRAY_SHUTDOWN_TIMEOUT)
        except subprocess.TimeoutExpired:
            logger.warning("Tray helper did not exit on EOF; terminating.")
            self._process.terminate()
            try:
                self._process.wait(timeout=config.TRAY_SHUTDOWN_TIMEOUT)
            except subprocess.TimeoutExpired:
                self._process.kill()
        finally:
            self._process = None
