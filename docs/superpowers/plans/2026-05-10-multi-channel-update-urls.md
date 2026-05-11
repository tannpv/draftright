# Multi-Channel Update URLs (Store + Direct)

**Date:** 2026-05-10
**Status:** Proposed — not started
**Estimated effort:** ~4 hours
**Priority:** High (unblocks pre-launch testing flow + makes post-approval transition zero-deploy)

## Problem

`app_releases` has one row per platform with a single `download_url`. Auto-updater + website download cards consume that one URL. Reality is bifurcated:

- **Pre-store-approval** (today, May 10): Apple in review, Play in 14-day closed testing, MS Store in cert. Store URLs lead to "not available in your region" / 404 / TestFlight invite. Only direct downloads work for end users.
- **Post-approval**: Stores are the ideal install path (auto-update via system, no Gatekeeper warnings, no manual sideload). Direct downloads stay alive for tester / admin / banned-region users.

Switching the URL today means manually rewriting `download_url` in DB, deploying nothing else — works but is brittle. After approval, the same operator needs to remember to flip it back. Lossy: only one URL exists at a time.

## Goal

Store both URLs simultaneously per platform. One admin toggle decides which one auto-updaters and the website surface. Flip it the moment the App/Play/MS Store approves us — zero rebuild, zero redeploy.

## Schema

```sql
-- New shape: PK is (platform, channel)
CREATE TABLE app_releases_v2 (
  platform     TEXT NOT NULL,                 -- 'ios' | 'android' | 'mac' | 'windows' | 'linux'
  channel      TEXT NOT NULL,                 -- 'store' | 'direct'
  version      TEXT NOT NULL,
  url          TEXT NOT NULL,
  notes        TEXT NOT NULL DEFAULT '',
  required     BOOLEAN NOT NULL DEFAULT FALSE,
  enabled      BOOLEAN NOT NULL DEFAULT TRUE, -- false = hide this channel from clients + website
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (platform, channel)
);

CREATE TABLE app_release_policy (
  platform     TEXT PRIMARY KEY,
  preferred    TEXT NOT NULL DEFAULT 'direct',  -- which channel /updates/latest returns by default
  store_status TEXT NOT NULL DEFAULT 'not_submitted',
                                                -- 'not_submitted' | 'in_review' | 'approved' | 'rejected' | 'n/a'
  notes        TEXT NOT NULL DEFAULT '',
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Migration:
1. Backfill existing `app_releases` rows as `(platform, 'direct', ...)` in `app_releases_v2`.
2. Insert `app_release_policy` rows for each known platform with `preferred='direct'`, `store_status` matching real state today (ios=in_review, android=in_review, windows=in_review, mac=n/a, linux=n/a).
3. Drop old `app_releases` after backend migrated.

## Backend changes

### `/updates/latest` (existing endpoint)

Per platform, read `app_release_policy.preferred`, return that channel's row from `app_releases_v2`. Same JSON shape as today — clients don't need to change.

Optional client override:
- `GET /updates/latest?channel=store` — force the store URL even if direct is preferred. Useful for testing the store flow before flipping the toggle.
- `GET /updates/latest?channel=direct` — force direct (testers always get direct).

If a row is `enabled=false`, skip that channel. If only one channel exists for a platform, use whatever's there regardless of `preferred`.

### `/admin/releases` (new module)

| Endpoint | Body | Purpose |
|---|---|---|
| `GET    /admin/releases` | — | List every (platform, channel) row + policy |
| `PUT    /admin/releases/:platform/:channel` | `{version, url, notes, required, enabled}` | Upsert one channel |
| `DELETE /admin/releases/:platform/:channel` | — | Remove a channel |
| `PUT    /admin/release-policies/:platform` | `{preferred, store_status, notes}` | Update toggle + status |

Guarded by `JwtAuthGuard + RolesGuard('admin')`. All writes return the new full state of that platform so the admin UI can update without a refetch.

## Admin portal page

`/admin/versions` (replaces or extends current page):

```
┌─ iOS ────────────────────────────────────────────────────────────────────┐
│ Store status: [in_review v]   Preferred: ( ) store  (•) direct            │
│ Notes: "TestFlight only until Apple approves the in-app purchase config." │
│                                                                           │
│  STORE                            DIRECT                                  │
│  Version: 2.1.0                   Version: —                              │
│  URL:     apps.apple.com/...      URL:     —                              │
│  Notes:                           Notes:                                  │
│  Required: [ ]                    Required: [ ]                           │
│  Enabled:  [x]                    Enabled:  [ ]                           │
│  [Edit] [Disable]                 [+ Add direct channel]                  │
└──────────────────────────────────────────────────────────────────────────┘
```

Per platform card:
- Header: store status badge (color-coded — green=approved, amber=in_review, red=rejected, grey=n/a)
- "Preferred" radio — which channel `/updates/latest` returns by default
- Two side-by-side panes (Store / Direct), each editable inline
- "Add channel" button if a channel doesn't exist yet
- Inline release notes (markdown) per channel

Cards use the existing Modernize dark-theme components from `admin/CLAUDE.md`.

## Client behavior

No client change required for v1 — they keep calling `/updates/latest` and get the preferred URL. v2 (later) can detect install source and pass `?channel=` automatically:

| Platform | Detection | Default channel |
|---|---|---|
| macOS | always direct | direct |
| iOS | always store | store (App Store is the only path) |
| Android | `packageManager.getInstallSourceInfo()` returns `com.android.vending` → store, else direct | per detect |
| Windows | `Windows.ApplicationModel.Package.Current.SignatureKind == Store` → store, else direct | per detect |
| Linux | always direct | direct |

This auto-detection makes the experience seamless — Play Store users get Play updates, sideload users get sideload updates, no manual config.

## Pre-launch flow (today's reality)

Day 0 — submit:
- Set every platform `preferred = 'direct'`, `store_status = 'in_review'` (or `not_submitted`).
- All testers/customers download from `draftright.info/downloads/`.

Day N — Play approves:
- Admin: open Versions page → Android card → status `approved`, preferred `store`. Save.
- Effective immediately: every Android app's "Check for Updates" returns the Play Store URL. Auto-update via Play.
- Direct channel stays available for sideload testers (still `enabled=true`).

Same flow for iOS, MS Store. Zero deploy, zero client rebuild.

## Stages

| # | Work | Time |
|---|---|---|
| 1 | TypeORM entities + migration (drop old `app_releases`, create v2 + policy) | 30 min |
| 2 | Backend `/updates/latest` reads from new tables (back-compat JSON shape) | 30 min |
| 3 | Backend `/admin/releases` + `/admin/release-policies` CRUD endpoints | 1 hr |
| 4 | Admin portal Versions page (React + Tailwind, two-column cards) | 1.5 hr |
| 5 | `release-publish.sh` updates `(platform, 'direct', ...)` row, leaves store row untouched | 30 min |
| 6 | Manual smoke: edit one platform via admin UI, hit `/updates/latest`, verify URL flips | 15 min |
| 7 | Optional v2: client install-source detection + `?channel=` query | 1 hr |

**Total v1: ~4 hours. v2 (client detect): +1 hr.**

## Risks / open questions

- **Store URL format** — Apple uses `apps.apple.com/...` with country-specific TLD; Play uses `play.google.com/store/apps/details?id=...&hl=...`; MS Store uses `apps.microsoft.com/store/detail/...`. Need to standardize what "the store URL" means. Recommend: locale-neutral canonical form, let store handle redirect.
- **Auto-update via store** — when preferred flips to store, the in-app "Check for Updates" button should *deep-link to the store listing* rather than try to download a binary. Existing macOS/Windows updaters expect to download + mount; need a code path that just opens the URL when channel='store'. Mark as v2 follow-up.
- **Required-update semantics** — if `required=true` on the store channel, what happens? Force-quit + open store listing? Block app usage until user updates? Defer this — `required=false` covers all cases for now.
- **iOS doesn't have a meaningful "direct" channel** — even TestFlight is a store-mediated install. UI should disable the direct pane for iOS (or label it "TestFlight build URL" if useful for debugging).

## Anti-goals

- Not building a full release-management dashboard (rollback history, A/B rollouts, percentage gates). Single current row per channel is enough.
- Not touching the build/publish scripts beyond the small change in stage 5.
- Not adding more channels than `store` + `direct` for now. If beta/canary becomes a real need, the schema supports it (`channel='beta'`) without migration.

## Related

- `reference_release_automation_parity.md` — current per-platform release flow that this builds on
- `feedback_macos_token_expiry.md` — analogous "silent state mismatch" problem; lessons about always making state observable
