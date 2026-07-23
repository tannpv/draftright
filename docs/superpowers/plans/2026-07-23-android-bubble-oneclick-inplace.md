# Plan — Android bubble one-click in-place rewrite (AccessibilityService)

**Status:** proposed (spike = GO, see `experiments/a11y-rewrite-spike/FINDINGS.md`).
**Goal:** make the existing Android floating bubble rewrite the text in the *currently focused field of any app, in place* — no copy, no picker — mirroring desktop One-Click mode. Fall back to the current clipboard flow where the OS blocks in-place replace.

## Why now
Spike proved on a Samsung A52 that an `AccessibilityService` reads the focused field and replaces it via `ACTION_SET_TEXT` in Google Messages, WhatsApp, Gmail, and **Zalo — with Vietnamese diacritics preserved**. That is the VN-wedge use case. Boundaries (WebView/Compose/secure fields) are handled by a clipboard fallback.

## What exists today (reuse, don't rebuild)
- `FloatingBubbleService.kt` — overlay bubble; today tap → `PendingIntent` → `MainActivity` with clipboard text → tone picker.
- `keyboard/BackendClient.kt` — `rewrite(text, tone, …)` → `POST {backendUrl}/rewrite`. **Reuse verbatim.**
- `keyboard/SharedSettings.kt` — `bearerToken` (extension token or JWT), `backendUrl`. **Reuse.**
- `keyboard/Tone.kt` — Tone enum with `apiValue`.
- Spike code in `experiments/a11y-rewrite-spike/` — the proven overlay + a11y mechanics to port.

## New / changed components
1. **`RewriteAccessibilityService`** (new, in the app module) — port from the spike. On trigger:
   - `findFocus(FOCUS_INPUT)` across active + interactive windows.
   - Read `node.text`; capture `isEditable` / `isPassword` / `className` / `packageName`.
   - Call `BackendClient.rewrite(text, presetTone)` (off the main thread; the service can host a coroutine/executor).
   - On success: `ACTION_SET_TEXT` with the rewrite. If it returns `false` → **fallback** (see below).
   - Never touch `isPassword` fields — skip + toast "can't rewrite password fields".
2. **Bubble wiring** — `FloatingBubbleService` tap: if the a11y service is enabled AND a field is focused → route to `RewriteAccessibilityService.rewriteFocusedField()` (in-process, like the spike). Else → current clipboard/picker behavior (unchanged).
3. **Preset tone (One-Click)** — add a stored preference `bubble_preset_tone` (default e.g. `IMPROVE`/`PROFESSIONAL`). Read in the service. Long-press bubble → quick tone switch (later; v1 can ship a single default).
4. **Fallback path** — when `ACTION_SET_TEXT` is unavailable/false (WebView/Compose/custom): put the rewrite on the clipboard + toast "Rewrite copied — paste to replace". Preserves usefulness everywhere; no dead ends.
5. **Loading/UX** — bubble shows a spinner/pulse during the network call; success = brief check; failure = toast with reason. Keep the `FLAG_NOT_FOCUSABLE` overlay so focus stays on the target field.
6. **Manifest + config** — add the `AccessibilityService` (`BIND_ACCESSIBILITY_SERVICE`, intent-filter, `accessibility_service_config.xml` with `canRetrieveWindowContent=true`, `flagRetrieveInteractiveWindows`).

## Play Store policy (REQUIRED — do first, it gates release)
AccessibilityService for a non-accessibility purpose needs:
- **Play Console Permissions Declaration** justifying the use.
- **Prominent in-app disclosure** screen before enabling (what it does, that it reads focused text to rewrite it, on-device consent).
- Real **rejection risk** — the messaging + VN envelope is the justification. Decide GO/NO-GO on shipping *before* building UI polish.

## Phasing
- **P0 — policy check:** draft the Play declaration + disclosure copy; sanity-check rejection risk. Cheap, decisive.
- **P1 — core:** port `RewriteAccessibilityService`; wire bubble → rewrite → `ACTION_SET_TEXT`; clipboard fallback; preset tone = one default. Behind an **opt-in setting** (off by default) + disclosure gate.
- **P2 — UX:** loading state, per-app supported/fallback messaging, long-press tone switch.
- **P3 — closed testing:** ship to the existing closed-testing testers; collect the real supported-app envelope; confirm the spike's untested rows (Chrome/Compose/password).

## Rule #1 compliance (clean · extensible · reusable · no hardcoding)
This is a design constraint, not an afterthought:
- **Reusable** — do NOT duplicate the rewrite call or auth. Reuse `keyboard/BackendClient.rewrite()`, `keyboard/SharedSettings` (`bearerToken`, `backendUrl`), and `keyboard/Tone`. Port the proven overlay/a11y mechanics from the spike rather than reinventing.
- **Extensible via an interface** — introduce a `RewriteTarget` port with implementations `AccessibilityTarget` (in-place `ACTION_SET_TEXT`) and `ClipboardTarget` (fallback). The bubble picks a target; adding a future target (e.g. IME-commit) means a new impl, no change to the bubble or the rewrite call. Mirrors the desktop One-Click strategy pattern.
- **No hardcoding** — preset tone, backend URL, timeouts, and any user-facing strings come from settings/config/resources, not literals. Preset tone key `bubble_preset_tone` stored in `SharedSettings`; strings in `strings.xml` (VN + EN). Status/target kinds are typed enums (`RewriteTargetKind`), never string/int literals.
- **Clean** — small, single-purpose units: the a11y service finds+reads+writes the node; a `BubbleRewriteCoordinator` orchestrates capture → rewrite → target; `FloatingBubbleService` only owns the overlay. No God-object; no dead code.
- **Design for the next platform** — keep the coordinator/target split platform-shaped so the same flow maps onto iOS's constraints later.

## Testing
- Re-run the spike matrix against the *real* rewrite (not uppercase): Messages, WhatsApp, Zalo, Gmail (expect ✅), + confirm Chrome/Compose/password fall back cleanly to clipboard.
- Vietnamese round-trip must preserve diacritics (spike already showed the a11y layer does; verify the backend rewrite path too).
- No-crash when: no field focused, password field, network error, not-signed-in (toast + no-op).

## Risks / open questions
- Play rejection (biggest). Mitigate with disclosure + declaration; have the fallback so the feature also works without a11y (clipboard) if a11y is disabled.
- `ACTION_SET_TEXT` whole-field replace loses cursor/selection — acceptable for a "rewrite the whole message" flow; document it.
- Battery: foreground overlay service — already the case today; no new cost.
- Samsung reverts adb-enabled a11y (dev-only annoyance; users enable via the disclosure flow).

## Definition of done (v1)
Opt-in setting + disclosure → enable a11y → tap bubble over a focused field in a supported app → text is AI-rewritten in place with the preset tone; unsupported fields fall back to clipboard with a clear toast; nothing crashes; Play declaration submitted.
