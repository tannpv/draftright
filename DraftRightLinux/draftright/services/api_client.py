"""
HTTP client for the DraftRight backend API.

Thread-safe — designed to be called from background threads with
GLib.idle_add callbacks forwarding results to the GTK main loop.
"""

import threading
import requests


class APIError(Exception):
    """Raised when an API call fails."""

    def __init__(self, message: str, status_code: int | None = None):
        super().__init__(message)
        self.status_code = status_code


class APIClient:
    """Thin wrapper around the DraftRight REST API."""

    TIMEOUT = 30  # seconds

    def __init__(self, backend_url: str = "https://api.draftright.app"):
        self._base_url = backend_url.rstrip("/")
        self._token: str | None = None
        self._lock = threading.Lock()

    # ------------------------------------------------------------------
    # Token management
    # ------------------------------------------------------------------

    def set_token(self, token: str | None) -> None:
        """Store (or clear) the Bearer token used for authenticated calls."""
        with self._lock:
            self._token = token

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _headers(self, auth: bool = False) -> dict[str, str]:
        headers = {"Content-Type": "application/json", "Accept": "application/json"}
        if auth:
            with self._lock:
                token = self._token
            if token:
                headers["Authorization"] = f"Bearer {token}"
        return headers

    def _url(self, path: str) -> str:
        return f"{self._base_url}/{path.lstrip('/')}"

    @staticmethod
    def _handle_response(resp: requests.Response) -> dict:
        """Raise a descriptive ``APIError`` on non-2xx status codes."""
        try:
            body = resp.json()
        except ValueError:
            body = {}

        if resp.ok:
            return body

        # Try to pull a human-readable message from the API response.
        message = (
            body.get("message")
            or body.get("error")
            or resp.reason
            or "Unknown API error"
        )
        raise APIError(
            f"[{resp.status_code}] {message}",
            status_code=resp.status_code,
        )

    # ------------------------------------------------------------------
    # Public API methods
    # ------------------------------------------------------------------

    def login(self, email: str, password: str) -> dict:
        """POST /auth/login — returns {access_token, refresh_token, user}."""
        resp = requests.post(
            self._url("/auth/login"),
            json={"email": email, "password": password},
            headers=self._headers(),
            timeout=self.TIMEOUT,
        )
        return self._handle_response(resp)

    def register(self, email: str, password: str, name: str) -> dict:
        """POST /auth/register — returns {access_token, refresh_token, user}."""
        resp = requests.post(
            self._url("/auth/register"),
            json={"email": email, "password": password, "name": name},
            headers=self._headers(),
            timeout=self.TIMEOUT,
        )
        return self._handle_response(resp)

    def rewrite(
        self, text: str, tone: str, target_language: str | None = None
    ) -> dict:
        """POST /rewrite — returns {rewritten_text, usage_today, daily_limit}."""
        payload: dict = {"text": text, "tone": tone}
        if target_language is not None:
            payload["target_language"] = target_language
        resp = requests.post(
            self._url("/rewrite"),
            json=payload,
            headers=self._headers(auth=True),
            timeout=self.TIMEOUT,
        )
        return self._handle_response(resp)

    def get_subscription(self) -> dict:
        """GET /subscription — returns {plan, status, usage_today}."""
        resp = requests.get(
            self._url("/subscription"),
            headers=self._headers(auth=True),
            timeout=self.TIMEOUT,
        )
        return self._handle_response(resp)

    def check_health(self) -> str:
        """GET /auth/me — returns 'connected', 'not_logged_in', or 'offline'."""
        try:
            resp = requests.get(
                self._url("/auth/me"),
                headers=self._headers(auth=True),
                timeout=5,
            )
            if resp.status_code == 200:
                return "connected"
            elif resp.status_code == 401:
                return "not_logged_in"
            else:
                return "offline"
        except Exception:
            return "offline"
