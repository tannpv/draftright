"""Submits feature requests to the backend POST /feedback endpoint (JSON).

The Linux app manages auth via ``app.auth_service`` (an ``AuthService``
instance), not a module-level singleton.  Callers that have access to the
app object should pass the current bearer token explicitly via the
``bearer_token`` keyword argument.  When no token is available the request
is sent unauthenticated and ``user_email`` is included instead.
"""
from __future__ import annotations

import json
import urllib.request
import urllib.error

from .settings_service import settings_service

_TARGET_PLATFORMS = ("playground", "mobile", "windows", "mac", "linux")


def submit_feature_request(
    *,
    title: str,
    target_platform: str,
    description: str,
    user_email: str | None = None,
    bearer_token: str | None = None,
    timeout: float = 15.0,
) -> str:
    """POST a feature request to ``/feedback``.  Returns the new row id.

    Raises ``RuntimeError`` on a non-2xx response or transport error.
    ``target_platform`` must be one of playground|mobile|windows|mac|linux.

    Args:
        title: Short one-line description of the feature.
        target_platform: Which platform the feature targets.
        description: Detailed description of the request.
        user_email: Optional contact email (sent only when no bearer token).
        bearer_token: Optional JWT from ``app.auth_service.access_token``.
        timeout: HTTP request timeout in seconds.

    Returns:
        String representation of the new feedback row id.
    """
    if target_platform not in _TARGET_PLATFORMS:
        raise ValueError(f"target_platform must be one of {_TARGET_PLATFORMS}")

    base = settings_service.backend_url.rstrip("/")

    payload: dict[str, object] = {
        "kind": "feature",
        "title": title.strip(),
        "target_platform": target_platform,
        "description": description.strip(),
        "source": "linux-app",
    }
    if not bearer_token and user_email and user_email.strip():
        payload["user_email"] = user_email.strip()

    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(f"{base}/feedback", data=data, method="POST")
    req.add_header("Content-Type", "application/json")
    if bearer_token:
        req.add_header("Authorization", f"Bearer {bearer_token}")

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = json.loads(resp.read().decode("utf-8"))
            return str(body.get("id", ""))
    except urllib.error.HTTPError as e:
        raise RuntimeError(f"server returned {e.code}") from e
    except urllib.error.URLError as e:
        raise RuntimeError(f"network error: {e.reason}") from e
