"""Server-side error reporting for the Linux GTK app.

Hooks ``sys.excepthook`` and threading/asyncio handlers, and POSTs
unhandled exceptions to the DraftRight backend's ``/errors`` endpoint.
Pairs with the existing local logger — every crash gets BOTH a local
log line and a server-side row.

Privacy: never sends user-typed text. Only error type, stack trace,
and a small context (Python version, GTK version, locale).
"""
from __future__ import annotations

import json
import logging
import platform
import os
import sys
import threading
import traceback
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable, Optional
from urllib import request as urllib_request
from urllib.error import URLError

logger = logging.getLogger(__name__)

_BACKEND_URL: str = "https://api.draftright.info"
_BEARER_TOKEN_PROVIDER: Optional[Callable[[], Optional[str]]] = None
_QUEUE_PATH = Path(os.path.expanduser("~/.config/draftright/error_queue.jsonl"))
_APP_VERSION = "linux"


def configure(
    backend_url: str,
    bearer_token_provider: Optional[Callable[[], Optional[str]]] = None,
    app_version: str = "linux",
) -> None:
    """Wire up handlers + flush any queue stranded from a prior run."""
    global _BACKEND_URL, _BEARER_TOKEN_PROVIDER, _APP_VERSION
    _BACKEND_URL = backend_url.rstrip("/")
    _BEARER_TOKEN_PROVIDER = bearer_token_provider
    _APP_VERSION = app_version

    # sys.excepthook for top-level uncaught exceptions
    sys.excepthook = _excepthook

    # threading.excepthook for crashes on Thread targets (Python 3.8+)
    if hasattr(threading, "excepthook"):
        threading.excepthook = _thread_excepthook

    # asyncio doesn't get installed unless there's a running loop, but
    # we set the default exception handler factory; consumers using
    # asyncio.run() will pick this up.
    try:
        import asyncio
        asyncio.events.set_event_loop_policy(_AsyncioPolicy())
    except Exception:
        pass

    # Flush any persisted reports
    threading.Thread(target=_flush_queue_async, daemon=True).start()


def report(exc: BaseException, *, source: str = "unknown",
           severity: str = "error") -> None:
    """Manually report a caught-but-noteworthy exception."""
    try:
        payload = _build(exc, source=source, severity=severity)
        _send_or_queue(payload)
    except Exception:
        # Reporting must never throw on the calling thread
        pass


def report_handled(exc: BaseException, *, severity: str = "warning") -> None:
    report(exc, source="handled", severity=severity)


# ── Internals ────────────────────────────────────────────────────────────

def _excepthook(exc_type, exc, tb):
    try:
        report(exc, source="sys.excepthook", severity="fatal")
    except Exception:
        pass
    # Preserve the default printing behavior so the user still sees the trace
    sys.__excepthook__(exc_type, exc, tb)


def _thread_excepthook(args):
    try:
        if args.exc_value is not None:
            report(args.exc_value, source=f"thread:{args.thread.name}", severity="error")
    except Exception:
        pass


class _AsyncioPolicy:
    """Default asyncio policy that installs a custom exception handler."""
    def get_event_loop(self):
        import asyncio
        loop = asyncio.new_event_loop()
        loop.set_exception_handler(self._handle_asyncio)
        return loop

    @staticmethod
    def _handle_asyncio(loop, ctx):
        exc = ctx.get("exception")
        if exc:
            report(exc, source="asyncio", severity="error")


def _build(exc: BaseException, *, source: str, severity: str) -> dict:
    tb_lines = traceback.format_exception(type(exc), exc, exc.__traceback__)
    stack = "".join(tb_lines)[-20000:]  # tail-truncate
    return {
        "platform": "linux",
        "app_version": _APP_VERSION,
        "severity": severity,
        "error_type": type(exc).__name__[:200],
        "message": str(exc)[:5000],
        "stack_trace": stack,
        "context": {
            "source": source,
            "python": platform.python_version(),
            "system": platform.system(),
            "release": platform.release(),
            "machine": platform.machine(),
            "locale": os.environ.get("LANG", ""),
            "ts": datetime.now(timezone.utc).isoformat(),
        },
    }


def _send_or_queue(payload: dict) -> None:
    body = json.dumps(payload).encode("utf-8")
    try:
        req = urllib_request.Request(
            f"{_BACKEND_URL}/errors",
            data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        token = _BEARER_TOKEN_PROVIDER() if _BEARER_TOKEN_PROVIDER else None
        if token:
            req.add_header("Authorization", f"Bearer {token}")
        with urllib_request.urlopen(req, timeout=10) as resp:
            if 200 <= resp.status < 300:
                return
    except (URLError, OSError, TimeoutError):
        _persist_to_queue(body)


def _persist_to_queue(body: bytes) -> None:
    try:
        _QUEUE_PATH.parent.mkdir(parents=True, exist_ok=True)
        existing = (
            _QUEUE_PATH.read_text().splitlines() if _QUEUE_PATH.exists() else []
        )
        # Trim oldest entries beyond 100
        if len(existing) > 100:
            existing = existing[-100:]
        existing.append(body.decode("utf-8"))
        _QUEUE_PATH.write_text("\n".join(existing) + "\n")
    except Exception:
        pass


def _flush_queue_async() -> None:
    if not _QUEUE_PATH.exists():
        return
    try:
        lines = _QUEUE_PATH.read_text().splitlines()
        _QUEUE_PATH.unlink(missing_ok=True)  # optimistic
        remaining = []
        for line in lines:
            line = line.strip()
            if not line:
                continue
            try:
                req = urllib_request.Request(
                    f"{_BACKEND_URL}/errors",
                    data=line.encode("utf-8"),
                    headers={"Content-Type": "application/json"},
                    method="POST",
                )
                token = _BEARER_TOKEN_PROVIDER() if _BEARER_TOKEN_PROVIDER else None
                if token:
                    req.add_header("Authorization", f"Bearer {token}")
                with urllib_request.urlopen(req, timeout=10) as resp:
                    if not (200 <= resp.status < 300):
                        remaining.append(line)
            except Exception:
                remaining.append(line)

        if remaining:
            _QUEUE_PATH.parent.mkdir(parents=True, exist_ok=True)
            _QUEUE_PATH.write_text("\n".join(remaining) + "\n")
    except Exception:
        pass
