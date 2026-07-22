# Handover — PayPal Subscriptions (native), 2026-07-21

Paste this into a fresh Claude Code session in the DraftRight repo to continue.

## Goal
Add **PayPal as a native recurring-subscription** payment method (Model A) to the
backend, alongside the existing LemonSqueezy/VietQR. Motive: LS already offers
PayPal as merchant-of-record, but takes a MoR cut; native PayPal pays out to the
VN PayPal balance at PayPal's lower rate. Trade-off accepted: we handle tax.

## Hard constraints (verified 2026-07-21)
- **Stripe is NOT available to a VN-registered business** → Stripe + the native
  Stripe-based Apple/Google Pay path are dead for us (gated off, harmless).
- **PayPal supports USD only, not VND.** Native PayPal charges the **USD** plans
  ($4.99/mo, $39.99/yr) only. VN-domestic buyers keep **VietQR**.
- PayPal has **no hosted customer portal** → cancel is via API; no portal URL.

## Status: BACKEND CODE COMPLETE + TESTS GREEN. Not committed. Not deployed.
Branch: `feature/paypal-subscriptions-20260721` (off develop).

Dev checklist state:
- [x] 1. Test cases FIRST — `docs/test-cases.xlsx` sheet `PAYPAL` (PAYPAL-001..020)
- [x] 2. Feature branch
- [x] 3. Implement
- [x] 4. `npx tsc --noEmit` → exit 0
- [x] 5. Unit tests — 33 green (9 strategy + 24 service); full payment suite 47/47
- [ ] 6. Commit + merge develop (NOT done — awaiting go)
- [ ] 7–16. Deploy testing → sandbox E2E → prod (GATED on PayPal credentials)

## Architecture (matches existing strategy plugin)
- `payment/strategies/paypal.strategy.ts` — `PayPalStrategy extends BasePaymentStrategy`:
  - OAuth2 client-credentials token (cached in-memory to ~60s before expiry).
  - `baseUrl` from `paypal_mode` (sandbox/live) — named class constants.
  - `createCheckout` → POST `/v1/billing/subscriptions` `{plan_id (monthly|yearly),
    custom_id: reference_code, application_context{return/cancel_url, SUBSCRIBE_NOW}}`
    → returns `links[rel=approve].href` as `redirect_url`.
  - `verifyWebhook` → POST `/v1/notifications/verify-webhook-signature` (PayPal
    does NOT HMAC-sign) → on SUCCESS dispatch on `event_type`.
  - `cancelSubscription` → POST `/v1/billing/subscriptions/{id}/cancel`.
- Webhook events → `WebhookAction`:
  - `BILLING.SUBSCRIPTION.ACTIVATED` → `paypal_payment_success` (has custom_id=ref,
    resource.id=subId, next_billing_time) → first-cycle: completePayment + stampStoreRef.
  - `PAYMENT.SALE.COMPLETED` → renewal: no ref → fetch sub's next_billing_time →
    `extendByStoreRef(StoreType.PAYPAL, subId, expiry)`. First-cycle SALE after
    ACTIVATED is an idempotent no-op (no pending payment).
  - `CANCELLED`/`EXPIRED`|`SUSPENDED`/`PAYMENT.FAILED` → cancel/expire/emailed.
- Reuses generic `SubscriptionsService.{stampStoreRef,extendByStoreRef,
  findByStoreRef,cancelByStoreRef,expireByStoreRef}` with `StoreType.PAYPAL`.

## Files
New: `backend/src/payment/strategies/paypal.strategy.ts` (+ `.spec.ts`),
`backend/sql/2026-07-21-paypal-subscription-settings.sql`,
`backend/scripts/paypal-create-plans.ts`,
`backend/docs/plans/2026-07-21-paypal-subscriptions.md`.
Modified: `payment.controller.ts` (`POST /payment/webhook/paypal`),
`payment.module.ts` (+PayPalStrategy provider), `payment.service.ts`
(inject + strategy map + cancel switch + `paypal_*` webhook cases),
`strategies/payment-strategy.interface.ts` (`paypal_*` WebhookAction variants),
`admin/entities/app-settings.entity.ts` (paypal_webhook_id, paypal_plan_monthly,
paypal_plan_yearly — note client_id/secret/mode already existed),
`config/env.schema.ts` (PAYPAL_* keys), `common/app-config.ts` (`appName()`).

## Clean-code rule honored
No hardcoding: brand via `appName()` (env `APP_NAME`), PayPal hosts as constants,
plan prices in the create-plans script pulled from `/plans` (single source).

## REMAINING — needs the human (external, I can't do)
1. Create a **PayPal Business account** (VN OK to *receive* international).
2. PayPal Developer → REST app → **sandbox** Client ID + Secret (later: live).
3. Set in Admin → Settings → Payment: `paypal_client_id`, `paypal_client_secret`,
   `paypal_mode=sandbox`. (Or env `PAYPAL_CLIENT_ID/SECRET/MODE` for first deploy.)
4. Create billing plans:
   ```
   PAYPAL_CLIENT_ID=... PAYPAL_CLIENT_SECRET=... PAYPAL_MODE=sandbox \
   BACKEND_URL=http://localhost:3000 npx ts-node scripts/paypal-create-plans.ts
   ```
   Paste printed `paypal_plan_monthly` / `paypal_plan_yearly` into Admin Settings.
5. Register PayPal webhook → `https://api.draftright.info/payment/webhook/paypal`
   (or the testing host) subscribed to: BILLING.SUBSCRIPTION.ACTIVATED,
   PAYMENT.SALE.COMPLETED, BILLING.SUBSCRIPTION.CANCELLED,
   BILLING.SUBSCRIPTION.EXPIRED, BILLING.SUBSCRIPTION.SUSPENDED,
   BILLING.SUBSCRIPTION.PAYMENT.FAILED → copy Webhook ID → set `paypal_webhook_id`.
6. Run the migration on the target DB:
   `psql "$DATABASE_URL" -f backend/sql/2026-07-21-paypal-subscription-settings.sql`
7. Enable the method: add `paypal` to `payment_methods_enabled` (Admin Settings).
8. **Sandbox E2E** (PAYPAL-020): checkout → approve with a sandbox buyer →
   ACTIVATED webhook activates Pro → renewal extends → in-app cancel keeps access
   to period end.
9. Deploy to **testing** first, verify, then merge develop→main + prod.
10. **Frontend follow-on** (not started): add PayPal to macOS `MethodPickerView` /
    `SubscriptionView` + web checkout. Show only for USD plans.

## Verify commands
```
cd backend
npx tsc --noEmit
npx jest src/payment        # 47/47 green expected
```

## Gotchas
- PayPal webhook verification needs the transmission headers
  (`paypal-transmission-id/-time/-sig`, `paypal-cert-url`, `paypal-auth-algo`);
  controller passes rawBody, strategy parses + sends parsed event to verify API.
  If sandbox verification is flaky on re-serialization, that's the place to look.
- `scripts/` is likely excluded from the project tsconfig (ops script) — run via ts-node.
- Do NOT enable `paypal` in prod before sandbox E2E passes.
