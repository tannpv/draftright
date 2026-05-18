package com.draftright.keyboard

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.inputmethodservice.InputMethodService
import android.os.Handler
import android.os.Looper
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.ExtractedTextRequest
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import com.draftright.keyboard.lang.EnglishLanguagePack

class DraftRightIME : InputMethodService(), KeyboardActionListener {

    private lateinit var settings: SharedSettings
    private val aiClient = BackendClient()
    private val mainHandler = Handler(Looper.getMainLooper())
    private var toolbar: ToolbarView? = null
    private var keyboard: QwertyKeyboardView? = null
    private var languageStrip: LanguageStripView? = null
    private var diffSheet: DiffSheetView? = null
    private var rootLayout: LinearLayout? = null
    private var originalText: String? = null

    // Step B: construct LanguageRegistry + KeyboardController but do NOT wire
    // any UI for them yet. Goal: prove that loading 7 LanguagePack singletons
    // + a KeyboardController in the IME process doesn't break the view.
    private val registry: LanguageRegistry by lazy {
        try {
            LanguageRegistry.PRODUCTION
        } catch (t: Throwable) {
            android.util.Log.e("DraftRightIME", "registry init failed", t)
            LanguageRegistry(listOf(EnglishLanguagePack))
        }
    }
    private var controller: KeyboardController? = null

    override fun onCreateInputView(): View {
        settings = SharedSettings(this)

        controller = KeyboardController(
            registry,
            enabledIds = settings.enabledLanguageIds,
            activeId = settings.activeLanguageId,
        )

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }

        // Language strip removed in 2.3.1+51 in favor of Samsung-style
        // swipe-on-space-bar gesture (onSpaceSwipe). Switching feels native
        // and saves vertical real estate above the tone toolbar.

        val tb = ToolbarView(this,
            onToneSelected = { tone -> handleToneSelected(tone) },
            onUndo = { handleUndo() }
        )
        toolbar = tb
        root.addView(tb)

        val kb = QwertyKeyboardView(this, this)
        // Step D: assign languagePack via the setter (triggers second
        // buildKeyboard()). With EN as the only enabled pack, controller.current
        // == EnglishLanguagePack which equals kb.languagePack's default, so
        // this is semantically a no-op — but the setter still fires
        // buildKeyboard() once more. Goal: rule out the setter / second
        // buildKeyboard call as the trigger.
        kb.languagePack = controller!!.current
        keyboard = kb
        root.addView(kb)

        rootLayout = root
        return root
    }

    // --- KeyboardActionListener ---

    private fun refreshKeyboardForActiveLanguage() {
        currentInputConnection?.finishComposingText()
        controller?.let { keyboard?.languagePack = it.current }
    }

    override fun onCharTyped(char: String) {
        val ic = currentInputConnection ?: return
        val c = controller
        if (c == null || char.length != 1) {
            ic.commitText(char, 1)
            return
        }
        when (val outcome = c.onKey(char[0])) {
            is KeystrokeOutcome.Commit -> ic.commitText(outcome.text, 1)
            is KeystrokeOutcome.Composing -> ic.setComposingText(outcome.text, 1)
            KeystrokeOutcome.DeleteOne -> ic.deleteSurroundingText(1, 0)
            KeystrokeOutcome.NoChange -> { /* no-op */ }
        }
    }

    override fun onBackspace() {
        val ic = currentInputConnection ?: return
        val c = controller
        if (c == null) {
            ic.deleteSurroundingText(1, 0)
            return
        }
        when (val outcome = c.onBackspace()) {
            is KeystrokeOutcome.Commit -> ic.commitText(outcome.text, 1)
            is KeystrokeOutcome.Composing -> ic.setComposingText(outcome.text, 1)
            KeystrokeOutcome.DeleteOne -> ic.deleteSurroundingText(1, 0)
            KeystrokeOutcome.NoChange -> {
                // Composer's buffer was just emptied by stripOneLayer. The IC's
                // marked-text composing region still shows the last displayed
                // value — we have to explicitly clear it here, otherwise the
                // user appears to be stuck on the first character.
                ic.setComposingText("", 1)
                ic.finishComposingText()
            }
        }
    }

    override fun onEnter() {
        val ic = currentInputConnection ?: return
        val ei = currentInputEditorInfo
        if (ei != null && ei.imeOptions and EditorInfo.IME_FLAG_NO_ENTER_ACTION == 0) {
            val action = ei.imeOptions and EditorInfo.IME_MASK_ACTION
            if (action != EditorInfo.IME_ACTION_NONE && action != EditorInfo.IME_ACTION_UNSPECIFIED) {
                ic.performEditorAction(action)
                return
            }
        }
        ic.commitText("\n", 1)
    }

    override fun onSpace() {
        val ic = currentInputConnection ?: return
        // Route through the composer so any pending Telex composition gets
        // committed FIRST, then the space appended. Without this, ic.commitText
        // would replace the marked-text region (e.g. "viet") with just " ",
        // looking like the whole word was deleted.
        val c = controller
        if (c == null) {
            ic.commitText(" ", 1)
            return
        }
        when (val outcome = c.onKey(' ')) {
            is KeystrokeOutcome.Commit -> ic.commitText(outcome.text, 1)
            is KeystrokeOutcome.Composing -> ic.setComposingText(outcome.text, 1)
            KeystrokeOutcome.DeleteOne -> ic.deleteSurroundingText(1, 0)
            KeystrokeOutcome.NoChange -> ic.commitText(" ", 1)
        }
    }

    override fun onSwitchKeyboard() {
        val c = controller
        if (c != null && c.enabled.size > 1) {
            c.cycleLanguage()
            refreshKeyboardForActiveLanguage()
        } else {
            val imm = getSystemService(INPUT_METHOD_SERVICE) as android.view.inputmethod.InputMethodManager
            imm.showInputMethodPicker()
        }
    }

    override fun onSpaceSwipe(direction: Int) {
        val c = controller ?: return
        if (c.enabled.size <= 1) return
        c.cycleLanguage(reverse = direction < 0)
        refreshKeyboardForActiveLanguage()
    }

    override fun onSwitchKeyboardLongPress() {
        val imm = getSystemService(INPUT_METHOD_SERVICE) as android.view.inputmethod.InputMethodManager
        imm.showInputMethodPicker()
    }

    // --- Existing tone/rewrite logic ---

    private fun readFullText(): String {
        val ic = currentInputConnection ?: return ""
        val extracted = ic.getExtractedText(ExtractedTextRequest(), 0) ?: return ""
        return extracted.text?.toString() ?: ""
    }

    private fun replaceAllText(newText: String) {
        val ic = currentInputConnection ?: return
        // 1. Commit any in-flight Telex composition (the setMarkedText region)
        //    so it becomes part of the extractable text. Without this, the
        //    composing region survives the selection and the replacement
        //    appears partial.
        ic.finishComposingText()
        controller?.composer?.reset()
        // 2. Read the full extractable text (now including the
        //    just-finalized composition).
        val extracted = ic.getExtractedText(ExtractedTextRequest(), 0) ?: return
        val length = extracted.text?.length ?: 0
        // 3. Replace the entire field.
        ic.setSelection(0, length)
        ic.commitText(newText, 1)
    }

    private fun handleToneSelected(tone: Tone) {
        val text = readFullText().trim()
        if (text.isEmpty()) return

        if (settings.accessToken.isEmpty()) {
            showBanner("Please login in DraftRight app")
            return
        }

        originalText = text
        toolbar?.setLoading(tone)

        aiClient.rewrite(text, tone, settings) { result ->
            mainHandler.post {
                toolbar?.clearLoading()
                result.onSuccess { rewritten ->
                    showDiffSheet(text, rewritten)
                }
                result.onFailure { error ->
                    showBanner(error.message ?: "Rewrite failed")
                }
            }
        }
    }

    private fun showDiffSheet(original: String, rewritten: String) {
        dismissDiffSheet()

        val sheet = DiffSheetView(this, original, rewritten,
            onReplace = { text ->
                replaceAllText(text)
                dismissDiffSheet()
                toolbar?.showUndo()
            },
            onCopy = { text ->
                val clipboard = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
                clipboard.setPrimaryClip(ClipData.newPlainText("DraftRight", text))
                Toast.makeText(this, "Copied", Toast.LENGTH_SHORT).show()
                dismissDiffSheet()
            },
            onCancel = {
                dismissDiffSheet()
            }
        )

        sheet.layoutParams = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT,
            dpToPx(280)
        )
        diffSheet = sheet
        rootLayout?.addView(sheet)
    }

    private fun dismissDiffSheet() {
        diffSheet?.let { rootLayout?.removeView(it) }
        diffSheet = null
    }

    private fun handleUndo() {
        originalText?.let {
            replaceAllText(it)
            toolbar?.hideUndo()
            originalText = null
        }
    }

    private fun showBanner(message: String) {
        val banner = TextView(this).apply {
            text = message
            textSize = 12f
            setTextColor(0xFFFF9800.toInt())
            setBackgroundColor(0x1AFF9800)
            setPadding(dpToPx(16), dpToPx(4), dpToPx(16), dpToPx(4))
        }
        rootLayout?.addView(banner, 1)
        mainHandler.postDelayed({ rootLayout?.removeView(banner) }, 3000)
    }

    private fun dpToPx(dp: Int): Int {
        return (dp * resources.displayMetrics.density).toInt()
    }
}
