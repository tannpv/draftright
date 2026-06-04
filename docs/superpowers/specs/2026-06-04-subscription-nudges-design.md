# Subscription Lifecycle Nudges + Expiry → Free

**Date:** 2026-06-04
**Status:** Approved design — ready for implementation plan
**Author:** Tan Nguyen (+ Claude)

## 1. Problem & Goal

Goal is a habit-forming free tier: when a Pro subscription lapses, the
customer keeps using DraftRight on the free plan (10 rewrites/day) and
sees a subtle, ever-present nudge back to Pro — never a hard lockout.

Two gaps block this today:

1. **Expired users are locked out, not demoted.** Every user starts with
   a Free subscription (ACTIVE). On upgrade, `activateSubscription`
   cancels the prior row and creates Pro ACTIVE. On expiry, the cron
   flips Pro → EXPIRED. `/rewrite` calls `findActiveByUserId` (ACTIVE
   only), so an expired user has Free=CANCELLED + Pro=EXPIRED → **no
   active row → 403 "No active subscription"**. They cannot rewrite at
   all — the opposite of the habit loop.
2. **No in-app nudge** and **no upgrade/plan-change confirmation.**
   Renewal-reminder and expired emails already exist
   (`subscriptions.cron.ts`); there is no in-app surface and no
   confirmation when a user upgrades or switches plan.

## 2. Scope

In scope:
- Backend entitlement resolver so expired/cancelled/missing → Free tier.
- Subscription lifecycle notification events (in-app + email).
- An app-wide in-app nudge strip driven entirely by backend state.

Out of scope (YAGNI):
- No `notifications` table, no push (APNs/FCM) infra.
- No per-event dismissal preferences beyond dismiss-until-tomorrow.
- macOS/Windows/Linux UI wiring (model is designed to extend there
  later; this spec ships Flutter + the shared backend).
- Auto-renewal via Stripe Subscriptions (tracked separately).

## 3. Architecture

Built per the standing clean-code rule: enums over literals, one source
of truth, shared logic extracted, designed for the next platform.

### Part 1 — Entitlement resolver (backend)

The binary "active sub or 403" cannot express free-tier. Replace it with
a computed entitlement, used everywhere.

```ts
export enum EntitlementTier { PRO = 'pro', FREE = 'free' }

export interface Entitlement {
  tier: EntitlementTier;
  dailyLimit: number;        // -1 = unlimited (Pro), 10 = Free
  status: SubscriptionStatus | null; // raw row status, for display/telemetry
  expiresAt: Date | null;    // Pro only; null for Free
  planName: string;
}
```

- `SubscriptionsService.resolveEntitlement(userId): Promise<Entitlement>`
  is the single source of truth:
  - Active Pro row → `{ PRO, dailyLimit: plan.daily_limit (-1), status, expiresAt, planName }`.
  - Otherwise (EXPIRED / CANCELLED / no row) → resolve the canonical
    Free plan via the existing `plansService.findFreePlan()` →
    `{ FREE, dailyLimit: freePlan.daily_limit (10), status, expiresAt: null, planName: 'Free' }`.
- `/rewrite` quota guard and `/subscription` both call
  `resolveEntitlement` and delete their duplicated active-sub lookups.
  The `/rewrite` 403 "No active subscription" branch is removed —
  everyone resolves to at least FREE.
- The expiry cron is unchanged (still flips ACTIVE/CANCELLED past
  `expires_at` → EXPIRED). Demotion is now a read-time concern, so we do
  not recreate Free rows.

**Why read-time, not a new Free row on expiry:** one behaviour in one
place; no risk of duplicate ACTIVE rows; the historical Pro row stays
intact for analytics (`store_type`, dates).

### Part 2 — Notification events (backend)

```ts
export enum SubscriptionEvent {
  UPGRADED = 'upgraded',
  PLAN_CHANGED = 'plan_changed',
  EXPIRING_SOON = 'expiring_soon',
  EXPIRED = 'expired',
}
```

- A thin `SubscriptionNotifier` maps each event → an email send (reusing
  `EmailService`). Adding a channel later (e.g. push) = one method, no
  caller changes.
- Emitters:
  - `activateSubscription` → `UPGRADED` (new Pro) or `PLAN_CHANGED`
    (Pro→Pro of a different plan/period).
  - `subscriptions.cron` → `EXPIRING_SOON` (existing renewal reminder,
    reworded) and `EXPIRED` (existing expired email, reworded for
    clarity: "Your Pro expired — you're on Free (10/day). Restore Pro.").
- New email template for `UPGRADED`/`PLAN_CHANGED` confirmation.

### Part 3 — In-app nudge (backend payload + Flutter widget)

Backend owns the message state so copy changes ship without an app
release.

```ts
export enum NudgeBanner {
  NONE = 'none',                 // active Pro, not near expiry
  PRO_EXPIRING = 'pro_expiring', // active Pro, expires <= 3 days
  JUST_EXPIRED = 'just_expired', // expired within last 7 days
  FREE_COUNTER = 'free_counter', // ongoing free (incl. expired > 7 days)
}

export interface NudgeState {
  tier: EntitlementTier;
  usageToday: number;
  dailyLimit: number;     // -1 unlimited
  expiresAt: string | null;
  banner: NudgeBanner;
}
```

- `GET /subscription` returns `nudge: NudgeState`. Construction lives in
  exactly one place — `SubscriptionsService.buildNudgeState(userId)` —
  which calls `resolveEntitlement`, reads `usageToday` from
  `UsageService`, and runs a single pure `deriveBanner(entitlement,
  usageToday, lastExpiredAt)` function (the JUST_EXPIRED window uses the
  most recent EXPIRED row's `updated_at` within 7 days). The controller
  only forwards the result; no surface duplicates this logic, so
  macOS/Win/Linux reuse the same payload. `deriveBanner` is pure →
  table-tested in isolation.
- Flutter `SubscriptionNudgeStrip(nudge)` — one reusable widget in the
  shared app scaffold:
  - Renders a thin top strip with state-specific copy (table below).
  - `NONE` → renders nothing.
  - Dismiss hides it until the next local-midnight reset (stored in
    SharedPreferences as a date string), so it stays gentle but returns
    daily.
  - Tapping the CTA routes to the existing Subscription/upgrade screen.
- **Soft paywall at 0/10:** no new endpoint — the existing `/rewrite`
  429 "Daily limit reached" is the trigger; on that response clients
  show the fuller upgrade card. The strip just counts down to it.
- **Keyboard / share extensions:** no room for a strip — render
  `usageToday/dailyLimit` as a small chip in the toolbar, reading the
  same `NudgeState`. (Wiring optional in first cut; model supports it.)

#### Copy (server-supplied)

| Banner | Strip text | CTA |
|---|---|---|
| `PRO_EXPIRING` | `Pro ends in {n} days` | `Renew` |
| `JUST_EXPIRED` | `You're on Free now — 10/day` | `Restore Pro` |
| `FREE_COUNTER` | `{used} / {limit} left today` | `Go Pro` |
| `NONE` | (hidden) | — |

Reset cadence is **local calendar day** (matches existing
`usage.service` `setHours(0,0,0,0)`); "resets at midnight" copy aligns.

## 4. Data Flow

1. Client calls `GET /subscription` on app open / resume.
2. Backend `resolveEntitlement` + banner derivation → `NudgeState`.
3. `SubscriptionNudgeStrip` renders (or hides) accordingly.
4. Each `/rewrite` returns `usage_today` + `daily_limit`; client may
   update the strip counter live without re-fetching `/subscription`.
5. On 429, client shows the upgrade card (soft paywall).
6. Lifecycle emails fire from the cron / `activateSubscription` via
   `SubscriptionNotifier`.

## 5. Error Handling

- `resolveEntitlement` never throws for "no sub" — it falls back to
  Free. If `findFreePlan()` itself fails (seed missing), that is a real
  500 (mirrors today's behaviour) and is logged.
- `NudgeState` is best-effort: if banner derivation fails, default to
  `NONE` (never block the subscription screen on a nudge).
- Notifier sends are wrapped per-recipient in try/catch (as the cron
  already does) so one failure never aborts the batch.

## 6. Testing Strategy

- `resolveEntitlement`: unit tests — active Pro → PRO/unlimited; EXPIRED
  → FREE/10; CANCELLED → FREE/10; no row → FREE/10.
- `/rewrite`: an expired user now gets 200 within 10/day and 429 at the
  cap (no more 403).
- Banner derivation: table-driven unit test over the four states
  (incl. the 3-day and 7-day boundaries).
- `SubscriptionNotifier`: each event maps to the expected template;
  send failures are swallowed per-recipient.
- Flutter widget test: each `NudgeBanner` renders the right copy/CTA;
  `NONE` renders nothing; dismiss persists until next day.

## 7. Rollout

- Pure additive backend (new resolver + payload field + reworded
  emails). No DB migration — uses existing tables/columns.
- Behaviour change: expired users gain free-tier access (intended).
  Verify on testing with a manually-expired sub before prod.
- Ship behind no flag; the nudge strip is self-hiding for active Pro.

## 8. Open Questions

None outstanding — Pro is unlimited (strip hidden except expiring),
surfaces are in-app + email, reset is local midnight.
