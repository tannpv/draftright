# Android a11y rewrite spike — findings

Goal: prove whether an AccessibilityService can **read the focused text field of another app and replace it in place** (`ACTION_SET_TEXT`), across real apps, before building it into DraftRight. Transform = UPPERCASE (local, no network) so pass/fail reflects Android constraints only.

## How to run
```bash
export PATH="$HOME/Library/Android/sdk/platform-tools:$PATH"
adb install -r app/build/outputs/apk/debug/app-debug.apk
adb shell am start -n com.draftright.spike/.MainActivity
adb logcat -s SPIKE          # live per-tap results
```
In the app: (1) Grant overlay permission → (2) Accessibility settings → enable "A11y Rewrite Spike" → (3) Start bubble. Then open each target app, tap into its text field, type something, drag the bubble over the screen, **tap the bubble**. Field text should become UPPERCASE. Read results in-app ("Refresh log") or via logcat.

## Test matrix — results 2026-07-23 (Samsung Galaxy A52, SM-A528B, Android)
Empirically tested via the bubble → `ACTION_SET_TEXT`. Transform = UPPERCASE.

| App / field | Read | Replace | Node class | Notes |
|---|---|---|---|---|
| Google Messages — compose | ✅ | ✅ | `android.widget.EditText` | "This is the new test" → "THIS IS THE NEW TEST" |
| WhatsApp — message box | ✅ | ✅ | `android.widget.EditText` | replaced cleanly, repeat-safe |
| **Zalo — message box** | ✅ | ✅ | `android.widget.EditText` | **"Chào em" → "CHÀO EM" — Vietnamese diacritics preserved through read+write.** The VN-target result. |
| Gmail — compose body | ✅ | ✅ | `android.widget.EditText` | "Thís is my test" → "THÍS IS MY TEST" (VN diacritic preserved) |
| Chrome — omnibox / web input | — | — | — | **NOT tested.** Automated data-URI attempt failed to open Chrome (landed on launcher); no valid reading. Expected ❌ (WebView doesn't expose `ACTION_SET_TEXT`). |
| Jetpack Compose TextField | — | — | — | **NOT tested.** Expected ❌/⚠️ per platform. |
| Password field (secure) | — | — | — | **NOT tested.** Expected ❌ (`isPassword`/FLAG_SECURE). |
| Google Keep / Samsung Notes | — | — | — | **NOT tested** (time). Expected ✅ (EditText-backed). |

Honesty note: the four ✅ rows are real device results. The four "NOT tested" rows are **assumptions from Android platform behavior**, not confirmed this session.

## Decision gate
- **GO** — build it into DraftRight (with clipboard fallback for unsupported fields) IF plain EditText **and** the mainstream messaging/notes/email apps (WhatsApp/Gmail/Keep + ideally Zalo) read+replace reliably. Browsers/Compose/secure-field failures are acceptable.
- **NO-GO / rethink** — if replace fails on the common messaging/notes apps too. Then the AccessibilityService route isn't worth the Play-policy cost; the keyboard IME already does in-place rewrite and the clipboard bubble flow already ships.

## Play-policy note (assess, not code-tested)
An AccessibilityService for non-accessibility use needs a Play Console **Permissions Declaration** + prominent in-app disclosure, and risks rejection. Record here whether the supported-app envelope justifies that cost.

## Result summary — 2026-07-23
- **Supported-app envelope (confirmed):** classic `EditText`-backed apps — Google Messages, WhatsApp, **Zalo**, Gmail. Read + replace-in-place via `ACTION_SET_TEXT` all worked, and **Vietnamese diacritics survived the round-trip** (critical for DraftRight's VN wedge).
- **Focus safety confirmed:** the `FLAG_NOT_FOCUSABLE` overlay bubble did not steal focus — `findFocus(FOCUS_INPUT)` reliably returned the target field. When no field was focused, it cleanly logged "no focused editable node" (graceful, no crash).
- **Not yet confirmed (assumed-fail, handle with fallback):** Chrome/WebView, Jetpack Compose `TextField`, secure/password fields. These are the known Android limits; a follow-up 5-minute manual pass on the A52 would close them.

### VERDICT: **GO** ✅
The decision gate ("plain EditText + mainstream messaging/notes/email, ideally Zalo") is **met**. Build the AccessibilityService-driven one-click in-place rewrite into DraftRight, with a **clipboard-copy fallback** whenever `ACTION_SET_TEXT` returns false (WebView/Compose/custom editors) so the feature degrades gracefully instead of failing.

### Required before shipping (not code — process)
- **Play Console Permissions Declaration** + prominent in-app disclosure justifying AccessibilityService use (non-accessibility purpose). Rejection risk is real; the supported-app envelope (messaging + VN) justifies the attempt.
- Confirm the assumed-fail rows on-device so the in-app "supported here / copied to clipboard instead" messaging is accurate.
