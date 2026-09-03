# Play Production Launch Plan — DraftRight V2 Android

**Status (2026-09-04):** blocked by Google policy, countdown not yet started.
**Owner:** Tan. **App:** `com.draftright.draftright_mobile.v2`, Play Console
account "Southern Martin" (7501576792460573393), app id 4975566743623670491.

## The gate (why there is no production release yet)

DraftRight V2 has **never been on Play production** (track: Inactive). Google's
personal-developer-account policy requires, before "Apply for production" even
unlocks:

1. ✅ A published closed-testing release
2. ❌ **≥ 12 testers opted in** to the closed test (currently 0 of the 16
   recruited — the list "SouthernMartin" is attached to the Alpha track but
   nobody used the opt-in link)
3. ❌ Closed test running with those testers for **≥ 14 consecutive days**

Every `track_promote_to: production` API call fails
`Precondition check failed` because of this gate — no fastlane lane, flag, or
console click bypasses it. Do not burn time re-diagnosing.

## Current state

| Piece | State |
|---|---|
| Build 79 (2.4.16, VI auto-correct #207) | Play **internal** (live) + TestFlight (live) |
| Closed testing – Alpha | Build 79 **submitted for Google review** 2026-09-03 (managed publishing; typical ≤ 7 days; auto-publishes to testers on approval) |
| Promote lane | `promote_internal_production_draft` in `DraftRightMobile/android/fastlane/Fastfile`, dispatchable via `play-deploy.yml` — ready for after approval |
| develop / main | In sync, all #207 + CI work merged |

## Countdown steps

1. **Now (Tan, ~30 min):** send the 16 testers the opt-in link + ask them to
   install from Play after opting in. Need **12+** to stick.
   - Web opt-in: <https://play.google.com/apps/testing/com.draftright.draftright_mobile.v2>
   - Store page (after opt-in): <https://play.google.com/store/apps/details?id=com.draftright.draftright_mobile.v2>
   - Tester Gmail list + shared password: maintainer memory
     `reference_android_closed_testers`.
2. **When Alpha review passes:** confirm testers see 2.4.16 and the opt-in
   count on the Dashboard ("Apply for access to production" card) reaches 12+.
   **The 14-day clock runs from when the threshold holds.**
3. **Day 14+:** Dashboard → "Apply for production" → answer Google's
   closed-test questionnaire → wait for approval.
4. **On approval:**
   `gh workflow run play-deploy.yml -R tannpv/draftrightmobile --ref main -f track=promote_internal_production_draft`
   then Play Console → Production → Releases → start rollout (staged % chosen
   there; promote+staged-rollout in one API call is what trips the
   precondition error).
5. Label #207 `status: deployed to production`, post health-check comment,
   leave the issue open per workflow.

## Landmines (learned 2026-09-03, don't rediscover)

- A version code already on ANY track cannot be re-uploaded — **promote**, never
  re-upload, when moving a build between tracks.
- Managed publishing is ON: every console release needs Publishing overview →
  "Send for review" or it silently sits as a draft change.
- iOS App Store production is a **separate project**: no live iOS version
  exists; blocked on the StoreKit IAP work (PR #213 handoff) + Apple agreement
  state. TestFlight is iOS "production" until that lands.
