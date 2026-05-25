#!/usr/bin/env bash
# Smoke test for the payment API changes made 2026-05-25.
# Asserts the *deployed* behavior end-to-end. Usage:
#   BASE_URL=https://api.draftright.info bash scripts/smoke-payment-api.sh
set -uo pipefail
BASE="${BASE_URL:-https://api.draftright.info}"
pass=0; fail=0
ok()  { echo "  ✓ $1"; pass=$((pass+1)); }
no()  { echo "  ✗ $1"; fail=$((fail+1)); }

echo "== payment API smoke @ $BASE =="

# 1. GET /plans is public + returns active plans
body=$(curl -s "$BASE/plans"); code=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/plans")
[ "$code" = "200" ] && ok "GET /plans -> 200" || no "GET /plans -> $code"
echo "$body" | grep -q '"is_active":true' && ok "/plans returns active plans" || no "/plans has no active plans"

# 2. GET /payment/methods returns enabled methods, WITHOUT momo/paypal
m=$(curl -s "$BASE/payment/methods")
echo "$m" | grep -q '"methods"' && ok "GET /payment/methods -> {methods}" || no "/payment/methods shape"
echo "$m" | grep -qiE 'momo|paypal' && no "/payment/methods still lists momo/paypal: $m" || ok "/payment/methods excludes momo + paypal"

# 3. Removed provider webhooks now 404
for p in momo paypal; do
  c=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/payment/webhook/$p" -H 'Content-Type: application/json' -d '{}')
  [ "$c" = "404" ] && ok "webhook/$p removed (404)" || no "webhook/$p -> $c (expected 404)"
done

# 4. SePay webhook: live + secured (bad key rejected)
c=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/payment/webhook/sepay" \
     -H 'Content-Type: application/json' -H 'Authorization: Apikey WRONG' -d '{"content":"x","transferAmount":1}')
[ "$c" = "401" ] && ok "webhook/sepay rejects bad key (401)" || no "webhook/sepay -> $c (expected 401)"

echo "== $pass passed, $fail failed =="
[ "$fail" -eq 0 ]
