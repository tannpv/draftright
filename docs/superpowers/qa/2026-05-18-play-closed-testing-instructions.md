# Google Play Closed Testing — Tester Onboarding

**Created:** 2026-05-18
**Current AAB:** 2.3.0+39 (Tier β multi-language keyboard)
**Track:** Closed testing
**Testers:** 12
**Clock:** 14 days from the most recent AAB deploy

This is the message Tan sends to each of the 12 closed-testing testers. Paste it into email / Slack / WhatsApp. Replace `<opt-in URL>` with the link from Play Console → Testing → Closed testing → Testers → "Copy link".

---

## Message to copy/paste to testers

**Subject:** You're invited to test DraftRight on Android — 5-minute setup

Hi,

Thanks for agreeing to test **DraftRight 2.3.0** before we ship to the public Play Store. Below is what to do.

### Step 1 — Use the Google account I added you under

I added your tester email to Google Play. Make sure that's the **same Google account signed in** on your Android phone (Settings → Accounts → Google).

If you signed up with one email but use a different one on your phone, tell me — I'll add the right one. It won't work otherwise.

### Step 2 — Tap the opt-in link, ON YOUR PHONE

`<opt-in URL>`

Open the link on your **Android phone** (not laptop). You'll see a page that says *"Become a tester"*. Tap the button. That's it — you're enrolled.

### Step 3 — Install DraftRight from the Play Store

Wait about 10–30 minutes after Step 2 (Play Store needs to refresh). Then:

1. Open the **Google Play Store** app.
2. Search **"DraftRight"** — or tap this direct link on your phone: `https://play.google.com/store/apps/details?id=com.draftright.draftright_mobile.v2`
3. Tap **Install**. The Store page will say *"You're a tester for this app"* somewhere near the top — that's how you know the opt-in worked.

### Step 4 — Use the app on at least **3 different days over the next 14 days**

This is Google's rule for us to graduate to production. The exact wording from Google: "12 testers must use the app on at least 3 different days within a 14-day window."

It doesn't have to be a long session — opening the app, signing in, and doing a single rewrite counts. But please use it for real if you can — that's how we catch bugs.

### Step 5 — What to test (priority order)

1. **Sign up / sign in** (Google, Facebook, or email/password).
2. **Open any app you type in** — WhatsApp, Messages, Gmail, Notes — type something.
3. **Long-press the text → tap "DraftRight"** in the share / process-text menu. The rewrite panel should open.
4. **Pick a tone** (Polished, Friendly, Concise, Expand). Wait ~3 seconds. The rewritten text should appear. Tap **Replace** or **Copy**.
5. **NEW in 2.3.0 — multi-language keyboard.** Settings → Keyboard languages. Enable a second language (e.g., Tiếng Việt). Open WhatsApp again. In the message field, tap the keyboard icon at the bottom-right (the globe 🌐) and switch to **DraftRight Keyboard**. Try typing in Vietnamese (or French, Spanish, German, Italian, Portuguese). Tap the globe key on the DraftRight keyboard to switch languages.
6. **Floating bubble** (optional): Settings → Floating Bubble → enable. A small DraftRight icon appears over other apps. Long-press selected text in any app, tap the bubble.

### Step 6 — Report bugs

Two ways:

- **Inside the app** — Settings → Help → "Report a bug". Attach a screenshot if possible. (Recommended — easiest for me to track.)
- **Email me** — reply to this thread with what went wrong.

What's useful in a bug report: which Android phone, what you were doing, what you expected, what actually happened. A screenshot is gold.

### Important notes

- **Don't share the opt-in URL publicly.** This is a closed test.
- The app will auto-update through the Play Store as we ship fixes — no need to reinstall.
- If something is broken, it's broken on purpose ;) you're testing pre-release software. Hence why you're getting it before everyone else.

Thanks! I'll send a follow-up around day 7 to check in.

— Tan

---

## Operational notes (for Tan, not the testers)

| Item | Value |
|---|---|
| Current closed-testing AAB version | 2.3.0+39 (Tier β multi-language keyboard) |
| Opt-in URL location | Play Console → Testing → Closed testing → Testers tab → "Copy link" |
| Direct app page (after opt-in) | https://play.google.com/store/apps/details?id=com.draftright.draftright_mobile.v2 |
| 14-day clock | Restarts whenever a new AAB is deployed to closed testing. 2.3.0+39 just shipped today → clock now starts today. Do NOT push another build during the 14-day window or the clock restarts. |
| Tracking dashboard | Play Console → Testing → Closed testing → Statistics tab. Watch "Daily active testers". Need: ≥12 unique testers × ≥3 distinct days each within 14 days. |
| Promotion gate | Day 14 (or whenever stats show "Requirements met"). Play Console → Production → "Apply for production access". |

## Suggested follow-up cadence with testers

| Day | What to send |
|---|---|
| Day 1 | Send the tester message above + opt-in URL. |
| Day 3 | Quick check: "Did everyone install OK? Reply with a screenshot of the app open on your phone if yes." |
| Day 7 | Reminder: "Halfway through. Please open the app today + try one rewrite if you haven't already." |
| Day 12 | Final push: "Last few days. If you found bugs, send them now — easier to fix before we ship to production." |
| Day 14 | Thank-yous + click "Apply for production access" in Play Console. |

If fewer than 12 testers cross the 3-day threshold by Day 13, ask one or two reliable testers to pull in a friend who has an Android device. Easier than restarting the clock with a new build.

## Common tester problems + answers

| Problem | Answer |
|---|---|
| "I clicked the opt-in link but the Play Store doesn't show the app" | Wait 30 min. Play Store cache refresh is slow. Then search "DraftRight". If still not visible after 1 h, check that the tester's Google account in Settings → Accounts matches the email Tan added in Play Console. |
| "I see a different app called DraftRight on the Store" | Make sure the package ID matches: `com.draftright.draftright_mobile.v2`. Tap the direct link in Step 3 instead of searching. |
| "App crashes when I sign in" | Tester should send a bug report from inside the app (Settings → Help → Report a bug). If app crashes before Settings is reachable, email Tan with the crash time + phone model — backend `/errors` table may have it. |
| "Can my colleague who isn't on the list also test?" | Reply to Tan with their email so he can add them in Play Console. Don't forward the opt-in URL. |
| "What if I want to leave the test?" | Open opt-in URL again → "Leave the program". App stays installed but stops getting closed-test updates. |

## Reference

- Original closed-testing setup notes: see [[project_session_20260506]] in memory
- Multi-language keyboard (Tier β) scope: `docs/superpowers/specs/2026-05-17-android-multilang-keyboard-design.md`
- Manual QA matrix for Tier β: `docs/superpowers/qa/2026-05-17-android-multilang-keyboard-qa.md`
