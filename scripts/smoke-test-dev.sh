#!/usr/bin/env bash
#
# Smoke-test the dev environment (api.dev / dev / admin.dev).  Run
# after every redeploy from develop — fails fast on any wired-up
# regression before users discover it.
#
# Usage:
#   ./scripts/smoke-test-dev.sh
#   ./scripts/smoke-test-dev.sh --verbose      # show full responses
#
# Exits 0 only when every check passes; non-zero with a red ✗ on the
# first failure.
#
set -uo pipefail

API="https://api.dev.draftright.info"
WEB="https://dev.draftright.info"
ADMIN="https://admin.dev.draftright.info"

VERBOSE=0
for arg in "$@"; do
    [[ "$arg" == "--verbose" || "$arg" == "-v" ]] && VERBOSE=1
done

PASS=0
FAIL=0

check() {
    local label="$1"; shift
    local cmd="$*"
    local out
    if out=$(eval "$cmd" 2>&1); then
        echo "✓ $label"
        PASS=$((PASS+1))
        [[ $VERBOSE -eq 1 ]] && printf '  %s\n' "$out" | head -c 600 && echo
        return 0
    fi
    echo "✗ $label"
    echo "  → $out" | head -c 600
    echo
    FAIL=$((FAIL+1))
    return 1
}

echo "── Dev env smoke test ─────────────────────────────────"
echo "API:   $API"
echo "Web:   $WEB"
echo "Admin: $ADMIN"
echo

# Backend reachable + identifies as DraftRight
check "API /health returns app=draftright" \
    "curl -fsS --max-time 5 '$API/health' | jq -e '.app == \"draftright\"' >/dev/null"

# Public plans endpoint reachable + non-empty
check "API /plans returns ≥1 plan" \
    "curl -fsS --max-time 5 '$API/plans' | jq -e 'length > 0' >/dev/null"

# Payment methods discoverable (admin must have enabled at least one)
check "API /payment/methods returns ≥1 method" \
    "curl -fsS --max-time 5 '$API/payment/methods' | jq -e '.methods | length > 0' >/dev/null"

# New /payment/portal endpoint should exist (401 without auth = good;
# 404 means deploy missed the unified-portal feature)
check "API /payment/portal exists (401 without auth)" \
    "test \"\$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 '$API/payment/portal')\" = '401'"

# Website AASA file served as application/json (Apple iOS Universal
# Link verification looks at Content-Type)
check "Web AASA has Content-Type application/json" \
    "curl -fsSI --max-time 5 '$WEB/.well-known/apple-app-site-association' | grep -qi 'content-type: application/json'"

# Android assetlinks.json reachable + parseable
check "Web assetlinks.json is valid JSON" \
    "curl -fsS --max-time 5 '$WEB/.well-known/assetlinks.json' | jq -e 'type == \"array\" and length > 0' >/dev/null"

# Payment success page renders (200, contains expected string).
# Astro builds page dirs with trailing slashes → -L follows the 308.
check "Web /payment/success page renders" \
    "curl -fLsS --max-time 5 '$WEB/payment/success' | grep -q 'Payment confirmed'"

# Payment cancel page renders
check "Web /payment/cancel page renders" \
    "curl -fLsS --max-time 5 '$WEB/payment/cancel' | grep -q 'Checkout cancelled'"

# Admin SPA loads (200 + serves the React shell)
check "Admin SPA serves" \
    "curl -fsSI --max-time 5 '$ADMIN/' | grep -q '200 OK\|200'"

echo
echo "── Result ────────────────────────────────────────────"
echo "Passed: $PASS    Failed: $FAIL"
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
