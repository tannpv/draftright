package com.draftright.keyboard.ime

import android.content.Context

/**
 * Process-wide hook used by [LanguagePack.candidateEngine] implementations
 * to reach Android resources without dragging a [Context] into the pack
 * interface itself. The IME service sets it once in onCreate (with the
 * application context, never an activity); packs read it lazily.
 *
 * Why a singleton instead of a constructor parameter: LanguagePack is an
 * `object` (Kotlin singleton) registered statically in LanguageRegistry,
 * so its API can't carry runtime state. The alternative — exposing
 * Context on the interface — bleeds Android types into the candidate
 * engine layer that we want to keep pure for the future Multiplatform /
 * iOS port.
 *
 * Per Rule #1: the seam stays narrow (one nullable get/set) so the rest
 * of the engine layer remains Android-agnostic.
 */
object ImeContext {
    @Volatile
    private var app: Context? = null

    /** Called once from [com.draftright.keyboard.DraftRightIME].onCreate. */
    fun attach(applicationContext: Context) {
        app = applicationContext.applicationContext
    }

    /** Returns null when the IME service hasn't bound yet (e.g. unit tests). */
    fun appOrNull(): Context? = app
}
