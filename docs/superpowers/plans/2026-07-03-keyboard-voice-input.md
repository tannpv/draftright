# Keyboard Voice Input (Dictate → AI-Polish) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Android keyboard mic key: speak → on-device STT → `/rewrite` polish in selected tone → committed text; parity-safe `input_kind` field on both backends.

**Architecture:** New `VoiceInput` interface + `SpeechRecognizerVoiceInput` in the Android keyboard module drives a 3-state session; final transcript goes through the existing `BackendClient.rewrite` with `input_kind="speech"`; ALL outcomes funnel through one commit path with raw-transcript fallback. Backends (NestJS = parity authority, then Go) accept optional `input_kind` and prepend a speech-cleanup preamble to the tone prompt.

**Tech Stack:** Kotlin (keyboard module, JUnit4 unit tests), NestJS + class-validator (Jest), Go (parity `validate.go`/`prompts.go` + domain), shadowdiff gate fixtures, openpyxl for test-cases sheet.

**Spec:** `docs/superpowers/specs/2026-07-03-keyboard-voice-input-design.md` (approved 2026-07-03). Branch: `feature/keyboard-voice-input-20260703` (exists, spec committed).

## Global Constraints

- Rule #1 (spec §Rule #1): enums at every layer, `sttLocale` as LanguagePack property, single `commitResult()` path, server-side prompt SSOT, named constants, generic permission trampoline.
- Golden rule: **any polish failure commits the raw transcript** — never drop speech.
- Wire value strings: `"typed"`, `"speed"` is WRONG — exactly `"typed" | "speech"`. Absent field ⇒ byte-identical behavior to today.
- Node is parity authority: implement Node first, Go must match its validation message byte-for-byte: `input_kind must be one of the following values: typed, speech`.
- Test commands: Node `cd backend && npm run test`; Go `cd backend-rewrite-go && go test ./... -race -count=1 && gofmt -l . && go vet ./...`; Android `cd DraftRightMobile/android && ./gradlew :app:testDebugUnitTest`.
- Commit prefixes `feat:`/`test:`/`docs:`; all commits end with `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>`.

---

### Task 1: Test cases sheet (MANDATORY first)

**Files:**
- Modify: `docs/test-cases.xlsx` (add sheet `VOICE`)

**Interfaces:** Produces TC IDs `VOICE-001..VOICE-012` referenced in code comments by later tasks.

- [ ] **Step 1: Add VOICE sheet via openpyxl**

```bash
cd /opt/openAi/DraftRight && python3 - <<'EOF'
import openpyxl
wb = openpyxl.load_workbook('docs/test-cases.xlsx')
ws = wb.create_sheet('VOICE')
ws.append(('TC-ID','Title','Preconditions','Steps','Expected','Verified by','Priority','Owner'))
rows = [
 ('VOICE-001','Node: input_kind=speech prepends speech preamble','—','resolvePrompt-equivalent called with tone=polished, input_kind=speech','Prompt = SPEECH_PREAMBLE + polished tone prompt',"rewrite.service.spec.ts: 'speech input_kind prepends preamble'",'P0','Tan'),
 ('VOICE-002','Node: absent/typed input_kind = unchanged prompt','—','POST /rewrite without input_kind and with input_kind=typed','Prompt identical to today; both requests 201',"rewrite.service.spec.ts: 'typed/absent leaves prompt unchanged'",'P0','Tan'),
 ('VOICE-003','Node: invalid input_kind rejected 400','—','POST /rewrite input_kind=banana','400, message: input_kind must be one of the following values: typed, speech',"DTO validation via e2e/validation spec",'P0','Tan'),
 ('VOICE-004','Go parity: same three behaviors as VOICE-001..003 byte-identical','Node running','shadowdiff fixtures rewrite_input_kind_speech / _typed / _invalid','Gate rows PASS (Node/Go identical)','shadowdiff fixtures','P0','Tan'),
 ('VOICE-005','Kotlin: InputKind enum apiValue','—','InputKind.SPEECH.apiValue','"speech"; TYPED → "typed"','InputKindTest.kt','P1','Tan'),
 ('VOICE-006','VoiceSession: happy path EN','fake recognizer returns final transcript','start → partial → final → polish success','state IDLE→LISTENING→PROCESSING→IDLE; commitResult(polished)','VoiceSessionControllerTest.kt happy path','P0','Tan'),
 ('VOICE-007','VoiceSession: polish failure commits RAW transcript','fake recognizer ok, fake client fails (401/timeout/quota)','start → final → polish error','commitResult(raw transcript) + hint banner; NEVER empty commit','VoiceSessionControllerTest.kt fallback tests','P0','Tan'),
 ('VOICE-008','VoiceSession: cancel inserts nothing','fake recognizer','start → cancel while LISTENING','state → IDLE, no commit call','VoiceSessionControllerTest.kt cancel','P0','Tan'),
 ('VOICE-009','Long-press mic = raw mode, no AI call','fake recognizer','long-press → speak → final','commitResult(raw), backend client NOT invoked','VoiceSessionControllerTest.kt raw mode','P1','Tan'),
 ('VOICE-010','sttLocale per pack','—','EnglishPack.sttLocale / VietnamesePack.sttLocale / JapanesePack.sttLocale','en-US / vi-VN / null (mic hidden for JA)','LanguagePackSttLocaleTest.kt','P1','Tan'),
 ('VOICE-011','Permission trampoline grant/deny','fresh install, RECORD_AUDIO not granted','tap mic → trampoline → grant; repeat with deny','grant: listening starts; deny: stays IDLE, key remains, re-tap re-explains','manual on A52','P1','Tan'),
 ('VOICE-012','Manual E2E vi-VN on A52','A52, VI pack active, logged in','mic → speak Vietnamese with fillers → observe','polished Vietnamese committed; fillers/punctuation fixed','manual on A52','P0','Tan'),
]
for r in rows: ws.append(r)
wb.save('docs/test-cases.xlsx')
print('VOICE sheet added,', len(rows), 'cases')
EOF
```
Expected: `VOICE sheet added, 12 cases`

- [ ] **Step 2: Commit**

```bash
git add docs/test-cases.xlsx && git commit -m "test: add VOICE test cases (VOICE-001..012) for keyboard voice input"
```

---

### Task 2: Node backend — `input_kind` (parity authority)

**Files:**
- Modify: `backend/src/rewrite/tones.ts` (add `INPUT_KIND_IDS`, `SPEECH_PREAMBLE`)
- Modify: `backend/src/rewrite/dto/rewrite.dto.ts`
- Modify: `backend/src/rewrite/rewrite.service.ts` (`resolvePrompt` + `rewrite`/`trialRewrite`/`callAI` threading)
- Modify: `backend/src/rewrite/rewrite.controller.ts` (pass `dto.input_kind`)
- Test: `backend/src/rewrite/rewrite.service.spec.ts`

**Interfaces:**
- Produces: `resolvePrompt(tone, targetLanguage?, sourceLanguage?, inputKind?)`; wire field `input_kind?: 'typed'|'speech'`; `SPEECH_PREAMBLE` exact string (Go copies it byte-for-byte in Task 3):
  `The input is dictated speech: remove filler words and false starts, restore punctuation and casing, keep the meaning and language. `
  (note trailing single space; preamble is PREPENDED to the resolved prompt for ALL tones when input_kind === 'speech').

- [ ] **Step 1: Write failing tests** (`rewrite.service.spec.ts`, follow existing `buildService` helper style in that file; TC: VOICE-001/002)

```typescript
describe('input_kind', () => {
  it('speech input_kind prepends preamble', () => {
    expect(resolvePrompt('polished', undefined, undefined, 'speech'))
      .toBe(SPEECH_PREAMBLE + TONE_PROMPTS['polished']);
  });
  it('typed/absent leaves prompt unchanged', () => {
    expect(resolvePrompt('polished', undefined, undefined, 'typed')).toBe(TONE_PROMPTS['polished']);
    expect(resolvePrompt('polished')).toBe(TONE_PROMPTS['polished']);
  });
  it('speech preamble also applies to translate', () => {
    const base = resolvePrompt('translate', 'Vietnamese');
    expect(resolvePrompt('translate', 'Vietnamese', undefined, 'speech')).toBe(SPEECH_PREAMBLE + base);
  });
});
```
(export `resolvePrompt` from the service file — it is module-level today; add `export` and import in spec together with `SPEECH_PREAMBLE`, `TONE_PROMPTS`.)

- [ ] **Step 2: Run** `cd backend && npm run test -- rewrite.service` — Expected: FAIL (SPEECH_PREAMBLE undefined).

- [ ] **Step 3: Implement**

`tones.ts` additions:
```typescript
export const INPUT_KIND_IDS = ['typed', 'speech'] as const;
export type InputKind = (typeof INPUT_KIND_IDS)[number];
/** Prepended to the tone prompt when the client marks input as dictated speech. */
export const SPEECH_PREAMBLE =
  'The input is dictated speech: remove filler words and false starts, restore punctuation and casing, keep the meaning and language. ';
```

`rewrite.dto.ts` addition (below `source_language`):
```typescript
  @ApiPropertyOptional({ example: 'speech', enum: INPUT_KIND_IDS, description: 'Marks dictated input; speech adds a cleanup preamble to the prompt' })
  @IsOptional()
  @IsIn(INPUT_KIND_IDS as unknown as string[])
  input_kind?: string;
```

`rewrite.service.ts` — extend `resolvePrompt` (keep existing body, wrap return):
```typescript
export function resolvePrompt(tone: string, targetLanguage?: string, sourceLanguage?: string, inputKind?: string): string | null {
  const base = /* existing body unchanged, assigned instead of returned */;
  if (base && inputKind === 'speech') return SPEECH_PREAMBLE + base;
  return base;
}
```
Thread `inputKind` through `rewrite(...)`, `trialRewrite(...)`, `callAI(...)` as a new trailing optional parameter; controller passes `dto.input_kind` in both handlers.

- [ ] **Step 4: Run** `npm run test` — Expected: full suite PASS. Also `npx tsc --noEmit`.

- [ ] **Step 5: Commit** `feat(backend): optional input_kind on /rewrite — speech preamble (VOICE-001..003)`

---

### Task 3: Go parity path — `input_kind`

**Files:**
- Modify: `backend-rewrite-go/internal/rewrite/parity/validate.go`
- Modify: `backend-rewrite-go/internal/rewrite/parity/prompts.go`
- Modify: `backend-rewrite-go/internal/rewrite/parity/service.go` (thread value to `ResolvePrompt` call sites)
- Test: `backend-rewrite-go/internal/rewrite/parity/prompts_test.go`, `validate_test.go`

**Interfaces:**
- Consumes: `SPEECH_PREAMBLE` string from Task 2 — copy byte-for-byte as `SpeechPreamble`.
- Produces: `ResolvePrompt(tone, targetLanguage, sourceLanguage, inputKind string) string` (4th param added; existing callers updated in same task).

- [ ] **Step 1: Failing tests**

`prompts_test.go`:
```go
func TestResolvePromptSpeechPreamble(t *testing.T) {
	base := ResolvePrompt("polished", "", "", "")
	got := ResolvePrompt("polished", "", "", "speech")
	if got != SpeechPreamble+base {
		t.Fatalf("speech prompt = %q, want preamble+base", got)
	}
	if ResolvePrompt("polished", "", "", "typed") != base {
		t.Fatalf("typed must not change prompt")
	}
}
```
`validate_test.go` (mirror existing message-assembly test style in that file):
```go
func TestValidateRewriteInputKind(t *testing.T) {
	_, _, _, _, _, msg := validateRewrite([]byte(`{"text":"hi","tone":"simple","input_kind":"banana"}`))
	want := "input_kind must be one of the following values: typed, speech"
	if msg != want {
		t.Fatalf("msg = %q, want %q", msg, want)
	}
	_, _, _, _, kind, msg := validateRewrite([]byte(`{"text":"hi","tone":"simple","input_kind":"speech"}`))
	if msg != "" || kind != "speech" {
		t.Fatalf("valid speech rejected: kind=%q msg=%q", kind, msg)
	}
}
```
(`validateRewrite` gains a 5th return `inputKind string` — adjust existing callers/tests in this task.)

- [ ] **Step 2: Run** `go test ./internal/rewrite/... -count=1` — Expected: FAIL (signature).

- [ ] **Step 3: Implement**
  - `prompts.go`: `const SpeechPreamble = "The input is dictated speech: remove filler words and false starts, restore punctuation and casing, keep the meaning and language. "` + 4th param on `ResolvePrompt`, `if p != "" && inputKind == "speech" { return SpeechPreamble + p }` applied to every non-empty return (grammar_check, translate, tone map).
  - `validate.go`: add `"input_kind"` to the `known` whitelist map; optional-field validation mirroring `target_language` pattern but with the IsIn message when present and not in {`typed`,`speech`} (non-string also → same IsIn message, matching class-validator behavior for @IsIn on non-strings).
  - `service.go`: pass the validated value into `ResolvePrompt`.

- [ ] **Step 4: Run** `go test ./... -race -count=1 && gofmt -l . && go vet ./...` — Expected: PASS, no gofmt output.

- [ ] **Step 5: Commit** `feat(go): parity input_kind on /rewrite + /rewrite/trial (VOICE-004)`

---

### Task 4: Go native `/v1/rewrite` — `input_kind`

**Files:**
- Modify: `backend-rewrite-go/internal/rewrite/domain/input_kind.go` (create)
- Modify: `backend-rewrite-go/internal/rewrite/domain/rewrite_request.go`
- Modify: `backend-rewrite-go/internal/rewrite/transport/handler_rewrite.go` (rewriteBody + NewRewriteRequest call)
- Modify: the adapter call site that builds the prompt from `req.Tone()` (follows `ResolvePrompt`; pass `req.InputKind().String()`)
- Test: `backend-rewrite-go/internal/rewrite/domain/input_kind_test.go`

**Interfaces:**
- Produces: `domain.InputKind` (`InputKindTyped Tone-style const "typed"`, `InputKindSpeech "speech"`), `ParseInputKind(s string) (InputKind, error)` — empty string → `InputKindTyped` (default), unknown → `ErrInvalidInput`; `RewriteRequest.InputKind()` getter; `NewRewriteRequest(text, toneStr, lang, inputKindStr string)`.

- [ ] **Step 1: Failing test** (`input_kind_test.go`, mirror `tone.go` test style):
```go
func TestParseInputKind(t *testing.T) {
	if k, err := ParseInputKind(""); err != nil || k != InputKindTyped {
		t.Fatalf("empty must default typed, got %v %v", k, err)
	}
	if k, err := ParseInputKind("speech"); err != nil || k != InputKindSpeech {
		t.Fatalf("speech: %v %v", k, err)
	}
	if _, err := ParseInputKind("banana"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("banana must be ErrInvalidInput")
	}
}
```
- [ ] **Step 2: Run** — Expected: FAIL (undefined).
- [ ] **Step 3: Implement** `input_kind.go` mirroring `tone.go` exactly (type, consts, Parse, String). Add `inputKind InputKind` field + getter to `RewriteRequest`; `NewRewriteRequest` parses it (4th param). `rewriteBody` gains `InputKind string \`json:"input_kind,omitempty"\``; `parseBody` passes it. Update ALL existing `NewRewriteRequest(` call sites (tests included) with `""` for the new param.
- [ ] **Step 4: Run** full Go suite — PASS.
- [ ] **Step 5: Commit** `feat(go): input_kind on native /v1/rewrite streaming path`

---

### Task 5: Shadow-gate fixtures

**Files:**
- Create: `backend-rewrite-go/cmd/shadowdiff/fixtures/rewrite/rewrite_input_kind_speech.json`
- Create: `backend-rewrite-go/cmd/shadowdiff/fixtures/rewrite/rewrite_input_kind_invalid.json`

- [ ] **Step 1: Add fixtures** (copy `rewrite_jwt_ok.json` shape):
```json
{
  "name": "rewrite_input_kind_speech",
  "method": "POST",
  "path": "/rewrite",
  "headers": { "Authorization": "Bearer {{user_token}}", "Content-Type": "application/json" },
  "body": { "text": "um so hello world", "tone": "simple", "input_kind": "speech" },
  "ignore_value_of": ["rewritten_text", "usage_today"]
}
```
```json
{
  "name": "rewrite_input_kind_invalid",
  "method": "POST",
  "path": "/rewrite",
  "headers": { "Authorization": "Bearer {{user_token}}", "Content-Type": "application/json" },
  "body": { "text": "hello", "tone": "simple", "input_kind": "banana" },
  "ignore_value_of": []
}
```
- [ ] **Step 2: Run local gate** per `backend-rewrite-go` gate runbook (Node + Go up locally, run shadowdiff). Expected: both new rows PASS (invalid case: identical 400 body from both).
- [ ] **Step 3: Commit** `test(go): shadow-gate fixtures for input_kind (VOICE-004)`

---

### Task 6: Kotlin `InputKind` + BackendClient wire-through

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/InputKind.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/BackendClient.kt` (`rewrite` gains `inputKind: InputKind = InputKind.TYPED`)
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/InputKindTest.kt`

**Interfaces:**
- Produces: `enum class InputKind { TYPED, SPEECH; val apiValue: String }`; `BackendClient.rewrite(text, tone, settings, inputKind = InputKind.TYPED, onResult)` — JSON body gains `"input_kind"` ONLY when `inputKind != TYPED` (absent = today's bytes, older backends unaffected).

- [ ] **Step 1: Failing test**
```kotlin
class InputKindTest {
    @Test fun apiValues() {
        assertEquals("typed", InputKind.TYPED.apiValue)
        assertEquals("speech", InputKind.SPEECH.apiValue)
    }
}
```
- [ ] **Step 2: Run** `./gradlew :app:testDebugUnitTest --tests '*InputKindTest*'` — FAIL (unresolved).
- [ ] **Step 3: Implement** (mirror `Tone.kt` style):
```kotlin
package com.draftright.keyboard

/** How the user produced the text. SPEECH adds a server-side cleanup preamble. */
enum class InputKind {
    TYPED, SPEECH;
    val apiValue: String get() = when (this) {
        TYPED -> "typed"
        SPEECH -> "speech"
    }
}
```
In `BackendClient.rewrite` body construction, after the `target_language` line:
```kotlin
    if (inputKind != InputKind.TYPED) put("input_kind", inputKind.apiValue)
```
- [ ] **Step 4: Run** full `./gradlew :app:testDebugUnitTest` — PASS.
- [ ] **Step 5: Commit** `feat(android): InputKind enum + input_kind wire-through (VOICE-005)`

---

### Task 7: `sttLocale` on LanguagePack

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/LanguagePack.kt`
- Modify: pack objects under `.../keyboard/lang/` — English pack `"en-US"`, Vietnamese pack `"vi-VN"`, all others untouched (inherit null default)
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/LanguagePackSttLocaleTest.kt`

**Interfaces:**
- Produces: `LanguagePack.sttLocale: String?` — default `null` = voice unsupported (mic key hidden). BCP-47 tag consumed by `RecognizerIntent.EXTRA_LANGUAGE`.

- [ ] **Step 1: Failing test** (TC: VOICE-010)
```kotlin
class LanguagePackSttLocaleTest {
    @Test fun localesPerPack() {
        assertEquals("en-US", EnglishPack.sttLocale)
        assertEquals("vi-VN", VietnamesePack.sttLocale)
        assertNull(JapanesePack.sttLocale)
    }
}
```
(use the actual pack object names found in `.../keyboard/lang/` — adjust imports to match `LanguageRegistryTest.kt`.)
- [ ] **Step 2: Run** — FAIL.
- [ ] **Step 3: Implement** — in the interface, below `longPressAccents`:
```kotlin
    /** BCP-47 tag for on-device speech recognition; null = voice input unavailable for this pack. */
    val sttLocale: String? get() = null
```
Override in English + Vietnamese packs only.
- [ ] **Step 4: Run** full unit tests — PASS.
- [ ] **Step 5: Commit** `feat(android): sttLocale per language pack (VOICE-010)`

---

### Task 8: `VoiceInput` port + `VoiceSessionController` state machine

**Files:**
- Create: `.../keyboard/voice/VoiceInput.kt` (interface + config)
- Create: `.../keyboard/voice/SpeechRecognizerVoiceInput.kt` (real impl)
- Create: `.../keyboard/voice/VoiceSessionController.kt`
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/voice/VoiceSessionControllerTest.kt`

**Interfaces:**
- Produces (consumed by Task 10):
```kotlin
interface VoiceInput {
    fun start(localeTag: String, listener: Listener)
    fun stop()      // finish listening, deliver final result
    fun cancel()    // abandon, no result
    interface Listener {
        fun onPartial(text: String)
        fun onFinal(text: String)
        fun onError(error: VoiceError)
    }
}
enum class VoiceError { PERMISSION_DENIED, RECOGNIZER_UNAVAILABLE, NO_SPEECH, NETWORK, OTHER }
object VoiceConfig {
    const val LISTEN_TIMEOUT_MS = 30_000L
    const val PARTIAL_DEBOUNCE_MS = 150L
}
sealed class VoiceOutcome {
    data class Polished(val text: String) : VoiceOutcome()
    data class Raw(val text: String, val hint: String?) : VoiceOutcome()
    object Nothing_ : VoiceOutcome()
}
class VoiceSessionController(
    private val voice: VoiceInput,
    private val polish: (text: String, onResult: (Result<String>) -> Unit) -> Unit,
    private val onState: (State) -> Unit,
    private val onOutcome: (VoiceOutcome) -> Unit,
) {
    enum class State { IDLE, LISTENING, PROCESSING }
    fun startSession(localeTag: String, rawMode: Boolean)
    fun cancelSession()
    fun onPartialText(cb: (String) -> Unit)  // candidate-bar live transcript
}
```
- Controller rules: final transcript + `rawMode` → `Raw` outcome, no polish call (VOICE-009). Polish success → `Polished`. Polish failure of ANY kind → `Raw(transcript, hint)` (VOICE-007, golden rule). `cancelSession` in LISTENING → `Nothing_` (VOICE-008). Recognizer `onError` → `Nothing_` + state IDLE (nothing to preserve — no transcript existed).

- [ ] **Step 1: Failing tests** — `VoiceSessionControllerTest.kt` with `FakeVoiceInput` (records listener, exposes `emitPartial/emitFinal/emitError`) and a scripted `polish` lambda. Tests: happy path state sequence + `Polished` outcome; each polish failure → `Raw` with same transcript; cancel → `Nothing_`, no polish invocation; rawMode → no polish invocation; states reported in order IDLE→LISTENING→PROCESSING→IDLE.

```kotlin
class VoiceSessionControllerTest {
    private class FakeVoiceInput : VoiceInput {
        var listener: VoiceInput.Listener? = null
        override fun start(localeTag: String, listener: VoiceInput.Listener) { this.listener = listener }
        override fun stop() {}
        override fun cancel() { listener = null }
    }

    @Test fun polishFailureCommitsRawTranscript() {  // TC: VOICE-007
        val fake = FakeVoiceInput()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.failure(Exception("timeout"))) },
            onState = {}, onOutcome = { outcomes.add(it) })
        c.startSession("vi-VN", rawMode = false)
        fake.listener!!.onFinal("xin chào ừm thế giới")
        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Raw("xin chào ừm thế giới", hint = "Polish failed — inserted your words as spoken")), outcomes)
    }
    // + happyPath, cancelInsertsNothing (VOICE-008), rawModeSkipsPolish (VOICE-009), stateSequence tests in same style
}
```
- [ ] **Step 2: Run** — FAIL (unresolved classes).
- [ ] **Step 3: Implement** the three files. `SpeechRecognizerVoiceInput` wraps `android.speech.SpeechRecognizer` (`createSpeechRecognizer`, `RecognizerIntent.ACTION_RECOGNIZE_SPEECH` with `EXTRA_LANGUAGE = localeTag`, `EXTRA_PARTIAL_RESULTS = true`), maps `SpeechRecognizer.ERROR_*` codes to `VoiceError` values; not unit-tested (thin adapter, device-only), controller carries all logic.
- [ ] **Step 4: Run** full unit tests — PASS.
- [ ] **Step 5: Commit** `feat(android): VoiceInput port + VoiceSessionController with raw-fallback golden rule (VOICE-006..009)`

---

### Task 9: Generic permission trampoline

**Files:**
- Create: `.../keyboard/RequestPermissionActivity.kt`
- Modify: `DraftRightMobile/android/app/src/main/AndroidManifest.xml`

**Interfaces:**
- Produces: `RequestPermissionActivity.launch(context, Manifest.permission.RECORD_AUDIO)`; broadcasts result via `SharedSettings` flag re-read by the IME on `onWindowShown`.

- [ ] **Step 1: Implement** transparent activity (`Theme.Translucent.NoTitleBar`, `excludeFromRecents`, `taskAffinity=""`):
```kotlin
class RequestPermissionActivity : Activity() {
    companion object {
        private const val EXTRA_PERMISSION = "permission"
        fun launch(context: Context, permission: String) {
            context.startActivity(Intent(context, RequestPermissionActivity::class.java).apply {
                putExtra(EXTRA_PERMISSION, permission)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            })
        }
    }
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val permission = intent.getStringExtra(EXTRA_PERMISSION) ?: return finish()
        requestPermissions(arrayOf(permission), 1)
    }
    override fun onRequestPermissionsResult(code: Int, perms: Array<out String>, results: IntArray) = finish()
}
```
Manifest: `<uses-permission android:name="android.permission.RECORD_AUDIO" />` in the existing permission block + activity entry beside `ProcessTextActivity`'s declaration pattern.
- [ ] **Step 2: Build check** `./gradlew :app:assembleDebug` — Expected: BUILD SUCCESSFUL.
- [ ] **Step 3: Commit** `feat(android): generic RequestPermissionActivity + RECORD_AUDIO (VOICE-011)`

---

### Task 10: Mic key in ToolbarView + IME wiring (single commit path)

**Files:**
- Modify: `.../keyboard/ToolbarView.kt` (mic button, follows `toneButtons` pattern: Material icon `mic`, brand-blue tint, pulse while LISTENING, spinner while PROCESSING — reuse `setLoading` visual pattern)
- Modify: `.../keyboard/DraftRightIME.kt`
- Test: covered by Task 8 controller tests; IME layer stays thin (no new unit tests — device-bound)

**Interfaces:**
- Consumes: `VoiceSessionController`, `InputKind`, `sttLocale`, `RequestPermissionActivity`.
- Produces (in `DraftRightIME`): single `commitVoiceResult(outcome: VoiceOutcome)`.

- [ ] **Step 1: ToolbarView** — add after tone buttons, only when `onMicTapped != null` passed by IME (IME passes null when `currentPack.sttLocale == null` or `!SpeechRecognizer.isRecognitionAvailable(this)`): tap → `onMicTapped(false)`, long-press (reuse `SpecialKeys.LONG_PRESS_MS`) → `onMicTapped(true)` (raw mode). Expose `setVoiceState(state)` to drive pulse/spinner (mirror `setLoading`/`clearLoading`).
- [ ] **Step 2: IME wiring** —
```kotlin
private fun handleMicTapped(rawMode: Boolean) {
    val locale = currentPack.sttLocale ?: return
    if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
        RequestPermissionActivity.launch(this, Manifest.permission.RECORD_AUDIO); return
    }
    voiceSession.startSession(locale, rawMode)
}

private fun commitVoiceResult(outcome: VoiceOutcome) = mainHandler.post {
    toolbar?.setVoiceState(VoiceSessionController.State.IDLE)
    when (outcome) {
        is VoiceOutcome.Polished -> currentInputConnection?.commitText(outcome.text, 1)
        is VoiceOutcome.Raw -> {
            currentInputConnection?.commitText(outcome.text, 1)
            outcome.hint?.let { showBanner(it) }
        }
        VoiceOutcome.Nothing_ -> Unit
    }
}
```
`voiceSession` built in `onCreate` with `SpeechRecognizerVoiceInput(this)` and `polish = { text, cb -> aiClient.rewrite(text, currentTone, settings, InputKind.SPEECH, cb) }`; partials → candidate bar via existing candidate-bar view setter; controller `onState` → `toolbar?.setVoiceState`. Back key while LISTENING → `voiceSession.cancelSession()`.
- [ ] **Step 3: Run** `./gradlew :app:testDebugUnitTest && ./gradlew :app:assembleDebug` — PASS/SUCCESS.
- [ ] **Step 4: Commit** `feat(android): mic key — dictate → AI-polish in selected tone (VOICE-006..009,011)`

---

### Task 11: Manual E2E on A52 + wrap-up

- [ ] **Step 1:** `./gradlew :app:installDebug` on A52 (wireless ADB per `reference_admin_credentials` memory). Run VOICE-011 (grant + deny) and VOICE-012 (vi-VN dictation with fillers), plus EN happy path, airplane-mode polish failure → raw fallback, long-press raw. Record pass/fail per TC in the VOICE sheet's `Verified by` column notes.
- [ ] **Step 2:** Full local suites one last time: Node, Go (+ gofmt/vet), Android unit tests.
- [ ] **Step 3:** Commit any fixes; then use superpowers:finishing-a-development-branch (merge `feature/keyboard-voice-input-20260703` → develop `--no-ff`, deploy backend to TESTING first per pipeline, labels per issue workflow — file a GitHub issue for the feature if none exists yet and walk it through `status: developed` → testing → prod).

---

## Self-review notes

- Spec coverage: UX states (T8/T10), long-press raw (T8/T10), golden rule (T8), locale-per-pack (T7), enum layers (T2/T3/T4/T6), prompt SSOT server-side (T2/T3), whitelist parity + gate (T3/T5), trampoline (T9), test-cases-first (T1), manual E2E (T11). iOS = out of scope per spec.
- `resolvePrompt` export change (T2) is deliberate — the spec test imports it; keep the function module-level exported, no class move.
- T4 updates every existing `NewRewriteRequest` call site — compiler enforces; expect ~6 sites (handler, tests).
