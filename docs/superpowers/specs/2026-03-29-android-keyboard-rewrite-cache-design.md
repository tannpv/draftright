# Android Keyboard Rewrite Cache — Design Spec

**Date:** 2026-03-29
**Scope:** V1 Android keyboard extension (Kotlin) only
**Goal:** When user taps a tone, batch-fetch all 5 rewrite tones in parallel so subsequent tone taps return instantly from cache.

## Requirements

- Cache lives in the Kotlin keyboard extension (IME), not Flutter
- When a tone is tapped, fire all 5 rewrite tones (Simple, Natural, Polished, Concise, Technical) in parallel
- Translate is excluded from batching — always fires on demand
- Cache invalidates when the input text changes (per-text-input invalidation)
- Only one text entry cached at a time
- Spinner shows only on the tapped tone; other tones show no loading indicator
- Cached results display instantly when a previously-batched tone is tapped

## Architecture

### New File: `RewriteCache.kt`

In-memory cache with a single-entry design:

```
class RewriteCache {
    private var cachedText: String? = null
    private val results: MutableMap<Tone, String> = mutableMapOf()
    private var batchStarted: Boolean = false

    fun get(text: String, tone: Tone): String?
    fun put(text: String, tone: Tone, result: String)
    fun isBatchStarted(text: String): Boolean
    fun markBatchStarted(text: String)
    fun clear()
}
```

- `get(text, tone)` — returns cached result if text matches and tone exists, else null. If text doesn't match, clears cache and returns null.
- `put(text, tone, result)` — stores result. If text differs from cached text, clears first.
- `isBatchStarted(text)` — returns true if background batch was already kicked off for this text.
- `markBatchStarted(text)` — flags that batch is in progress for this text.
- `clear()` — wipes everything.

### Modified File: `DraftRightIME.kt`

Changes to `handleToneSelected(tone)`:

```
1. text = readFullText().trim()
2. cached = rewriteCache.get(text, tone)
3. IF cached != null:
     → showDiffSheet(text, cached) immediately, no spinner
     → RETURN
4. ELSE:
     → show spinner on tapped tone
     → fire OpenAIClient.rewrite(text, tone, settings) for tapped tone
     → on result: cache it, show diff sheet
5. IF !rewriteCache.isBatchStarted(text):
     → rewriteCache.markBatchStarted(text)
     → for each rewrite tone != tapped tone && != TRANSLATE:
         fire OpenAIClient.rewrite(text, otherTone, settings) in background
         on result: rewriteCache.put(text, otherTone, result) silently
```

### Unchanged Files

- `OpenAIClient.kt` — no changes, used as-is for each request
- `ToolbarView.kt` — no changes, spinner behavior unchanged
- `SharedSettings.kt` — no changes
- `Tone.kt` — no changes
- `QwertyKeyboardView.kt` — no changes

## Edge Cases

- **Text changes mid-batch:** Background requests complete but `put()` auto-discards them since text no longer matches.
- **Same tone tapped twice quickly:** First tap starts the request, second tap while loading should be ignored (existing `isEnabled = false` on toolbar handles this).
- **Empty text:** Handled by existing early return in `handleToneSelected`.
- **API errors on background tones:** Silently ignored — user only sees errors for the tone they tapped. They can tap the failed tone later for a fresh request.

## Token Cost Impact

- First tap: 5 API calls instead of 1 (batch)
- Subsequent taps on same text: 0 API calls (cache hit)
- Net saving depends on usage pattern: saves tokens if users try multiple tones on the same text, costs more if they only ever use one tone.
