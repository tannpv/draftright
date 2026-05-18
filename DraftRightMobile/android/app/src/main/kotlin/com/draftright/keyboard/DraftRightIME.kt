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
    private lateinit var controller: KeyboardController
    private val aiClient = BackendClient()
    private val mainHandler = Handler(Looper.getMainLooper())
    private var toolbar: ToolbarView? = null
    private var keyboard: QwertyKeyboardView? = null
    private var languageStrip: LanguageStripView? = null
    private var diffSheet: DiffSheetView? = null
    private var rootLayout: LinearLayout? = null
    private var originalText: String? = null

    private val registry: LanguageRegistry by lazy {
        try {
            LanguageRegistry.PRODUCTION
        } catch (t: Throwable) {
            // Defensive: if any LanguagePack constructor throws during the
            // PRODUCTION lazy load, fall back to an English-only registry so
            // the IME still functions. Without this, the whole IME process
            // crashes and Android shows blank for every keyboard until the
            // user uninstalls DraftRight Keyboard (the bug that broke
            // 2.3.0+40).
            android.util.Log.e(
                "DraftRightIME",
                "LanguageRegistry.PRODUCTION init failed — falling back to EN-only",
                t,
            )
            LanguageRegistry(listOf(EnglishLanguagePack))
        }
    }

    override fun onCreate() {
        super.onCreate()
        // Initialize settings + controller HERE, not in onCreateInputView.
        // Android can fire onStartInput() BEFORE onCreateInputView() in some
        // app contexts — accessing the lateinit `controller` there crashed
        // every keyboard on the device in 2.3.0+40. Doing it in onCreate
        // makes both available the moment the IME service is bound.
        try {
            settings = SharedSettings(this)
            controller = KeyboardController(
                registry,
                enabledIds = settings.enabledLanguageIds,
                activeId = settings.activeLanguageId,
            )
        } catch (t: Throwable) {
            android.util.Log.e("DraftRightIME", "onCreate failed", t)
            throw t
        }
    }

    override fun onCreateInputView(): View {
        return try {
            buildRootInputView()
        } catch (t: Throwable) {
            android.util.Log.e(
                "DraftRightIME",
                "onCreateInputView crashed — falling back to minimal EN view",
                t,
            )
            // Last-resort fallback: build a minimal EN-only keyboard so the
            // user gets SOMETHING typeable instead of a blank IME the OS
            // can't recover from.
            val fallback = QwertyKeyboardView(this, this)
            fallback.languagePack = EnglishLanguagePack
            keyboard = fallback
            rootLayout = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                addView(fallback)
            }
            rootLayout!!
        }
    }

    private fun buildRootInputView(): View {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }

        val strip = LanguageStripView(this, controller) { refreshKeyboardForActiveLanguage() }
        languageStrip = strip
        root.addView(strip)

        val tb = ToolbarView(this,
            onToneSelected = { tone -> handleToneSelected(tone) },
            onUndo = { handleUndo() }
        )
        toolbar = tb
        root.addView(tb)

        val kb = QwertyKeyboardView(this, this)
        kb.languagePack = controller.current
        keyboard = kb
        root.addView(kb)

        rootLayout = root
        return root
    }

    private fun refreshKeyboardForActiveLanguage() {
        currentInputConnection?.finishComposingText()
        if (::controller.isInitialized) {
            keyboard?.languagePack = controller.current
            languageStrip?.refresh()
        }
    }

    override fun onStartInput(attribute: EditorInfo?, restarting: Boolean) {
        super.onStartInput(attribute, restarting)
        // Defensive: onCreate should have run before onStartInput, but Android
        // has been observed firing onStartInput before lateinit initialization
        // on some app launches — bug that crashed 2.3.0+40.
        if (::controller.isInitialized) controller.composer?.reset()
    }

    // --- KeyboardActionListener ---

    override fun onCharTyped(char: String) {
        val ic = currentInputConnection ?: return
        // If the controller didn't initialize (fallback EN-only path), commit
        // the raw char directly so typing still works.
        if (!::controller.isInitialized) {
            ic.commitText(char, 1)
            return
        }
        if (char.length == 1) {
            when (val outcome = controller.onKey(char[0])) {
                is KeystrokeOutcome.Commit -> ic.commitText(outcome.text, 1)
                is KeystrokeOutcome.Composing -> ic.setComposingText(outcome.text, 1)
                KeystrokeOutcome.DeleteOne -> ic.deleteSurroundingText(1, 0)
                KeystrokeOutcome.NoChange -> { /* no-op */ }
            }
        } else {
            ic.commitText(char, 1)
        }
    }

    override fun onBackspace() {
        val ic = currentInputConnection ?: return
        if (!::controller.isInitialized) {
            ic.deleteSurroundingText(1, 0)
            return
        }
        when (val outcome = controller.onBackspace()) {
            is KeystrokeOutcome.Commit -> ic.commitText(outcome.text, 1)
            is KeystrokeOutcome.Composing -> ic.setComposingText(outcome.text, 1)
            KeystrokeOutcome.DeleteOne -> ic.deleteSurroundingText(1, 0)
            KeystrokeOutcome.NoChange -> { /* no-op */ }
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
        currentInputConnection?.commitText(" ", 1)
    }

    override fun onSwitchKeyboard() {
        if (::controller.isInitialized && controller.enabled.size > 1) {
            controller.cycleLanguage()
            refreshKeyboardForActiveLanguage()
        } else {
            val imm = getSystemService(INPUT_METHOD_SERVICE) as android.view.inputmethod.InputMethodManager
            imm.showInputMethodPicker()
        }
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
        val extracted = ic.getExtractedText(ExtractedTextRequest(), 0) ?: return
        val length = extracted.text?.length ?: 0
        ic.setSelection(0, length)
        ic.commitText(newText, 1)
    }

    private fun handleToneSelected(tone: Tone) {
        val text = readFullText().trim()
        if (text.isEmpty()) return

        if (!::settings.isInitialized || settings.accessToken.isEmpty()) {
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
