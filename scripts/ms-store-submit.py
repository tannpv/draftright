#!/usr/bin/env python3
"""Automated Microsoft Store submission for DraftRight (Windows MSIX).

Drives the Microsoft Store submission REST API end-to-end:
  token -> create submission (clones last published) -> replace the app
  package with the new .msixbundle -> upload -> commit -> poll status.

Prerequisites
-------------
1. Credentials in env (or sourced from ~/.config/draftright/ms-store.env):
     AZURE_TENANT_ID, MS_STORE_CLIENT_ID, MS_STORE_CLIENT_SECRET,
     MS_STORE_APPLICATION_ID   (the Store ID, e.g. 9P5WWDGGVCS3)
2. The Azure AD app MUST be added to the Partner Center account
   (Account settings -> User management -> Azure AD applications) with a
   Manager/Developer role. Without it the API returns:
     "A valid account could not be found with given authorization token".

Usage
-----
    source ~/.config/draftright/ms-store.env
    python3 scripts/ms-store-submit.py path/to/DraftRight-Store-<sha>.msixbundle
    # add --commit to actually submit; without it, does a dry run that
    # creates+populates the submission but stops before commit.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
import time
import urllib.request
import urllib.error
import urllib.parse
import zipfile
import tempfile

API = "https://manage.devcenter.microsoft.com/v1.0/my"
TOKEN_HOST = "https://login.microsoftonline.com"
RESOURCE = "https://manage.devcenter.microsoft.com"


def _req(method: str, url: str, token: str | None = None,
         body: bytes | None = None, headers: dict | None = None,
         expect_json: bool = True):
    h = dict(headers or {})
    if token:
        h["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(url, data=body, method=method, headers=h)
    try:
        with urllib.request.urlopen(req) as resp:
            data = resp.read()
            if expect_json and data:
                return resp.status, json.loads(data)
            return resp.status, data
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", "replace")
        raise SystemExit(f"HTTP {e.code} {method} {url}\n{detail}")


def get_token() -> str:
    tenant = os.environ["AZURE_TENANT_ID"]
    form = urllib.parse.urlencode({
        "grant_type": "client_credentials",
        "client_id": os.environ["MS_STORE_CLIENT_ID"],
        "client_secret": os.environ["MS_STORE_CLIENT_SECRET"],
        "resource": RESOURCE,
    }).encode()
    _, data = _req("POST", f"{TOKEN_HOST}/{tenant}/oauth2/token", body=form,
                   headers={"Content-Type": "application/x-www-form-urlencoded"})
    tok = data.get("access_token")
    if not tok:
        raise SystemExit(f"No access_token in response: {data}")
    return tok


def get_app(token: str, app_id: str) -> dict:
    _, app = _req("GET", f"{API}/applications/{app_id}", token)
    return app


def delete_pending_submission(token: str, app_id: str, app: dict):
    pending = app.get("pendingApplicationSubmission")
    if pending:
        sid = pending["id"]
        print(f"Deleting existing pending submission {sid}…")
        _req("DELETE", f"{API}/applications/{app_id}/submissions/{sid}",
             token, expect_json=False)


def create_submission(token: str, app_id: str) -> dict:
    # Clones the last published submission (metadata, listings, etc.).
    _, sub = _req("POST", f"{API}/applications/{app_id}/submissions", token,
                  body=b"", headers={"Content-Type": "application/json"})
    return sub


def build_upload_zip(bundle_path: str) -> tuple[str, str]:
    """Zip the .msixbundle; the entry name must match the package fileName."""
    file_name = os.path.basename(bundle_path)
    tmp = tempfile.NamedTemporaryFile(suffix=".zip", delete=False)
    tmp.close()
    with zipfile.ZipFile(tmp.name, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(bundle_path, arcname=file_name)
    return tmp.name, file_name


def upload_zip(upload_url: str, zip_path: str):
    with open(zip_path, "rb") as f:
        data = f.read()
    # Azure block blob PUT.
    _req("PUT", upload_url, body=data, expect_json=False,
         headers={"x-ms-blob-type": "BlockBlob",
                  "Content-Type": "application/zip"})


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("bundle", help="path to the .msixbundle / .msixupload")
    ap.add_argument("--commit", action="store_true",
                    help="commit the submission (without this: dry run, no commit)")
    ap.add_argument("--poll", action="store_true",
                    help="poll commit status until it leaves CommitStarted")
    args = ap.parse_args()

    for k in ("AZURE_TENANT_ID", "MS_STORE_CLIENT_ID", "MS_STORE_CLIENT_SECRET",
              "MS_STORE_APPLICATION_ID"):
        if not os.environ.get(k):
            raise SystemExit(f"Missing env {k} — source ~/.config/draftright/ms-store.env")
    if not os.path.isfile(args.bundle):
        raise SystemExit(f"Bundle not found: {args.bundle}")

    app_id = os.environ["MS_STORE_APPLICATION_ID"]
    token = get_token()
    print("Auth OK.")

    app = get_app(token, app_id)
    print(f"App: {app.get('primaryName', app_id)}")
    delete_pending_submission(token, app_id, app)

    sub = create_submission(token, app_id)
    sid = sub["id"]
    upload_url = sub["fileUploadUrl"]
    print(f"Created submission {sid}.")

    file_name = os.path.basename(args.bundle)
    # Replace packages: mark all existing for delete, add the new bundle.
    for pkg in sub.get("applicationPackages", []):
        pkg["fileStatus"] = "PendingDelete"
    sub["applicationPackages"].append({
        "fileName": file_name,
        "fileStatus": "PendingUpload",
    })

    # PUT the updated submission metadata.
    body = json.dumps(sub).encode()
    _req("PUT", f"{API}/applications/{app_id}/submissions/{sid}", token,
         body=body, headers={"Content-Type": "application/json"})
    print("Submission packages updated.")

    zip_path, _ = build_upload_zip(args.bundle)
    print(f"Uploading {file_name}…")
    upload_zip(upload_url, zip_path)
    os.unlink(zip_path)
    print("Upload complete.")

    if not args.commit:
        print(f"\nDRY RUN — submission {sid} populated but NOT committed.")
        print("Re-run with --commit to submit for certification.")
        return

    _req("POST", f"{API}/applications/{app_id}/submissions/{sid}/commit",
         token, body=b"", headers={"Content-Type": "application/json"})
    print(f"Committed submission {sid}. Microsoft will now pre-process + certify.")

    if args.poll:
        while True:
            _, st = _req("GET",
                         f"{API}/applications/{app_id}/submissions/{sid}/status",
                         token)
            status = st.get("status")
            print(f"  status: {status}")
            if status not in ("CommitStarted",):
                break
            time.sleep(30)


if __name__ == "__main__":
    main()
