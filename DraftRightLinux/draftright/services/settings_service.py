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

from draftright import config
from draftright.models.app_mode import AppMode
from draftright.models.tone import Tone

log = logging.getLogger(__name__)

# Single source of truth: derive from the Tone enum so adding a tone needs no
# edit here (Rule #1).
_ALL_TONE_VALUES = [t.api_value for t in Tone]

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
_DEFAULTS: dict[str, object] = {
    "backend_url": config.DEFAULT_BACKEND_URL,
    "hotkey": config.DEFAULT_HOTKEY,
    "translate_language": "Vietnamese",
    "auto_start": False,
    "enabled_tones": list(_ALL_TONE_VALUES),  # all tones enabled by default
    "default_tone": "",
    # #96 One-Click: hotkey behaviour + the tone it uses.
    "app_mode": AppMode.ADVANCED.value,
    "one_click_tone": Tone.POLISHED.api_value,
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

# The Settings UI addressed two keys with hyphens while the service (and every
# property here) used underscores, so choices made in Settings were written to
# keys nothing read — a user could pick a translate language and have the app
# keep using the old one.  The UI now uses the canonical names; these aliases
# fold any already-persisted hyphen keys back in on load so that choice is
# honoured rather than silently discarded.
_LEGACY_KEY_ALIASES = {
    "translate-language": "translate_language",
    "auto-start": "auto_start",
}


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
        # Dev/self-host override: the DRAFTRIGHT_BACKEND env var (same name the
        # macOS/Windows apps use) wins over the persisted file value. UI no
        # longer exposes this field, so this is the only way to point a release
        # build at a non-prod backend.
        return config.backend_url_override() or str(
            self._data.get("backend_url", _DEFAULTS["backend_url"])
        )

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

    @property
    def app_mode(self) -> AppMode:
        """How the hotkey behaves (#96)."""
        return AppMode.from_wire(self._data.get("app_mode"))

    @app_mode.setter
    def app_mode(self, value: AppMode) -> None:
        self._data["app_mode"] = value.value

    @property
    def one_click_tone(self) -> str:
        """Tone api_value used by One-Click; only read in that mode."""
        raw = str(self._data.get("one_click_tone", _DEFAULTS["one_click_tone"]))
        # Guard a stale/removed tone so the hotkey cannot fail silently, and a
        # tone that returns an analysis instead of replacement text — One-Click
        # has no UI to review one in, so it could only fail (#107).
        usable = {t.api_value for t in Tone if t.produces_replacement_text}
        return raw if raw in usable else _DEFAULTS["one_click_tone"]

    @one_click_tone.setter
    def one_click_tone(self, value: str) -> None:
        self._data["one_click_tone"] = value

    # -- key-addressed access ----------------------------------------------

    def get(self, key: str, default=None):
        """Return a setting by key, for UI code that addresses settings generically.

        Falls back to the built-in default for *key* before *default*, so a
        caller that passes no default still gets the documented value.
        """
        if key in self._data:
            return self._data[key]
        return _DEFAULTS.get(key, default)

    def set(self, key: str, value) -> None:
        """Set a setting by key and persist immediately."""
        self._data[key] = value
        self.save()

    # -- persistence -------------------------------------------------------

    def load(self) -> None:
        """Load settings from disk, creating defaults if the file is missing."""
        path = _settings_file()
        if path.exists():
            try:
                stored = json.loads(path.read_text(encoding="utf-8"))
                # Merge stored values over defaults so new keys get defaults.
                self._data = {**_DEFAULTS, **stored}
                if self._migrate_legacy_keys():
                    self.save()
                log.info("Settings loaded from %s", path)
                return
            except Exception as exc:
                log.warning("Failed to read settings: %s — using defaults.", exc)

        # First run or corrupt file — write defaults.
        self._data = dict(_DEFAULTS)
        self.save()

    def _migrate_legacy_keys(self) -> bool:
        """Fold hyphenated keys onto their canonical names.  True if changed.

        The hyphenated value wins: it is what the user last chose in Settings,
        even though nothing ever read it back.
        """
        changed = False
        for legacy, canonical in _LEGACY_KEY_ALIASES.items():
            if legacy not in self._data:
                continue
            value = self._data.pop(legacy)
            if value != self._data.get(canonical):
                log.info(
                    "Migrating setting %r → %r (value %r)", legacy, canonical, value
                )
                self._data[canonical] = value
            changed = True
        return changed

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
        """Start DraftRight at login, and keep it alive during the session.

        Prefers a systemd user service (#100): it starts the app at login
        *and* respawns it if it crashes or is OOM-killed, which an autostart
        desktop entry cannot do.  The XDG entry stays as the fallback for
        sessions without systemd.

        Exactly one mechanism is ever active — both would launch the app at
        login, and the loser exits silently against the single-instance lock.
        """
        # Imported here: keepalive imports config, and settings is imported
        # very early, so a module-level import risks a cycle.
        from draftright.services import keepalive_service

        self.auto_start = enabled
        self.save()

        desktop_file = _autostart_file()

        if enabled and keepalive_service.install():
            # systemd owns startup now; drop the duplicate XDG entry.
            if desktop_file.exists():
                desktop_file.unlink()
                log.info("Replaced the autostart entry with %s",
                         keepalive_service.UNIT_NAME)
            return

        # Either disabling, or systemd is unavailable — fall back to XDG.
        keepalive_service.uninstall()

        if enabled:
            desktop_file.write_text(_DESKTOP_ENTRY, encoding="utf-8")
            log.info("Autostart entry created: %s", desktop_file)
        else:
            if desktop_file.exists():
                desktop_file.unlink()
                log.info("Autostart entry removed: %s", desktop_file)
