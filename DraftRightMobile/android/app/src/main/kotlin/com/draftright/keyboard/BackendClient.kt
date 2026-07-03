package com.draftright.keyboard

import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

class BackendClient {

    private companion object {
        /** Connect + read timeout for the /rewrite call (ms). */
        const val NETWORK_TIMEOUT_MS = 15_000
        /** Backend enforces a max input length; truncate client-side to match. */
        const val MAX_INPUT_CHARS = 3000
        /** Shown when the backend gives no usable user-facing message. */
        const val GENERIC_ERROR = "Rewrite service is temporarily unavailable. Please try again."
        /** Shown on 401 — the keyboard's saved token expired; re-login in the app. */
        const val AUTH_ERROR = "Session expired — open DraftRight and log in again."

        /**
         * Pull the backend's user-facing `error` field out of an error
         * response body. Returns null when the body is missing, isn't JSON,
         * or has no non-blank `error` — callers fall back to [GENERIC_ERROR].
         * Never returns the raw body, which can carry provider internals.
         */
        fun parseErrorMessage(body: String?): String? {
            if (body.isNullOrBlank()) return null
            return try {
                JSONObject(body).optString("error").takeIf { it.isNotBlank() }
            } catch (_: Exception) {
                null
            }
        }
    }

    fun rewrite(
        text: String,
        tone: Tone,
        settings: SharedSettings,
        inputKind: InputKind = InputKind.TYPED,
        onResult: (Result<String>) -> Unit
    ) {
        thread {
            try {
                // Prefer the long-lived dr_ext_* token; fall back to the
                // access JWT for users on a build older than the one that
                // mints it.
                val bearerToken = settings.bearerToken
                if (bearerToken.isEmpty()) {
                    onResult(Result.failure(Exception("Please login in DraftRight app")))
                    return@thread
                }

                val inputText = if (text.length > MAX_INPUT_CHARS) text.substring(0, MAX_INPUT_CHARS) else text

                val body = JSONObject().apply {
                    put("text", inputText)
                    put("tone", tone.apiValue)
                    if (tone == Tone.TRANSLATE) {
                        put("target_language", settings.translateLanguage)
                    }
                    if (inputKind != InputKind.TYPED) put("input_kind", inputKind.apiValue)
                }

                val endpoint = settings.backendUrl.trimEnd('/') + "/rewrite"
                val url = URL(endpoint)
                val conn = url.openConnection() as HttpURLConnection
                try {
                    conn.requestMethod = "POST"
                    conn.setRequestProperty("Content-Type", "application/json")
                    conn.setRequestProperty("Authorization", "Bearer $bearerToken")
                    conn.connectTimeout = NETWORK_TIMEOUT_MS
                    conn.readTimeout = NETWORK_TIMEOUT_MS
                    conn.doOutput = true
                    conn.outputStream.use { it.write(body.toString().toByteArray()) }

                    val responseCode = conn.responseCode
                    if (responseCode >= 400) {
                        val errorBody = conn.errorStream?.bufferedReader()?.use { it.readText() }
                        // 401 = the keyboard's token is expired/invalid. The
                        // keyboard can't show a login screen, so point the user
                        // to the app instead of a bare "invalid token".
                        val message = if (responseCode == 401) AUTH_ERROR
                            // Surface only the backend's user-facing `error` field.
                            // Never render the raw body — it can carry provider
                            // internals (API key prefixes, upstream JSON, request ids).
                            else parseErrorMessage(errorBody) ?: GENERIC_ERROR
                        onResult(Result.failure(Exception(message)))
                        return@thread
                    }

                    val responseBody = conn.inputStream.bufferedReader().use { it.readText() }
                    val json = JSONObject(responseBody)

                    // Grammar check returns {"grammar": {score, issues[]}} instead of rewritten_text
                    val grammar = json.optJSONObject("grammar")
                    if (tone == Tone.GRAMMAR_CHECK && grammar != null) {
                        val score = grammar.optInt("score", 0)
                        val issues = grammar.optJSONArray("issues")
                        val sb = StringBuilder()
                        sb.append("Score: $score/100")
                        if (issues != null && issues.length() > 0) {
                            sb.append("\n\nIssues:")
                            for (i in 0 until issues.length()) {
                                val issue = issues.getJSONObject(i)
                                val original = issue.optString("original", "")
                                val suggestion = issue.optString("suggestion", "")
                                val reason = issue.optString("reason", "")
                                sb.append("\n• \"$original\" → \"$suggestion\"")
                                if (reason.isNotBlank()) sb.append(" ($reason)")
                            }
                        } else {
                            sb.append("\n\nNo issues found. Your text looks great!")
                        }
                        onResult(Result.success(sb.toString()))
                    } else {
                        val rewrittenText = json.getString("rewritten_text").trim()
                        onResult(Result.success(rewrittenText))
                    }
                } finally {
                    conn.disconnect()
                }
            } catch (e: Exception) {
                onResult(Result.failure(e))
            }
        }
    }
}
