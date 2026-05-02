package com.draftright.keyboard

import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

class BackendClient {

    fun rewrite(
        text: String,
        tone: Tone,
        settings: SharedSettings,
        onResult: (Result<String>) -> Unit
    ) {
        thread {
            try {
                val accessToken = settings.accessToken
                if (accessToken.isEmpty()) {
                    onResult(Result.failure(Exception("Please login in DraftRight app")))
                    return@thread
                }

                val inputText = if (text.length > 3000) text.substring(0, 3000) else text

                val body = JSONObject().apply {
                    put("text", inputText)
                    put("tone", tone.apiValue)
                    if (tone == Tone.TRANSLATE) {
                        put("target_language", settings.translateLanguage)
                    }
                }

                val endpoint = settings.backendUrl.trimEnd('/') + "/rewrite"
                val url = URL(endpoint)
                val conn = url.openConnection() as HttpURLConnection
                conn.requestMethod = "POST"
                conn.setRequestProperty("Content-Type", "application/json")
                conn.setRequestProperty("Authorization", "Bearer $accessToken")
                conn.connectTimeout = 15000
                conn.readTimeout = 15000
                conn.doOutput = true
                conn.outputStream.use { it.write(body.toString().toByteArray()) }

                val responseCode = conn.responseCode
                if (responseCode >= 400) {
                    val errorBody = conn.errorStream?.bufferedReader()?.readText() ?: "Unknown error"
                    onResult(Result.failure(Exception("HTTP $responseCode: $errorBody")))
                    return@thread
                }

                val responseBody = BufferedReader(InputStreamReader(conn.inputStream)).readText()
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
            } catch (e: Exception) {
                onResult(Result.failure(e))
            }
        }
    }
}
