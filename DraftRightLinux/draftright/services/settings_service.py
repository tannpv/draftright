"""
JSON-based user settings at ``~/.config/draftright/settings.json``.

Respects ``XDG_CONFIG_HOME`` when set.
"""

from __future__ import annotations

import json
import logging
import os
from pathlib import Path
from typing import List

log = logging.getLogger(__name__)

# All tone API values (must match models/tone.py Tone enum)
_ALL_TONE_VALUES = [
    "simple", "natural", "polished", "concise",
    "technical", "claude", "grammar_check", "translate",
]

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
_DEFAULTS: dict[str, object] = {
    "backend_url": "https://api.draftright.info",
    "hotkey": "Ctrl+Shift+R",
    "translate_language": "Vietnamese",
    "auto_start": False,
    "enabled_tones": list(_ALL_TONE_VALUES),  # all tones enabled by default
    "default_tone": "",  # empty = no auto-run tone
    "last_seen_version": "",  # drives the one-time post-update "What's New"
}

# Desktop entry used for auto-start
_DESKTOP_ENTRY = """\
[Desktop Entry]
Type=Application
Name=DraftRight
Exec=draftright
Icon=com.draftright.app
X-GNOME-Autostart-enabled=true
"""

_AUTOSTART_NAME = "com.draftright.app.desktop"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _config_dir() -> Path:
    base = os.environ.get("XDG_CONFIG_HOME", os.path.expanduser("~/.config"))
    d = Path(base) / "draftright"
    d.mkdir(parents=True, exist_ok=True)
    return d


def _settings_file() -> Path:
    return _config_dir() / "settings.json"


def _autostart_dir() -> Path:
    base = os.environ.get("XDG_CONFIG_HOME", os.path.expanduser("~/.config"))
    d = Path(base) / "autostart"
    d.mkdir(parents=True, exist_ok=True)
    return d


def _autostart_file() -> Path:
    return _autostart_dir() / _AUTOSTART_NAME


# ---------------------------------------------------------------------------
# SettingsService
# ---------------------------------------------------------------------------

class SettingsService:
    """Read / write application settings as JSON."""

    def __init__(self) -> None:
        self._data: dict[str, object] = dict(_DEFAULTS)

    # -- properties --------------------------------------------------------

    @property
    def backend_url(self) -> str:
        return str(self._data.get("backend_url", _DEFAULTS["backend_url"]))

    @backend_url.setter
    def backend_url(self, value: str) -> None:
        self._data["backend_url"] = value

    @property
    def hotkey(self) -> str:
        return str(self._data.get("hotkey", _DEFAULTS["hotkey"]))

    @hotkey.setter
    def hotkey(self, value: str) -> None:
        self._data["hotkey"] = value

    @property
    def translate_language(self) -> str:
        return str(self._data.get("translate_language", _DEFAULTS["translate_language"]))

    @translate_language.setter
    def translate_language(self, value: str) -> None:
        self._data["translate_language"] = value

    @property
    def auto_start(self) -> bool:
        return bool(self._data.get("auto_start", _DEFAULTS["auto_start"]))

    @auto_start.setter
    def auto_start(self, value: bool) -> None:
        self._data["auto_start"] = value

    @property
    def enabled_tones(self) -> List[str]:
        """List of enabled tone API values (e.g. ['simple', 'polished'])."""
        raw = self._data.get("enabled_tones", _DEFAULTS["enabled_tones"])
        if isinstance(raw, list):
            return list(raw)
        return list(_ALL_TONE_VALUES)

    @enabled_tones.setter
    def enabled_tones(self, value: List[str]) -> None:
        self._data["enabled_tones"] = list(value)

    @property
    def default_tone(self) -> str:
        """Default tone for auto-run (empty string = none)."""
        return str(self._data.get("default_tone", _DEFAULTS["default_tone"]))

    @default_tone.setter
    def default_tone(self, value: str) -> None:
        self._data["default_tone"] = value

    @property
    def last_seen_version(self) -> str:
        return str(self._data.get("last_seen_version", _DEFAULTS["last_seen_version"]))

    @last_seen_version.setter
    def last_seen_version(self, value: str) -> None:
        self._data["last_seen_version"] = value

    # -- persistence -------------------------------------------------------

    def load(self) -> None:
        """Load settings from disk, creating defaults if the file is missing."""
        path = _settings_file()
        if path.exists():
            try:
                stored = json.loads(path.read_text(encoding="utf-8"))
                # Merge stored values over defaults so new keys get defaults.
                self._data = {**_DEFAULTS, **stored}
                log.info("Settings loaded from %s", path)
                return
            except Exception as exc:
                log.warning("Failed to read settings: %s — using defaults.", exc)

        # First run or corrupt file — write defaults.
        self._data = dict(_DEFAULTS)
        self.save()

    def save(self) -> None:
        """Persist current settings to disk."""
        path = _settings_file()
        path.write_text(
            json.dumps(self._data, indent=2, ensure_ascii=False),
            encoding="utf-8",
        )
        log.debug("Settings saved to %s", path)

    # -- auto-start --------------------------------------------------------

    def set_auto_start(self, enabled: bool) -> None:
        """Create or remove the XDG autostart desktop entry."""
        self.auto_start = enabled
        self.save()

        desktop_file = _autostart_file()
        if enabled:
            desktop_file.write_text(_DESKTOP_ENTRY, encoding="utf-8")
            log.info("Autostart entry created: %s", desktop_file)
        else:
            if desktop_file.exists():
                desktop_file.unlink()
                log.info("Autostart entry removed: %s", desktop_file)
