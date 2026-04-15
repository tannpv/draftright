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
