"""Submits bug reports to ``POST /bug-reports`` (multipart/form-data).

Mirrors ``DraftRight/Services/BugReportService.swift`` 1:1 — same endpoint,
same field names, same validation — so one backend handler serves every
platform.  Only ``source``/``platform`` differ (``linux`` here).

The endpoint accepts anonymous reports; pass ``bearer_token`` when the user is
signed in so the report is attributed, or ``user_email`` when it is not.
"""

from __future__ import annotations

import json
import locale
import logging
import platform
from datetime import datetime, timezone
from pathlib import Path

import requests

from draftright import config
from draftright.__version__ import __version__

log = logging.getLogger(__name__)

SOURCE = "linux"

# Matches the macOS client's guards so the same input is accepted everywhere.
MIN_DESCRIPTION_CHARS = 10
MAX_SCREENSHOT_BYTES = 5 * 1024 * 1024

_MIME_BY_SUFFIX = {
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".gif": "image/gif",
    ".webp": "image/webp",
}
_MIME_FALLBACK = "application/octet-stream"


def mime_for(filename: str) -> str:
    """Return the MIME type for *filename*, defaulting to octet-stream."""
    return _MIME_BY_SUFFIX.get(Path(filename).suffix.lower(), _MIME_FALLBACK)


def _os_info() -> str:
    """Human-readable OS string, e.g. ``Linux 6.14.0 (Ubuntu 25.10)``."""
    parts = [platform.system(), platform.release()]
    try:
        pretty = platform.freedesktop_os_release().get("PRETTY_NAME")
        if pretty:
            parts.append(f"({pretty})")
    except (OSError, AttributeError):
        # freedesktop_os_release is 3.10+ and can raise when /etc/os-release
        # is absent; the report is still useful without the distro name.
        pass
    return " ".join(p for p in parts if p)


def submit_bug_report(
    *,
    description: str,
    screenshot_path: str | None = None,
    user_email: str | None = None,
    bearer_token: str | None = None,
    backend_url: str | None = None,
    timeout: float = config.API_TIMEOUT,
) -> str:
    """POST a bug report.  Returns the server-assigned report id.

    Args:
        description: What went wrong; at least ``MIN_DESCRIPTION_CHARS``.
        screenshot_path: Optional image to attach (≤ 5 MB).
        user_email: Contact address, used for anonymous reports.
        bearer_token: JWT from ``app.auth_service.access_token`` when signed in.
        backend_url: Override; defaults to the persisted/env backend.
        timeout: HTTP timeout in seconds.

    Raises:
        ValueError: Description too short, or the screenshot is too large.
        RuntimeError: Transport failure or a non-2xx response.
    """
    text = description.strip()
    if len(text) < MIN_DESCRIPTION_CHARS:
        raise ValueError(
            f"Please describe the problem in at least {MIN_DESCRIPTION_CHARS} characters."
        )

    base = (backend_url or config.default_backend_url()).rstrip("/")
    url = f"{base}/bug-reports"

    context = {
        "platform": SOURCE,
        "locale": locale.getlocale()[0] or "",
        "ts": datetime.now(timezone.utc).isoformat(),
    }
    fields = {
        "description": text,
        "source": SOURCE,
        "app_version": __version__,
        "os_info": _os_info(),
        "context": json.dumps(context),
    }
    if user_email and user_email.strip():
        fields["user_email"] = user_email.strip()

    headers = {}
    if bearer_token:
        headers["Authorization"] = f"Bearer {bearer_token}"

    handle = None
    try:
        files = None
        if screenshot_path:
            path = Path(screenshot_path)
            size = path.stat().st_size
            if size > MAX_SCREENSHOT_BYTES:
                raise ValueError("Screenshot is larger than 5 MB.")
            handle = path.open("rb")
            files = {"screenshot": (path.name, handle, mime_for(path.name))}

        # `data` + `files` makes requests build the multipart body; when there
        # is no attachment it still posts multipart, matching the macOS client.
        response = requests.post(
            url,
            data=fields,
            files=files or {"_": (None, "")},
            headers=headers,
            timeout=timeout,
        )
    except OSError as exc:
        raise RuntimeError(f"Could not read the screenshot: {exc}") from exc
    except requests.RequestException as exc:
        raise RuntimeError(f"Could not reach the server: {exc}") from exc
    finally:
        if handle is not None:
            handle.close()

    if response.status_code >= 400:
        raise RuntimeError(
            f"Server rejected the report ({response.status_code}): {response.text[:200]}"
        )

    try:
        report_id = response.json().get("id")
    except ValueError:
        report_id = None
    if not report_id:
        raise RuntimeError("Server did not return a report id.")

    log.info("Bug report submitted: %s", report_id)
    return str(report_id)
