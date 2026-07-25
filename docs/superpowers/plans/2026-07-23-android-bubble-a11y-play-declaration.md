# P0 — Play Console permissions declaration + in-app disclosure (bubble in-place rewrite)

Gating artifact for shipping the AccessibilityService-driven in-place rewrite
(`RewriteAccessibilityService`). Google reviews *every* app that binds an
AccessibilityService for a non-accessibility purpose; a weak or missing
declaration = rejection or removal. Everything below is ready to paste/adapt.

## 1. What Google needs to approve
Per the Play **Accessibility API** and **Prominent Disclosure & Consent** policies:
1. A **Permissions Declaration** in Play Console (App content → Sensitive/Accessibility) stating the feature and why the API is required.
2. A **prominent in-app disclosure** shown *before* the user enables the feature and *before* any data access, with an affirmative opt-in.
3. Accurate **Data safety** entries (what text is read, where it goes).

## 2. Play Console — Accessibility permissions declaration (paste)
**Which feature uses the AccessibilityService?**
> DraftRight's optional "one-tap rewrite" bubble. When the user taps the floating
> bubble, the service reads the text currently in the focused input field and
> replaces it with an AI-rewritten version in place, so the user doesn't have to
> copy, switch apps, and paste.

**Why is the AccessibilityService API required (no alternative)?**
> Android exposes no other API that lets an app read and replace the text of the
> *currently focused field in another app*. `ACTION_SET_TEXT` on the focused
> `AccessibilityNodeInfo` is the only supported mechanism for in-place rewrite.
> Where it is unavailable (WebView/Compose/secure fields) the app falls back to
> the clipboard and does not use the accessibility data.

**Is it the core purpose?** Enhancement to the core rewrite product; **opt-in, off by default**.

**User benefit:** faster, in-place AI rewriting in any messaging/email/notes app without copy-paste; especially useful for Vietnamese writing correction.

## 3. Prominent disclosure screen (shown before enabling)
Affirmative action required (a "Turn on" button that is NOT pre-checked). Copy:

**EN**
> **Turn on one-tap rewrite in any app**
> To rewrite text right where you type it, DraftRight uses Android's Accessibility
> service. When — and only when — you tap the DraftRight bubble, it reads the text
> in the field you're editing and replaces it with your chosen rewrite.
>
> • It runs **only on your tap** — never in the background.
> • Your text is sent **only to the DraftRight rewrite service you configured**, to
>   produce the rewrite. It is not sold, shared, or used for ads.
> • Password fields are always skipped.
> • You can turn this off anytime in Settings.
>
> [ Not now ]   [ Turn on ]

**VN**
> **Bật viết lại một chạm trong mọi ứng dụng**
> Để viết lại văn bản ngay tại chỗ bạn đang gõ, DraftRight dùng dịch vụ Trợ năng của
> Android. Chỉ khi bạn chạm vào bong bóng DraftRight, ứng dụng mới đọc văn bản trong
> ô bạn đang chỉnh sửa và thay bằng bản viết lại bạn chọn.
>
> • Chỉ chạy **khi bạn chạm** — không bao giờ chạy ngầm.
> • Văn bản chỉ được gửi tới **dịch vụ viết lại DraftRight bạn đã cấu hình** để tạo bản
>   viết lại. Không bán, không chia sẻ, không dùng cho quảng cáo.
> • Luôn bỏ qua ô mật khẩu.
> • Bạn có thể tắt bất cứ lúc nào trong Cài đặt.
>
> [ Để sau ]   [ Bật ]

## 4. Consent flow (what P2 builds)
1. Settings toggle "One-tap rewrite (in-place)" — tapping it opens the disclosure above.
2. **Not now** → nothing changes (`bubbleInPlaceEnabled` stays false).
3. **Turn on** → write `flutter.draftright.bubbleInPlaceEnabled = true`, then deep-link to
   `Settings.ACTION_ACCESSIBILITY_SETTINGS` so the user enables "DraftRight" there.
4. The feature only acts when BOTH the flag is true AND the a11y service is bound
   (already enforced in `FloatingBubbleService.onBubbleTap`).
5. Toggling off writes the flag false (bubble reverts to clipboard/picker); optionally
   guide the user to disable the a11y service too.

## 5. Data safety (Play Console)
- Data type: **Text you enter / messages** (the focused field contents).
- Collected? Processed transiently to produce the rewrite; sent to the configured
  DraftRight backend over HTTPS. Not stored on-device by this feature beyond the
  in-flight request; not shared with third parties; not used for ads.
- Note the on-device fallback (clipboard) uses no accessibility data.

## 6. Risk + mitigation
- **Rejection risk is real.** Mitigate: opt-in/off-by-default, tap-only (no event
  listening), password-field skip, clipboard fallback so the app is useful even
  without the permission, and the disclosure above. If rejected, the feature can be
  shipped disabled while appealing — the rest of the app is unaffected.
- Keep the declaration answers and the in-app disclosure **wording consistent** —
  reviewers compare them.

## Decision
GO to build P2 (the toggle + disclosure) using this copy, then submit the declaration
with the release that first contains the enabled feature. Do **not** enable the flag
in a release until the declaration is approved.
