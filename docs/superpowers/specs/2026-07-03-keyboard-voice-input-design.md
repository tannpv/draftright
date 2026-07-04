# Keyboard Voice Input — Dictate → AI-Polish (Design)

**Date:** 2026-07-03
**Status:** Approved (Tan, 2026-07-03)
**Scope v1:** Android keyboard + backend `input_kind` flag. iOS designed here, built in v2.

## Goal

Market differentiator for the DraftRight keyboard: tap mic, speak roughly, get
**polished text in the selected tone** committed into the host app's field.
Plain dictation is a Gboard commodity; dictation → tone-polished writing is not.

## Platform constraint that shapes the design

iOS keyboard extensions **cannot access the microphone — ever**, even with
`RequestsOpenAccess` (Apple blocks mic for all extension types since iOS 8; no
workaround). Android IMEs have no such restriction (`SpeechRecognizer` in the
IME process, Gboard model). Therefore: Android ships the real in-keyboard
experience first; iOS gets a main-app round-trip flow in v2.

## Decisions (locked)

| Decision | Choice |
|---|---|
| Core behavior | Dictate → AI-polish through selected tone (not raw dictation, not voice commands) |
| Platforms | Android v1; iOS v2 (designed below) |
| Languages | EN (`en-US`) + VI (`vi-VN`) at launch; locale map extendable per language pack |
| STT | Device speech service (`SpeechRecognizer` / `SFSpeechRecognizer`) — free; may use the platform provider's network path. No third-party cloud STT |
| Polish | Existing `POST /rewrite` + new optional `input_kind` field |
| Golden rule | **Never lose the user's words** — any polish failure commits the raw transcript |

## 1. Android UX

- **Mic key** in the tone/suggestion strip (brand-blue functional key, Material
  mic icon), rendered only when a recognizer is available on-device; otherwise
  the key is not shown at all.
- State machine: `IDLE → LISTENING → PROCESSING → IDLE`
  - LISTENING: mic key pulses; live partial transcript displayed in the
    candidate bar. Tap mic again or Back = cancel → IDLE (nothing inserted).
  - PROCESSING: spinner on mic key; final transcript sent to `/rewrite` with
    the keyboard's currently selected tone; on success the polished text is
    committed via `ic.commitText`.
- **Long-press mic = raw mode**: insert the transcript as-is (no AI call).
  Works offline and over quota.
- Failure handling (all paths → commit raw transcript + brief inline hint):
  401/refresh-failed, timeout (15s), network error, quota exceeded, provider
  error. Speech is unrepeatable; silence-and-drop is forbidden.
- STT locale follows the active language pack: VI pack → `vi-VN`, default →
  `en-US`. Mapping lives beside the pack definitions (enum-style, one entry per
  future pack — FR/JA/KO/ZH later; CJK quality TBD per locale before enabling).

## 2. Android internals

- New `VoiceInputController` class in the keyboard module
  (`android/.../keyboard/voice/`): owns the `SpeechRecognizer` lifecycle,
  exposes the state machine via a listener interface to `DraftRightIME`.
  Constructor takes a recognizer factory → unit-testable with a fake (no
  device services in tests).
- `RECORD_AUDIO` is a runtime permission and an IME has no Activity to request
  it: add a transparent trampoline `PermissionActivity` in the main app
  (standard IME pattern). Mic key checks permission → launches trampoline once
  → resumes listening on grant. Deny → key stays, tapping re-explains.
- Recognizer availability check via `SpeechRecognizer.isRecognitionAvailable`;
  absent (no Google speech service) → mic key hidden.
- Manifest: add `RECORD_AUDIO` permission + trampoline activity entry.

## 3. Backend — `input_kind` (parity-safe)

- `POST /rewrite` DTO gains optional `input_kind`: `"typed"` (default) |
  `"speech"`. Absent field ⇒ behavior byte-identical to today (zero risk to
  existing clients).
- When `"speech"`: prepend to the tone prompt — *"The input is dictated
  speech: remove filler words and false starts, restore punctuation and
  casing, keep the meaning and language."* (VI fillers — "ừm", "à", "kiểu" —
  covered by instruction, not enumeration.)
- Implemented in **both** backends: NestJS first (parity authority), then Go;
  shadow-gate row added. Same field accepted on `/rewrite/trial` and Go-native
  `/v1/rewrite` for consistency.

## 4. iOS (v2 — build after Android validates)

- Mic key on the iOS keyboard → opens the containing app via responder-chain
  `openURL` (known-flaky across iOS versions; on failure show an instruction
  sheet: "Open DraftRight to dictate").
- Main app voice screen: `SFSpeechRecognizer` (on-device), live transcript,
  tone picker (defaults to keyboard's last tone via App Group), Polish button.
- Result staged in App Group as `pendingVoiceText` (+ timestamp, TTL ~5 min).
- Keyboard, on next appearance in any host app, shows a **"Voice text ready —
  tap to insert"** chip; tap → `textDocumentProxy.insertText`, clear staging.
- Same golden rule: polish failure in-app offers "Use raw transcript".

## 5. Testing

- Test cases added to `docs/test-cases.xlsx` **before code** (checklist step
  1): permission trampoline grant/deny, EN + VI dictation happy path, cancel,
  long-press raw, each failure→raw-fallback path, quota, offline, recognizer
  absent, locale switch mid-session.
- Kotlin unit tests: `VoiceInputController` state machine (fake recognizer),
  locale mapping, failure fallbacks.
- Backend: Node + Go unit tests for `input_kind` prompt assembly; shadow gate
  green before deploy.
- Manual E2E on Samsung A52 (VI + EN) before merge to develop.

## Rule #1 compliance (clean / extendable / reusable / no hardcoding)

Binding on the implementation plan — each item names the mechanism, not an
intention:

1. **STT locale = language-pack property, not a switch.** `LanguagePack`
   gains `sttLocale` (nullable: null = voice unsupported for that pack).
   Enabling FR later = set one field on the FR pack. No `if (lang == "vi")`
   anywhere. Same property lands on the iOS `LanguagePack` in v2 (platform
   parity, mirrors the existing composer() pattern).
2. **`input_kind` is an enum at every layer.** Kotlin `InputKind` enum with
   `apiValue` (mirrors `Tone.kt`); NestJS DTO validated with `@IsEnum`; Go
   `domain.InputKind` type + `ParseInputKind` returning the domain sentinel
   (mirrors `tone.go`). No `"speech"` string literals at call sites.
3. **Speech-cleanup instruction lives server-side with the tone prompts** —
   single source, one place per backend, byte-identical Node/Go. Clients never
   embed prompt text (same principle as open issue #24 tone-list SSOT).
4. **Voice capture behind a consumer-owned interface.** Kotlin `VoiceInput`
   interface (start/stop/cancel + listener) with `SpeechRecognizerVoiceInput`
   impl; recognizer injected via factory (fake in tests). iOS v2 implements
   the same shape as a Swift protocol — the IME/state-machine code is written
   against the interface once per platform, swappable engine.
5. **One commit path.** All outcomes (polished, raw fallback, long-press raw)
   funnel through a single `commitResult()`; failure taxonomy is a sealed
   class mapped to fallback behavior — no per-error duplicated insert logic.
6. **Mic key registered through the existing shared functional-key system**
   (QwertyLayout key definitions, same mechanism as the other brand-blue
   functional keys) — not a bolted-on view.
7. **No magic numbers.** Listening timeout, partial-result debounce, App Group
   staging TTL (v2) are named constants in one config object per platform;
   rewrite timeout reuses the existing shared 15s constant.
8. **Permission trampoline is generic.** `RequestPermissionActivity` takes the
   permission name as an extra — reusable for any future IME-side permission,
   not a RECORD_AUDIO one-off.

Note (resolves a self-contradiction with item 6 above): the mic key actually
lives in the tone strip (`ToolbarView`), per §1 Android UX — `QwertyLayout`'s
key-def system covers the letter/functional keyboard rows only and doesn't
apply to the toolbar row, so item 6's "registered through the existing shared
functional-key system" refers to ToolbarView's own button mechanism (same
pattern as the tone buttons), not QwertyLayout.

## Out of scope (v1)

iOS build, FR/JA/KO/ZH locales, TTS voice prompts, voice commands/intent
parsing (Moncar-style), streaming partial polish, Windows/Linux/macOS voice.

## Reference

Moncar voice-fill tech guide (github.com/tannpv/moncar
`docs/voice-fill-tech.md`) — proved on-device STT + LLM parse pattern and
vi-VN quality; DraftRight reuses the STT layer idea but polishes via the
existing `/rewrite` pipeline instead of an intent registry.
