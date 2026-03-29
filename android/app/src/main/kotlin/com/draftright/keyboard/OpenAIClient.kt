package com.draftright.keyboard

import org.json.JSONArray
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import kotlin.concurrent.thread

class OpenAIClient {

    fun rewrite(
        text: String,
        tone: Tone,
        settings: SharedSettings,
        onResult: (Result<String>) -> Unit
    ) {
        thread {
            try {
                val inputText = if (text.length > 3000) text.substring(0, 3000) else text
                val messages = JSONArray().apply {
                    put(JSONObject().put("role", "system").put("content", tone.systemPrompt(settings.translateLanguage)))
                    put(JSONObject().put("role", "user").put("content", inputText))
                }

                val body = JSONObject().apply {
                    put("model", settings.model)
                    put("messages", messages)
                    put("temperature", settings.temperature)
                    put("max_tokens", 1024)
                }

                val url = URL(settings.endpoint)
                val conn = url.openConnection() as HttpURLConnection
                conn.requestMethod = "POST"
                conn.setRequestProperty("Content-Type", "application/json")
                conn.connectTimeout = 15000
                conn.readTimeout = 15000

                val apiKey = settings.apiKey
                if (apiKey.isNotEmpty()) {
                    conn.setRequestProperty("Authorization", "Bearer $apiKey")
                }

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
                val choices = json.getJSONArray("choices")
                if (choices.length() == 0) {
                    onResult(Result.failure(Exception("No response from AI")))
                    return@thread
                }

                val content = choices.getJSONObject(0).getJSONObject("message").getString("content").trim()
                onResult(Result.success(content))
            } catch (e: Exception) {
                onResult(Result.failure(e))
            }
        }
    }
}
