# Extension Tokens — Verification Record

For each row, after performing the test, fill in: PASS / FAIL, the date/time,
and a 1-line note. If FAIL, include the observed error.

Plan: `docs/superpowers/plans/2026-05-02-extension-tokens.md`
Test cases: `docs/test-cases.xlsx` sheet `EXTTOK` (TC-IDs in parens below)

---

## A. Backend dual-accept — verified against PRODUCTION (api.draftright.info)

All 8 rows below verified by `scripts/test-extension-tokens-e2e.py` against
production. Re-run any time:
```
python3 scripts/test-extension-tokens-e2e.py
```

| # | TC | Test | Result | Date | Note |
|---|---|------|--------|------|------|
| A1 | 008 | `POST /rewrite` with valid user JWT returns rewritten text | ✅ PASS | 2026-05-03 | Automated |
| A2 | 001 | `POST /auth/extension-tokens` with user JWT returns `{ token, id }` and token starts with `dr_ext_` | ✅ PASS | 2026-05-03 | Automated; token format `dr_ext_[A-Za-z0-9_-]{43}` validated |
| A3 | 009 | `POST /rewrite` with extension token returns rewritten text | ✅ PASS | 2026-05-03 | Automated |
| A4 | 010 | `GET /auth/me` with extension token returns 401 | ✅ PASS | 2026-05-03 | Automated; scope enforcement |
| A5 | 002 | `GET /auth/extension-tokens` lists rows; `token_hash` and `user_id` fields stripped from JSON | ✅ PASS | 2026-05-03 | Automated |
| A6 | 007 | `DELETE /auth/extension-tokens/:id` with user JWT returns 204 | ✅ PASS | 2026-05-03 | Automated |
| A7 | 005 | After A6, `POST /rewrite` with the revoked extension token returns 401 | ✅ PASS | 2026-05-03 | Automated |
| A8 | 003 | Re-mint with same `device_id` invalidates the old token (old returns 401, new works) | ✅ PASS | 2026-05-03 | Automated; partial-unique-index rotation working |

Reproduce locally:

```bash
JWT=$(curl -s -X POST http://localhost:3000/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"<test-user>","password":"<password>"}' | jq -r .access_token)

# A2
EXT=$(curl -s -X POST http://localhost:3000/auth/extension-tokens \
  -H "Authorization: Bearer $JWT" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"00000000-0000-0000-0000-000000000001","device_name":"verify"}' \
  | jq -r .token)
echo "$EXT" | head -c 8  # expect: dr_ext_

# A3
curl -fsS -X POST http://localhost:3000/rewrite \
  -H "Authorization: Bearer $EXT" \
  -H 'Content-Type: application/json' \
  -d '{"text":"hello there","tone":"casual"}' | jq .

# A4
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:3000/auth/me \
  -H "Authorization: Bearer $EXT"  # expect: 401
```

---

## B. iOS — fresh install, real device

Prerequisites: Apple Developer Program active, manual portal + Xcode steps complete (see plan Stage 2).

| # | TC | Test | Result | Date | Note |
|---|---|------|--------|------|------|
| B1 | 011 | Install new build, log in with test account | | | |
| B2 | 011, 014 | After login, the keyboard's first rewrite call succeeds within 5 minutes | | | |
| B3 | **015** | Wait 30 minutes (longer than access JWT TTL of 15m), then invoke keyboard rewrite — it succeeds | | | This is the bug repro. PASS = bug fixed. |
| B4 | **016** | Wait 30 minutes, then invoke share extension — it succeeds | | | |
| B5 | | After 24 hours of no main-app activity, keyboard still works | | | |
| B6 | 012, 020 | Log out in main app — keyboard rewrite shows "Please login in DraftRight app" | | | |
| B7 | 014 | Log back in — keyboard works again on first attempt (no need to "warm" by visiting playground) | | | |

---

## C. Android — fresh install, real device

No Apple Developer Program dependency — can be tested today.

| # | TC | Test | Result | Date | Note |
|---|---|------|--------|------|------|
| C1 | 011 | Install new build (`flutter build apk --debug && flutter install`), log in with test account | | | |
| C2 | 011 | After login, the IME's first rewrite call succeeds within 5 minutes | | | |
| C3 | **017** | Wait 30 minutes, then invoke IME rewrite — it succeeds | | | This is the bug repro on Android. PASS = bug fixed. |
| C4 | | After 24 hours of no main-app activity, IME still works | | | |
| C5 | 012, 020 | Log out in main app — IME rewrite shows the auth-required error | | | |
| C6 | | Log back in — IME works again on first attempt | | | |

---

## D. Migration safety (existing user upgrades from old build)

| # | TC | Test | Result | Date | Note |
|---|---|------|--------|------|------|
| D1 | 018 | Install OLD build (pre-Stage-3), log in. Confirm keyboard works. Note: old access token is in shared storage. | | | |
| D2 | 018 | Install NEW build OVER the old install (no fresh install). Do NOT open main app. | | | |
| D3 | 018 | Invoke keyboard. Within the access-JWT lifetime, it should still work via the fallback path. | | | |
| D4 | 018 | Open main app once. ExtensionTokenService mints. Confirm via Charles/mitmproxy that the mint endpoint was called. | | | |
| D5 | 018 | Wait 30 minutes. Keyboard now uses the extension token (Authorization header starts with `Bearer dr_ext_`). | | | |

---

## Production releases

Fill in once Stage 7 deploy lands.

```
- Backend: <YYYY-MM-DD HH:MM TZ>, build <commit-sha>
- iOS App Store: submitted <YYYY-MM-DD>, expected review <YYYY-MM-DD>
- Android Play Store: rolled out <YYYY-MM-DD>
```

---

## Final outcome (Stage 7.7)

Fill in after self-verification on production.

```
- iPhone keyboard 30-min idle test: PASS / FAIL on <date>
- iPhone share extension 30-min idle test: PASS / FAIL on <date>
- Android IME 30-min idle test: PASS / FAIL on <date>
- Production back-compat smoke (Stage 7.5 step 4): PASS / FAIL on <date>
- Verified by: Tan
```
