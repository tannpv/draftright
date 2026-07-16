# Hold-to-talk Voice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the DraftRight Android keyboard mic press-and-hold: hold to record, release on the mic to insert the raw transcript, slide away and release to cancel.

**Architecture:** Reuse the existing `VoiceSessionController` state machine and `SpeechRecognizerVoiceInput`. Add one method (`finishSession`) for the release→commit path, extend recognizer silence tolerance so a held pause doesn't cut off, and replace the ToolbarView mic's tap/long-press wiring with a down/move/up gesture whose pure decision logic lives in a testable helper.

**Tech Stack:** Kotlin, Android `InputMethodService` + `SpeechRecognizer`, JUnit4 (JVM unit tests under `app/src/test/`).

## Global Constraints

- Android keyboard only (`DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/`). No iOS/desktop changes.
- Voice session always uses `rawMode = true` on this path — no AI polish. Tone buttons remain for post-hoc rewrite.
- Pure/JVM-testable logic must not import `android.*` (the composer/voice tests run under `app/src/test/`, no device).
- Run tests with: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:testDebugUnitTest --tests "<class>"`
- Spec: `docs/superpowers/specs/2026-07-16-hold-to-talk-voice-design.md`.

---

### Task 1: `finishSession()` on VoiceSessionController

Adds the release→commit path. The controller currently only starts and cancels; the tap flow relied on the recognizer auto-ending. Hold-to-talk needs an explicit "stop listening and commit whatever was said."

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/VoiceSessionController.kt`
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/voice/VoiceSessionControllerTest.kt`

**Interfaces:**
- Consumes: `VoiceInput.stop()` (already on the interface — "stop listening and let the recognizer deliver its final result").
- Produces: `VoiceSessionController.finishSession()` — call while a session is LISTENING; asks the recognizer to finalize. The final arrives via the existing `onFinal` listener → `VoiceOutcome.Raw(text, hint=null)` (because sessions on this path start with `rawMode=true`). No-op if not LISTENING.

- [ ] **Step 1: Add `stopCalls` to the test fake**

In `VoiceSessionControllerTest.kt`, the `FakeVoiceInput` class, add the counter and count it in `stop()`:

```kotlin
    private class FakeVoiceInput : VoiceInput {
        var listener: VoiceInput.Listener? = null
        var startCalls = 0
        var cancelCalls = 0
        var stopCalls = 0

        override fun start(localeTag: String, listener: VoiceInput.Listener) {
            startCalls++
            this.listener = listener
        }
        override fun stop() { stopCalls++ }   // keeps listener; real recognizer then fires onFinal
        override fun cancel() {
            cancelCalls++
            listener = null
        }
    }
```

- [ ] **Step 2: Write the failing tests**

Add to `VoiceSessionControllerTest.kt`:

```kotlin
    @Test fun `finishSession stops recognizer then commits raw final`() {
        val fake = FakeVoiceInput()
        val outcomes = mutableListOf<VoiceOutcome>()
        val c = VoiceSessionController(fake,
            polish = { _, cb -> cb(Result.success("should-not-run")) },
            onState = {}, onOutcome = { outcomes.add(it) })

        c.startSession("vi-VN", rawMode = true)
        c.finishSession()
        assertEquals(1, fake.stopCalls)          // release asks recognizer to finalize
        fake.listener!!.onFinal("xin chào")      // recognizer delivers the final

        assertEquals(listOf<VoiceOutcome>(VoiceOutcome.Raw("xin chào", hint = null)), outcomes)
    }

    @Test fun `finishSession when idle is a no-op`() {
        val fake = FakeVoiceInput()
        val c = VoiceSessionController(fake, polish = { _, _ -> }, onState = {}, onOutcome = {})
        c.finishSession()
        assertEquals(0, fake.stopCalls)
    }
```

- [ ] **Step 3: Run tests, verify they fail**

Run: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.voice.VoiceSessionControllerTest"`
Expected: FAIL — `finishSession` unresolved reference.

- [ ] **Step 4: Implement `finishSession()`**

In `VoiceSessionController.kt`, add after `cancelSession()`:

```kotlin
    /**
     * Release on the mic (hold-to-talk): stop the recognizer and let it
     * deliver the final transcript through the existing onFinal path (→
     * VoiceOutcome.Raw for a rawMode session). No-op unless LISTENING — a
     * pause longer than the recognizer's silence window may already have
     * ended the session, in which case there is nothing to finalize.
     * Synchronized on the same lock as cancelSession()/handleFinal so a
     * release racing an auto-delivered final can't double-emit.
     */
    fun finishSession() {
        synchronized(this) {
            if (state != State.LISTENING) return
            voice.stop()
        }
    }
```

- [ ] **Step 5: Run tests, verify they pass**

Run: same command as Step 3.
Expected: PASS (all VoiceSessionControllerTest tests green).

- [ ] **Step 6: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/VoiceSessionController.kt DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/voice/VoiceSessionControllerTest.kt
git commit -m "feat(voice): finishSession() for hold-to-talk release-to-commit"
```

---

### Task 2: MicHoldGesture pure helper (cancel-arm decision)

The slide-away decision is pure geometry — extract it so it's unit-tested without Android touch events.

**Files:**
- Create: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/MicHoldGesture.kt`
- Test: `DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/voice/MicHoldGestureTest.kt`

**Interfaces:**
- Produces: `object MicHoldGesture { fun isCancelArmed(dxPx: Float, dyPx: Float, slopPx: Float): Boolean }` — `dxPx`/`dyPx` are the finger's displacement from the touch-down point (current − down). Returns true when the finger has slid far enough to mean "cancel". Also exposes `const val DEFAULT_SLOP_DP = 32` and `const val ARM_MS = 180L` used by the view.

- [ ] **Step 1: Write the failing test**

```kotlin
package com.draftright.keyboard.voice

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MicHoldGestureTest {
    private val slop = 96f

    @Test fun `no movement is not cancel-armed`() {
        assertFalse(MicHoldGesture.isCancelArmed(0f, 0f, slop))
    }

    @Test fun `small jitter within slop is not cancel-armed`() {
        assertFalse(MicHoldGesture.isCancelArmed(20f, -20f, slop))
    }

    @Test fun `sliding up past slop is cancel-armed`() {
        assertTrue(MicHoldGesture.isCancelArmed(0f, -200f, slop))
    }

    @Test fun `sliding left past slop is cancel-armed`() {
        assertTrue(MicHoldGesture.isCancelArmed(-200f, 0f, slop))
    }

    @Test fun `sliding down within field is not cancel-armed at small distance`() {
        assertFalse(MicHoldGesture.isCancelArmed(0f, 40f, slop))
    }
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.voice.MicHoldGestureTest"`
Expected: FAIL — `MicHoldGesture` unresolved.

- [ ] **Step 3: Implement the helper**

```kotlin
package com.draftright.keyboard.voice

import kotlin.math.hypot

/**
 * Pure geometry for the hold-to-talk mic gesture. Kept android-free so it is
 * unit-testable without touch events. The view feeds displacement from the
 * touch-down point; this decides whether the finger has slid far enough to
 * mean "cancel" (slide-away-to-cancel, Zalo/WhatsApp style).
 */
object MicHoldGesture {
    /** Delay before a hold starts recording; a shorter press is a no-op tap. */
    const val ARM_MS = 180L
    /** Default slide-to-cancel threshold, in dp (the view converts to px). */
    const val DEFAULT_SLOP_DP = 32

    /** True once the finger has slid [slopPx] or more from the down point. */
    fun isCancelArmed(dxPx: Float, dyPx: Float, slopPx: Float): Boolean =
        hypot(dxPx, dyPx) >= slopPx
}
```

- [ ] **Step 4: Run test, verify it passes**

Run: same as Step 2. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/MicHoldGesture.kt DraftRightMobile/android/app/src/test/kotlin/com/draftright/keyboard/voice/MicHoldGestureTest.kt
git commit -m "feat(voice): MicHoldGesture pure cancel-arm geometry"
```

---

### Task 3: Recognizer keeps listening during a held pause

Extend `SpeechRecognizerVoiceInput` so a mid-sentence pause doesn't auto-end the session before the user releases.

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/SpeechRecognizerVoiceInput.kt`
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/VoiceInput.kt` (add `HOLD_SILENCE_MS` to `VoiceConfig`)

**Interfaces:**
- Consumes: `RecognizerIntent.EXTRA_SPEECH_INPUT_COMPLETE_SILENCE_LENGTH_MILLIS`, `EXTRA_SPEECH_INPUT_POSSIBLY_COMPLETE_SILENCE_LENGTH_MILLIS`, `EXTRA_SPEECH_INPUT_MINIMUM_LENGTH_MILLIS`.
- Produces: nothing new to callers; behaviour only.

> **No unit test:** this touches the real `android.speech` intent, which the JVM test suite deliberately never imports. Verified manually on device in Task 5.

- [ ] **Step 1: Add the constant**

In `VoiceInput.kt`, inside `object VoiceConfig`, add:

```kotlin
    /** Silence window while holding the mic — long enough that a pause
     *  mid-sentence doesn't auto-end the session before the user releases. */
    const val HOLD_SILENCE_MS = 10_000L
```

- [ ] **Step 2: Set the silence extras on the intent**

In `SpeechRecognizerVoiceInput.start()`, extend the `intent` builder:

```kotlin
        val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
            putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            putExtra(RecognizerIntent.EXTRA_LANGUAGE, localeTag)
            putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
            // Hold-to-talk: keep listening through pauses until the user
            // releases (which calls stop()). These are hints; some OEM
            // recognizers ignore them — the 30 s LISTEN_TIMEOUT_MS backstop
            // and the release-to-stop path still bound the session.
            putExtra(RecognizerIntent.EXTRA_SPEECH_INPUT_COMPLETE_SILENCE_LENGTH_MILLIS, VoiceConfig.HOLD_SILENCE_MS)
            putExtra(RecognizerIntent.EXTRA_SPEECH_INPUT_POSSIBLY_COMPLETE_SILENCE_LENGTH_MILLIS, VoiceConfig.HOLD_SILENCE_MS)
            putExtra(RecognizerIntent.EXTRA_SPEECH_INPUT_MINIMUM_LENGTH_MILLIS, 1_000L)
        }
```

- [ ] **Step 3: Compile check**

Run: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:compileDebugKotlin`
Expected: BUILD SUCCESSFUL.

- [ ] **Step 4: Commit**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/SpeechRecognizerVoiceInput.kt DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/voice/VoiceInput.kt
git commit -m "feat(voice): extend recognizer silence window for hold-to-talk"
```

---

### Task 4: ToolbarView hold gesture + VoiceHoldListener

Replace the mic's tap/long-press wiring with a down/arm/move/up/cancel gesture. The IME receives lifecycle callbacks instead of `onMicTapped(Boolean)`.

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ToolbarView.kt`

**Interfaces:**
- Produces: nested `interface ToolbarView.VoiceHoldListener { fun onHoldStart(); fun onCancelArmedChanged(armed: Boolean); fun onHoldEnd(cancelled: Boolean) }`. ToolbarView's constructor param changes from `onMicTapped: ((Boolean) -> Unit)? = null` to `voiceHold: VoiceHoldListener? = null` (null still hides the mic).
- Consumes: `MicHoldGesture.ARM_MS`, `MicHoldGesture.DEFAULT_SLOP_DP`, `MicHoldGesture.isCancelArmed`.

> **No unit test:** touch wiring on a `View`; the testable geometry is already covered by `MicHoldGestureTest` (Task 2). Verified manually in Task 5.

- [ ] **Step 1: Change the constructor param**

Replace the `onMicTapped` property (around line 30) and its KDoc:

```kotlin
    /**
     * Voice hold-to-talk callbacks, or null to hide the mic entirely. IME
     * passes null when the active pack has no [LanguagePack.sttLocale] or the
     * device has no speech recognizer (capability check at the call site).
     */
    private val voiceHold: VoiceHoldListener? = null
```

- [ ] **Step 2: Add the listener interface**

Inside the `ToolbarView` class body (top, near the property declarations):

```kotlin
    /** Hold-to-talk lifecycle for the mic button. */
    interface VoiceHoldListener {
        /** Finger held past the arm threshold — begin recording. */
        fun onHoldStart()
        /** The slide-away-to-cancel state changed (update the hint). */
        fun onCancelArmedChanged(armed: Boolean)
        /** Finger lifted — [cancelled] true = discard, false = insert. */
        fun onHoldEnd(cancelled: Boolean)
    }
```

- [ ] **Step 3: Replace the mic touch handler**

Replace the whole `onMicTapped?.let { tapped -> ... }` block (the `mic` ImageView build + its `setOnTouchListener` + `micButton = mic; row.addView(mic)`) with:

```kotlin
        voiceHold?.let { hold ->
            val mic = ImageView(context).apply {
                setImageResource(KeyIcons.resolve("mic"))
                imageTintList = iconTint
                scaleType = ImageView.ScaleType.CENTER_INSIDE
                setBackgroundResource(android.R.drawable.btn_default)
                val pad = dpToPx(9)
                setPadding(pad, pad, pad, pad)
                layoutParams = LayoutParams(dpToPx(44), dpToPx(40)).apply { marginEnd = dpToPx(2) }
                contentDescription = "Hold to talk"
            }

            val slopPx = dpToPx(MicHoldGesture.DEFAULT_SLOP_DP).toFloat()
            var downX = 0f
            var downY = 0f
            var started = false        // arm fired → recording
            var cancelArmed = false
            val armRunnable = Runnable {
                started = true
                cancelArmed = false
                hold.onHoldStart()
            }

            mic.setOnTouchListener { v, event ->
                when (event.action) {
                    MotionEvent.ACTION_DOWN -> {
                        downX = event.rawX; downY = event.rawY
                        started = false; cancelArmed = false
                        v.postDelayed(armRunnable, MicHoldGesture.ARM_MS)
                        true
                    }
                    MotionEvent.ACTION_MOVE -> {
                        if (started) {
                            val armed = MicHoldGesture.isCancelArmed(
                                event.rawX - downX, event.rawY - downY, slopPx)
                            if (armed != cancelArmed) {
                                cancelArmed = armed
                                hold.onCancelArmedChanged(armed)
                            }
                        }
                        true
                    }
                    MotionEvent.ACTION_UP -> {
                        v.removeCallbacks(armRunnable)
                        if (started) hold.onHoldEnd(cancelled = cancelArmed)
                        // else: released before arm → treated as a no-op tap
                        started = false; cancelArmed = false
                        true
                    }
                    MotionEvent.ACTION_CANCEL -> {
                        v.removeCallbacks(armRunnable)
                        if (started) hold.onHoldEnd(cancelled = true)
                        started = false; cancelArmed = false
                        true
                    }
                    else -> false
                }
            }

            micButton = mic
            row.addView(mic)
        }
```

- [ ] **Step 4: Compile check**

Run: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:compileDebugKotlin`
Expected: FAIL — `DraftRightIME` still passes `onMicTapped=`. That is fixed in Task 5; this task's file compiles in isolation but the module won't link until Task 5. Proceed to Task 5 before committing, OR temporarily expect the DraftRightIME error only.

- [ ] **Step 5: Commit (with Task 5, since the caller must change together)**

Deferred — commit at the end of Task 5 so the module compiles.

---

### Task 5: Wire the IME to the hold gesture

Point `DraftRightIME` at the new listener; start a raw session on hold, finish on release, cancel on slide-away; drive the "slide to cancel" hint.

**Files:**
- Modify: `DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`

**Interfaces:**
- Consumes: `ToolbarView.VoiceHoldListener`, `VoiceSessionController.startSession`, `.finishSession`, `.cancelSession`, `CandidateBarView.setPartialTranscript`.

- [ ] **Step 1: Replace the ToolbarView construction**

In `onCreateInputView()`, the `micHandler` block builds a `((Boolean)->Unit)?` and passes `onMicTapped = micHandler`. Replace with a `VoiceHoldListener?` built the same way (capability check unchanged):

```kotlin
        val voiceHold: ToolbarView.VoiceHoldListener? =
            if (controller!!.current.sttLocale != null && SpeechRecognizer.isRecognitionAvailable(this)) {
                object : ToolbarView.VoiceHoldListener {
                    override fun onHoldStart() = handleVoiceHoldStart()
                    override fun onCancelArmedChanged(armed: Boolean) = handleVoiceCancelArmed(armed)
                    override fun onHoldEnd(cancelled: Boolean) = handleVoiceHoldEnd(cancelled)
                }
            } else null

        val tb = ToolbarView(this,
            onToneSelected = { tone -> handleToneSelected(tone) },
            onUndo = { handleUndo() },
            voiceHold = voiceHold
        )
```

- [ ] **Step 2: Replace `handleMicTapped` with the hold handlers**

Delete `private fun handleMicTapped(rawMode: Boolean) { ... }` and add:

```kotlin
    private fun handleVoiceHoldStart() {
        if (voiceState != VoiceSessionController.State.IDLE) return
        val locale = controller?.current?.sttLocale ?: return
        if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
            // Hold-to-talk always records raw; persist for the permission
            // trampoline's resume path (kept for parity with the prior flow).
            settings.pendingVoiceRawMode = true
            settings.voicePermissionRequested = true
            RequestPermissionActivity.launch(this, Manifest.permission.RECORD_AUDIO)
            return
        }
        voiceSession.startSession(locale, rawMode = true)
    }

    private fun handleVoiceCancelArmed(armed: Boolean) = mainHandler.post {
        candidateBar?.setPartialTranscript(
            if (armed) "◀ release to cancel" else null
        )
    }

    private fun handleVoiceHoldEnd(cancelled: Boolean) {
        if (cancelled) voiceSession.cancelSession() else voiceSession.finishSession()
    }
```

- [ ] **Step 3: Build the whole module (Tasks 4 + 5 link)**

Run: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:compileDebugKotlin`
Expected: BUILD SUCCESSFUL.

- [ ] **Step 4: Run the full voice + composer test suite (no regressions)**

Run: `cd DraftRightMobile/android && JAVA_HOME=/Library/Java/JavaVirtualMachines/jdk-17.jdk/Contents/Home ./gradlew :app:testDebugUnitTest --tests "com.draftright.keyboard.voice.*"`
Expected: BUILD SUCCESSFUL.

- [ ] **Step 5: Commit Tasks 4 + 5 together**

```bash
git add DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/ToolbarView.kt DraftRightMobile/android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt
git commit -m "feat(voice): hold-to-talk mic gesture (hold=record, release=insert, slide=cancel)"
```

---

### Task 6: On-device manual verification

Unit tests cover the state machine and gesture geometry; the touch + recognizer integration is device-only.

**Files:** none (verification).

- [ ] **Step 1: Build + install a release APK on the A52**

```bash
cd DraftRightMobile && flutter build apk --release && adb install -r build/app/outputs/flutter-apk/app-release.apk
adb shell ime set com.draftright.draftright_mobile.v2/com.draftright.keyboard.DraftRightIME
```

- [ ] **Step 2: Verify each behaviour (DraftRight keyboard, a VI/EN pack with STT)**

  - Hold mic → speak → release on mic → raw transcript inserted at cursor. ✅
  - Hold → speak → slide finger up/away (hint flips to "◀ release to cancel") → release → nothing inserted. ✅
  - Quick tap (no hold) → nothing recorded (brief no-op). ✅
  - Hold → speak with a ~2 s pause mid-sentence → not cut off before release. ✅
  - Hold → say nothing → release → nothing inserted, no error toast. ✅
  - First-ever hold with no mic permission → system permission prompt; after granting, next hold records. ✅

- [ ] **Step 3: If all pass, the feature is complete.** File any device-specific cutoff (Task 3 note) as a follow-up if a pause still truncates on this OEM.

---

## Notes for the implementer

- `settings.pendingVoiceRawMode` / `voicePermissionRequested` and the `RequestPermissionActivity` trampoline already exist from the tap flow — reuse, don't reinvent. The `onWindowShown` resume path that reads `pendingVoiceRawMode` continues to work (value is now always `true`).
- Do **not** remove `VoiceOutcome.Polished` / the `polish` wiring — other entry points (if any) and the raw-fallback contract still use it. Hold-to-talk simply never triggers polish (`rawMode=true`).
- `mainHandler` is the IME's main-thread `Handler` (already a field).
