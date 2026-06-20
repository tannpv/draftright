#!/usr/bin/env python3
"""Add an Azure AD application to Partner Center via device-code flow.

This registers the service principal (934644fb-...) as a Partner Center user
with Manager role, so ms-store-submit.py can authenticate via client credentials.

Usage:
    python3 scripts/add-aad-app-to-partner-center.py

You will be prompted to visit https://microsoft.com/devicelogin and sign in
as draftright-admin@tannpvhotmail.onmicrosoft.com (complete MFA on phone).
"""
from __future__ import annotations
import json
import os
import sys
import time
import urllib.request
import urllib.parse
import urllib.error

TENANT_ID     = "fb755f3c-f66f-407b-ab28-9f7e548e1b5e"
# Use our own app (public client flows now enabled in Azure Portal)
PUBLIC_CLIENT = "934644fb-aba7-4813-97b3-cd2c0a21f87f"
RESOURCE      = "https://manage.devcenter.microsoft.com"
# v2 scope — dynamic consent, no Azure Portal pre-registration required
SCOPE         = f"{RESOURCE}/user_impersonation offline_access"

# The AAD app to add as a Partner Center user
AAD_APP_CLIENT_ID = "934644fb-aba7-4813-97b3-cd2c0a21f87f"


def post_form(url: str, data: dict) -> dict:
    body = urllib.parse.urlencode(data).encode()
    req  = urllib.request.Request(url, data=body,
                                  headers={"Content-Type": "application/x-www-form-urlencoded"})
    try:
        with urllib.request.urlopen(req) as r:
            return json.loads(r.read())
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", "replace")
        raise SystemExit(f"HTTP {e.code} POST {url}\n{detail}")


def device_code_flow() -> str:
    print("Requesting device code…")
    # v2 endpoint with scope= — no Azure Portal permission pre-registration needed
    dc = post_form(
        f"https://login.microsoftonline.com/{TENANT_ID}/oauth2/v2.0/devicecode",
        {"client_id": PUBLIC_CLIENT, "scope": SCOPE},
    )
    print(f"\n>>> {dc['message']}\n")

    interval = int(dc.get("interval", 5))
    expires  = int(dc.get("expires_in", 900))
    deadline = time.time() + expires

    while time.time() < deadline:
        time.sleep(interval)
        poll_body = urllib.parse.urlencode({
            "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
            "client_id":  PUBLIC_CLIENT,
            "device_code": dc["device_code"],
        }).encode()
        poll_req = urllib.request.Request(
            f"https://login.microsoftonline.com/{TENANT_ID}/oauth2/v2.0/token",
            data=poll_body,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
        )
        try:
            with urllib.request.urlopen(poll_req) as r:
                tok = json.loads(r.read())
            if "access_token" in tok:
                print("Authentication successful.")
                return tok["access_token"]
        except urllib.error.HTTPError as e:
            body = json.loads(e.read())
            err  = body.get("error", "")
            if err == "authorization_pending":
                continue
            if err == "slow_down":
                interval += 5
                continue
            raise SystemExit(f"Auth error: {body}")
    raise SystemExit("Device code expired.")


def _req(method: str, url: str, token: str, body: bytes | None = None,
         expect_json: bool = True):
    h = {"Authorization": f"Bearer {token}", "Accept": "application/json"}
    if body:
        h["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=body, method=method, headers=h)
    try:
        with urllib.request.urlopen(req) as r:
            data = r.read()
            return r.status, json.loads(data) if (expect_json and data) else data
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", "replace")
        raise SystemExit(f"HTTP {e.code} {method} {url}\n{detail}")


def main():
    api_base = f"https://manage.devcenter.microsoft.com/v1.0/my"

    token = device_code_flow()

    # Get current account info to find tenantId
    print("Fetching account info…")
    _, acct = _req("GET", f"{api_base}", token)
    print(f"Account: {json.dumps(acct, indent=2)[:400]}")

    # List tenants
    _, tenants = _req("GET", f"{api_base}/tenants", token)
    print(f"Tenants: {json.dumps(tenants, indent=2)[:400]}")

    tenant_entries = tenants.get("value") or tenants.get("items") or []
    if not tenant_entries:
        raise SystemExit("No tenants returned — check API response above.")

    # Use the tannpvhotmail tenant
    tenant_id = None
    for t in tenant_entries:
        tid = t.get("tenantId") or t.get("id") or ""
        if "fb755f3c" in tid or "tannpvhotmail" in str(t).lower():
            tenant_id = tid
            break
    if not tenant_id:
        tenant_id = tenant_entries[0].get("tenantId") or tenant_entries[0].get("id")
    print(f"Using tenant: {tenant_id}")

    # Add the Azure AD app as a Partner Center user with Manager role
    payload = json.dumps({
        "loginName": AAD_APP_CLIENT_ID,
        "type":      "AzureAD",
        "roles": [{"roleName": "Manager"}],
    }).encode()

    print(f"Adding AAD app {AAD_APP_CLIENT_ID} with Manager role…")
    status, resp = _req("POST",
                        f"{api_base}/tenants/{tenant_id}/users",
                        token, body=payload)
    print(f"Status: {status}")
    print(json.dumps(resp, indent=2)[:600] if isinstance(resp, dict) else resp[:600])


if __name__ == "__main__":
    main()
