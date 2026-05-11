# macOS App: Adopt Extension Token Model (`dr_ext_*`)

**Date:** 2026-05-09
**Status:** Proposed — not started
**Estimated effort:** 1.5–2 days
**Priority:** High (kills recurring "had to sign in again" papercut for paying users)

## Problem

The macOS app uses 15-min JWT access tokens + 90-day JWT refresh tokens. Refresh tokens are signed with `JWT_REFRESH_SECRET`. Two recurring failure modes wipe user credentials silently:

1. **Backend redeploy regenerates `JWT_REFRESH_SECRET`** → every refresh token globally invalidated overnight. Users open the app, see "Sign in" again. No clear cause shown to user.
2. **Admin lowers `refresh_token_expiry_days` setting** → existing tokens expire immediately. Same outcome.

The 2026-05-09 client patch (`AppModel.silentRefresh` classifies `RefreshResult.unauthorized` vs `.transient`) prevents *transient* network failures from wiping tokens, but does NOT address the two failure modes above — when backend explicitly says 401, we still wipe.

## Why extension tokens fix this

iOS keyboard, iOS share extension, and Android keyboard already use server-side `dr_ext_*` tokens (see `backend/CLAUDE.md` → "Extension tokens"). Properties:

- **Stored as `sha256(token_hash)`** in `extension_tokens` table — not signed, no secret to rotate
- **Sliding 90-day window** — `last_used_at` resets on every successful API call; only fully idle devices expire
- **Per-device** — `(user_id, device_id)` unique pair
- **Scoped** — `rewrite` scope only; cannot hit `/auth/me`, `/payment`, `/admin`
- **Survive `JWT_REFRESH_SECRET` rotation** entirely (the secret isn't used for these tokens)
- **Revocable individually** without affecting other devices/sessions

Adopting them for the macOS app means: a user signs in once, the app mints a `dr_ext_*` token tied to a stable macOS device ID, and the user stays signed in for at least 90 days of any usage — through any number of backend deploys, any secret rotation, any admin setting tweak.

## Architecture

```
Current:  Login → JWT access (15min) + JWT refresh (90d)
                 ↓ refresh on 401
                 Backend signs new pair with JWT_REFRESH_SECRET ← brittle

Proposed: Login → JWT access (15min) + JWT refresh (90d, kept as fallback)
                 ↓ on first successful login, also:
                 POST /auth/extension-tokens { device_id, scopes: ["rewrite"] }
                 → returns dr_ext_* token (one-time exposure)
                 → app stores in Keychain alongside JWT pair
                 ↓
                 /rewrite calls send dr_ext_* via Authorization: Bearer dr_ext_xxx
                 (RewriteAuthGuard already accepts both forms)
                 ↓ if dr_ext_* fails 401
                 Try JWT refresh path as fallback (existing code)
                 ↓ if both fail
                 sessionExpired = true (existing UI banner)
```

## Stages

### Stage 1 — Backend audit (0.5 day)

- Verify `RewriteAuthGuard` accepts `dr_ext_*` (already does per `backend/CLAUDE.md`)
- Confirm extension-tokens endpoints exist for desktop clients (currently scoped to iOS/Android — may need a `device_type` field if we want to track macOS separately)
- Decision: do we want a separate `macos` device_type for analytics, or treat as generic? Recommend track separately.

### Stage 2 — macOS device ID + Keychain entry (0.5 day)

- Add stable `deviceID` to `AppModel`: derive from `IORegistryEntryCreateCFProperty(IOPlatformExpertDevice, "IOPlatformUUID", ...)` — survives reinstall.
- Add `extensionToken: String` property to `AppModel`, stored in Keychain (service: `com.draftright.app.v2.exttoken`).

### Stage 3 — Mint flow on first login (0.5 day)

- After successful sign-in (email/password OR Google), if `extensionToken.isEmpty`, POST to `/auth/extension-tokens` with `{ device_id: deviceID, device_name: Host.current().localizedName, scopes: ["rewrite"] }`.
- Persist returned token to Keychain.
- Existing JWT pair stays — used only if extension token gets revoked or for non-rewrite endpoints (`/auth/me`, etc.).

### Stage 4 — Rewrite path uses extension token first (0.25 day)

- `BackendClient.rewrite` Authorization header: prefer `extensionToken` if present, fall back to `accessToken`.
- 401 handling:
  - If failed with `extensionToken` → try JWT refresh + retry once
  - If failed with `accessToken` → existing refresh path
  - Both fail → `sessionExpired = true`, but DO NOT wipe `extensionToken` unless backend returned a clear "token revoked" signal (need a distinct error code from backend, e.g. `EXT_TOKEN_REVOKED`).

### Stage 5 — Settings UI (0.25 day)

- New "Devices" section in Settings showing this device's extension token (last_used_at, created_at, name).
- "Revoke" button per device (calls `DELETE /auth/extension-tokens/:id`).
- Show all the user's devices, not just this one — gives Tan visibility into iOS/Android too.

### Stage 6 — Migration + ship (0.25 day)

- App on launch: if user is logged in (has refresh_token) but `extensionToken.isEmpty`, mint one silently using current access token. No re-login needed.
- Bump `CFBundleShortVersionString` to 2.1.9 (or 2.2.0 if breaking enough).
- Ship via existing `scripts/build-macos-dmg.sh` + `scripts/release-publish.sh macos`.

## Backend changes needed

- None for endpoints (already exist).
- Maybe add `device_type` enum (`ios_keyboard`, `ios_share`, `android_keyboard`, `macos`, `windows`, `linux`) for future analytics — non-breaking column addition.
- Document `EXT_TOKEN_REVOKED` error code so client can distinguish revoke from expiry.

## Failure modes after this lands

- User on a fresh macOS install with no internet → can't mint extension token → falls back to JWT pair (existing behavior). Once online, mints on next launch.
- User explicitly revokes from Settings → `sessionExpired = true` → re-login mints fresh.
- Backend `JWT_REFRESH_SECRET` rotates → only affects that one launch's silent refresh; rewrite calls keep working with extension token. User never sees a sign-in prompt.
- 90-day idle device → token expires server-side → on next call, backend returns 401 + `EXT_TOKEN_REVOKED` → re-login → fresh mint.

## Anti-goals

- Don't store password or any plaintext secret on disk (Keychain only).
- Don't auto-mint without an authenticated session — must be tied to a real login event.
- Don't extend extension tokens to non-rewrite endpoints (`/auth/me`, `/admin`, `/payment` stay JWT-only — keeps blast radius bounded if a token leaks).

## Decision points needed before kickoff

1. Add `device_type` column now or later? (Recommend now — non-breaking, painful to retrofit.)
2. Track this as a separate feature branch (`feature/macos-extension-tokens-20260509`) or fold into next macOS release?
3. Windows + Linux native apps should adopt the same model — ship together or separate?

## Related memories

- `feedback_macos_token_expiry.md` — original symptom report (2026-04-?? UTC)
- `feedback_exttoken_init_mint.md` — gotcha for AuthService.init mint flow on iOS/Android, applies here too
- `docs/superpowers/plans/2026-05-02-extension-tokens.md` — original extension-token plan for keyboards
