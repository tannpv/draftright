#!/usr/bin/env python3
"""
Push a new MSIX package to the Microsoft Store via the Store submission API.

ARMED but BLOCKED until the Azure AD app is linked to the Partner Center account:
  Partner Center → Account settings → User management → Azure AD applications →
  Add the app (MS_STORE_CLIENT_ID) with Manager role.
Until then auth succeeds but the API returns 400 "A valid account could not be
found with given authorization token".

NOTE: this updates PACKAGES only. Product declarations (e.g. the 11.16
generative-AI checkbox) are NOT part of the submission API schema — set those in
Partner Center → Properties → Product declarations (web UI).

Usage:
  set -a; . ~/.config/draftright/ms-store.env; set +a
  python3 scripts/ms-store-submit.py --msix /path/to/DraftRight.msixupload
"""
import argparse, os, sys, time, zipfile, tempfile, json, urllib.request, urllib.error

BASE = "https://manage.devcenter.microsoft.com/v1.0/my"


def _req(method, url, token, body=None, headers=None):
    h = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    if headers:
        h.update(headers)
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method, headers=h)
    try:
        with urllib.request.urlopen(req) as r:
            raw = r.read()
            return json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        sys.exit(f"HTTP {e.code} {method} {url}\n{e.read().decode()[:500]}")


def get_token():
    tid = os.environ["AZURE_TENANT_ID"]
    cid = os.environ["MS_STORE_CLIENT_ID"]
    sec = os.environ["MS_STORE_CLIENT_SECRET"]
    data = (f"grant_type=client_credentials&client_id={cid}&client_secret={sec}"
            "&scope=https://manage.devcenter.microsoft.com/.default").encode()
    req = urllib.request.Request("https://login.microsoftonline.com/%s/oauth2/v2.0/token" % tid, data=data)
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())["access_token"]


def upload_zip(sas_url, msix_path):
    # The submission API expects a ZIP containing the package(s).
    with tempfile.NamedTemporaryFile(suffix=".zip", delete=False) as tf:
        zip_path = tf.name
    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as z:
        z.write(msix_path, arcname=os.path.basename(msix_path))
    body = open(zip_path, "rb").read()
    req = urllib.request.Request(sas_url, data=body, method="PUT",
                                 headers={"x-ms-blob-type": "BlockBlob"})
    urllib.request.urlopen(req)
    os.unlink(zip_path)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--msix", required=True)
    args = ap.parse_args()
    if not os.path.isfile(args.msix):
        sys.exit(f"MSIX not found: {args.msix}")

    app_id = os.environ["MS_STORE_APPLICATION_ID"]
    token = get_token()
    print("✓ token")

    app = _req("GET", f"{BASE}/applications/{app_id}", token)
    # Remove any in-progress submission so we can create a fresh one.
    pending = app.get("pendingApplicationSubmission")
    if pending:
        _req("DELETE", f"{BASE}/applications/{app_id}/submissions/{pending['id']}", token)
        print("✓ deleted pending submission")

    sub = _req("POST", f"{BASE}/applications/{app_id}/submissions", token)
    sub_id, upload_url = sub["id"], sub["fileUploadUrl"]
    print(f"✓ created submission {sub_id}")

    fname = os.path.basename(args.msix)
    for p in sub.get("applicationPackages", []):
        p["fileStatus"] = "PendingDelete"
    sub["applicationPackages"].append({"fileName": fname, "fileStatus": "PendingUpload"})
    _req("PUT", f"{BASE}/applications/{app_id}/submissions/{sub_id}", token, body=sub)
    print("✓ submission updated (new package queued)")

    upload_zip(upload_url, args.msix)
    print("✓ MSIX uploaded")

    _req("POST", f"{BASE}/applications/{app_id}/submissions/{sub_id}/commit", token)
    print("✓ commit started — polling…")
    while True:
        st = _req("GET", f"{BASE}/applications/{app_id}/submissions/{sub_id}/status", token)
        s = st.get("status")
        print(f"  status: {s}")
        if s in ("CommitFailed", "PreProcessingFailed", "CertificationFailed", "Release", "Published"):
            break
        if s == "CommitStarted":
            time.sleep(20)
            continue
        time.sleep(20)
    print("Done. Set the generative-AI declaration in Partner Center → Properties if not already.")


if __name__ == "__main__":
    main()
