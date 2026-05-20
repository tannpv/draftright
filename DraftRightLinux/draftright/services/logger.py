"""Centralized file logger for DraftRight."""

import logging
import logging.handlers
import os
from pathlib import Path

LOG_DIR = Path(os.environ.get("XDG_DATA_HOME", Path.home() / ".local" / "share")) / "draftright" / "logs"
LOG_FILE = LOG_DIR / "draftright.log"


def setup_logging():
    """Configure logging to write to both console and file.

    Call this once at app startup. All modules using
    ``logging.getLogger(__name__)`` will automatically write to the log file.
    """
    LOG_DIR.mkdir(parents=True, exist_ok=True)

    file_handler = logging.handlers.RotatingFileHandler(
        LOG_FILE, maxBytes=2 * 1024 * 1024, backupCount=3, encoding="utf-8",
    )
    file_handler.setFormatter(
        logging.Formatter("[%(asctime)s] [%(name)s] [%(levelname)s] %(message)s",
                          datefmt="%Y-%m-%d %H:%M:%S")
    )

    console_handler = logging.StreamHandler()
    console_handler.setFormatter(
        logging.Formatter("[%(levelname)s] %(name)s: %(message)s")
    )

    root = logging.getLogger()
    root.setLevel(logging.DEBUG)
    root.addHandler(file_handler)
    root.addHandler(console_handler)


def get_log_path() -> str:
    """Return the path to the log file for display in settings."""
    return str(LOG_FILE)


# Maps the backend's client_log_level to a root-logger threshold. 'info' (the
# default) keeps DEBUG so the app's normal verbosity is unchanged; 'off' uses a
# level above CRITICAL so nothing — not even errors — is written.
_OFF_LEVEL = logging.CRITICAL + 1
_LEVEL_MAP = {
    "off": _OFF_LEVEL,
    "errors": logging.ERROR,
    "error": logging.ERROR,
    "warnings": logging.WARNING,
    "warning": logging.WARNING,
    "warn": logging.WARNING,
    "info": logging.DEBUG,
}

_current_min = logging.DEBUG


def set_min_level_from_server(value):
    """Apply the admin-controlled ``client_log_level`` (off | errors | warnings
    | info) from ``GET /health`` as the root logger's threshold. Unknown/empty
    falls back to full logging. No-op when unchanged; only a genuine change is
    announced (so the ~30s health poll doesn't spam the log)."""
    global _current_min
    key = (value or "").strip().lower()
    new_level = _LEVEL_MAP.get(key, logging.DEBUG)
    if new_level == _current_min:
        return

    old = _current_min
    log = logging.getLogger(__name__)
    msg = "Client log level changed: %s -> %s (server '%s')"
    # When narrowing (raising the threshold, e.g. -> off) announce under the OLD
    # threshold first so the line isn't itself dropped; when widening, set then
    # announce.
    if new_level > old:
        log.warning(msg, logging.getLevelName(old), logging.getLevelName(new_level), value)
        logging.getLogger().setLevel(new_level)
    else:
        logging.getLogger().setLevel(new_level)
        log.warning(msg, logging.getLevelName(old), logging.getLevelName(new_level), value)
    _current_min = new_level
