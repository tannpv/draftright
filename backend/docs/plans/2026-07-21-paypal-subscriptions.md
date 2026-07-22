# Plan — PayPal Subscriptions payment strategy (Model A, recurring)

Date: 2026-07-21 · Branch: `feature/paypal-subscriptions-20260721`

## Goal
Add PayPal as a recurring-subscription payment method, following the existing
strategy plugin pattern (mirrors `LemonSqueezyStrategy`). International card +
PayPal-balance path; USD plans only (PayPal has no VND). VietQR stays domestic VN.

## Constraints / facts
- PayPal REST. OAuth2 client-credentials for every API call (cache token to expiry).
- Base URL sandbox `https://api-m.sandbox.paypal.com` / live `https://api-m.paypal.com`.
- Webhook verified via PayPal API `POST /v1/notifications/verify-webhook-signature`
  (NOT HMAC) using the configured Webhook ID + transmission headers.
- USD plans only ($4.99/mo, $39.99/yr). VND plans not chargeable via PayPal.
- No hosted customer portal → cancel via API; `getCustomerPortalUrl` returns null.

## PREREQUISITES — user must do (I cannot); code works against mocks without them
1. PayPal **Business account** (verified; VN OK for receiving intl).
2. PayPal Developer **REST app** → sandbox + live **Client ID + Secret**.
3. **Product + 2 Billing Plans** (Pro Monthly $4.99, Pro Yearly $39.99, 30-day
   trial cycle) → two `paypal_plan_id` (`P-XXXX`). Helper script in Task 9 creates these.
4. **Webhook** in PayPal dashboard → `https://api.draftright.info/payment/webhook/paypal`
   subscribed to: BILLING.SUBSCRIPTION.ACTIVATED, PAYMENT.SALE.COMPLETED,
   BILLING.SUBSCRIPTION.CANCELLED, BILLING.SUBSCRIPTION.EXPIRED,
   BILLING.SUBSCRIPTION.PAYMENT.FAILED → copy the **Webhook ID**.

## Code tasks (TDD — unit tests mock PayPal HTTP; buildable now)
1. **env.schema.ts** — add optional `PAYPAL_CLIENT_ID`, `PAYPAL_SECRET`,
   `PAYPAL_WEBHOOK_ID`, `PAYPAL_PLAN_MONTHLY`, `PAYPAL_PLAN_YEARLY`,
   `PAYPAL_ENV` (`sandbox`|`live`, default `sandbox`).
2. **AppSettings entity + migration** — add `paypal_client_id`, `paypal_secret`,
   `paypal_webhook_id`, `paypal_plan_monthly`, `paypal_plan_yearly`, `paypal_env`
   (admin-editable, rotate without redeploy — same as LS fields).
3. **payment-strategy.interface.ts** — add `WebhookAction` variants:
   `paypal_payment_success` (reference_code, paypal_subscription_id, current_period_end),
   `paypal_payment_failed` (paypal_subscription_id),
   `paypal_subscription_canceled` (paypal_subscription_id),
   `paypal_subscription_expired` (paypal_subscription_id).
4. **strategies/paypal.strategy.ts** (new) extends BasePaymentStrategy:
   - `getCredentials()` — settings + env fallback (resolveCredential).
   - `baseUrl()` — sandbox/live from `paypal_env`.
   - `getAccessToken()` — OAuth2 client_credentials, in-memory cache until expiry.
   - `createCheckout(payment, opts)` — POST `/v1/billing/subscriptions`
     `{plan_id: monthly|yearly per plan.billing_period, custom_id: reference_code,
     subscriber.email_address, application_context{brand_name, return_url,
     cancel_url, user_action: SUBSCRIBE_NOW}}` → return `links[rel=approve].href`
     as `redirect_url`. Throw if plan id / creds missing (LS-style messages).
   - `verifyWebhook(payload, headers)` — call verify-webhook-signature; on
     `SUCCESS` dispatch on `event_type`; map `resource.custom_id`→reference_code,
     `resource.id`/`resource.billing_agreement_id`→paypal_subscription_id,
     `resource.billing_info.next_billing_time`→current_period_end. Invalid → 400.
   - `cancelSubscription(id)` — POST `/v1/billing/subscriptions/{id}/cancel`.
   - `getCustomerPortalUrl()` → null (no PayPal portal).
5. **Register** — payment.module.ts providers + payment.service `strategies` map
   (`PaymentMethod.PAYPAL` → PayPalStrategy).
6. **payment.service.handleWebhook** — add `paypal_*` cases mirroring
   `lemonsqueezy_*` (extend/cancel/expire ByStoreRef with `StoreType.PAYPAL`;
   first-activation-or-renewal via reference_code like the LS success handler).
7. **payment.service** portal/cancel switches — add `PAYPAL` arm →
   `cancelSubscription`.
8. **payment.controller.ts** — add `@Post('webhook/paypal')` → `handleWebhook('paypal', body, headers)` (raw body available for logging; verify uses parsed body + headers).
9. **scripts/paypal-create-plans.ts** — one-shot: create Product + 2 Billing Plans
   via API, print the two `paypal_plan_id`. Run once creds exist.
10. **Frontend (follow-on)** — add PayPal to macOS `MethodPickerView`/`SubscriptionView`
    + web checkout. Backend-first; ship after sandbox e2e passes.

## Tests
- `paypal.strategy.spec.ts` — mock fetch: token fetch; createCheckout returns
  approve URL + picks monthly/yearly; verifyWebhook SUCCESS→each action; verify
  FAIL→400; cancel. No live network.
- `payment.service.spec.ts` additions — paypal_* actions call the right
  ByStoreRef helper with `StoreType.PAYPAL`.

## Verify
- `npx tsc --noEmit` green · `npm test` green.
- Enable `paypal` in `payment_methods_enabled` ONLY after creds + plan ids set.
- Sandbox e2e (gated on creds): checkout → approve → ACTIVATED webhook activates
  sub → renewal extend → cancel.
- Deploy to **testing** first, smoke, then prod. Never enable live before sandbox passes.

## Not in scope
- VND via PayPal (unsupported). Refunds/disputes UI. Existing LS/VietQR untouched.
