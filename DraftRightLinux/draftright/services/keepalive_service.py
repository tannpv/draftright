"""Respawn DraftRight when it dies unexpectedly (#100).

Linux counterpart to the macOS launchd KeepAlive agent and the Windows
Task Scheduler RestartOnFailure trigger: a **systemd user service** with
``Restart=on-failure``.

Why a service and not just the XDG autostart entry: an autostart ``.desktop``
only fires at login.  If the app is OOM-killed or crashes mid-session nothing
brings it back until the next login.  ``Restart=on-failure`` covers that, and
deliberately does *not* respawn a clean quit — exit status 0 from the tray's
Quit is final, matching macOS.

Unit naming matters.  xdg-desktop-portal derives an unsandboxed app's identity
from its systemd unit, and the Wayland global shortcut (#99) is refused
without one ("NotAllowed: An app id is required").  The unit is therefore
named ``app-com.draftright.app@autostart.service``, the systemd convention for
a desktop application, rather than something like ``draftright.service`` which
the portal cannot parse an app id out of.

Enabling this replaces the XDG autostart entry: both would launch the app at
login, and while the single-instance GApplication would collapse them, the
loser logs a confusing silent exit.
"""

from __future__ import annotations

import logging
import shutil
import subprocess
import sys
from pathlib import Path

from draftright import config

log = logging.getLogger(__name__)

UNIT_NAME = f"app-{config.APP_ID}@autostart.service"

_UNIT_TEMPLATE = """\
[Unit]
Description=DraftRight — AI-powered text rewriting
Documentation=https://draftright.info
# Wait for a graphical session; the app needs a display and a session bus.
After=graphical-session.target
PartOf=graphical-session.target
# Give up after repeated rapid failures instead of hammering the session.
# These belong in [Unit]; systemd silently ignores them under [Service].
StartLimitBurst={burst}
StartLimitIntervalSec={interval}

[Service]
Type=simple
ExecStart={exec_start}
# Respawn on crash / OOM-kill, but NOT on a clean quit from the tray, which
# exits 0. Mirrors launchd KeepAlive.SuccessfulExit=false on macOS.
Restart=on-failure
RestartSec={restart_sec}

[Install]
WantedBy=graphical-session.target
"""


def _unit_dir() -> Path:
    return Path.home() / ".config" / "systemd" / "user"


def unit_path() -> Path:
    return _unit_dir() / UNIT_NAME


def systemd_available() -> bool:
    """True when a systemd user manager is reachable."""
    if shutil.which("systemctl") is None:
        return False
    try:
        result = subprocess.run(
            ["systemctl", "--user", "is-system-running"],
            capture_output=True,
            text=True,
            timeout=config.SUBPROCESS_TIMEOUT,
        )
        # "degraded" / "starting" are still usable; only a hard failure to
        # talk to the manager means unavailable.
        return result.returncode == 0 or bool(result.stdout.strip())
    except (OSError, subprocess.SubprocessError):
        return False


def executable_command() -> str:
    """Absolute ExecStart command for the running installation.

    Prefers the installed ``draftright`` console script; falls back to running
    the module with the current interpreter, which is what a source checkout
    needs.
    """
    installed = shutil.which("draftright")
    if installed:
        return installed
    # -m keeps this working from a source tree; the interpreter is absolute so
    # systemd (which has no PATH of ours) can find it.
    return f"{sys.executable} -m draftright"


def _systemctl(*args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["systemctl", "--user", *args],
        capture_output=True,
        text=True,
        timeout=config.SUBPROCESS_TIMEOUT,
    )


def is_installed() -> bool:
    return unit_path().exists()


def is_enabled() -> bool:
    if not is_installed():
        return False
    try:
        return _systemctl("is-enabled", UNIT_NAME).returncode == 0
    except (OSError, subprocess.SubprocessError):
        return False


def install() -> bool:
    """Write, reload and enable the unit.  False (logged) on any failure."""
    if not systemd_available():
        log.warning("systemd --user is unavailable; cannot install the keep-alive unit.")
        return False

    path = unit_path()
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            _UNIT_TEMPLATE.format(
                exec_start=executable_command(),
                restart_sec=config.KEEPALIVE_RESTART_SEC,
                burst=config.KEEPALIVE_START_LIMIT_BURST,
                interval=config.KEEPALIVE_START_LIMIT_INTERVAL,
            ),
            encoding="utf-8",
        )
    except OSError as exc:
        log.warning("Could not write %s: %s", path, exc)
        return False

    try:
        _systemctl("daemon-reload")
        result = _systemctl("enable", UNIT_NAME)
    except (OSError, subprocess.SubprocessError) as exc:
        log.warning("systemctl failed while enabling the keep-alive unit: %s", exc)
        return False

    if result.returncode != 0:
        log.warning("Could not enable %s: %s", UNIT_NAME, result.stderr.strip())
        return False

    log.info("Keep-alive service installed (%s)", path)
    return True


def uninstall() -> bool:
    """Disable and remove the unit.  Safe to call when nothing is installed."""
    ok = True
    if shutil.which("systemctl") is not None:
        try:
            _systemctl("disable", UNIT_NAME)
        except (OSError, subprocess.SubprocessError) as exc:
            log.debug("Ignoring systemctl disable failure: %s", exc)
            ok = False

    path = unit_path()
    try:
        if path.exists():
            path.unlink()
            log.info("Keep-alive service removed (%s)", path)
    except OSError as exc:
        log.warning("Could not remove %s: %s", path, exc)
        ok = False

    if shutil.which("systemctl") is not None:
        try:
            _systemctl("daemon-reload")
        except (OSError, subprocess.SubprocessError):
            pass
    return ok
