"""Google sign-in for a native Linux app (#97).

Implements the OAuth 2.0 native-app flow from RFC 8252: PKCE plus a **loopback
redirect**.  macOS uses an iOS-type client with a reversed-scheme redirect
(``com.googleusercontent.apps.<id>:/oauth2callback``); that scheme is only
accepted by iOS-type clients, so Linux uses the loopback form instead, which is
what Google's "Desktop app" client type accepts.

    1. Generate a PKCE verifier + S256 challenge and a random ``state``.
    2. Bind 127.0.0.1 on an ephemeral port; that URL is the redirect_uri.
    3. Open the system browser at Google's authorize endpoint.
    4. Google redirects back to the loopback server with ``code``.
    5. Exchange code + verifier at the token endpoint → ``id_token``.

The ``id_token`` then goes to ``POST /auth/social``, which verifies it with
Google server-side.  No client secret is involved: a native app is a *public*
client and PKCE is the proof-of-possession.
"""

from __future__ import annotations

import base64
import hashlib
import http.server
import logging
import secrets
import threading
import urllib.parse
import webbrowser

import requests

from draftright import config

log = logging.getLogger(__name__)

_SUCCESS_HTML = b"""<!doctype html><html><head><meta charset="utf-8">
<title>DraftRight</title></head><body style="font-family:system-ui;text-align:center;padding:3rem">
<h2>You're signed in</h2><p>You can close this tab and return to DraftRight.</p>
</body></html>"""

_FAILURE_HTML = b"""<!doctype html><html><head><meta charset="utf-8">
<title>DraftRight</title></head><body style="font-family:system-ui;text-align:center;padding:3rem">
<h2>Sign-in failed</h2><p>Return to DraftRight and try again.</p>
</body></html>"""


class GoogleOAuthError(RuntimeError):
    """Sign-in did not complete."""


class _CallbackHandler(http.server.BaseHTTPRequestHandler):
    """Captures the single OAuth redirect, then lets the server stop."""

    # Set by the server instance.
    result: dict = {}

    def do_GET(self):  # noqa: N802 — BaseHTTPRequestHandler API
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(parsed.query)
        # parse_qs gives lists; collapse to the first value of each.
        self.server.result = {k: v[0] for k, v in params.items() if v}

        ok = "code" in self.server.result
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.end_headers()
        self.wfile.write(_SUCCESS_HTML if ok else _FAILURE_HTML)

    def log_message(self, *_args):
        """Silence BaseHTTPRequestHandler's stderr logging."""


class _LoopbackServer(http.server.HTTPServer):
    result: dict = {}


def _pkce_pair() -> tuple[str, str]:
    """Return (verifier, S256 challenge), base64url without padding."""
    verifier = base64.urlsafe_b64encode(secrets.token_bytes(64)).rstrip(b"=").decode()
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = base64.urlsafe_b64encode(digest).rstrip(b"=").decode()
    return verifier, challenge


def sign_in(timeout: float = config.GOOGLE_OAUTH_TIMEOUT) -> str:
    """Run the browser sign-in flow and return a Google ``id_token``.

    Blocks until the user finishes in the browser, so call it off the GTK main
    thread.

    Raises:
        GoogleOAuthError: cancelled, timed out, or the exchange failed.
    """
    verifier, challenge = _pkce_pair()
    state = secrets.token_urlsafe(24)

    # Port 0 → the kernel picks a free port; Google allows any port on the
    # loopback interface for native clients (RFC 8252 §7.3).
    server = _LoopbackServer(("127.0.0.1", 0), _CallbackHandler)
    server.result = {}
    redirect_uri = f"http://127.0.0.1:{server.server_address[1]}"

    thread = threading.Thread(target=server.handle_request, daemon=True)
    thread.start()

    query = urllib.parse.urlencode(
        {
            "client_id": config.google_client_id(),
            "response_type": "code",
            "redirect_uri": redirect_uri,
            "scope": config.GOOGLE_OAUTH_SCOPES,
            "code_challenge": challenge,
            "code_challenge_method": "S256",
            "state": state,
            # Always show the picker; otherwise a signed-in browser silently
            # reuses an account the user may not have meant to link.
            "prompt": "select_account",
        }
    )
    auth_url = f"{config.GOOGLE_AUTH_ENDPOINT}?{query}"
    log.info("Opening the browser for Google sign-in…")
    if not webbrowser.open(auth_url):
        server.server_close()
        raise GoogleOAuthError("Couldn't open a browser for Google sign-in.")

    thread.join(timeout)
    result = dict(server.result)
    server.server_close()

    if not result:
        raise GoogleOAuthError("Timed out waiting for Google sign-in.")
    if "error" in result:
        raise GoogleOAuthError(f"Google returned: {result['error']}")
    if result.get("state") != state:
        # Mismatched state means the response isn't ours — refuse it.
        raise GoogleOAuthError("Sign-in state mismatch; please try again.")
    code = result.get("code")
    if not code:
        raise GoogleOAuthError("Google did not return an authorization code.")

    return _exchange_code(code, verifier, redirect_uri)


def _exchange_code(code: str, verifier: str, redirect_uri: str) -> str:
    """Swap the authorization code for an id_token."""
    try:
        response = requests.post(
            config.GOOGLE_TOKEN_ENDPOINT,
            data={
                "client_id": config.google_client_id(),
                # Required by Google for Desktop-type clients; see config.
                "client_secret": config.google_client_secret(),
                "code": code,
                "code_verifier": verifier,
                "grant_type": "authorization_code",
                "redirect_uri": redirect_uri,
            },
            timeout=config.API_TIMEOUT,
        )
    except requests.RequestException as exc:
        raise GoogleOAuthError(f"Couldn't reach Google: {exc}") from exc

    if response.status_code >= 400:
        raise GoogleOAuthError(
            f"Google rejected the sign-in ({response.status_code}): "
            f"{response.text[:200]}"
        )

    id_token = response.json().get("id_token")
    if not id_token:
        raise GoogleOAuthError("Google's response contained no id_token.")
    return id_token
