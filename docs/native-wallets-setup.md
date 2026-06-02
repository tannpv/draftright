# Native Wallets (Apple Pay + Google Pay) — Setup

Code is shipped; the rest is dashboard config. Both manual one-time
steps, no rebuild required after.

## 1. Apple Developer portal — register Merchant ID

1. https://developer.apple.com/account → **Identifiers** → **+**
2. Pick **Merchant IDs** → **Continue**
3. Identifier: `merchant.com.draftright.app.v2`
   Description: `DraftRight Apple Pay`
4. **Continue → Register**

The entitlement file already lists this ID; rebuild + sign on next
push.

## 2. Stripe dashboard — enable Apple Pay + Google Pay

1. https://dashboard.stripe.com → **Settings** → **Payment methods**
2. **Apple Pay** → **Configure** → **Add new domain** is for the web
   flow only.  For iOS app:
   1. Click **Manage Apple Pay** at the bottom
   2. **Add new Apple Pay Merchant** → paste
      `merchant.com.draftright.app.v2`
   3. **Continue** → Stripe shows a verification CSR you upload
      back to Apple Dev portal (Identifiers → your merchant ID
      → Apple Pay Payment Processing Certificate → +)
   4. Apple returns a verification file; upload back to Stripe → wait
      ~30 seconds for green check
3. **Google Pay** → toggle ON in the same Payment methods panel
   (no per-merchant config needed; Stripe handles)

## 3. Backend env vars

Add to dev + prod:

```bash
STRIPE_PUBLISHABLE_KEY=pk_test_…   # from Stripe dashboard → Developers → API keys
APPLE_PAY_MERCHANT_ID=merchant.com.draftright.app.v2
```

Dev:
```bash
ssh draftright
cd /home/deploy/deploys/draftright-dev
echo 'STRIPE_PUBLISHABLE_KEY=pk_test_…' >> .env.dev
echo 'APPLE_PAY_MERCHANT_ID=merchant.com.draftright.app.v2' >> .env.dev
docker compose -f docker-compose.yml -f docker-compose.dev.yml --env-file .env.dev up -d --no-deps --force-recreate backend-dev
docker network connect draftright_default dr-backend-dev 2>/dev/null
docker restart dr-backend-dev
```

Prod:  add the same two vars to `.env`, rebuild + restart
`draftright-backend-1`.

## 4. Stripe Price IDs

Pro plan rows must have a Stripe Price ID for the Apple Pay /
Google Pay flow.  The catalog already maps `plans.stripe_price_id`
on monthly + yearly Pro USD plans.  Verify:

```sql
SELECT id, name, billing_period, currency, stripe_price_id
FROM plans
WHERE is_active=true AND currency='USD' AND billing_period IN ('monthly','yearly');
```

Both rows should have `price_…` IDs.  If not, run
`backend/scripts/sync-stripe-prices.ts`.

## 5. Enable methods (admin Settings → Payment)

Either via DB or via admin portal Settings → Payment → Enabled
methods CSV:

```
lemonsqueezy,apple_pay,google_pay,vietqr
```

Or env override `PAYMENT_ENABLED_METHODS` on backend container.

## 6. iOS App Store policy

This branch is for **TestFlight + sideload** distribution.  Apple
App Store Guideline 3.1.1 forbids non-IAP payments (including Apple
Pay via Stripe) for digital subscriptions in production App Store
builds — those need StoreKit IAP instead.  See separate StoreKit
IAP plan when the App Store ship-date approaches.

## 7. Test on device

Apple Pay sandbox cards: https://developer.apple.com/apple-pay/sandbox-testing/
Google Pay test cards: https://developers.google.com/pay/api/android/guides/resources/test-card-suite

In-app flow:

1. Sign in
2. Subscription tab
3. Toggle Monthly/Yearly
4. Tap **Apple Pay** (iOS) or **Google Pay** (Android)
5. Native sheet appears; FaceID / fingerprint
6. Backend webhook activates Pro
7. Subscription screen refreshes → Pro
