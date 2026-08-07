"""
HTTP client for the DraftRight backend API.

Thread-safe — designed to be called from background threads with
GLib.idle_add callbacks forwarding results to the GTK main loop.
"""

import threading
from typing import Callable, Optional
import requests

from draftright import config
from draftright.models.health import HealthStatus, RefreshOutcome


class APIError(Exception):
    """Raised when an API call fails."""

    def __init__(self, message: str, status_code: int | None = None):
        super().__init__(message)
        self.status_code = status_code


class SessionUnverified(APIError):
    """The session could not be confirmed — not proof that it ended.

    Raised when a 401 was met but the refresh could not be completed (backend
    unreachable, 5xx). Distinct from a plain 401, which means the server
    actively rejected the credentials.
    """


class APIClient:
    """Thin wrapper around the DraftRight REST API."""

    TIMEOUT = config.API_TIMEOUT  # seconds

    def __init__(self, backend_url: str = config.DEFAULT_BACKEND_URL):
        self._base_url = backend_url.rstrip("/")
        self._token: str | None = None
        self._lock = threading.Lock()
        # Invoked when an authed call gets a 401. Should attempt a token
        # refresh and return True if the caller may retry. Wired by the
        # application to AuthService.refresh_session. Mirrors the macOS /
        # Windows / mobile "refresh-then-retry" behaviour so an expired
        # access token recovers silently instead of stranding the user.
        self.on_unauthorized: Optional[Callable[[], "RefreshOutcome"]] = None

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

    def _authed(self, do_request: Callable[[], dict]) -> dict:
        """Run an authed request; on 401, refresh once via ``on_unauthorized``
        and retry.

        Raises :class:`SessionUnverified` when the refresh could not be
        completed at all, so callers can tell "you are signed out" from "we
        could not check" — reporting the first for the second made the tray
        claim a sign-out on a flaky connection while the session was live.
        """
        try:
            return do_request()
        except APIError as exc:
            if exc.status_code != 401 or self.on_unauthorized is None:
                raise
            outcome = self.on_unauthorized()
            if outcome is RefreshOutcome.UNAVAILABLE:
                raise SessionUnverified(
                    "could not reach the backend to refresh the session"
                ) from exc
            if getattr(outcome, "may_retry", bool(outcome)):
                return do_request()
            raise

    def refresh(self, refresh_token: str) -> dict:
        """POST /auth/refresh — exchange a refresh token for a new pair.
        Returns {access_token, refresh_token}."""
        resp = requests.post(
            self._url("/auth/refresh"),
            json={"refresh_token": refresh_token},
            headers=self._headers(),
            timeout=self.TIMEOUT,
        )
        return self._handle_response(resp)

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

    def social_login(self, provider: str, id_token: str) -> dict:
        """POST /auth/social — exchange a provider id_token for our session.

        The backend verifies the token with the provider, so nothing here
        needs the client secret.
        """
        resp = requests.post(
            self._url("/auth/social"),
            json={"provider": provider, "id_token": id_token},
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

        def _do() -> dict:
            resp = requests.post(
                self._url("/rewrite"),
                json=payload,
                headers=self._headers(auth=True),
                timeout=self.TIMEOUT,
            )
            return self._handle_response(resp)

        return self._authed(_do)

    def get_subscription(self) -> dict:
        """GET /subscription — returns {plan, status, usage_today}."""

        def _do() -> dict:
            resp = requests.get(
                self._url("/subscription"),
                headers=self._headers(auth=True),
                timeout=self.TIMEOUT,
            )
            return self._handle_response(resp)

        return self._authed(_do)

    # ------------------------------------------------------------------
    # Payment
    # ------------------------------------------------------------------

    def list_payment_methods(self) -> list[str]:
        """GET /payment/methods — returns the raw wire-name list."""
        resp = requests.get(
            self._url("/payment/methods"),
            headers=self._headers(),
            timeout=self.TIMEOUT,
        )
        body = self._handle_response(resp)
        return list(body.get("methods", []))

    def list_plans(self) -> list[dict]:
        """GET /plans (unauthenticated) — returns raw plan rows."""
        resp = requests.get(
            self._url("/plans"),
            headers=self._headers(),
            timeout=self.TIMEOUT,
        )
        if not resp.ok:
            raise APIError(f"[{resp.status_code}] {resp.reason}", status_code=resp.status_code)
        return resp.json() or []

    def create_checkout(self, plan_id: str, method: str) -> dict:
        """POST /payment/checkout — returns the raw checkout envelope.

        Caller wraps with :meth:`models.payment.CheckoutResult.from_json`.
        """
        def _do() -> dict:
            resp = requests.post(
                self._url("/payment/checkout"),
                json={"plan_id": plan_id, "method": method},
                headers=self._headers(auth=True),
                timeout=self.TIMEOUT,
            )
            return self._handle_response(resp)

        return self._authed(_do)

    def get_payment_status(self, reference_code: str) -> dict:
        """GET /payment/status/:ref — one status snapshot."""
        resp = requests.get(
            self._url(f"/payment/status/{reference_code}"),
            headers=self._headers(),
            timeout=self.TIMEOUT,
        )
        return self._handle_response(resp)

    def get_customer_portal_url(self) -> str:
        """GET /payment/portal — one-shot Customer Portal URL.

        Backend dispatches per the user's active subscription source
        (Lemon Squeezy / Stripe).  VietQR / bank / admin-granted have
        no self-service portal and return 404.
        """
        def _do() -> dict:
            resp = requests.get(
                self._url("/payment/portal"),
                headers=self._headers(auth=True),
                timeout=self.TIMEOUT,
            )
            return self._handle_response(resp)

        body = self._authed(_do)
        url = body.get("url")
        if not url:
            raise APIError("Backend did not return a portal URL")
        return str(url)

    def check_health(self) -> HealthStatus:
        """GET /health then /auth/me — returns a :class:`HealthStatus`."""
        # Step 1: Check /health for app identity
        try:
            resp = requests.get(
                self._url("/health"),
                headers={"Accept": "application/json"},
                timeout=config.HEALTH_TIMEOUT,
            )
            if resp.status_code != 200:
                return HealthStatus.OFFLINE
            body = resp.json()
            if body.get("app") != "draftright":
                return HealthStatus.WRONG_SERVER
            # Apply admin-controlled log verbosity once we've confirmed this is
            # really a DraftRight backend. Older backends omit the field.
            from draftright.services.logger import set_min_level_from_server
            set_min_level_from_server(body.get("client_log_level"))
        except Exception:
            return HealthStatus.OFFLINE

        # Step 2: Check /auth/me for login state.
        #
        # Routed through _authed() rather than a bare request so it inherits
        # the one definition of refresh-then-retry. A 401 here almost always
        # means the 15-minute access token aged out, not that the user signed
        # out — every other authed call recovered from that and the health
        # probe did not, so the tray flipped to "not logged in" a quarter of
        # an hour after each launch while rewriting kept working. Hand-rolling
        # the retry here instead would leave two copies of the policy to drift.
        def _probe() -> dict:
            resp = requests.get(
                self._url("/auth/me"),
                headers=self._headers(auth=True),
                timeout=config.HEALTH_TIMEOUT,
            )
            return self._handle_response(resp)

        try:
            self._authed(_probe)
            return HealthStatus.CONNECTED
        except SessionUnverified:
            # The refresh could not be completed, so auth state is unknown.
            # Calling that "not logged in" contradicted AuthService, which
            # still held a live session.
            return HealthStatus.OFFLINE
        except APIError as exc:
            # Still 401 after a completed refresh — genuinely signed out.
            if exc.status_code == 401:
                return HealthStatus.NOT_LOGGED_IN
            return HealthStatus.OFFLINE
        except Exception:
            return HealthStatus.OFFLINE
