# Android Keyboard Rewrite Cache — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a user taps a rewrite tone in the Android keyboard, batch-fetch all 5 rewrite tones in parallel so subsequent tone taps return instantly from cache.

**Architecture:** A new `RewriteCache` class stores results keyed by input text + tone. `DraftRightIME` checks the cache before making API calls, and kicks off background requests for the other 4 tones on first tap. Cache auto-invalidates when text changes.

**Tech Stack:** Kotlin, Android InputMethodService, existing OpenAIClient

**Important:** All work happens on the `v1-stable` branch in the DraftRightMobile submodule. Checkout with `cd DraftRightMobile && git checkout v1-stable`. Build with `flutter build apk --release` (never debug). Install with `adb install -r build/app/outputs/flutter-apk/app-release.apk`.

---

## File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `android/app/src/main/kotlin/com/draftright/keyboard/RewriteCache.kt` | In-memory single-entry cache: text → tone → result |
| Modify | `android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt` | Check cache before API call, fire background batch |

No changes to: `OpenAIClient.kt`, `ToolbarView.kt`, `SharedSettings.kt`, `Tone.kt`, `QwertyKeyboardView.kt`

---

### Task 1: Create RewriteCache class

**Files:**
- Create: `android/app/src/main/kotlin/com/draftright/keyboard/RewriteCache.kt`

- [ ] **Step 1: Create `RewriteCache.kt`**

```kotlin
package com.draftright.keyboard

class RewriteCache {
    private var cachedText: String? = null
    private val results: MutableMap<Tone, String> = mutableMapOf()
    private var batchStarted: Boolean = false

    /**
     * Returns cached rewrite result if text matches and tone exists.
     * If text doesn't match current cache, clears everything and returns null.
     */
    fun get(text: String, tone: Tone): String? {
        if (text != cachedText) {
            clear()
            return null
        }
        return results[tone]
    }

    /**
     * Stores a rewrite result. If text differs from cached text, clears first.
     */
    fun put(text: String, tone: Tone, result: String) {
        if (text != cachedText) {
            clear()
            cachedText = text
        }
        results[tone] = result
    }

    /**
     * Returns true if background batch was already started for this text.
     */
    fun isBatchStarted(text: String): Boolean {
        return text == cachedText && batchStarted
    }

    /**
     * Marks that a background batch has been kicked off for this text.
     */
    fun markBatchStarted(text: String) {
        if (text != cachedText) {
            clear()
            cachedText = text
        }
        batchStarted = true
    }

    /**
     * Wipes all cached data.
     */
    fun clear() {
        cachedText = null
        results.clear()
        batchStarted = false
    }
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd DraftRightMobile && cd android && ./gradlew compileDebugKotlin 2>&1 | tail -5`
Expected: `BUILD SUCCESSFUL`

- [ ] **Step 3: Commit**

```bash
git add android/app/src/main/kotlin/com/draftright/keyboard/RewriteCache.kt
git commit -m "feat: add RewriteCache class for keyboard rewrite result caching"
```

---

### Task 2: Integrate cache into DraftRightIME — cache check and store

**Files:**
- Modify: `android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt`

- [ ] **Step 1: Add cache field to DraftRightIME**

Add after the existing `private var originalText: String? = null` line:

```kotlin
    private val rewriteCache = RewriteCache()
```

- [ ] **Step 2: Replace `handleToneSelected` method**

Replace the entire `handleToneSelected` method with:

```kotlin
    private fun handleToneSelected(tone: Tone) {
        val text = readFullText().trim()
        if (text.isEmpty()) return

        if (settings.aiProvider == "openai" && settings.apiKey.isEmpty()) {
            showBanner("Open DraftRight app to set up API key")
            return
        }

        originalText = text

        // Check cache first
        val cached = rewriteCache.get(text, tone)
        if (cached != null) {
            showDiffSheet(text, cached)
            return
        }

        // Cache miss — fire request with spinner
        toolbar?.setLoading(tone)

        aiClient.rewrite(text, tone, settings) { result ->
            mainHandler.post {
                toolbar?.clearLoading()
                result.onSuccess { rewritten ->
                    rewriteCache.put(text, tone, rewritten)
                    showDiffSheet(text, rewritten)
                }
                result.onFailure { error ->
                    showBanner(error.message ?: "Rewrite failed")
                }
            }
        }

        // Kick off background batch for other rewrite tones (excluding Translate)
        if (!rewriteCache.isBatchStarted(text)) {
            rewriteCache.markBatchStarted(text)
            val rewriteTones = listOf(Tone.SIMPLE, Tone.NATURAL, Tone.POLISHED, Tone.CONCISE, Tone.TECHNICAL)
            for (otherTone in rewriteTones) {
                if (otherTone == tone) continue
                aiClient.rewrite(text, otherTone, settings) { result ->
                    result.onSuccess { rewritten ->
                        rewriteCache.put(text, otherTone, rewritten)
                    }
                    // Silently ignore errors for background tones
                }
            }
        }
    }
```

- [ ] **Step 3: Verify it compiles**

Run: `cd DraftRightMobile && cd android && ./gradlew compileDebugKotlin 2>&1 | tail -5`
Expected: `BUILD SUCCESSFUL`

- [ ] **Step 4: Commit**

```bash
git add android/app/src/main/kotlin/com/draftright/keyboard/DraftRightIME.kt
git commit -m "feat: integrate RewriteCache into keyboard — cache check, store, background batch"
```

---

### Task 3: Build, install, and test on device

**Files:** None changed — build and manual test only.

- [ ] **Step 1: Build release APK**

```bash
cd DraftRightMobile
flutter build apk --release
```

Expected: `✓ Built build/app/outputs/flutter-apk/app-release.apk`

- [ ] **Step 2: Install on Samsung A52**

```bash
adb -s 192.168.1.14:39385 install -r build/app/outputs/flutter-apk/app-release.apk
```

Note: ADB IP/port may have changed — check `adb devices` first. Re-pair if needed.

Expected: `Success`

- [ ] **Step 3: Set DraftRight as active keyboard**

```bash
adb -s 192.168.1.14:39385 shell ime set com.draftright.draftright_mobile/com.draftright.keyboard.DraftRightIME
```

- [ ] **Step 4: Manual test — cache miss (first tap)**

1. Open any text field, type "Hello world this is a test"
2. Tap the ✎ (Simple) tone button
3. Verify: spinner shows on Simple, diff sheet appears with rewritten text

- [ ] **Step 5: Manual test — cache hit (second tap)**

1. Dismiss the diff sheet (tap Cancel)
2. Tap the 💬 (Natural) tone button
3. Verify: result appears **instantly** with no spinner (was fetched in background)

- [ ] **Step 6: Manual test — cache invalidation**

1. Dismiss the diff sheet
2. Type additional text in the field (e.g., add " and more text")
3. Tap any tone button
4. Verify: spinner shows again (cache was invalidated because text changed)

- [ ] **Step 7: Manual test — Translate is not cached**

1. Type some text, tap ✎ (Simple) to trigger batch
2. Wait a moment for background fetches
3. Tap 🌐 (Translate)
4. Verify: spinner shows (Translate is always on-demand, not batched)

- [ ] **Step 8: Commit tag**

```bash
git tag -a v1.2 -m "DraftRight V1.2 — rewrite cache for Android keyboard"
```
