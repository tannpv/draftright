# Universal Link / App Link deploy notes

**Shipped 2026-05-31** as `feat/universal-link-payment-return-20260531`.
Brings the user back into the app after LS hosted-checkout —
no manual app-switching required.

## What ships in this PR

| File | Role |
|---|---|
| `website/public/.well-known/apple-app-site-association` | iOS AASA — declares which paths open the app |
| `website/public/.well-known/assetlinks.json` | Android App Link verification |
| `website/src/pages/payment/success.astro` | Fallback page for web/desktop visitors |
| `website/src/pages/payment/cancel.astro` | Same for cancel flow |
| `DraftRightMobile/ios/Runner/Runner.entitlements` | `applinks:draftright.info` added |
| `DraftRightMobile/android/app/src/main/AndroidManifest.xml` | `autoVerify=true` intent-filter on `https://draftright.info/payment/*` |
| `DraftRightMobile/lib/services/deep_link_service.dart` | Stream-based listener; classifies URIs into typed events |
| `DraftRightMobile/lib/main.dart` (HomeScreen) | Wires cold-start + warm-state deep-links to SubscriptionScreen |

## After deploying the website

1. `cd website && npm run build && rsync -avz dist/ root@<vps>:/var/www/draftright.info/`
2. Verify AASA serves over HTTPS with the right Content-Type:
   ```
   curl -sI https://draftright.info/.well-known/apple-app-site-association
   # Expect:
   #   HTTP/2 200
   #   content-type: application/json
   ```
   If Content-Type is wrong (e.g. `text/plain` because the file has no
   extension), add to the site's Caddy block:
   ```
   handle_path /.well-known/apple-app-site-association {
       header Content-Type "application/json"
       root * /var/www/draftright.info
   }
   ```
3. Verify Android assetlinks:
   ```
   curl -s https://draftright.info/.well-known/assetlinks.json | jq .
   ```
   Use Google's verifier:
   <https://developers.google.com/digital-asset-links/tools/generator>

## After installing the new app build

### iOS verification
1. Install the new IPA on a device.
2. From a different app (e.g. Messages), tap a link `https://draftright.info/payment/success?ref=TEST`.
3. App should open at SubscriptionScreen (not Safari).
4. If iOS opens Safari instead, iOS hasn't fetched AASA yet — kill the app, wait ~30s, retry.

### Android verification
1. Install the new APK/AAB.
2. ```
   adb shell pm get-app-links com.draftright.draftright_mobile.v2
   ```
   Look for `Status: verified` next to draftright.info.  If it says
   `none`, Android hasn't run verification yet:
   ```
   adb shell pm verify-app-links --re-verify com.draftright.draftright_mobile.v2
   ```

## Play Store SHA-256 (PENDING)

**Important:** `assetlinks.json` currently contains only the
**upload-keystore** SHA-256 (`C9:28:F5:…:15:5A`).

For Play Store distribution, Google re-signs the APK with their own
key (Play App Signing).  Once we ship a release build to Play, we
need to:

1. Play Console → DraftRight → Setup → **App signing**
2. Copy the **App signing key certificate** SHA-256
3. Add it as a second fingerprint in
   `website/public/.well-known/assetlinks.json`
4. Redeploy the website

Without that addition, App Link verification fails on Play-installed
builds — Android falls back to the chooser dialog.

See also [[reference_android_signing]] in memory for the upload-keystore
location, password, and Play upload-key reset status.

## Backend already does the right thing

`backend/src/payment/strategies/lemonsqueezy.strategy.ts:104` already
constructs:
```
${websiteUrl()}/payment/success?ref=${payment.reference_code}
```
as the LS `redirect_url`.  Mobile doesn't need to pass `success_url`
in the `/payment/checkout` call — the default already lands at the
universal-link path.  The mobile DTO field `success_url` is still
available for client overrides (e.g. desktop apps that want a
different URL).

## What "didn't" ship

- Stripe / VietQR strategies don't construct payment-return URLs yet.
  Today's poller + AppLifecycleState.resumed handle the return for
  those — universal-link sugar is LS-only for now.
- iOS Universal Link verification requires the user to open the link
  via a non-Safari app at least once.  Putting a "Tap to open" link
  inside the in-app browser would let users self-bootstrap.
