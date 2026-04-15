"""
Authentication service with secure token storage.

Storage strategy:
  1. GNOME Keyring via ``gi.repository.Secret`` (libsecret) — preferred.
  2. Fallback: ``~/.config/draftright/auth.json`` with 0600 permissions.
"""

from __future__ import annotations

import json
import logging
import os
import stat
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from draftright.services.api_client import APIClient

log = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# libsecret schema
# ---------------------------------------------------------------------------
_secret_available = False
try:
    import gi

    gi.require_version("Secret", "1")
    from gi.repository import Secret  # type: ignore[attr-defined]

    _SCHEMA = Secret.Schema.new(
        "com.draftright.app",
        Secret.SchemaFlags.NONE,
        {"token-type": Secret.SchemaAttributeType.STRING},
    )
    _secret_available = True
except Exception:
    _SCHEMA = None

# ---------------------------------------------------------------------------
# XDG config path helpers
# ---------------------------------------------------------------------------

def _config_dir() -> Path:
    base = os.environ.get("XDG_CONFIG_HOME", os.path.expanduser("~/.config"))
    d = Path(base) / "draftright"
    d.mkdir(parents=True, exist_ok=True)
    return d


def _auth_file() -> Path:
    return _config_dir() / "auth.json"


# ---------------------------------------------------------------------------
# Token persistence – libsecret
# ---------------------------------------------------------------------------

def _store_secret(token_type: str, value: str) -> bool:
    """Store *value* in GNOME Keyring. Returns True on success."""
    if not _secret_available:
        return False
    try:
        Secret.password_store_sync(
            _SCHEMA,
            {"token-type": token_type},
            Secret.COLLECTION_DEFAULT,
            f"DraftRight {token_type}",
            value,
            None,
        )
        return True
    except Exception as exc:
        log.debug("libsecret store failed: %s", exc)
        return False


def _load_secret(token_type: str) -> str | None:
    """Load a token from GNOME Keyring. Returns ``None`` on failure."""
    if not _secret_available:
        return None
    try:
        return Secret.password_lookup_sync(
            _SCHEMA, {"token-type": token_type}, None
        )
    except Exception as exc:
        log.debug("libsecret lookup failed: %s", exc)
        return None


def _clear_secrets() -> None:
    if not _secret_available:
        return
    for token_type in ("access", "refresh"):
        try:
            Secret.password_clear_sync(
                _SCHEMA, {"token-type": token_type}, None
            )
        except Exception:
            pass


# ---------------------------------------------------------------------------
# Token persistence – JSON file fallback
# ---------------------------------------------------------------------------

def _store_file(access: str | None, refresh: str | None) -> None:
    path = _auth_file()
    data = {"access_token": access, "refresh_token": refresh}
    path.write_text(json.dumps(data), encoding="utf-8")
    os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)  # 0600


def _load_file() -> tuple[str | None, str | None]:
    path = _auth_file()
    if not path.exists():
        return None, None
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
        return data.get("access_token"), data.get("refresh_token")
    except Exception:
        return None, None


def _clear_file() -> None:
    path = _auth_file()
    if path.exists():
        path.unlink(missing_ok=True)


# ---------------------------------------------------------------------------
# AuthService
# ---------------------------------------------------------------------------

class AuthService:
    """Manages authentication lifecycle and token persistence."""

    def __init__(self, api_client: "APIClient"):
        self._api = api_client
        self._access_token: str | None = None
        self._refresh_token: str | None = None
        self._user: dict | None = None

    # -- properties --------------------------------------------------------

    @property
    def is_logged_in(self) -> bool:
        return self._access_token is not None

    @property
    def access_token(self) -> str | None:
        return self._access_token

    @property
    def user(self) -> dict | None:
        return self._user

    # -- public API --------------------------------------------------------

    def login(self, email: str, password: str) -> dict:
        """Authenticate and persist tokens. Returns the API response."""
        data = self._api.login(email, password)
        self._save(data)
        return data

    def register(self, email: str, password: str, name: str) -> dict:
        """Register a new account and persist tokens."""
        data = self._api.register(email, password, name)
        self._save(data)
        return data

    def logout(self) -> None:
        """Clear tokens from memory and storage."""
        self._access_token = None
        self._refresh_token = None
        self._user = None
        self._api.set_token(None)
        _clear_secrets()
        _clear_file()

    def restore_session(self) -> bool:
        """Attempt to load tokens from storage on startup.

        Returns ``True`` if a session was restored.
        """
        access = _load_secret("access")
        refresh = _load_secret("refresh")

        if not access:
            access, refresh = _load_file()

        if access:
            self._access_token = access
            self._refresh_token = refresh
            self._api.set_token(access)
            log.info("Session restored from stored tokens.")
            return True

        log.info("No stored session found.")
        return False

    # -- internal ----------------------------------------------------------

    def _save(self, data: dict) -> None:
        self._access_token = data.get("access_token")
        self._refresh_token = data.get("refresh_token")
        self._user = data.get("user")
        self._api.set_token(self._access_token)

        # Try keyring first, fall back to file.
        ok = _store_secret("access", self._access_token or "")
        ok = _store_secret("refresh", self._refresh_token or "") and ok
        if not ok:
            _store_file(self._access_token, self._refresh_token)
