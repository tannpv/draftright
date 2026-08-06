#!/usr/bin/env bash
# Verify that a published download URL actually serves the artifact we uploaded.
#
# Usage:
#   verify-published-artifact.sh <url> <expected-bytes> [--sha256 <64-hex>] [--deep]
#
# Examples:
#   verify-published-artifact.sh https://draftright.info/downloads/x.dmg 3659797
#   verify-published-artifact.sh https://draftright.info/downloads/x.dmg 3659797 \
#       --sha256 ac3df6e6…07d5 --deep
#
# Why this exists (issue #144):
#   A *missing* file under /downloads does not 404. Caddy falls through to the
#   marketing site's SPA handler and returns its HTML with a 200, so every
#   status-code-only check passes against a completely absent artifact:
#
#     $ curl -sI .../DraftRight-Linux-2.1.0.tar.gz
#     HTTP/2 200
#     content-type: text/html; charset=utf-8      <-- the tell
#     content-length: 33825                       <-- and this
#
#   That is how a nonexistent Linux download sat advertised in
#   /updates/latest and versions.json without anyone noticing. Checking the
#   status code is not enough; check what came back.
#
# Checks (cheap, HEAD-only by default):
#   1. HTTP status is 200
#   2. Content-Type is not text/html  — the SPA-fallback signature
#   3. Content-Length equals the local file's byte count exactly
#
# With --deep it additionally downloads the artifact and compares its SHA-256
# to --sha256. That is the strongest check but re-downloads the file, so it is
# opt-in: a 134 MB Windows installer does not need re-fetching on every publish.
set -euo pipefail

if [ $# -lt 2 ]; then
  echo "Usage: $0 <url> <expected-bytes> [--sha256 <64-hex>] [--deep]" >&2
  exit 1
fi

URL="$1"
EXPECTED_BYTES="$2"
shift 2
SHA256=""
DEEP=0
while [ $# -gt 0 ]; do
  case "$1" in
    --sha256) SHA256="$2"; shift 2 ;;
    --deep)   DEEP=1; shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

fail() { echo "    ✗ $*" >&2; exit 1; }

# One HEAD request supplies all three cheap checks.
HEADERS=$(curl -sSI --max-time 60 "$URL" || fail "HEAD request failed for $URL")

status=$(printf '%s' "$HEADERS" | awk 'tolower($1) ~ /^http/ {print $2}' | tail -n1)
# Header names are case-insensitive per RFC 9110, and the value may carry
# parameters (e.g. "text/html; charset=utf-8") — normalise both.
ctype=$(printf '%s' "$HEADERS" | awk -F': *' 'tolower($1)=="content-type" {print tolower($2)}' | tail -n1 | tr -d '\r')
clen=$(printf '%s' "$HEADERS"  | awk -F': *' 'tolower($1)=="content-length" {print $2}' | tail -n1 | tr -dc '0-9')

echo "    url:       $URL"
echo "    status:    ${status:-?}"
echo "    type:      ${ctype:-?}"
echo "    length:    ${clen:-?} (expected $EXPECTED_BYTES)"

[ "$status" = "200" ] || fail "expected HTTP 200, got ${status:-no status}"

case "$ctype" in
  text/html*)
    fail "served text/html — the file is MISSING and Caddy returned the marketing site (see issue #144)"
    ;;
esac

if [ -z "$clen" ]; then
  fail "no Content-Length header — cannot confirm the artifact is intact"
fi
if [ "$clen" != "$EXPECTED_BYTES" ]; then
  fail "size mismatch: server has $clen bytes, local artifact is $EXPECTED_BYTES"
fi

if [ "$DEEP" = "1" ]; then
  [ -n "$SHA256" ] || fail "--deep requires --sha256"
  TMP=$(mktemp)
  # shellcheck disable=SC2064  # expand TMP now, not at trap time
  trap "rm -f '$TMP'" EXIT
  curl -fsS --max-time 600 -o "$TMP" "$URL" || fail "download failed"
  if command -v sha256sum >/dev/null 2>&1; then
    got=$(sha256sum "$TMP" | awk '{print $1}')
  else
    got=$(shasum -a 256 "$TMP" | awk '{print $1}')
  fi
  echo "    sha256:    $got"
  [ "$got" = "$SHA256" ] || fail "sha256 mismatch: served $got, expected $SHA256"
  echo "    ✓ deep verify passed — served bytes hash to the published digest"
fi

echo "    ✓ artifact verified"
