#!/usr/bin/env python3
"""Smoke test for the DraftRight Linux app's network layer.

Imports the actual APIClient class from DraftRightLinux/draftright/services/
api_client.py and exercises it against production. Doesn't touch GTK4 or
libadwaita — those are tested only by running the actual app on Linux.

What this proves: when running on a real Linux machine, the network layer
of the DraftRight Linux app will successfully:
  - Reach api.draftright.info /health
  - Log in with valid credentials
  - Maintain auth state in APIClient._token
  - Call /auth/me with the token
  - Call /rewrite with a valid tone
  - Hit /subscription

What this does NOT prove: the GTK UI renders, the global hotkey works, the
floating panel positions correctly, the system tray appears. Those need
a real Linux desktop session.

Run from the project root:
    python3 scripts/test-linux-api-smoke.py
"""

from __future__ import annotations

import sys
from pathlib import Path

# Add Linux app to sys.path so we can import its real APIClient
ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "DraftRightLinux"))

try:
    from draftright.services.api_client import APIClient, APIError
except ImportError as e:
    print(f"FAIL: could not import APIClient: {e}")
    print("Hint: install 'requests' first → pip install requests")
    sys.exit(2)


EMAIL = "admin@draftright.info"
PASSWORD = "MyP@ssword1"


def main() -> int:
    print("Linux app API smoke test")
    print("Backend (default in APIClient class):", APIClient().__init__.__defaults__[0])
    print("=" * 60)

    client = APIClient()  # uses the new default: api.draftright.info
    failed = 0
    passed = 0

    def check(name: str, ok: bool, detail: str = ""):
        nonlocal failed, passed
        marker = "✓" if ok else "✗"
        print(f"  {marker} {name}")
        if detail:
            print(f"      → {detail}")
        if ok:
            passed += 1
        else:
            failed += 1

    # 1. /health (no auth)
    state = client.check_health()
    check(
        "1. check_health() before login",
        state in ("not_logged_in", "connected"),
        f"state={state!r}",
    )

    # 2. login
    try:
        login_resp = client.login(EMAIL, PASSWORD)
        ok = "access_token" in login_resp
        check("2. login(email, password)", ok,
              f"got fields: {sorted(login_resp.keys())}")
        if ok:
            client.set_token(login_resp["access_token"])
    except APIError as e:
        check("2. login(email, password)", False, str(e))
        return _summary(passed, failed)

    # 3. /health again, with token set
    state = client.check_health()
    check(
        "3. check_health() after setting token",
        state == "connected",
        f"state={state!r}",
    )

    # 4. rewrite with auth — use a valid tone (Linux app uses same tones)
    try:
        rw = client.rewrite("hey can u send me the file", tone="polished")
        ok = "rewritten_text" in rw and len(rw["rewritten_text"]) > 0
        preview = rw.get("rewritten_text", "")[:80]
        check("4. rewrite('hey can u send me the file', 'polished')",
              ok, f"got: {preview!r}")
    except APIError as e:
        check("4. rewrite()", False, str(e))

    # 5. get_subscription
    try:
        sub = client.get_subscription()
        # subscription endpoint returns various shapes; just check we got JSON back
        ok = isinstance(sub, dict)
        check("5. get_subscription()", ok,
              f"keys: {sorted(sub.keys()) if ok else sub}")
    except APIError as e:
        check("5. get_subscription()", False, str(e))

    # 6. clear token, /auth/me should now be 'not_logged_in'
    client.set_token(None)
    state = client.check_health()
    check(
        "6. check_health() after clearing token",
        state == "not_logged_in",
        f"state={state!r}",
    )

    return _summary(passed, failed)


def _summary(passed: int, failed: int) -> int:
    print("=" * 60)
    print(f"RESULT: {passed} passed, {failed} failed")
    print("=" * 60)
    return 0 if failed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
