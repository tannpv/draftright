# Entity Extraction from Shared Chat Messages — Design

**Date:** 2026-05-14
**Author:** Tan Nguyen
**Status:** Approved for planning
**Target clients:** DraftRight Mobile (Flutter — Android + iOS)
**Future clients:** macOS, Windows, Linux desktop (regex catalog reused)

---

## 1. Problem

When a user receives a chat message that contains multiple pieces of information (phone numbers, addresses, bank accounts, URLs, OTP codes, etc.), copying any single piece is awkward on mobile — they have to manually select the substring, fight the OS text selector, and hope to hit the right boundaries. The current "long-press → text selection handles" pattern is the bottleneck.

DraftRight already receives shared text via the system share sheet on both platforms and pipes it into a tone-picker for rewriting. We will extend that flow: when the shared text contains structured entities, the app shows an **entity sheet** with one-tap copy per detected item instead of the tone picker.

## 2. Goals

- One-tap copy for phone, email, URL, OTP, credit card on every share, fully offline (regex).
- Optional ✨ Smart scan adds address, person name, date/time, bank account via the user's configured AI provider.
- Falls back to existing tone-picker behavior when no entities are detected — zero regression for users who share prose.
- No new Settings toggles. Per-share opt-in for the LLM call.
- Same UX on Android and iOS via the existing share extensions.
- Regex catalog written in pure Dart, factored for later reuse on macOS/Windows/Linux.

## 3. Non-goals

- No background clipboard monitoring (deferred — different feature surface).
- No new keyboard-extension surface (the keyboards already have their own constraints).
- No Settings toggle for "enable Smart extraction" — replaced by per-share button.
- No detection of business names, product SKUs, or domain-specific identifiers outside the listed entity kinds.
- No analytics of which entities the user copied — out of scope for this spec.

## 4. User flow

### 4.1 Happy path (regex)

1. User long-presses a message in any chat app → Share → DraftRight.
2. Android `MainActivity` / iOS `ActionViewController` extract the shared text and forward it to Flutter via the existing `draftright/share` MethodChannel.
3. `ShareRewriteScreen` (existing) intercepts the text and runs `EntityExtractor.extract()` synchronously.
4. If `entities.isNotEmpty`, `Navigator.pushReplacement` → `EntitySheetScreen` (grouped-list layout). Otherwise the existing tone-picker flow runs unchanged.
5. User taps the copy icon next to any entity → `Clipboard.setData` → SnackBar "Phone copied" (kind-specific label).
6. User taps "Done" → app backgrounds (Android `moveTaskToBack`, iOS share-extension dismiss).

### 4.2 Smart-scan path (LLM)

1. From the entity sheet the user taps **✨ Smart scan**.
2. Button transitions to a spinner; `extraction_api.llmExtract(text)` POSTs to `/extract` with JWT auth.
3. Backend builds a strict-JSON extraction prompt, calls the user's AI provider, validates and dedupes the response, returns `ExtractResponseDto`.
4. Client merges LLM entities with the existing regex set (dedupe by `(kind, value)` case-insensitive). Smart-scan button hides on success; SnackBar shows "Found N more".
5. On network/auth/5xx/malformed-JSON: `ExtractionUnavailableException` → SnackBar "Smart scan unavailable", regex results remain visible, button re-enabled.

### 4.3 No entities

`EntityExtractor.extract()` returns empty → entity sheet is never mounted → user sees the existing tone-picker. No regression.

## 5. Architecture

```
┌─────────── Flutter app ───────────┐         ┌──────── Backend (NestJS) ────────┐
│                                   │         │                                  │
│  ShareRewriteScreen (existing)    │         │  POST /extract                   │
│      │                            │         │      │                           │
│      ├─ getEntities()─────────────┼─────────┤      ├─ JwtAuthGuard             │
│      │     ↓                      │  HTTPS  │      ├─ ExtractionController     │
│      │  EntityExtractor (Dart)    │         │      │     │                     │
│      │     ├─ regex pass          │         │      │     ↓                     │
│      │     └─ returns Entity[]    │         │      │  ExtractionService        │
│      │                            │         │      │     ├─ build prompt       │
│      ├─ if entities non-empty ────┼─────────┤      │     ├─ call AIProvider    │
│      │     EntitySheetScreen      │         │      │     ├─ parse + validate   │
│      │  else → existing tones     │         │      │     ├─ resolve offsets    │
│      │                            │         │      │     └─ dedupe + return    │
│      └─ ✨ Smart scan → extract ──┼◀────────┼──────┘                           │
└───────────────────────────────────┘  JSON   └──────────────────────────────────┘

Reused infra:
  • ShareService (intent → onSharedText)
  • BackendClient (JWT auth + refresh)
  • AIProviderService (existing per-user provider abstraction)
  • ErrorReporter (POST /errors)
  • Existing rewrite quota + rate-limit middleware (extended to /extract)

New units:
  Dart:
    lib/models/entity.dart                       data class
    lib/services/entity_extractor.dart           pure-Dart regex engine
    lib/services/extraction_api.dart             backend client for LLM pass
    lib/screens/entity_sheet_screen.dart         grouped-list UI
  Backend:
    backend/src/extraction/extraction.module.ts
    backend/src/extraction/extraction.controller.ts
    backend/src/extraction/extraction.service.ts
    backend/src/extraction/dto/extract.dto.ts
```

Each new unit has one clear responsibility, is independently testable, and depends only on existing primitives. `EntityExtractor` has no Flutter imports — it can be lifted into a Dart package later for the desktop clients (or re-implemented in Swift/C#/Python from the same regex catalog).

## 6. Component contracts

### 6.1 `Entity` (Dart)

```dart
enum EntityKind { phone, email, url, otp, creditCard, address, personName, dateTime, bankAccount }

class Entity {
  final EntityKind kind;
  final String value;        // canonical form, e.g. "+84912345678"
  final String display;      // pretty form for UI, e.g. "+84 912 345 678"
  final int start;           // offset in original text (UTF-16 code units, matches Dart String indices)
  final int end;             // exclusive
  final String source;       // "regex" or "llm"
  final double confidence;   // 0.0–1.0
  final Map<String, String> meta; // bank name, country code, etc.
}
```

Identity for dedupe: `(kind, value)` case-insensitive.

### 6.2 `EntityExtractor` (Dart, pure)

- `static List<Entity> extract(String text)` — runs every registered detector, returns deduped entities sorted by `start`.
- Internal registry: `final List<EntityDetector> _detectors` populated at file load — adding a new entity kind = one new detector class.
- Detectors:
  - **Phone:** Vietnamese-aware. Patterns: `+84\s?\d{9,10}`, `0\d{9,10}`, generic international `\+\d{1,3}[\s\-]?\d{4,14}`. Normalizer converts `0xx` → `+84xx`, strips spaces/dashes.
  - **Email:** RFC-ish — `[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`.
  - **URL:** `https?://\S+` and bare `\b(?:[a-z0-9\-]+\.)+(?:com|vn|net|org|io|co|app|me|info|biz|asia|tv|gg)(?:/\S*)?\b`. Strips trailing `.,!?;:)` before commit.
  - **OTP:** `\b\d{4,8}\b` only when within 20 chars of trigger `(OTP|mã|code|verification|xác minh|password)` (case-insensitive). Avoids matching every year number.
  - **Credit card:** 13–19 digit groups, normalize spaces/dashes, Luhn-check before accepting. UI masks to `**** **** **** XXXX`; copy emits full value.
  - **Bank account (regex layer):** lookup table of common VN bank names (Vietcombank, BIDV, MB, ACB, Techcombank, VPBank, VietinBank, Sacombank, TPBank, OCB, …). When a bank name occurs and an 8–19 digit number appears within 30 chars on the same line, emit `bankAccount` with `meta.bank` set.
- Dedupe rule: same `(kind, value)` keeps the entry with higher `confidence`. Ties broken by `source = "regex"` over `"llm"`.

### 6.3 `extraction_api.dart` (Dart)

- `Future<List<Entity>> llmExtract(String text)` — POSTs to `/extract` with JWT. 10s timeout. Throws `ExtractionUnavailableException` on network error, 4xx other than 402, 5xx, malformed body.
- 402 → throws `ExtractionQuotaException` (separate handling for the UI snackbar).

### 6.4 `EntitySheetScreen` (Flutter widget)

- Constructor: `EntitySheetScreen({required String text, required List<Entity> initial})`.
- State: `List<Entity> entities`, `bool smartScanLoading`, `bool smartScanDismissed`.
- Layout (Grouped list — design choice A):
  - One `Card` per `EntityKind` group (header = type label + icon).
  - Each row: `display` value + trailing `IconButton(Icons.copy)`. Tap copies `value`.
  - Credit-card rows show masked display by default; long-press toggles reveal.
  - Footer: ✨ Smart scan button (until pressed and succeeded) and a "Done" button.
- Sorted within group by appearance (`start` ascending).
- Empty state cannot render — guard at construction site.

### 6.5 `POST /extract` (NestJS)

Request DTO:

```ts
class ExtractRequestDto {
  @IsString() @MaxLength(8000) text!: string;
  @IsOptional() @IsArray() @ArrayMaxSize(20) kinds?: EntityKind[];
}
```

Response DTO:

```ts
class ExtractedEntityDto {
  kind!: EntityKind;
  value!: string;
  display!: string;
  start!: number;
  end!: number;
  confidence!: number;
  meta?: Record<string, string>;
}

class ExtractResponseDto {
  entities!: ExtractedEntityDto[];
  provider!: string;        // 'openai' | 'anthropic' | 'ollama' | 'custom'
  tokensUsed!: number;
}
```

### 6.6 `ExtractionService` (NestJS)

- `extract(userId, dto)` builds the prompt:
  - System: "You extract structured entities from short messages. Return strict JSON array, no commentary. Each item: `{kind, value, display, confidence, meta}`. Kinds the model MAY emit: `address|personName|dateTime|bankAccount`. Kinds the model MUST NOT emit (handled by client regex): `phone|email|url|otp|creditCard`."
  - Few-shot examples for VN address ("123 Lê Lợi, Q1, TP.HCM") and bank line ("Vietcombank 0123456789").
  - User: the message text.
  - Service-side filter additionally drops any entity whose `kind` is in the excluded set, defense-in-depth against models that ignore the instruction.
- Calls `AIProviderService` with user's selected provider.
- Parses response as JSON. Drops entries that fail class-validator. Drops entries whose `value` substring is not present in the original text (per `feedback_llm_offsets_unreliable.md`).
- Recomputes `start`/`end` via `text.indexOf(value)` because LLM offsets are unreliable.
- Returns merged + deduped list. Records `tokensUsed` to usage table for the existing quota system.

## 7. Error handling

| Failure | Detection | UX | Telemetry |
|---|---|---|---|
| Regex never matches | empty result | Fall back to existing tone picker | none |
| Shared text > 8000 chars | DTO `@MaxLength(8000)` rejects | Client truncates to 8000 for the LLM call only; regex runs on full text | log "extraction_truncated" |
| Network failure on `/extract` | `SocketException`, `TimeoutException` (10s) | `ExtractionUnavailableException` → snackbar, regex results stay | ErrorReporter breadcrumb |
| Backend 401 (token expired) | HTTP 401 | BackendClient refresh-token flow auto-retries once; second failure → snackbar | log "extraction_auth_expired" |
| Backend 5xx | HTTP 5xx | Snackbar; regex results stay | ErrorReporter POST `/errors` |
| AI provider returns non-JSON | server JSON parse fails | Service returns 200 with `entities: []`, `tokensUsed: 0` | log raw output (PII-stripped) |
| AI returns malformed entry | class-validator fails | Service drops that entry, keeps the rest | log dropped count |
| AI hallucinates `value` not in text | `text.indexOf(value) === -1` | Service drops entry | log "extraction_hallucination" |
| User quota exceeded | HTTP 402 from existing middleware | Client snackbar "Smart scan limit reached" | none new |
| Duplicate entity regex + LLM | `(kind, value)` dedupe | Keep regex entry | none |
| Clipboard copy throws | catch in tap handler | Snackbar "Copy failed — long-press to select" | log |

## 8. Security & privacy

- `/extract` is JWT-guarded; no anonymous access.
- Existing rewrite rate-limit + quota middleware applies (DTO + auth chain identical).
- Backend never persists `text`, `value`, or any entity content. Only `kind` counts and `tokensUsed` are written to the existing usage table.
- Access logs mask the `text` field on `POST /extract` via a NestJS interceptor. Card numbers are not logged anywhere.
- Credit-card UI: masked by default (`**** **** **** 4242`), revealed via long-press, copy emits full value (intentional — that's the whole feature).
- OTP entries are never logged server-side or client-side beyond in-memory state.
- LLM output is parsed as strict JSON. Schema validation drops anything that does not match. No tool calls, no eval, no execution of model output.
- Prompt-injection attempts in the input text cannot escape the JSON schema — at worst they cause the service to return `entities: []`.

## 9. Testing

### 9.1 Dart unit tests — `test/services/entity_extractor_test.dart`

Covers each detector against a fixture table, dedupe behavior, offset round-trip (every entity's `text.substring(start, end)` matches the source), and i18n cases (Vietnamese diacritics, emoji, UTF-16 surrogate pairs). Empty input, 10kB blob, whitespace-only, single character.

### 9.2 Dart widget tests — `test/screens/entity_sheet_screen_test.dart`

- One Card per group, rows sorted by `start`.
- Copy icon invokes mocked `Clipboard.setData` with `entity.value`.
- Smart scan loading state disables button.
- Smart scan success merges new entities and hides button.
- Smart scan failure shows snackbar, button re-enabled, regex entities unchanged.
- Credit-card row masked by default; long-press toggles reveal.

### 9.3 Backend Jest — `extraction.service.spec.ts`

Prompt construction, AIProvider mocking, JSON parse failure path, offset recomputation, hallucination guard, dedupe with caller-known entities, token-count accumulation, DTO truncation rejection.

### 9.4 Backend e2e — `extraction.controller.e2e.spec.ts`

`supertest`: 401 without JWT, 200 with valid request, 400 on missing/oversized text, 402 on quota exceeded (mocked), audit log shape (`kind` counts only).

### 9.5 Manual QA — `docs/test-cases.xlsx`

Per `CLAUDE.md` mandate, test cases are written **before** implementation.

| TC ID | Description |
|---|---|
| EXTRACT-001 | Share message with phone+email → entity sheet, 2 rows, both copy |
| EXTRACT-002 | Share message with no entities → tone picker shows (regression check) |
| EXTRACT-003 | Smart scan adds Vietnamese address `123 Lê Lợi Q1` |
| EXTRACT-004 | Smart scan offline → snackbar, regex results intact |
| EXTRACT-005 | Card number masked on render, full value copied |
| EXTRACT-006 | OTP detected only when trigger word present |
| EXTRACT-007 | Long message (5000 chars) → regex < 50ms, no UI freeze |
| EXTRACT-008 | iOS share extension same flow |
| EXTRACT-009 | Quota exceeded → 402 snackbar, regex results stay |
| EXTRACT-010 | Vietnamese diacritics + emoji in source → offsets correct |

### 9.6 CI

`flutter test` for `DraftRightMobile/`, `npm test` for `backend/`. No new tooling.

## 10. Rollout

1. Feature branch `feature/entity-extraction-20260514` from `develop`.
2. Add test-case IDs to `docs/test-cases.xlsx` first.
3. Implement backend (`/extract`) and Dart (`EntityExtractor`) in parallel (independent units).
4. Wire UI; manual QA on Android emulator and an iOS device.
5. Merge to `develop`, deploy to testing server, run E2E suite.
6. Apply `status: deployed to testing` label on the tracking issue.
7. Merge `develop` → `main`, deploy to production.
8. Apply `status: deployed to production` + write `## ✅ How to Verify` comment.
9. Leave the issue open for the boss to verify.

## 11. Open questions

None at design lock. Implementation plan will surface any remaining detail decisions.

## 12. Future work (explicitly out of scope here)

- Clipboard-monitor entry point (floating bubble opens entity sheet automatically).
- Per-entity quick actions (call phone, open URL in browser, save contact, open Maps for address).
- Entity history (a log of extracted items in the app for "what was that bank number Mark sent me yesterday?").
- macOS / Windows / Linux desktop ports of the regex catalog (separate platform-specific UI).
- Entity-extraction analytics (track copy rate to learn which detectors are valuable).
- **DraftRight keyboard chip-strip** (Android IME + iOS keyboard ext) — clipboard-driven chips above the keys that insert at the caret. Defers until MVP ships and adoption justifies the regex-catalog port from Dart to Kotlin/Swift (recommend JSON-catalog refactor at that point so all three engines compile identical patterns). Out of scope here because (a) Flutter engine doesn't run inside the IME, (b) Apple's "Full Access" prompt depresses adoption, and (c) memory note `project_android_keyboard_strategy.md` flagged the IME as a secondary surface behind share intent.
