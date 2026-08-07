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

# Runtime import (not TYPE_CHECKING): refresh_session has to tell a rejected
# refresh token apart from an unreachable backend, and only the former may
# clear the session.
from draftright.models.health import RefreshOutcome
from draftright.services.api_client import APIError

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

    def is_authenticated(self) -> bool:
        """Method form of :attr:`is_logged_in`, used by the settings UI."""
        return self.is_logged_in

    def get_user(self) -> dict:
        """Return a copy of the current user, or an empty dict when signed out.

        Always returns a mapping so callers can use ``.get()`` without a
        ``None`` check.
        """
        return dict(self._user or {})

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

    def social_login(self, provider: str, id_token: str) -> dict:
        """Sign in with a provider id_token and persist the session."""
        data = self._api.social_login(provider, id_token)
        self._save(data)
        return data

    def refresh_session(self) -> RefreshOutcome:
        """Exchange the stored refresh token for a fresh access token.

        Wired into ``APIClient.on_unauthorized`` so an expired access token
        recovers silently mid-request.

        Returns a :class:`RefreshOutcome` rather than a bool because the two
        failure modes are not interchangeable. Only a refresh token the
        **server rejects** clears the session; a network blip or a 5xx must
        not. This runs unattended from the health poll, and ``logout()``
        erases the refresh token from the keyring, so treating a transient
        failure as a sign-out would strand the user at the login screen with
        no way back but re-entering credentials — while UNAVAILABLE lets the
        caller say "offline" instead of falsely reporting a sign-out.
        """
        if not self._refresh_token:
            return RefreshOutcome.REJECTED
        try:
            data = self._api.refresh(self._refresh_token)
        except APIError as exc:
            if exc.status_code in (401, 403):
                log.info("Refresh token rejected (%s); signing out.", exc)
                self.logout()
                return RefreshOutcome.REJECTED
            log.warning("Token refresh failed with %s; keeping the session.", exc)
            return RefreshOutcome.UNAVAILABLE
        except Exception as exc:  # network down, DNS, timeout, TLS…
            log.warning(
                "Token refresh could not reach the backend (%s); keeping the "
                "session.", exc
            )
            return RefreshOutcome.UNAVAILABLE

        access = data.get("access_token")
        if not access:
            # A 2xx with no token is a broken contract, not a transient fault.
            self.logout()
            return RefreshOutcome.REJECTED
        self._access_token = access
        self._refresh_token = data.get("refresh_token") or self._refresh_token
        self._api.set_token(self._access_token)
        ok = _store_secret("access", self._access_token or "")
        ok = _store_secret("refresh", self._refresh_token or "") and ok
        if not ok:
            _store_file(self._access_token, self._refresh_token)
        log.info("Access token refreshed.")
        return RefreshOutcome.REFRESHED

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
