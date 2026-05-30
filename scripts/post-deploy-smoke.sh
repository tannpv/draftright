#!/usr/bin/env bash
# Post-deploy smoke check for the DraftRight backend stack.
#
# Hits the critical endpoints that ANY client (macOS / iOS / Android /
# Windows / Linux) depends on and asserts their wire shape stays intact.
# Designed to gate a deploy: run it after Caddy reload or container
# restart; non-zero exit = roll back.
#
# Usage:
#   ./scripts/post-deploy-smoke.sh <base-url> [--jwt <token>]
#
# Examples:
#   ./scripts/post-deploy-smoke.sh https://api.draftright.info
#       → public-only checks (/, /health, /updates/latest)
#
#   ./scripts/post-deploy-smoke.sh https://api.draftright.info --jwt "$JWT"
#       → also drives /auth/me + both /rewrite and /v1/rewrite
#
# Covers:
#   1. /health                          NestJS responds, identity ok
#   2. /updates/latest?platform=mac     auto-update wire ok
#   3. /auth/me                         flags envelope present
#   4. /rewrite (NestJS)                rewritten_text path
#   5. /v1/rewrite JSON (Go)            text path
#   6. /v1/rewrite SSE (Go)             delta + [DONE] frames
#   7. error envelope                   { error, code, request_id }

set -euo pipefail

BASE="${1:-}"
JWT=""
while [[ $# -gt 1 ]]; do
  case "$2" in
    --jwt) JWT="$3"; shift 2 ;;
    *) echo "Unknown flag: $2" >&2; exit 2 ;;
  esac
done

if [[ -z "$BASE" ]]; then
  echo "usage: $0 <base-url> [--jwt <token>]" >&2
  exit 2
fi
BASE="${BASE%/}"

PASS=0
FAIL=0

red() { printf '\033[31m%s\033[0m\n' "$*"; }
grn() { printf '\033[32m%s\033[0m\n' "$*"; }

step() { printf '── %s\n' "$*"; }
pass() { grn "  PASS"; PASS=$((PASS+1)); }
fail() { red "  FAIL: $*"; FAIL=$((FAIL+1)); }

# ---------- 1. /health ---------------------------------------------------
step "GET $BASE/health"
body=$(curl -fsS --max-time 5 "$BASE/health" 2>/dev/null || echo '')
case "$body" in
  *'"status":"ok"'*) pass ;;
  *) fail "health body missing status:ok — got: $body" ;;
esac

# ---------- 2. /updates/latest ------------------------------------------
step "GET $BASE/updates/latest?platform=mac"
body=$(curl -fsS --max-time 5 "$BASE/updates/latest?platform=mac" 2>/dev/null || echo '')
case "$body" in
  *'"version"'*'"mac_url"'*) pass ;;
  *) fail "updates payload shape unexpected — got: $body" ;;
esac

# ---------- 3. error envelope on a known-bad route ----------------------
# Endpoints requiring auth surface the envelope on 401 even without a
# valid JWT, so we can verify the shape without burning a real token.
step "POST $BASE/rewrite without auth → 401 envelope"
resp=$(curl -s --max-time 5 -w '\nSTATUS=%{http_code}' -X POST "$BASE/rewrite" \
  -H 'Content-Type: application/json' \
  -d '{"text":"x","tone":"polished"}' 2>/dev/null || echo '')
status=$(echo "$resp" | grep '^STATUS=' | cut -d= -f2)
body=$(echo "$resp" | sed '/^STATUS=/d')
case "$status" in
  401) ;;
  *) fail "expected 401, got $status — body: $body"; status=0 ;;
esac
if [[ "$status" == "401" ]]; then
  # Loose match: must have an "error" key. Once the envelope filter is
  # live everywhere, also assert "code" + "request_id".
  case "$body" in
    *'"error"'*) pass ;;
    *) fail "error envelope missing error field — got: $body" ;;
  esac
fi

# ---------- Authenticated checks (require JWT) --------------------------
if [[ -z "$JWT" ]]; then
  echo
  echo "(no --jwt supplied — skipping authenticated checks)"
  echo
  echo "Summary: $PASS passed, $FAIL failed"
  [[ $FAIL -eq 0 ]] && exit 0 || exit 1
fi

# ---------- 4. /auth/me with flags envelope -----------------------------
step "GET $BASE/auth/me"
body=$(curl -fsS --max-time 5 "$BASE/auth/me" -H "Authorization: Bearer $JWT" 2>/dev/null || echo '')
case "$body" in
  *'"id"'*'"flags"'*'"use_go_backend"'*) pass ;;
  *) fail "auth/me payload missing id/flags/use_go_backend — got: $body" ;;
esac

# ---------- 5. NestJS /rewrite happy path -------------------------------
step "POST $BASE/rewrite (NestJS path)"
body=$(curl -fsS --max-time 20 -X POST "$BASE/rewrite" \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"smoke test polished","tone":"polished"}' 2>/dev/null || echo '')
case "$body" in
  *'"rewritten_text"'*) pass ;;
  *) fail "rewrite payload missing rewritten_text — got: $body" ;;
esac

# ---------- 6. Go /v1/rewrite JSON --------------------------------------
step "POST $BASE/v1/rewrite (Go JSON path)"
body=$(curl -fsS --max-time 20 -X POST "$BASE/v1/rewrite" \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"smoke test go","tone":"polished"}' 2>/dev/null || echo '')
if [[ "$body" == *'"text"'* && "$body" == *'"service":"rewrite-go"'* ]]; then
  pass
else
  fail "go rewrite payload missing text/service — got: $body"
fi

# ---------- 7. Go /v1/rewrite SSE ---------------------------------------
step "POST $BASE/v1/rewrite (Go SSE path)"
# -N disables curl buffering so SSE frames land as they arrive.
sse=$(curl -fsSN --max-time 30 -X POST "$BASE/v1/rewrite" \
  -H "Authorization: Bearer $JWT" \
  -H 'Accept: text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{"text":"smoke test sse","tone":"polished"}' 2>/dev/null || echo '')
case "$sse" in
  *'"delta"'*'[DONE]'*) pass ;;
  *) fail "go SSE missing delta or [DONE] — got: $sse" ;;
esac

echo
echo "Summary: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
