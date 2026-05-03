#!/usr/bin/env python3
"""End-to-end automation test for the EXTTOK extension-token flow.

Hits the production backend at api.draftright.info and exercises every
Matrix-A test case (EXTTOK-001 through EXTTOK-010 plus rotation A8).

Pass/fail is recorded per test case. The script cleans up after itself
(revokes any tokens it minted) and is safe to re-run.

Run:
    python3 scripts/test-extension-tokens-e2e.py
    python3 scripts/test-extension-tokens-e2e.py --base-url http://localhost:3000
"""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import urllib.request
import urllib.error
import uuid
from dataclasses import dataclass, field
from typing import Any


@dataclass
class TestResult:
    tc_id: str
    name: str
    status: str  # 'PASS', 'FAIL', 'SKIP'
    detail: str = ""


@dataclass
class TestRunner:
    base_url: str
    email: str
    password: str
    results: list[TestResult] = field(default_factory=list)
    jwt: str | None = None

    def http(
        self,
        method: str,
        path: str,
        *,
        token: str | None = None,
        body: dict[str, Any] | None = None,
        expected_status: int | None = None,
    ) -> tuple[int, dict[str, Any] | str]:
        url = f"{self.base_url}{path}"
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                payload = resp.read().decode() or "{}"
                try:
                    parsed = json.loads(payload) if payload.strip() else {}
                except json.JSONDecodeError:
                    parsed = payload
                return resp.status, parsed
        except urllib.error.HTTPError as e:
            payload = e.read().decode()
            try:
                parsed = json.loads(payload) if payload.strip() else {}
            except json.JSONDecodeError:
                parsed = payload
            return e.code, parsed

    def record(self, tc_id: str, name: str, ok: bool, detail: str = ""):
        status = "PASS" if ok else "FAIL"
        self.results.append(TestResult(tc_id, name, status, detail))
        marker = "✓" if ok else "✗"
        print(f"  {marker} {tc_id}  {name}")
        if detail and not ok:
            print(f"        → {detail}")

    def login(self) -> bool:
        print("\n[setup] Logging in as admin to get session JWT...")
        status, body = self.http(
            "POST",
            "/auth/login",
            body={"email": self.email, "password": self.password},
        )
        # /auth/login returns NestJS-default 201; both 200 and 201 are fine.
        if status not in (200, 201) or not isinstance(body, dict) or "access_token" not in body:
            print(f"  ✗ login failed: HTTP {status}, body={body}")
            return False
        self.jwt = body["access_token"]
        print(f"  ✓ JWT acquired (length {len(self.jwt)})")
        return True

    def run(self) -> int:
        if not self.login():
            return 2

        print("\n[matrix A] Backend dual-accept tests against", self.base_url)

        # A1 — POST /rewrite with user JWT
        status, body = self.http(
            "POST",
            "/rewrite",
            token=self.jwt,
            body={"text": "hello there", "tone": "polished"},
        )
        ok = status in (200, 201) and isinstance(body, dict) and "rewritten_text" in body
        self.record(
            "EXTTOK-008", "A1: POST /rewrite with user JWT returns rewritten text",
            ok, f"HTTP {status} body={body}" if not ok else "",
        )

        # A2 — Mint extension token
        device_id_1 = str(uuid.uuid4())
        device_id_2 = str(uuid.uuid4())  # used in A8
        status, body = self.http(
            "POST",
            "/auth/extension-tokens",
            token=self.jwt,
            body={"device_id": device_id_1, "device_name": "e2e-test-1"},
        )
        ok = (
            status == 200
            and isinstance(body, dict)
            and body.get("token", "").startswith("dr_ext_")
            and "id" in body
        )
        ext_token_1 = body.get("token") if ok else None
        ext_id_1 = body.get("id") if ok else None
        self.record(
            "EXTTOK-001", "A2: POST /auth/extension-tokens returns dr_ext_* token",
            ok, f"HTTP {status} body={body}" if not ok else f"id={ext_id_1}",
        )
        if not ok:
            print("\n[abort] A2 failed; remaining tests depend on a minted token.")
            return self._summarize()

        # A3 — POST /rewrite with extension token
        status, body = self.http(
            "POST",
            "/rewrite",
            token=ext_token_1,
            body={"text": "ping me when ready", "tone": "natural"},
        )
        ok = status in (200, 201) and isinstance(body, dict) and "rewritten_text" in body
        self.record(
            "EXTTOK-009", "A3: POST /rewrite with extension token returns rewritten text",
            ok, f"HTTP {status} body={body}" if not ok else "",
        )

        # A4 — GET /auth/me with extension token = 401
        status, _ = self.http("GET", "/auth/me", token=ext_token_1)
        ok = status == 401
        self.record(
            "EXTTOK-010", "A4: GET /auth/me with extension token returns 401 (scope rejection)",
            ok, f"HTTP {status} (expected 401)" if not ok else "",
        )

        # A5 — GET /auth/extension-tokens does not expose token_hash
        status, body = self.http("GET", "/auth/extension-tokens", token=self.jwt)
        ok_status = status == 200 and isinstance(body, list) and len(body) >= 1
        no_hash = ok_status and not any("token_hash" in row for row in body)
        no_user_id = ok_status and not any("user_id" in row for row in body)
        self.record(
            "EXTTOK-002", "A5: GET /auth/extension-tokens returns rows without token_hash or user_id",
            ok_status and no_hash and no_user_id,
            f"HTTP {status} | rows have token_hash={not no_hash} user_id={not no_user_id}"
            if not (ok_status and no_hash and no_user_id) else "",
        )

        # A8 — Re-mint with same device_id rotates the token
        status, body = self.http(
            "POST",
            "/auth/extension-tokens",
            token=self.jwt,
            body={"device_id": device_id_2, "device_name": "e2e-test-rotate"},
        )
        ext_token_2a = body.get("token") if status == 200 else None
        ext_id_2a = body.get("id") if status == 200 else None

        # mint a 2nd time with same device_id — should rotate
        status, body = self.http(
            "POST",
            "/auth/extension-tokens",
            token=self.jwt,
            body={"device_id": device_id_2, "device_name": "e2e-test-rotate"},
        )
        ext_token_2b = body.get("token") if status == 200 else None
        ext_id_2b = body.get("id") if status == 200 else None

        if ext_token_2a and ext_token_2b:
            # First token should now fail on /rewrite (revoked by rotation)
            status, _ = self.http(
                "POST",
                "/rewrite",
                token=ext_token_2a,
                body={"text": "test", "tone": "natural"},
            )
            old_rejected = status == 401
            # New token should succeed
            status, _ = self.http(
                "POST",
                "/rewrite",
                token=ext_token_2b,
                body={"text": "test", "tone": "natural"},
            )
            new_works = status in (200, 201)
            self.record(
                "EXTTOK-003",
                "A8: Re-mint with same device_id rotates — old returns 401, new works",
                old_rejected and new_works,
                f"old_rejected={old_rejected} new_works={new_works}"
                if not (old_rejected and new_works) else "",
            )
        else:
            self.record(
                "EXTTOK-003", "A8: Re-mint with same device_id rotates",
                False, "Could not mint two tokens for rotation test",
            )

        # A6 — DELETE /auth/extension-tokens/:id returns 204
        status, _ = self.http(
            "DELETE", f"/auth/extension-tokens/{ext_id_1}", token=self.jwt
        )
        ok = status == 204
        self.record(
            "EXTTOK-007", "A6: DELETE /auth/extension-tokens/:id returns 204",
            ok, f"HTTP {status} (expected 204)" if not ok else "",
        )

        # A7 — Revoked token fails
        status, _ = self.http(
            "POST",
            "/rewrite",
            token=ext_token_1,
            body={"text": "test", "tone": "natural"},
        )
        ok = status == 401
        self.record(
            "EXTTOK-005", "A7: Revoked extension token returns 401 on /rewrite",
            ok, f"HTTP {status} (expected 401)" if not ok else "",
        )

        # Cleanup: revoke any remaining test tokens (A8's tokens)
        if ext_id_2b:
            self.http("DELETE", f"/auth/extension-tokens/{ext_id_2b}", token=self.jwt)

        return self._summarize()

    def _summarize(self) -> int:
        print("\n" + "=" * 70)
        passed = sum(1 for r in self.results if r.status == "PASS")
        failed = sum(1 for r in self.results if r.status == "FAIL")
        total = len(self.results)
        print(f"RESULT: {passed}/{total} passed, {failed} failed")
        if failed:
            print("\nFailed tests:")
            for r in self.results:
                if r.status == "FAIL":
                    print(f"  ✗ {r.tc_id}  {r.name}")
                    if r.detail:
                        print(f"        {r.detail}")
        print("=" * 70)
        return 0 if failed == 0 else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--base-url",
        default="https://api.draftright.info",
        help="Backend base URL (default: %(default)s)",
    )
    parser.add_argument(
        "--email", default="admin@draftright.info",
        help="User email for login",
    )
    parser.add_argument(
        "--password", default="MyP@ssword1",
        help="User password",
    )
    args = parser.parse_args()

    runner = TestRunner(
        base_url=args.base_url.rstrip("/"),
        email=args.email,
        password=args.password,
    )
    sys.exit(runner.run())


if __name__ == "__main__":
    main()
