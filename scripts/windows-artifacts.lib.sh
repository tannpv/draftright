#!/usr/bin/env bash
# Shared helpers for staging Windows CI artifacts (installer .exe / Store MSIX)
# to the public /downloads dir. Sourced by store-package.sh and
# windows-install.sh — RULE #1: one copy of the find-run / dispatch / download /
# stage / verify logic and the constants, not a copy per script.
#
# Requires: gh (authed), the `draftright` ssh alias, curl, unzip.

WORKFLOW="build-windows.yml"
DROPLET="draftright"
DOWNLOADS_PATH="/var/www/draftright/downloads"
DOWNLOADS_URL="https://draftright.info/downloads"
CSPROJ="DraftRightWindows/DraftRightWindows/DraftRightWindows.csproj"
MIN_ARTIFACT_BYTES=1000000   # below this a /downloads URL is the SPA HTML fallback (#144), not a real file

# App version from csproj <Version> (X.Y.Z).
wa_version() {
  grep -oE '<Version>[0-9]+\.[0-9]+\.[0-9]+</Version>' "$CSPROJ" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1
}

# Latest completed+successful build run id for a ref (branch or tag). Empty if none.
wa_latest_run() {
  gh run list --workflow="$WORKFLOW" --branch "$1" --limit 1 \
    --json databaseId,status,conclusion \
    --jq '.[] | select(.status=="completed" and .conclusion=="success") | .databaseId' | head -1
}

# Dispatch a build on a ref; echo the new run id once it registers.
wa_dispatch() {
  local ref="$1" rid=""
  gh workflow run "$WORKFLOW" --ref "$ref" -f runtime=win-x64 >&2
  for _ in $(seq 1 10); do
    rid=$(gh run list --workflow="$WORKFLOW" --branch "$ref" --limit 1 --json databaseId --jq '.[0].databaseId' 2>/dev/null)
    [ -n "$rid" ] && { echo "$rid"; return 0; }
    sleep 3
  done
  echo "ERROR: dispatched build for '$ref' did not register" >&2; return 1
}

# Resolve a ref to a usable successful run: reuse the latest, or dispatch+wait if
# none (or if FORCE_BUILD=1). Echoes the run id.
wa_resolve_run() {
  local ref="$1" rid
  if [ "${FORCE_BUILD:-0}" != "1" ]; then
    rid=$(wa_latest_run "$ref")
    if [ -n "$rid" ]; then echo "    using existing build run $rid for '$ref'" >&2; echo "$rid"; return 0; fi
  fi
  echo "==> No usable build for '$ref' — dispatching one (this takes ~10 min)..." >&2
  rid=$(wa_dispatch "$ref") || return 1
  echo "    watching run $rid..." >&2
  gh run watch "$rid" --exit-status --interval 30 >&2 || { echo "ERROR: build $rid failed" >&2; return 1; }
  echo "$rid"
}

# Download a named artifact from a run into a fresh temp dir; echo the file path.
wa_download() {
  local run_id="$1" artifact="$2" d
  d=$(mktemp -d)
  gh run download "$run_id" -n "$artifact" -D "$d" >&2 || { echo "ERROR: no '$artifact' artifact on run $run_id" >&2; return 1; }
  ls "$d"/* | head -1
}

# Stage a local file to /downloads as <remote_name>; echo the public URL.
wa_stage() {
  local local_file="$1" remote_name="$2"
  scp -o ConnectTimeout=30 "$local_file" "$DROPLET:/tmp/$remote_name" >&2
  ssh -o ConnectTimeout=30 "$DROPLET" "sudo mv /tmp/$remote_name $DOWNLOADS_PATH/$remote_name && sudo chmod 644 $DOWNLOADS_PATH/$remote_name" >&2
  echo "$DOWNLOADS_URL/$remote_name"
}

# Verify a staged URL serves real bytes (200 + big enough to not be the SPA
# HTML fallback, #144).
wa_verify() {
  local url="$1" head len
  head=$(curl -sI "$url")
  echo "$head" | grep -qiE "HTTP/[12](\.[0-9])? 200" || { echo "ERROR: $url did not return 200" >&2; return 1; }
  len=$(echo "$head" | grep -i '^content-length:' | grep -oE '[0-9]+' | head -1)
  if [ "${len:-0}" -lt "$MIN_ARTIFACT_BYTES" ]; then
    echo "ERROR: $url is only ${len:-0} bytes — likely the SPA HTML fallback, not the file (#144)" >&2; return 1
  fi
  echo "    verified: $url (${len} bytes)" >&2
}
