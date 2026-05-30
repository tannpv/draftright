#!/usr/bin/env bash
# Smoke test for the Go /rewrite microservice.
#
# Usage:
#   ./scripts/smoke.sh <base-url> [<jwt-token>]
#
# Examples:
#   ./scripts/smoke.sh http://localhost:3001
#       — only checks /health (no auth needed).
#   ./scripts/smoke.sh https://draftright.info "$JWT"
#       — also drives /v1/rewrite (SSE + JSON) with the token.
#
# Exits non-zero on any failure. Designed for use after a deploy:
# Caddy reload → run this → roll back if any check fails.

set -euo pipefail

base="${1:-}"
jwt="${2:-}"

if [[ -z "$base" ]]; then
  echo "usage: $0 <base-url> [<jwt-token>]" >&2
  exit 2
fi

# Strip trailing slash so we can naively concatenate paths.
base="${base%/}"

red() { printf '\033[31m%s\033[0m\n' "$*"; }
grn() { printf '\033[32m%s\033[0m\n' "$*"; }

step() { printf '─── %s\n' "$*"; }
fail() { red "FAIL: $*"; exit 1; }
ok()   { grn "OK:   $*"; }

# --- /health -------------------------------------------------------
step "GET $base/health"
body=$(curl -fsS --max-time 10 "$base/health")
case "$body" in
  *'"status":"ok"'*) ok "health" ;;
  *) fail "health body did not contain status:ok — got: $body" ;;
esac

# --- /v1/rewrite (JSON path) — needs JWT ---------------------------
if [[ -z "$jwt" ]]; then
  grn "Skipping authed checks (no JWT supplied)."
  exit 0
fi

step "POST $base/v1/rewrite  (Accept: application/json)"
json=$(curl -fsS --max-time 30 \
  -H "Authorization: Bearer $jwt" \
  -H 'Content-Type: application/json' \
  -d '{"text":"smoke test input","tone":"polished"}' \
  "$base/v1/rewrite")
case "$json" in
  *'"text"'*'"service":"rewrite-go"'*) ok "JSON rewrite returned" ;;
  *) fail "JSON rewrite shape unexpected — got: $json" ;;
esac

# --- /v1/rewrite (SSE path) ---------------------------------------
step "POST $base/v1/rewrite  (Accept: text/event-stream)"
# -N disables curl's output buffering so SSE chunks land in $sse as
# they arrive rather than being held until completion.
sse=$(curl -fsSN --max-time 60 \
  -H "Authorization: Bearer $jwt" \
  -H 'Accept: text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{"text":"smoke test SSE","tone":"polished"}' \
  "$base/v1/rewrite")

# Must contain at least one delta + the terminal marker.
case "$sse" in
  *'"delta"'*'[DONE]'*) ok "SSE stream produced deltas + [DONE]" ;;
  *) fail "SSE response missing delta or [DONE] — got:\n$sse" ;;
esac

grn "All smoke checks passed."
